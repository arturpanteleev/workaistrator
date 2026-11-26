package gate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/arturpanteleev/workaistrator/pkg/engine"
	"github.com/arturpanteleev/workaistrator/pkg/engine/approval"
	humandecision "github.com/arturpanteleev/workaistrator/pkg/engine/gate/human"
	gaterun "github.com/arturpanteleev/workaistrator/pkg/engine/gate/run"
	gatesched "github.com/arturpanteleev/workaistrator/pkg/engine/gate/sched"
	"github.com/arturpanteleev/workaistrator/pkg/engine/workflow"
)

const testHash = "1111111111111111111111111111111111111111111111111111111111111111"

func succeeded(verdict workflow.GateVerdict, reason string) workflow.GateAttempt {
	return workflow.GateAttempt{Execution: workflow.GateExecutionSucceeded, Verdict: verdict, Reason: reason}
}

// programParams are the definition and invocation parameters of a program gate.
func programParams() map[string]any {
	return map[string]any{
		"command":                     []string{"/bin/true"},
		gatesched.ParamAttemptID:      "att-1",
		gatesched.ParamArtifactSHA256: testHash,
		gatesched.ParamVisit:          2,
		gatesched.ParamGateName:       "ext",
		gatesched.ParamGroup:          1,
		gatesched.ParamPhase:          string(workflow.GatePhaseExit),
	}
}

func phrasesParams() map[string]any {
	return map[string]any{
		"phrases": []string{"итог"},
		"case":    "insensitive",
		"scope":   "result",
		"match":   "all",
	}
}

func humanParams() map[string]any {
	return map[string]any{
		"approval_id":  "ap-1",
		"action":       "approve",
		"subject_hash": testHash,
		"actor_id":     "user-1",
	}
}

// gateOf builds a gate of the given kind with its parameters.
func gateOf(kind string, params map[string]any) engine.Gate {
	version := 1
	if kind == KindRequiredPhrases {
		version = 2
	}
	return engine.Gate{ID: kind, Version: version, Kind: kind, Params: params}
}

type fakeProgram struct {
	requests []gaterun.Request
	attempt  workflow.GateAttempt
	err      error
}

func (f *fakeProgram) Run(_ context.Context, req gaterun.Request) (workflow.GateAttempt, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return workflow.GateAttempt{}, f.err
	}
	return f.attempt, nil
}

type fakeDecider struct {
	request humandecision.Request
	result  humandecision.Result
	err     error
}

func (f *fakeDecider) Decide(_ context.Context, req humandecision.Request) (humandecision.Result, error) {
	f.request = req
	return f.result, f.err
}

func approvingRoles(string) func(string) bool {
	return func(string) bool { return true }
}

// allDeps wires every kind with a passing implementation.
func allDeps(program *fakeProgram, decider *fakeDecider) Deps {
	return Deps{
		Program: program,
		Reviewer: ReviewerFunc(func(context.Context, engine.Gate, engine.StageContext, int) (string, error) {
			return "Verdict: PASSED", nil
		}),
		Human: decider,
		Content: ContentSourceFunc(func(context.Context, engine.Gate, engine.StageContext) (string, error) {
			return "ИТОГ: готово", nil
		}),
		Roles:    approvingRoles,
		RepoRoot: func(engine.StageContext) string { return "/repo" },
	}
}

func approvingDecision() humandecision.Result {
	return humandecision.Result{
		Approval: approval.PendingApproval{ID: "ap-1", ResolvedAction: "approve"},
		Resolved: true,
	}
}

// Acceptance: all four catalog kinds resolve through the registry and execute.
func TestRegistryExecutesAllKinds(t *testing.T) {
	program := &fakeProgram{attempt: succeeded(workflow.GateVerdictPass, "program ok")}
	decider := &fakeDecider{result: approvingDecision()}
	registry, err := NewRegistry(allDeps(program, decider))
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	cases := []struct {
		kind   string
		params map[string]any
	}{
		{KindProgram, programParams()},
		{KindLLM, nil},
		{KindHuman, humanParams()},
		{KindRequiredPhrases, phrasesParams()},
	}
	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			runner, err := registry.Gate(tc.kind)
			if err != nil {
				t.Fatalf("resolve %s: %v", tc.kind, err)
			}
			decision, err := runner.Check(context.Background(), gateOf(tc.kind, tc.params), engine.StageContext{RunID: "run-1", StageID: "spec"})
			if err != nil || !decision.Pass {
				t.Fatalf("%s: decision %+v err %v", tc.kind, decision, err)
			}
		})
	}
	if len(program.requests) != 1 {
		t.Fatalf("program gate was not executed: %d requests", len(program.requests))
	}
}

