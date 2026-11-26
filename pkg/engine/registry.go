package engine

import (
	"fmt"
	"sync"
)

// Registry хранит именованные реализации Executor и GateRunner. Регистрация
// нового типа не требует правок движка (open/closed principle).
type Registry struct {
	mu        sync.RWMutex
	executors map[string]Executor
	gates     map[string]GateRunner
}

func NewRegistry() *Registry {
	return &Registry{
		executors: make(map[string]Executor),
		gates:     make(map[string]GateRunner),
	}
}

func (r *Registry) RegisterExecutor(name string, e Executor) error {
	if name == "" || e == nil {
		return fmt.Errorf("engine: executor name and implementation are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.executors[name]; ok {
		return fmt.Errorf("engine: executor %q already registered", name)
	}
	r.executors[name] = e
	return nil
}

func (r *Registry) Executor(name string) (Executor, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.executors[name]
	if !ok {
		return nil, fmt.Errorf("engine: unknown executor %q", name)
	}
	return e, nil
}

func (r *Registry) RegisterGate(kind string, g GateRunner) error {
	if kind == "" || g == nil {
		return fmt.Errorf("engine: gate kind and implementation are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.gates[kind]; ok {
		return fmt.Errorf("engine: gate %q already registered", kind)
	}
	r.gates[kind] = g
	return nil
}

func (r *Registry) Gate(kind string) (GateRunner, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.gates[kind]
	if !ok {
		return nil, fmt.Errorf("engine: unknown gate %q", kind)
	}
	return g, nil
}
