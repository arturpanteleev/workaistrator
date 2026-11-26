// Package gate exposes the gate machinery of this directory as the unified
// engine.GateRunner contract.
//
// The catalog has four built-in kinds: program (an external program speaking
// the JSON gate protocol), llm (a reviewer agent with a verdict schema), human
// (a decision bound to the exact result version) and required_phrases (the
// deterministic Markdown phrase check). NewRegistry wires the existing
// implementations into an engine.Registry, so a stage can resolve a gate by
// kind without any switch in the scheduler or the state machine.
//
// The adapters translate engine.Gate{ID, Version, Kind, Params} and
// engine.StageContext onto the types the packages under this directory already
// use, and translate their result back into engine.Decision. Everything the
// existing machinery needs beyond the definition parameters (the attempt
// identity, the artifact hash, the visit) arrives through the reserved
// parameters the scheduler adds (see gatesched.ReservedParamPrefix); they are
// stripped before the definition parameters are validated.
package gate

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arturpanteleev/workaistrator/pkg/engine"
	humandecision "github.com/arturpanteleev/workaistrator/pkg/engine/gate/human"
	llmgate "github.com/arturpanteleev/workaistrator/pkg/engine/gate/llm"
	gatephrases "github.com/arturpanteleev/workaistrator/pkg/engine/gate/phrases"
	gaterun "github.com/arturpanteleev/workaistrator/pkg/engine/gate/run"
	gatesched "github.com/arturpanteleev/workaistrator/pkg/engine/gate/sched"
	"github.com/arturpanteleev/workaistrator/pkg/engine/workflow"
)

// Kinds of the unified gate catalog.
const (
	// KindProgram is a trusted external program (or the same command inside a
	// container) speaking the JSON gate protocol.
	KindProgram = "program"
	// KindLLM is a reviewer agent whose answer must match a verdict schema.
	KindLLM = "llm"
	// KindHuman is a human decision bound to the exact result version.
	KindHuman = "human"
	// KindRequiredPhrases is the deterministic Markdown phrase check.
	KindRequiredPhrases = "required_phrases"
)

// Kinds lists the built-in kinds in registration order.
var Kinds = []string{KindProgram, KindLLM, KindHuman, KindRequiredPhrases}

// ProgramExecutor runs one program gate. *gaterun.Runner implements it.
type ProgramExecutor interface {
	Run(ctx context.Context, req gaterun.Request) (workflow.GateAttempt, error)
}

// Reviewer performs one reviewer-agent call (attempt is 1-based). A returned
// error is a refusal of the runtime, never a verdict.
type Reviewer interface {
	Review(ctx context.Context, gate engine.Gate, sc engine.StageContext, attempt int) (string, error)
}

// ReviewerFunc adapts a function to Reviewer.
type ReviewerFunc func(ctx context.Context, gate engine.Gate, sc engine.StageContext, attempt int) (string, error)

// Review implements Reviewer.
func (f ReviewerFunc) Review(ctx context.Context, gate engine.Gate, sc engine.StageContext, attempt int) (string, error) {
	return f(ctx, gate, sc, attempt)
}

// ContentSource returns the Markdown result a required_phrases gate checks.
type ContentSource interface {
	Result(ctx context.Context, gate engine.Gate, sc engine.StageContext) (string, error)
}

// ContentSourceFunc adapts a function to ContentSource.
type ContentSourceFunc func(ctx context.Context, gate engine.Gate, sc engine.StageContext) (string, error)

// Result implements ContentSource.
func (f ContentSourceFunc) Result(ctx context.Context, gate engine.Gate, sc engine.StageContext) (string, error) {
	return f(ctx, gate, sc)
}

// HumanDecider applies one human decision. *humandecision.Service implements it.
type HumanDecider interface {
	Decide(ctx context.Context, req humandecision.Request) (humandecision.Result, error)
}

// HumanRequestFunc builds the decision request of one human gate invocation.
type HumanRequestFunc func(gate engine.Gate, sc engine.StageContext) (humandecision.Request, error)

