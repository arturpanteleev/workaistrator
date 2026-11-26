package gatesched

import (
	"context"

	"github.com/arturpanteleev/workaistrator/pkg/engine"
	"github.com/arturpanteleev/workaistrator/pkg/engine/workflow"
)

// hook.go — the additive bridge to the unified gate contract (task 2.2).
//
// The scheduler historically executes one gate through a Run method from the
// per-kind Runner map (WorkflowGateKind -> Runner). Options.Gates adds a second,
// preferred path: a gate kind is resolved to an engine.GateRunner and executed
// through Check(ctx, Gate, StageContext). The existing dispatch is untouched:
// when Options.Gates is nil, or the kind is not registered there, the scheduler
// falls back to the Runner map exactly as before.
//
// The unified contract carries only the engine types. Everything the existing
// machinery needs beyond the definition (the attempt identity, the artifact
// hash, the visit) is passed through engine.Gate.Params under reserved keys
// that start with ReservedParamPrefix. The adapters in pkg/engine/gate strip
// them before the definition parameters are validated, so a reserved key can
// never be mistaken for a configuration parameter.

// ReservedParamPrefix marks the invocation-identity keys the scheduler adds to
// engine.Gate.Params. Gate definitions cannot use it.
const ReservedParamPrefix = "__"

// Reserved invocation parameters. The scheduler always sets them when it
// builds an engine.Gate for a GateRunner.
const (
	ParamAttemptID      = ReservedParamPrefix + "attempt_id"
	ParamGateName       = ReservedParamPrefix + "gate_name"
	ParamArtifactSHA256 = ReservedParamPrefix + "artifact_sha256"
	ParamVisit          = ReservedParamPrefix + "visit"
	ParamGroup          = ReservedParamPrefix + "group"
	ParamPhase          = ReservedParamPrefix + "phase"
	ParamResumed        = ReservedParamPrefix + "resumed"
)

// GateResolver resolves a gate kind to the unified runner. *engine.Registry
// implements it (Registry.Gate).
type GateResolver interface {
	Gate(kind string) (engine.GateRunner, error)
}

// resolved returns the unified runner of the kind, if the hook serves it.
func (s *Scheduler) resolved(kind workflow.GateKind) (engine.GateRunner, bool) {
	if s.opts.Gates == nil {
		return nil, false
	}
	runner, err := s.opts.Gates.Gate(string(kind))
	if err != nil || runner == nil {
		return nil, false
	}
	return runner, true
}

// hasRunner reports whether a gate kind can be executed at all: through the
// unified hook when it serves the kind, otherwise through the Runner map.
func (s *Scheduler) hasRunner(kind workflow.GateKind) bool {
	if _, ok := s.resolved(kind); ok {
		return true
	}
	_, ok := s.runners[kind]
	return ok
}

// unifiedGate maps one scheduler invocation onto the unified contract. The
// reference parameters are copied (never mutated) and the invocation identity
// is added under the reserved keys.
func unifiedGate(ctx context.Context, in Invocation) (engine.Gate, engine.StageContext) {
	params := make(map[string]any, len(in.Ref.Params)+7)
	for key, value := range in.Ref.Params {
		params[key] = value
	}
	params[ParamAttemptID] = in.Attempt.AttemptID
	params[ParamGateName] = in.Ref.InstanceName()
	params[ParamArtifactSHA256] = in.Attempt.ArtifactSHA256
	params[ParamVisit] = in.Attempt.Visit
	params[ParamGroup] = in.Attempt.Group
	params[ParamPhase] = string(in.Attempt.Phase.Effective())
	params[ParamResumed] = in.Resumed
	gate := engine.Gate{ID: in.Def.ID, Version: in.Def.Version, Kind: string(in.Def.Kind), Params: params}
	stage := engine.StageContext{Ctx: ctx, RunID: in.Attempt.TaskID, StageID: in.Attempt.Stage}
	return gate, stage
}

// outcomeFromDecision translates the unified decision back to a scheduler
// outcome. A technical failure of the runner is an error, never a verdict; a
// canceled context keeps its own state.
func outcomeFromDecision(ctx context.Context, decision engine.Decision, err error) Outcome {
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Outcome{Execution: workflow.GateExecutionCanceled, Reason: err.Error()}
		}
		return TechnicalError(err.Error())
	}
	if decision.Pass {
		return Pass(decision.Message)
	}
	return Fail(decision.Message)
}
