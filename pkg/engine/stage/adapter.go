// Package stage exposes the existing stage executors as the unified
// engine.Executor contract. NewExecutorRegistry wires executors by name so the
// engine never switches on a concrete implementation type.
package stage

import (
	"context"
	"fmt"

	"github.com/arturpanteleev/workaistrator/pkg/engine"
	stageexec "github.com/arturpanteleev/workaistrator/pkg/engine/stage/exec"
)

// Executor is an alias for the unified engine contract. Existing stage
// executors implement it through StageExecutorAdapter.
type Executor = engine.Executor

// StageExecutorAdapter exposes the existing StageExecutor as engine.Executor.
// Build converts one StageContext into the controller's detailed attempt
// request; it is supplied by the composition root, where all stage metadata is
// known.
type StageExecutorAdapter struct {
	Executor stageexec.StageExecutor
	Build    func(engine.StageContext) stageexec.Request
}

// Run implements engine.Executor.
func (a StageExecutorAdapter) Run(ctx context.Context, sc engine.StageContext) (engine.Result, error) {
	if a.Executor == nil || a.Build == nil {
		return engine.Result{}, fmt.Errorf("stage executor is not wired")
	}
	request := a.Build(sc)
	request.RunID = sc.RunID
	request.Stage = sc.StageID
	request.AllowedPaths = append([]string(nil), sc.Paths...)
	result, err := a.Executor.Execute(ctx, request)
	status := engine.StatusFail
	if err == nil && (result.Verdict == "APPROVED" || result.Verdict == "PASS") {
		status = engine.StatusPass
	}
	return engine.Result{
		Status:  status,
		Message: result.Summary,
	}, err
}

// NewExecutorRegistry registers stage executors under stable names.
func NewExecutorRegistry(executors map[string]engine.Executor) (*engine.Registry, error) {
	registry := engine.NewRegistry()
	for name, executor := range executors {
		if err := registry.RegisterExecutor(name, executor); err != nil {
			return nil, err
		}
	}
	return registry, nil
}
