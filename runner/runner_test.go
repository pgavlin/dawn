package runner

import (
	"context"
	"fmt"
	"testing"
)

type testTarget func(engine Engine) error

func (t testTarget) Evaluate(_ context.Context, engine Engine) error {
	return t(engine)
}

type testTargets map[string]Target

func (tt testTargets) LoadTarget(_ context.Context, label string) (Target, error) {
	if t, ok := tt[label]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("unknown target %v", label)
}

func TestCyclicDependency(t *testing.T) {
	foo := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets(t.Context(), "bar")[0].Error
	})
	bar := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets(t.Context(), "foo")[0].Error
	})

	Run(t.Context(), testTargets{
		"foo": foo,
		"bar": bar,
	}, "foo")
}

func TestCyclicDependency_Inner(t *testing.T) {
	foo := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets(t.Context(), "bar")[0].Error
	})
	bar := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets(t.Context(), "baz")[0].Error
	})
	baz := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets(t.Context(), "bar")[0].Error
	})

	Run(t.Context(), testTargets{
		"foo": foo,
		"bar": bar,
		"baz": baz,
	}, "foo")
}
