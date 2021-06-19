package util

import (
	"io"
	"os"

	"go.starlark.net/starlark"
)

func SetStdio(thread *starlark.Thread, stdout, stderr io.Writer) {
	thread.SetLocal("stdout", stdout)
	thread.SetLocal("stderr", stderr)
}

func Stdio(thread *starlark.Thread) (io.Writer, io.Writer) {
	stdout, ok := thread.Local("stdout").(io.Writer)
	if !ok {
		stdout = os.Stdout
	}
	stderr, ok := thread.Local("stderr").(io.Writer)
	if !ok {
		stderr = os.Stderr
	}
	return stdout, stderr
}
