package util

import (
	"context"

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
