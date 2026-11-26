package main

import (
	_ "embed"
	"fmt"
	"io"
)

//go:embed demo/workflow.yaml
var demoWorkflowYAML []byte

// demoMaxStageVisits bounds the simulated traversal so a malformed graph
// cannot hang the command.
const demoMaxStageVisits = 32

// runDemo simulates the embedded demo workflow end-to-end with stub agents.
// It requires no API keys, no Docker and no git repository: stages are not
// executed, they only walk the graph and report progress.
//
// TODO(simplify-orchestrator): once pkg/engine exposes Executor/GateRunner,
// wire this YAML into the real engine and drop the local traversal. The demo
// contract (exit 0, no external dependencies) must stay the same.
func runDemo(out io.Writer) error {
	spec, err := parseWorkflow(demoWorkflowYAML)
	if err != nil {
		return fmt.Errorf("demo workflow: %w", err)
	}
	name := spec.Name
	if name == "" {
		name = "demo"
	}
	// Progress lines are best-effort: errcheck wants the discarded Fprintf
	// results marked explicitly, and a demo must not fail on a write error.
	_, _ = fmt.Fprintf(out, "▸ demo workflow %q (embedded; stub agents, no API keys, Docker or git)\n", name)

	// Demonstrate the on_fail loopback once on the first stage that loops back
	// to a different stage. The run still terminates because the failure is
	// remembered per stage.
	loopStage := ""
	for _, stage := range spec.Stages {
		if stage.Transitions.OnFail != "" && stage.Transitions.OnFail != stage.ID {
			loopStage = stage.ID
			break
		}
	}
	failedOnce := make(map[string]bool)
	current := spec.Stages[0].ID
	visits := 0
	for current != workflowDone {
		visits++
		if visits > demoMaxStageVisits {
			return fmt.Errorf("demo workflow: exceeded %d stage visits (cycle without terminal stage?)", demoMaxStageVisits)
		}
		position := spec.stageIndex(current)
		if position < 0 {
			return fmt.Errorf("demo workflow: unknown stage %q", current)
		}
		stage := spec.Stages[position]
		_, _ = fmt.Fprintf(out, "▶ stage %s (%s)\n", stage.ID, demoStageLabel(stage))

		passed := true
		if stage.ID == loopStage && !failedOnce[stage.ID] {
			failedOnce[stage.ID] = true
			passed = false
		}
		if passed {
			_, _ = fmt.Fprintf(out, "  ✓ %s passed\n", stage.ID)
			target := stage.Transitions.OnPass
			if target == "" || target == workflowDone {
				_, _ = fmt.Fprintf(out, "✓ demo completed: %d stage visits\n", visits)
				return nil
			}
			_, _ = fmt.Fprintf(out, "  → %s\n", target)
			current = target
			continue
		}
		target := stage.Transitions.OnFail
		if target == "" || target == workflowDone {
			return fmt.Errorf("demo workflow: stage %q failed with no on_fail transition", stage.ID)
		}
		_, _ = fmt.Fprintf(out, "  ↺ %s failed → loopback to %s\n", stage.ID, target)
		current = target
	}
	_, _ = fmt.Fprintf(out, "✓ demo completed: %d stage visits\n", visits)
	return nil
}

func demoStageLabel(stage workflowStage) string {
	switch {
	case stage.Agent != "":
		return "stub agent: " + stage.Agent
	case stage.Title != "":
		return stage.Title
	default:
		return "stage"
	}
}