// Deps are the existing executors the adapters delegate to. An executor that
// is not supplied makes its kind report a configuration error instead of a
// verdict, so a partially wired registry can never pass a check silently.
type Deps struct {
	Program  ProgramExecutor
	Reviewer Reviewer
	Human    HumanDecider
	Content  ContentSource
	// Artifact describes the checked result version of a program gate.
	// Optional: by default the type is md and the hash comes from the reserved
	// invocation parameters.
	Artifact func(gate engine.Gate, sc engine.StageContext) gaterun.ArtifactInfo
	// RepoRoot is the repository root of a program gate. Optional: "." by
	// default.
	RepoRoot func(sc engine.StageContext) string
	// HumanRequest builds a human decision request. Optional: by default the
	// request is read from the gate parameters (approval_id, action,
	// subject_hash, actor_id, comment, channel).
	HumanRequest HumanRequestFunc
	// Roles returns the role predicate of an actor for the human gate.
	// Optional: without it a decision cannot be attributed to a required role.
	Roles func(actorID string) func(role string) bool
	// Now is the clock of a program attempt. Optional: time.Now by default.
	Now func() time.Time
}

// NewRegistry returns an engine.Registry with the four built-in kinds
// registered. Every kind resolves to an engine.GateRunner that delegates to
// the package responsible for it.
func NewRegistry(deps Deps) (*engine.Registry, error) {
	registry := engine.NewRegistry()
	runners := map[string]engine.GateRunner{
		KindProgram: ProgramGate{
			Executor: deps.Program, Artifact: deps.Artifact, RepoRoot: deps.RepoRoot, Now: deps.Now,
		},
		KindLLM: LLMGate{Reviewer: deps.Reviewer},
		KindHuman: HumanGate{
			Decider: deps.Human, Request: deps.HumanRequest, Roles: deps.Roles,
		},
		KindRequiredPhrases: PhrasesGate{Content: deps.Content},
	}
	for _, kind := range Kinds {
		if err := registry.RegisterGate(kind, runners[kind]); err != nil {
			return nil, err
		}
	}
	return registry, nil
}

// definitionParams returns the definition parameters of a gate: the reserved
// invocation parameters the scheduler adds are not configuration and are
// removed before the definition schema validates them.
func definitionParams(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for key, value := range params {
		if strings.HasPrefix(key, gatesched.ReservedParamPrefix) {
			continue
		}
		out[key] = value
	}
	return out
}

// stringParam reads a string parameter (or a reserved invocation value).
func stringParam(params map[string]any, name string) string {
	value, _ := params[name].(string)
	return value
}

// intParam reads an integer parameter (or a reserved invocation value).
func intParam(params map[string]any, name string) int {
	switch value := params[name].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		if value == float64(int(value)) {
			return int(value)
		}
	}
	return 0
}

// ProgramGate executes the program kind through the existing gaterun.Runner.
type ProgramGate struct {
	Executor ProgramExecutor
	Artifact func(gate engine.Gate, sc engine.StageContext) gaterun.ArtifactInfo
	RepoRoot func(sc engine.StageContext) string
	Now      func() time.Time
}

// Check runs the program and maps its recorded attempt to a decision. A
// technical execution (error, timeout, canceled) is returned as an error,
// never as a verdict.
func (g ProgramGate) Check(ctx context.Context, gate engine.Gate, sc engine.StageContext) (engine.Decision, error) {
	if g.Executor == nil {
		return engine.Decision{}, fmt.Errorf("program gate %s: исполнитель программы не настроен", gate.ID)
	}
	config, err := gaterun.ParseParams(definitionParams(gate.Params))
	if err != nil {
		return engine.Decision{}, fmt.Errorf("program gate %s: %w", gate.ID, err)
	}
	attempt, err := g.attempt(gate, sc)
	if err != nil {
		return engine.Decision{}, fmt.Errorf("program gate %s: %w", gate.ID, err)
	}
	recorded, err := g.Executor.Run(ctx, gaterun.Request{
		Attempt: attempt, Config: config, Artifact: g.artifact(gate, sc), RepoRoot: g.repoRoot(sc),
	})
	if err != nil {
		return engine.Decision{}, fmt.Errorf("program gate %s: %w", gate.ID, err)
	}
	return decisionFromAttempt(gate.ID, recorded)
}