func TestRegistryUnknownKindErrors(t *testing.T) {
	registry, err := NewRegistry(Deps{})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	if _, err := registry.Gate("no_such_kind"); err == nil {
		t.Fatal("unknown kind resolved")
	}
	for _, kind := range Kinds {
		if _, err := registry.Gate(kind); err != nil {
			t.Fatalf("%s must be pre-registered: %v", kind, err)
		}
	}
}

func TestProgramGateMapsInvocation(t *testing.T) {
	program := &fakeProgram{attempt: succeeded(workflow.GateVerdictPass, "ok")}
	gate := gateOf(KindProgram, programParams())
	decision, err := ProgramGate{
		Executor: program, RepoRoot: func(engine.StageContext) string { return "/repo" },
	}.Check(context.Background(), gate, engine.StageContext{RunID: "run-1", StageID: "spec"})
	if err != nil || !decision.Pass {
		t.Fatalf("check: %+v %v", decision, err)
	}
	if len(program.requests) != 1 {
		t.Fatalf("requests: %d", len(program.requests))
	}
	req := program.requests[0]
	if req.Attempt.TaskID != "run-1" || req.Attempt.Stage != "spec" {
		t.Fatalf("stage identity: %+v", req.Attempt)
	}
	if req.Attempt.AttemptID != "att-1" || req.Attempt.Visit != 2 || req.Attempt.ArtifactSHA256 != testHash {
		t.Fatalf("attempt identity: %+v", req.Attempt)
	}
	if req.Attempt.Gate != KindProgram || req.Attempt.GateVersion != 1 || req.Attempt.GateName != "ext" || req.Attempt.Group != 1 {
		t.Fatalf("gate identity: %+v", req.Attempt)
	}
	// Reserved keys are not configuration: ParseParams must see only "command".
	if len(req.Config.Command) != 1 || req.Config.Command[0] != "/bin/true" {
		t.Fatalf("config: %+v", req.Config)
	}
	if req.RepoRoot != "/repo" || req.Artifact.Type != "md" || req.Artifact.SHA256 != testHash {
		t.Fatalf("request: %+v", req)
	}
}

func TestProgramGateFailAndTechnicalFailure(t *testing.T) {
	gate := gateOf(KindProgram, programParams())
	negative := &fakeProgram{attempt: succeeded(workflow.GateVerdictFail, "нет доступа")}
	sc := engine.StageContext{RunID: "run-1", StageID: "spec"}
	decision, err := ProgramGate{Executor: negative}.Check(context.Background(), gate, sc)
	if err != nil || decision.Pass {
		t.Fatalf("negative verdict: %+v %v", decision, err)
	}
	if decision.Message != "нет доступа" {
		t.Fatalf("message = %q", decision.Message)
	}

	failed := &fakeProgram{err: errors.New("нерaбочая программа")}
	if _, err := (ProgramGate{Executor: failed}).Check(context.Background(), gate, sc); err == nil {
		t.Fatal("a runner error must not become a verdict")
	}

	broken := &fakeProgram{attempt: workflow.GateAttempt{Execution: workflow.GateExecutionError, Reason: "crash"}}
	if _, err := (ProgramGate{Executor: broken}).Check(context.Background(), gate, sc); err == nil {
		t.Fatal("a technical execution must not become a verdict")
	}

	if _, err := (ProgramGate{}).Check(context.Background(), gate, sc); err == nil {
		t.Fatal("an unwired executor must be reported")
	}
}

func TestLLMGateContract(t *testing.T) {
	calls := 0
	reviewer := ReviewerFunc(func(_ context.Context, _ engine.Gate, _ engine.StageContext, attempt int) (string, error) {
		calls = attempt
		if attempt == 1 {
			return "", errors.New("runtime unavailable")
		}
		return "Verdict: PASSED", nil
	})
	gate := gateOf(KindLLM, map[string]any{"max_attempts": 2})
	decision, err := LLMGate{Reviewer: reviewer}.Check(context.Background(), gate, engine.StageContext{RunID: "run-1", StageID: "spec"})
	if err != nil || !decision.Pass || calls != 2 {
		t.Fatalf("retry: decision %+v err %v calls %d", decision, err, calls)
	}

	negative := ReviewerFunc(func(context.Context, engine.Gate, engine.StageContext, int) (string, error) {
		return "Verdict: FAILED", nil
	})
	if decision, err := (LLMGate{Reviewer: negative}).Check(context.Background(), gate, engine.StageContext{RunID: "run-1", StageID: "spec"}); err != nil || decision.Pass {
		t.Fatalf("negative: %+v %v", decision, err)
	}

	outside := ReviewerFunc(func(context.Context, engine.Gate, engine.StageContext, int) (string, error) {
		return "нет вердикта", nil
	})
	if _, err := (LLMGate{Reviewer: outside}).Check(context.Background(), gate, engine.StageContext{RunID: "run-1", StageID: "spec"}); err == nil {
		t.Fatal("an answer outside the schema must be a technical error")
	}

	// The library names the schema parameter verdict_schema.
	structured := gateOf(KindLLM, map[string]any{"verdict_schema": "json"})
	jsonReviewer := ReviewerFunc(func(context.Context, engine.Gate, engine.StageContext, int) (string, error) {
		return `{"verdict":"pass","reason":"ok"}`, nil
	})
	if decision, err := (LLMGate{Reviewer: jsonReviewer}).Check(context.Background(), structured, engine.StageContext{}); err != nil || !decision.Pass {
		t.Fatalf("verdict_schema json: %+v %v", decision, err)
	}
}

