package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/arturpanteleev/workaistrator/pkg/safeio"
	"gopkg.in/yaml.v3"
)

// onboarding.go implements the startup file generation for `workaistrator
// init`: a small, valid workflow graph and a commented environment example.
//
// The workflow schema here is intentionally minimal (stages + per-stage
// on_pass/on_fail transitions) and is shared with the embedded demo. The full
// control-plane config in pkg/config is broader; this file is the documented
// entry point for a first run.
//
// TODO(simplify-orchestrator): once pkg/engine exposes the unified graph
// loader, parse this file through it instead of keeping a second minimal
// schema in the CLI.

const initWorkflowFile = "workflow.yaml"
const initEnvFile = ".env.example"

// workflowFile is the canonical workflow model documented in
// docs/start/workflow-reference.md: an ordered list of stages with executors,
// results, gates and transitions. The first stage is the entry point; the
// terminal state is the reserved id "done".
type workflowFile struct {
	Name   string          `yaml:"name,omitempty"`
	Title  string          `yaml:"title,omitempty"`
	Stages []workflowStage `yaml:"stages"`
}

type workflowStage struct {
	ID          string              `yaml:"id"`
	Title       string              `yaml:"title,omitempty"`
	Executor    string              `yaml:"executor"`
	Agent       string              `yaml:"agent,omitempty"`
	Result      string              `yaml:"result"`
	Gates       []workflowGate      `yaml:"gates,omitempty"`
	Transitions workflowTransitions `yaml:"transitions,omitempty"`
	MaxVisits   int                 `yaml:"max_visits,omitempty"`
}

type workflowGate struct {
	Type   string         `yaml:"type"`
	Name   string         `yaml:"name,omitempty"`
	Mode   string         `yaml:"mode,omitempty"`
	Params map[string]any `yaml:"params,omitempty"`
}

type workflowTransitions struct {
	OnPass string `yaml:"on_pass,omitempty"`
	OnFail string `yaml:"on_fail,omitempty"`
}

// workflowDone is the reserved terminal stage id from the workflow reference.
const workflowDone = "done"

// workflowGateTypes is the documented built-in gate catalogue.
var workflowGateTypes = map[string]bool{
	"program": true, "llm": true, "human": true, "required_phrases": true,
}

// workflowResultTypes is the documented result contract.
var workflowResultTypes = map[string]bool{"md": true, "link": true, "approve": true}

// parseWorkflow decodes and validates a workflow YAML document. It rejects
// unknown fields so a misspelled key fails loudly instead of silently
// producing a graph with a missing transition.
func parseWorkflow(data []byte) (*workflowFile, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var spec workflowFile
	if err := decoder.Decode(&spec); err != nil {
		return nil, fmt.Errorf("decode workflow yaml: %w", err)
	}
	if err := spec.validate(); err != nil {
		return nil, err
	}
	return &spec, nil
}

