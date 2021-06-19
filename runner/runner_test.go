package runner

import (
	"fmt"
	"testing"
)

type testTarget func(engine Engine) error

func (t testTarget) Evaluate(engine Engine) error {
	return t(engine)
}

type testTargets map[string]Target

func (tt testTargets) LoadTarget(label string) (Target, error) {
	if t, ok := tt[label]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("unknown target %v", label)
}

func TestCyclicDependency(t *testing.T) {
	foo := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets("bar")[0].Error
	})
	bar := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets("foo")[0].Error
	})

	Run(testTargets{
		"foo": foo,
		"bar": bar,
	}, "foo")
}

func TestCyclicDependency_Inner(t *testing.T) {
	foo := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets("bar")[0].Error
	})
	bar := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets("baz")[0].Error
	})
	baz := testTarget(func(engine Engine) error {
		return engine.EvaluateTargets("bar")[0].Error
	})

	Run(testTargets{
		"foo": foo,
		"bar": bar,
		"baz": baz,
	}, "foo")
}
