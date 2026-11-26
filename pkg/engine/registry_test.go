package engine

import (
	"context"
	"testing"
)

type fakeExecutor struct{ called bool }

func (f *fakeExecutor) Run(_ context.Context, _ StageContext) (Result, error) {
	f.called = true
	return Result{Status: StatusPass}, nil
}

type fakeGate struct{}

func (fakeGate) Check(_ context.Context, _ Gate, _ StageContext) (Decision, error) {
	return Decision{Pass: true}, nil
}

func TestRegistryExecutor(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterExecutor("agent", &fakeExecutor{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := r.RegisterExecutor("agent", &fakeExecutor{}); err == nil {
		t.Fatal("expected duplicate registration error")
	}
	if _, err := r.Executor("missing"); err == nil {
		t.Fatal("expected unknown executor error")
	}
	e, err := r.Executor("agent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	res, err := e.Run(context.Background(), StageContext{})
	if err != nil || res.Status != StatusPass {
		t.Fatalf("run: %v %+v", err, res)
	}
}

func TestRegistryGate(t *testing.T) {
	r := NewRegistry()
	if err := r.RegisterGate("program", fakeGate{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	g, err := r.Gate("program")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	d, err := g.Check(context.Background(), Gate{Kind: "program"}, StageContext{})
	if err != nil || !d.Pass {
		t.Fatalf("check: %v %+v", err, d)
	}
}
