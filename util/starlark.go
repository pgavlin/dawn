package util

import (
	"context"
	"iter"

	"github.com/pgavlin/starlark-go/starlark"
)

func SetContext(ctx context.Context, thread *starlark.Thread) (done func()) {
	thread.SetLocal("context", ctx)

	stopChan := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			thread.Cancel(context.Cause(ctx).Error())
		case <-stopChan:
			// Done
		}
	}()
	return func() { close(stopChan) }
}

func GetContext(thread *starlark.Thread) context.Context {
	if ctx, ok := thread.Local("context").(context.Context); ok {
		return ctx
	}
	return context.Background()
}

func All[T starlark.Iterable](v T) iter.Seq[starlark.Value] {
	return func(yield func(starlark.Value) bool) {
		it := v.Iterate()
		defer it.Done()
		var e starlark.Value
		for it.Next(&e) {
			if !yield(e) {
				return
			}
		}
	}
}
