package sh

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func Exec(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	file, options, try, err := command(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}

	stdout, stderr := util.Stdio(thread)
	options = append(options, interp.StdIO(nil, stdout, stderr))

	if err := exec(context.Background(), file, options); err != nil {
		if try {
			return starlark.String(err.Error()), nil
		}
		return nil, err
	}
	return starlark.None, nil
}

func Output(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	file, options, try, err := command(thread, fn, args, kwargs)
	if err != nil {
		return nil, err
	}

	var stdout strings.Builder
	_, stderr := util.Stdio(thread)
	if try {
		stderr = io.Discard
	}
	options = append(options, interp.StdIO(nil, &stdout, stderr))

	if err := exec(context.Background(), file, options); err != nil {
		if try {
			return starlark.Tuple{starlark.None, starlark.String(err.Error())}, nil
		}
		return nil, err
	}

	out := starlark.String(stdout.String())
	if try {
		return starlark.Tuple{out, starlark.None}, nil
	}
	return out, nil
}

func command(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (*syntax.File, []interp.RunnerOption, bool, error) {
	var (
		command string
		cwd     string
		env     starlark.IterableMapping
		try     bool
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "cmd", &command, "cwd?", &cwd, "env?", &env, "try_?", &try); err != nil {
		return nil, nil, false, err
	}

	file, err := syntax.NewParser().Parse(strings.NewReader(command), "")
	if err != nil {
		return nil, nil, false, err
	}

	var options []interp.RunnerOption
	if cwd == "" {
		cwd = util.Getwd(thread)
	}
	options = append(options, interp.Dir(cwd))

	if env != nil {
		items := env.Items()

		pairs := make([]string, len(items))
		for i, kvp := range items {
			pairs[i] = fmt.Sprintf("%v=%v", kvp[0], kvp[1])
		}

		options = append(options, interp.Env(expand.ListEnviron(pairs...)))
	}

	return file, options, try, nil
}

func exec(ctx context.Context, file *syntax.File, options []interp.RunnerOption) error {
	runner, err := interp.New(options...)
	if err != nil {
		return err
	}
	return runner.Run(ctx, file)
}