// attempt rebuilds the attempt identity the scheduler supplied through the
// reserved parameters.
func (g ProgramGate) attempt(gate engine.Gate, sc engine.StageContext) (workflow.GateAttempt, error) {
	now := g.Now
	if now == nil {
		now = time.Now
	}
	attempt := workflow.GateAttempt{
		TaskID:         sc.RunID,
		Stage:          sc.StageID,
		Visit:          intParam(gate.Params, gatesched.ParamVisit),
		ArtifactSHA256: stringParam(gate.Params, gatesched.ParamArtifactSHA256),
		Gate:           gate.ID,
		GateVersion:    gate.Version,
		GateName:       stringParam(gate.Params, gatesched.ParamGateName),
		Mode:           workflow.GateModeRequired,
		AttemptID:      stringParam(gate.Params, gatesched.ParamAttemptID),
		Executor:       workflow.GateKindExternal,
		Group:          intParam(gate.Params, gatesched.ParamGroup),
		Phase:          workflow.GatePhase(stringParam(gate.Params, gatesched.ParamPhase)),
		Execution:      workflow.GateExecutionRunning,
		StartedAt:      now().UTC(),
	}
	if attempt.Visit <= 0 {
		attempt.Visit = 1
	}
	if attempt.GateName == "" {
		attempt.GateName = gate.ID
	}
	if err := attempt.Validate(); err != nil {
		return workflow.GateAttempt{}, err
	}
	return attempt, nil
}

func (g ProgramGate) artifact(gate engine.Gate, sc engine.StageContext) gaterun.ArtifactInfo {
	if g.Artifact != nil {
		return g.Artifact(gate, sc)
	}
	return gaterun.ArtifactInfo{
		Type:   string(workflow.ArtifactTypeMD),
		SHA256: stringParam(gate.Params, gatesched.ParamArtifactSHA256),
	}
}

func (g ProgramGate) repoRoot(sc engine.StageContext) string {
	if g.RepoRoot != nil {
		if root := strings.TrimSpace(g.RepoRoot(sc)); root != "" {
			return root
		}
	}
	return "."
}

// LLMGate executes the llm kind through the llmgate contract: the reviewer's
// answer must match the schema, and a technical failure stays a technical
// failure after the configured retries.
type LLMGate struct {
	Reviewer Reviewer
}

// Check runs the reviewer contract and maps its status to a decision.
func (g LLMGate) Check(ctx context.Context, gate engine.Gate, sc engine.StageContext) (engine.Decision, error) {
	if g.Reviewer == nil {
		return engine.Decision{}, fmt.Errorf("llm gate %s: проверяющий не настроен", gate.ID)
	}
	// The gate library names the schema "verdict_schema"; "schema" is kept as
	// an alias for callers that pass the contract directly.
	schema := stringParam(gate.Params, "verdict_schema")
	if schema == "" {
		schema = stringParam(gate.Params, "schema")
	}
	spec := llmgate.Spec{
		Schema:      llmgate.Schema(schema),
		MaxAttempts: intParam(gate.Params, "max_attempts"),
	}
	if !spec.Schema.Valid() {
		return engine.Decision{}, fmt.Errorf("llm gate %s: неизвестная схема вердикта %q", gate.ID, spec.Schema)
	}
	result := llmgate.Run(ctx, spec, func(ctx context.Context, attempt int) (string, error) {
		return g.Reviewer.Review(ctx, gate, sc, attempt)
	})
	switch result.Status {
	case llmgate.StatusPass:
		return engine.Decision{Pass: true, Message: result.Reason}, nil
	case llmgate.StatusFail:
		return engine.Decision{Pass: false, Message: result.Reason}, nil
	case llmgate.StatusCanceled:
		return engine.Decision{}, fmt.Errorf("llm gate %s: отменена: %s", gate.ID, result.Reason)
	default:
		return engine.Decision{}, fmt.Errorf("llm gate %s: %s", gate.ID, result.Reason)
	}
}

// PhrasesGate executes the required_phrases kind through gatephrases: the
// parameters are validated by the version of the definition, the result text
// comes from the supplied source.
type PhrasesGate struct {
	Content ContentSource
}

