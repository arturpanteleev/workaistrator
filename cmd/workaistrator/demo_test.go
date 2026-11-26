package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestEmbeddedDemoWorkflowValid(t *testing.T) {
	spec, err := parseWorkflow(demoWorkflowYAML)
	if err != nil {
		t.Fatalf("embedded demo workflow does not validate: %v", err)
	}
	if spec.Name != "demo" {
		t.Fatalf("want demo workflow name, got %q", spec.Name)
	}
}

func TestRunDemoSucceeds(t *testing.T) {
	var out bytes.Buffer
	if err := runDemo(&out); err != nil {
		t.Fatalf("runDemo: %v", err)
	}
	output := out.String()
	for _, want := range []string{"demo workflow", "loopback", "demo completed"} {
		if !strings.Contains(output, want) {
			t.Fatalf("demo output missing %q:\n%s", want, output)
		}
	}
	if !strings.Contains(output, "stub agent: stub-planner") {
		t.Fatalf("demo should report stub agents:\n%s", output)
	}
}

func TestRunDemoCLIHelper(t *testing.T) {
	if os.Getenv("AI_TEAM_DEMO_HELPER") != "1" {
		return
	}
	os.Args = []string{"workaistrator", "run", "--demo"}
	cmdRun()
}

// TestRunDemoCLIExitsZero exercises the real `run --demo` dispatch in a child
// process, so os.Exit(1) from fatal() fails the test instead of passing a
// direct function call.
func TestRunDemoCLIExitsZero(t *testing.T) {
	command := exec.Command(os.Args[0], "-test.run=^TestRunDemoCLIHelper$")
	command.Env = append(os.Environ(), "AI_TEAM_DEMO_HELPER=1")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("run --demo exited non-zero: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "demo completed") {
		t.Fatalf("run --demo did not report completion:\nstdout=%s\nstderr=%s", stdout.String(), stderr.String())
	}
}
