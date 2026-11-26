package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitWorkflowFilesCreated(t *testing.T) {
	dir := t.TempDir()
	written, skipped, err := writeInitFiles(dir, false)
	if err != nil {
		t.Fatalf("writeInitFiles: %v", err)
	}
	if len(skipped) != 0 || len(written) != 2 {
		t.Fatalf("want both files created, written=%v skipped=%v", written, skipped)
	}

	workflowData, err := os.ReadFile(filepath.Join(dir, initWorkflowFile))
	if err != nil {
		t.Fatalf("read %s: %v", initWorkflowFile, err)
	}
	spec, err := parseWorkflow(workflowData)
	if err != nil {
		t.Fatalf("generated workflow does not validate: %v\n%s", err, workflowData)
	}
	if len(spec.Stages) != 3 {
		t.Fatalf("want 3 example stages, got %d", len(spec.Stages))
	}
	if spec.Stages[0].ID != "plan" {
		t.Fatalf("want entry plan, got %q", spec.Stages[0].ID)
	}
	terminal := false
	loopback := false
	for _, stage := range spec.Stages {
		if stage.Executor == "" || stage.Result == "" {
			t.Fatalf("generated stage %q must declare executor and result: %+v", stage.ID, stage)
		}
		if stage.Transitions.OnPass == workflowDone {
			terminal = true
		}
		if stage.Transitions.OnFail != "" && spec.stageIndex(stage.Transitions.OnFail) < spec.stageIndex(stage.ID) {
			loopback = true
		}
	}
	if !terminal {
		t.Fatal("example workflow must use the documented on_pass: done terminal state")
	}
	if !loopback {
		t.Fatal("example workflow must demonstrate an on_fail loopback to an earlier stage")
	}
	if !strings.Contains(string(workflowData), "max_visits:") {
		t.Fatal("example workflow must bound its loopback with max_visits")
	}
	if !strings.Contains(string(workflowData), "gates:") {
		t.Fatal("example workflow must demonstrate the documented gates field")
	}

	envData, err := os.ReadFile(filepath.Join(dir, initEnvFile))
	if err != nil {
		t.Fatalf("read %s: %v", initEnvFile, err)
	}
	env := string(envData)
	if !strings.Contains(env, "ANTHROPIC_API_KEY=") || !strings.Contains(env, "OPENAI_API_KEY=") {
		t.Fatalf("env example must document provider keys, got:\n%s", env)
	}
	if !strings.Contains(env, "AI_TEAM_HARNESS_ENV_ALLOW=") {
		t.Fatalf("env example must document the runtime allowlist, got:\n%s", env)
	}
	if !strings.Contains(env, "#") {
		t.Fatal("env example must be commented")
	}
}

func TestInitWorkflowFilesDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	sentinel := []byte("sentinel: true\n")
	workflowPath := filepath.Join(dir, initWorkflowFile)
	if err := os.WriteFile(workflowPath, sentinel, 0644); err != nil {
		t.Fatal(err)
	}

	_, skipped, err := writeInitFiles(dir, false)
	if err != nil {
		t.Fatalf("idempotent init must not fail: %v", err)
	}
	if len(skipped) != 1 || skipped[0] != initWorkflowFile {
		t.Fatalf("existing %s must be reported as skipped, got %v", initWorkflowFile, skipped)
	}
	after, readErr := os.ReadFile(workflowPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(after) != string(sentinel) {
		t.Fatalf("existing workflow.yaml was modified: %q", after)
	}
	// The missing sibling is still created.
	if _, statErr := os.Stat(filepath.Join(dir, initEnvFile)); statErr != nil {
		t.Fatalf("%s should be created when missing: %v", initEnvFile, statErr)
	}
}

func TestInitWorkflowFilesForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{initWorkflowFile, initEnvFile} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("old\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	written, skipped, err := writeInitFiles(dir, true)
	if err != nil {
		t.Fatalf("writeInitFiles --force: %v", err)
	}
	if len(written) != 2 || len(skipped) != 0 {
		t.Fatalf("--force must rewrite both files, written=%v skipped=%v", written, skipped)
	}
	workflowData, err := os.ReadFile(filepath.Join(dir, initWorkflowFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(workflowData) == "old\n" {
		t.Fatal("--force did not replace workflow.yaml")
	}
	if _, err := parseWorkflow(workflowData); err != nil {
		t.Fatalf("forced workflow does not validate: %v", err)
	}
	envData, err := os.ReadFile(filepath.Join(dir, initEnvFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(envData) == "old\n" {
		t.Fatal("--force did not replace .env.example")
	}
}

func TestParseWorkflowRejectsInvalidGraphs(t *testing.T) {
	cases := map[string]string{
		"unknown field": `name: bad
stages: [{id: a, executor: human, result: approve, transtions: {on_pass: done}}]
`,
		"missing executor": `stages: [{id: a, result: approve}]
`,
		"missing result": `stages: [{id: a, executor: human}]
`,
		"unknown result": `stages: [{id: a, executor: human, result: text}]
`,
		"agent executor without agent": `stages: [{id: a, executor: agent, result: md}]
`,
		"unknown on_pass target": `stages: [{id: a, executor: human, result: approve, transitions: {on_pass: missing}}]
`,
		"unknown on_fail target": `stages: [{id: a, executor: human, result: approve, transitions: {on_fail: missing}}]
`,
		"duplicate id": `stages: [{id: a, executor: human, result: approve}, {id: a, executor: human, result: approve}]
`,
		"unreachable stage": `stages:
  - {id: a, executor: human, result: approve, transitions: {on_pass: done}}
  - {id: b, executor: human, result: approve, transitions: {on_pass: done}}
`,
		"cycle without max_visits": `stages:
  - {id: a, executor: human, result: approve, transitions: {on_pass: b}}
  - {id: b, executor: human, result: approve, transitions: {on_pass: a}}
`,
		"no terminal stage": `stages:
  - {id: a, executor: human, result: approve, max_visits: 2, transitions: {on_pass: b}}
  - {id: b, executor: human, result: approve, max_visits: 2, transitions: {on_pass: a}}
`,
		"unknown gate type": `stages: [{id: a, executor: human, result: approve, gates: [{type: mystery, params: {x: 1}}], transitions: {on_pass: done}}]
`,
	}
	for name, document := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWorkflow([]byte(document)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCanonicalWorkflowReferenceExampleValid(t *testing.T) {
	document := `name: feature-flow
title: Фича от задачи до ревью

stages:
  - id: intent
    title: Задача
    executor: human
    result: approve
    transitions: { on_pass: implementation }

  - id: implementation
    title: Реализация
    executor: agent
    agent: coder
    result: md
    transitions: { on_pass: done }
`
	spec, err := parseWorkflow([]byte(document))
	if err != nil {
		t.Fatalf("documented workflow example must validate: %v", err)
	}
	if spec.Stages[0].Transitions.OnPass != "implementation" || spec.Stages[1].Transitions.OnPass != workflowDone {
		t.Fatalf("unexpected transitions: %+v", spec.Stages)
	}
}

func TestRunWorkflowCLIHelper(t *testing.T) {
	if os.Getenv("AI_TEAM_WORKFLOW_HELPER") != "1" {
		return
	}
	os.Args = []string{"workaistrator", "run", "--workflow", os.Getenv("AI_TEAM_WORKFLOW_PATH")}
	cmdRun()
}

// TestRunWorkflowValidatesInitOutput runs the documented onboarding command
// `run --workflow workflow.yaml` against the file produced by init.
func TestRunWorkflowValidatesInitOutput(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := writeInitFiles(dir, false); err != nil {
		t.Fatalf("writeInitFiles: %v", err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestRunWorkflowCLIHelper$")
	command.Env = append(os.Environ(),
		"AI_TEAM_WORKFLOW_HELPER=1",
		"AI_TEAM_WORKFLOW_PATH="+filepath.Join(dir, initWorkflowFile),
	)
	var stdout, stderr strings.Builder
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("run --workflow exited non-zero: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String()+stderr.String(), "граф валиден") {
		t.Fatalf("run --workflow did not report validation:\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
}
