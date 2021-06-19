package os

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

func Exec(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	cmd, err := command(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = util.Stdio(thread)

	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %w", fn.Name(), err)
	}
	return starlark.None, nil
}

func Output(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	cmd, err := command(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}

	var stdout strings.Builder
	cmd.Stdout = &stdout
	_, cmd.Stderr = util.Stdio(thread)

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %w", fn.Name(), err)
	}

	return starlark.String(stdout.String()), nil
}

func command(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (*exec.Cmd, error) {
	var (
		command util.StringList
		cwd     string
		envV    starlark.IterableMapping
	)

	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "command", &command, "cwd?", &cwd, "env?", &envV); err != nil {
		return nil, err
	}

	if len(command) == 0 {
		return nil, fmt.Errorf("%v: command must have at least one element", fn.Name())
	}

	if cwd == "" {
		cwd = util.Getwd(thread)
	}

	env := os.Environ()
	if envV != nil {
		items := envV.Items()

		env = make([]string, len(items))
		for i, kvp := range items {
			env[i] = fmt.Sprintf("%v=%v", kvp[0], kvp[1])
		}
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = cwd

	return cmd, nil
}
