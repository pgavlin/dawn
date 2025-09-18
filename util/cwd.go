package util

import (
	"os"

	"github.com/pgavlin/starlark-go/starlark"
)

func Chdir(thread *starlark.Thread, wd string) {
	thread.SetLocal("wd", wd)
}

func Getwd(thread *starlark.Thread) string {
	wd, ok := thread.Local("wd").(string)
	if !ok {
		d, err := os.Getwd()
		if err == nil {
			wd = d
		}
	}
	return wd
}