// Check validates the parameters, checks the result and maps the outcome.
func (g PhrasesGate) Check(ctx context.Context, gate engine.Gate, sc engine.StageContext) (engine.Decision, error) {
	if g.Content == nil {
		return engine.Decision{}, fmt.Errorf("required_phrases gate %s: источник результата не настроен", gate.ID)
	}
	if gate.Version <= 0 {
		return engine.Decision{}, fmt.Errorf("required_phrases gate %s: версия определения обязательна", gate.ID)
	}
	spec, err := gatephrases.ParseVersion(gate.Version, definitionParams(gate.Params))
	if err != nil {
		return engine.Decision{}, fmt.Errorf("required_phrases gate %s: %w", gate.ID, err)
	}
	content, err := g.Content.Result(ctx, gate, sc)
	if err != nil {
		return engine.Decision{}, fmt.Errorf("required_phrases gate %s: %w", gate.ID, err)
	}
	result := spec.Check(content)
	return engine.Decision{Pass: result.Satisfied, Message: result.Reason}, nil
}

// HumanGate executes the human kind through humandecision.Service: the request
// is applied by the single service path, and the gate passes only when the
// decision resolved (the quorum is complete).
//
// The unified contract has no "waiting" state: a decision that was stored but
// did not resolve the quorum is reported as an error, so it is never mistaken
// for a negative verdict and can be retried when the remaining votes arrive.
type HumanGate struct {
	Decider HumanDecider
	Request HumanRequestFunc
	Roles   func(actorID string) func(role string) bool
}

// Check applies the decision and maps it to a decision of the gate. The gate
// passes only when the resolved action is the positive one: a rejection or a
// return is a negative verdict, exactly as the pipeline records it.
func (g HumanGate) Check(ctx context.Context, gate engine.Gate, sc engine.StageContext) (engine.Decision, error) {
	if g.Decider == nil {
		return engine.Decision{}, fmt.Errorf("human gate %s: сервис решения не настроен", gate.ID)
	}
	request, err := g.request(gate, sc)
	if err != nil {
		return engine.Decision{}, fmt.Errorf("human gate %s: %w", gate.ID, err)
	}
	if request.Actor.HasRole == nil && g.Roles != nil {
		request.Actor.HasRole = g.Roles(request.Actor.ID)
	}
	result, err := g.Decider.Decide(ctx, request)
	if err != nil {
		return engine.Decision{}, fmt.Errorf("human gate %s: %w", gate.ID, err)
	}
	if !result.Resolved {
		return engine.Decision{}, fmt.Errorf("human gate %s: решение записано, но кворум не собран (approval %s)", gate.ID, result.Approval.ID)
	}
	action := result.Approval.ResolvedAction
	if action == "" {
		action = request.Action
	}
	if action == humanApproveAction {
		return engine.Decision{Pass: true, Message: "решение принято"}, nil
	}
	return engine.Decision{Pass: false, Message: fmt.Sprintf("решение отклонено: %s", action)}, nil
}

// humanApproveAction is the positive action of a human decision; every other
// action (reject, return_to_*) does not accept the result.
const humanApproveAction = "approve"

func (g HumanGate) request(gate engine.Gate, sc engine.StageContext) (humandecision.Request, error) {
	if g.Request != nil {
		return g.Request(gate, sc)
	}
	request := humandecision.Request{
		RunID:       sc.RunID,
		ApprovalID:  stringParam(gate.Params, "approval_id"),
		Action:      stringParam(gate.Params, "action"),
		SubjectHash: stringParam(gate.Params, "subject_hash"),
		Comment:     stringParam(gate.Params, "comment"),
		Channel:     stringParam(gate.Params, "channel"),
		Actor:       humandecision.Actor{ID: stringParam(gate.Params, "actor_id")},
	}
	if request.ApprovalID == "" || request.Action == "" || request.Actor.ID == "" {
		return humandecision.Request{}, errors.New("параметры approval_id, action и actor_id обязательны")
	}
	return request, nil
}

// decisionFromAttempt maps a recorded attempt to a decision. Only a succeeded
// execution carries a verdict; everything else is a technical failure.
func decisionFromAttempt(id string, attempt workflow.GateAttempt) (engine.Decision, error) {
	if attempt.Execution != workflow.GateExecutionSucceeded {
		return engine.Decision{}, fmt.Errorf("gate %s: %s: %s", id, attempt.Execution, strings.TrimSpace(attempt.Reason))
	}
	switch attempt.Verdict {
	case workflow.GateVerdictPass, workflow.GateVerdictNotApplicable:
		return engine.Decision{Pass: true, Message: attempt.Reason}, nil
	default:
		return engine.Decision{Pass: false, Message: attempt.Reason}, nil
	}
}