func (spec *workflowFile) validate() error {
	if len(spec.Stages) == 0 {
		return errors.New("workflow has no stages")
	}
	spec.Name = strings.TrimSpace(spec.Name)
	spec.Title = strings.TrimSpace(spec.Title)

	index := make(map[string]int, len(spec.Stages))
	for position := range spec.Stages {
		stage := &spec.Stages[position]
		stage.ID = strings.TrimSpace(stage.ID)
		stage.Executor = strings.TrimSpace(stage.Executor)
		stage.Agent = strings.TrimSpace(stage.Agent)
		stage.Result = strings.TrimSpace(stage.Result)
		stage.Transitions.OnPass = strings.TrimSpace(stage.Transitions.OnPass)
		stage.Transitions.OnFail = strings.TrimSpace(stage.Transitions.OnFail)

		if stage.ID == "" {
			return fmt.Errorf("stage %d has no id", position+1)
		}
		if stage.ID == workflowDone {
			return fmt.Errorf("stage id %q is reserved for the terminal state", workflowDone)
		}
		if _, exists := index[stage.ID]; exists {
			return fmt.Errorf("duplicate stage id %q", stage.ID)
		}
		index[stage.ID] = position

		if stage.Executor == "" {
			return fmt.Errorf("stages[%d] (%s): executor is required", position, stage.ID)
		}
		if stage.Result == "" {
			return fmt.Errorf("stages[%d] (%s): result is required", position, stage.ID)
		}
		if !workflowResultTypes[stage.Result] {
			return fmt.Errorf("stages[%d] (%s): unknown result %q (want md, link or approve)", position, stage.ID, stage.Result)
		}
		if stage.Executor == "agent" && stage.Agent == "" {
			return fmt.Errorf("stages[%d] (%s): executor agent needs an agent", position, stage.ID)
		}
		if stage.MaxVisits < 0 {
			return fmt.Errorf("stages[%d] (%s): max_visits must be positive", position, stage.ID)
		}
		for gatePosition := range stage.Gates {
			gate := &stage.Gates[gatePosition]
			gate.Type = strings.TrimSpace(gate.Type)
			gate.Mode = strings.TrimSpace(gate.Mode)
			if !workflowGateTypes[gate.Type] {
				return fmt.Errorf("stages[%d] (%s): gates[%d]: unknown type %q", position, stage.ID, gatePosition, gate.Type)
			}
			if gate.Mode != "" && gate.Mode != "required" && gate.Mode != "advisory" {
				return fmt.Errorf("stages[%d] (%s): gates[%d]: unknown mode %q", position, stage.ID, gatePosition, gate.Mode)
			}
			if len(gate.Params) == 0 {
				return fmt.Errorf("stages[%d] (%s): gates[%d]: params are required", position, stage.ID, gatePosition)
			}
		}
	}

	// on_pass defaults to the next stage in order; the last stage ends the run.
	for position := range spec.Stages {
		stage := &spec.Stages[position]
		if stage.Transitions.OnPass != "" {
			continue
		}
		if position+1 < len(spec.Stages) {
			stage.Transitions.OnPass = spec.Stages[position+1].ID
		} else {
			stage.Transitions.OnPass = workflowDone
		}
	}

	hasTerminal := false
	for position := range spec.Stages {
		stage := &spec.Stages[position]
		if stage.Transitions.OnPass == workflowDone {
			hasTerminal = true
		}
		for name, target := range map[string]string{
			"on_pass": stage.Transitions.OnPass,
			"on_fail": stage.Transitions.OnFail,
		} {
			if target == "" || target == workflowDone {
				continue
			}
			if _, ok := index[target]; !ok {
				return fmt.Errorf("stages[%d] (%s): %s target %q does not exist", position, stage.ID, name, target)
			}
		}
	}
	if !hasTerminal {
		return fmt.Errorf("workflow has no path to %q", workflowDone)
	}

	// Every stage must be reachable from the entry (the first stage).
	reachable := map[string]bool{spec.Stages[0].ID: true}
	queue := []string{spec.Stages[0].ID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		stage := &spec.Stages[index[id]]
		for _, target := range []string{stage.Transitions.OnPass, stage.Transitions.OnFail} {
			if target == "" || target == workflowDone || reachable[target] {
				continue
			}
			reachable[target] = true
			queue = append(queue, target)
		}
	}
	for position := range spec.Stages {
		if !reachable[spec.Stages[position].ID] {
			return fmt.Errorf("stages[%d] (%s): unreachable from the entry stage", position, spec.Stages[position].ID)
		}
	}

	if err := spec.validateCycles(index); err != nil {
		return err
	}
	return nil
}