func TestPhrasesGate(t *testing.T) {
	passing := ContentSourceFunc(func(context.Context, engine.Gate, engine.StageContext) (string, error) {
		return "ИТОГ: всё готово", nil
	})
	decision, err := PhrasesGate{Content: passing}.Check(context.Background(), gateOf(KindRequiredPhrases, phrasesParams()), engine.StageContext{})
	if err != nil || !decision.Pass {
		t.Fatalf("pass: %+v %v", decision, err)
	}

	missing := ContentSourceFunc(func(context.Context, engine.Gate, engine.StageContext) (string, error) {
		return "ничего нет", nil
	})
	decision, err = PhrasesGate{Content: missing}.Check(context.Background(), gateOf(KindRequiredPhrases, phrasesParams()), engine.StageContext{})
	if err != nil || decision.Pass || decision.Message == "" {
		t.Fatalf("fail: %+v %v", decision, err)
	}

	broken := map[string]any{"phrases": []string{"x"}, "case": "insensitive", "scope": "result", "match": "unknown"}
	if _, err := (PhrasesGate{Content: passing}).Check(context.Background(), gateOf(KindRequiredPhrases, broken), engine.StageContext{}); err == nil {
		t.Fatal("invalid parameters must be a technical error")
	}
	if _, err := (PhrasesGate{}).Check(context.Background(), gateOf(KindRequiredPhrases, phrasesParams()), engine.StageContext{}); err == nil {
		t.Fatal("an unwired content source must be reported")
	}
}

func TestHumanGate(t *testing.T) {
	decider := &fakeDecider{result: approvingDecision()}
	gate := gateOf(KindHuman, humanParams())
	decision, err := HumanGate{Decider: decider, Roles: approvingRoles}.Check(context.Background(), gate, engine.StageContext{RunID: "run-1"})
	if err != nil || !decision.Pass {
		t.Fatalf("decide: %+v %v", decision, err)
	}
	request := decider.request
	if request.RunID != "run-1" || request.ApprovalID != "ap-1" || request.Action != "approve" || request.Actor.ID != "user-1" {
		t.Fatalf("request: %+v", request)
	}
	if request.SubjectHash != testHash {
		t.Fatalf("subject hash: %q", request.SubjectHash)
	}
	if request.Actor.HasRole == nil || !request.Actor.HasRole("owner") {
		t.Fatal("roles were not applied")
	}

	// A resolved rejection is a negative verdict of the gate, not a pass.
	rejected := &fakeDecider{result: humandecision.Result{
		Approval: approval.PendingApproval{ID: "ap-1", ResolvedAction: "reject"},
		Resolved: true,
	}}
	decision, err = HumanGate{Decider: rejected, Roles: approvingRoles}.Check(context.Background(), gate, engine.StageContext{RunID: "run-1"})
	if err != nil || decision.Pass {
		t.Fatalf("reject: %+v %v", decision, err)
	}
	if !strings.Contains(decision.Message, "reject") {
		t.Fatalf("reject message = %q", decision.Message)
	}

	unresolved := &fakeDecider{result: humandecision.Result{Approval: approval.PendingApproval{ID: "ap-1"}}}
	if _, err := (HumanGate{Decider: unresolved, Roles: approvingRoles}).Check(context.Background(), gate, engine.StageContext{RunID: "run-1"}); err == nil {
		t.Fatal("an unresolved quorum must not pass the gate")
	}

	refused := &fakeDecider{err: humandecision.ErrNotFound}
	if _, err := (HumanGate{Decider: refused}).Check(context.Background(), gate, engine.StageContext{RunID: "run-1"}); !errors.Is(err, humandecision.ErrNotFound) {
		t.Fatalf("decider error must be propagated: %v", err)
	}

	if _, err := (HumanGate{Decider: decider}).Check(context.Background(), gateOf(KindHuman, nil), engine.StageContext{}); err == nil {
		t.Fatal("a request without required parameters must fail")
	}
}

func TestDecisionFromAttempt(t *testing.T) {
	if decision, err := decisionFromAttempt("g", succeeded(workflow.GateVerdictNotApplicable, "")); err != nil || !decision.Pass {
		t.Fatalf("not applicable: %+v %v", decision, err)
	}
	if _, err := decisionFromAttempt("g", workflow.GateAttempt{Execution: workflow.GateExecutionTimeout}); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("timeout: %v", err)
	}
}