// validateCycles rejects a loop that no stage in the loop bounds with
// max_visits. Both on_pass and on_fail edges can form loops.
func (spec *workflowFile) validateCycles(index map[string]int) error {
	const (
		white = iota
		gray
		black
	)
	state := make(map[string]int, len(spec.Stages))
	stack := make([]string, 0, len(spec.Stages))

	var visit func(id string) error
	visit = func(id string) error {
		state[id] = gray
		stack = append(stack, id)
		stage := &spec.Stages[index[id]]
		for _, target := range []string{stage.Transitions.OnPass, stage.Transitions.OnFail} {
			if target == "" || target == workflowDone {
				continue
			}
			switch state[target] {
			case gray:
				for position, node := range stack {
					if node != target {
						continue
					}
					cycle := stack[position:]
					bounded := false
					for _, cycleID := range cycle {
						if spec.Stages[index[cycleID]].MaxVisits > 0 {
							bounded = true
							break
						}
					}
					if !bounded {
						return fmt.Errorf("workflow cycle %s has no max_visits", strings.Join(cycle, " -> "))
					}
					break
				}
			case white:
				if err := visit(target); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = black
		return nil
	}

	for position := range spec.Stages {
		id := spec.Stages[position].ID
		if state[id] == white {
			if err := visit(id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (spec *workflowFile) stageIndex(id string) int {
	for position := range spec.Stages {
		if spec.Stages[position].ID == id {
			return position
		}
	}
	return -1
}

// initWorkflowYAML is the example written by `workaistrator init`. It uses the
// canonical format from docs/start/workflow-reference.md.
const initWorkflowYAML = `# WorkAiStrator workflow — one YAML file describes the whole graph.
#
# The first stage is the entry point. Every stage declares:
#   executor — human, agent or the name of an external adapter
#   result   — md, link or approve
#   gates    — optional checks of the stage result
#   transitions.on_pass — next stage, or done to end the run
#   transitions.on_fail — where to loop back when a gate fails
#   max_visits          — bounds a loopback
#
# This example plans, implements and reviews a change, looping back to
# implementation when review fails.
name: example
title: Plan, implement and review a change

stages:
  - id: plan
    title: Plan the change
    executor: agent
    agent: analyst
    result: md
    max_visits: 2
    gates:
      - type: required_phrases
        mode: advisory
        params:
          phrases: ["Критерии приёмки"]
    transitions:
      on_pass: implement
      on_fail: plan        # re-plan when planning checks fail

  - id: implement
    title: Implement the change
    executor: agent
    agent: coder
    result: md
    max_visits: 3
    transitions:
      on_pass: review
      on_fail: implement   # fix and retry in place

  - id: review
    title: Review the result
    executor: agent
    agent: reviewer
    result: md
    transitions:
      on_pass: done
      on_fail: implement   # loopback to implementation
`

// initEnvExample is the commented environment example written by
// `workaistrator init`. Values stay empty on purpose: `init` never writes
// secrets.
const initEnvExample = `# WorkAiStrator environment example.
#
# Copy to .env and export the variables in your shell or CI, or configure the
# same names in your secret manager. The demo workflow runs without any of
# them: ` + "`workaistrator run --demo`" + ` needs no API keys.

# Model provider credentials (at least one is needed for real agent runs).
ANTHROPIC_API_KEY=
OPENAI_API_KEY=
CODEX_API_KEY=

# Mandatory allowlist of environment variable names that are passed through to
# the agent runtime. Provider keys stay invisible to the agent until their name
# is listed here (comma-separated), so a run fails on authorization without it.
AI_TEAM_HARNESS_ENV_ALLOW=ANTHROPIC_API_KEY

# Optional: extra agent registry directories, separated by the OS path list
# separator (":" on macOS/Linux, ";" on Windows).
AI_TEAM_AGENT_PATH=

# Optional: signing secret for cloud auth tokens (` + "`workaistrator auth-token`" + `).
AI_TEAM_AUTH_SECRET=
`

// initFiles lists the startup files written by init in write order.
func initFiles() []struct {
	name string
	data []byte
} {
	return []struct {
		name string
		data []byte
	}{
		{name: initWorkflowFile, data: []byte(initWorkflowYAML)},
		{name: initEnvFile, data: []byte(initEnvExample)},
	}
}

// writeInitFiles creates workflow.yaml and .env.example in dir. Existing
// files are never overwritten unless force is true; with force they are
// atomically replaced. Symlinked targets are always rejected. Init stays
// idempotent: a repeat run reports the files it skipped instead of failing.
func writeInitFiles(dir string, force bool) (written, skipped []string, err error) {
	for _, file := range initFiles() {
		path := filepath.Join(dir, file.name)
		if _, statErr := os.Lstat(path); statErr == nil {
			if !force {
				skipped = append(skipped, file.name)
				continue
			}
			err = safeio.ReplaceRegularFileNoFollow(path, file.data, 0644)
		} else if errors.Is(statErr, os.ErrNotExist) {
			err = safeio.WriteRegularFileNoFollow(path, file.data, 0644)
		} else {
			return written, skipped, statErr
		}
		if err != nil {
			return written, skipped, err
		}
		written = append(written, file.name)
	}
	return written, skipped, nil
}
