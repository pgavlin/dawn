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

// def exec(command, cwd=None, env=None, try_=None):
//     """
//     Execute a shell command. The command must be a valid POSIX Shell, Bash,
//     or mksh command. Any commands that are not shell builtins must be
//     available on the path. If the command fails, the calling module will
//     abort unless `try_` is set to True, in which case the contents of
//     standard error will be returned.
//
//     :param command: the command to execute.
//     :param cwd: the working directory for the command. Defaults to the
//                 calling module's directory.
//     :param env: any environment variables to set when running the command.
//     :param `try_`: when True, the calling module will not be aborted if
//                  the shell command fails.
//
//     :returns: the contents of standard error if `try_` is set and None
//               otherwise. To capture the command's output, use output.
//     """
//
//starlark:builtin factory=NewExec,function=Exec
func exec(thread *starlark.Thread, fn *starlark.Builtin, cmd, cwd string, env starlark.IterableMapping, try bool) (starlark.Value, error) {
	file, options, try, err := command(thread, cmd, cwd, env, try)
	if err != nil {
		return nil, err
	}

	stdout, stderr := util.Stdio(thread)
	options = append(options, interp.StdIO(nil, stdout, stderr))

	if err := run(context.Background(), file, options); err != nil {
		if try {
			return starlark.String(err.Error()), nil
		}
		return nil, err
	}
	return starlark.None, nil
}

// def output(command, cwd=None, env=None, try_=None):
//     """
//     Execute a shell command and return its output. The command must be a
//     valid POSIX Shell, Bash, or mksh command. Any commands that are not
//     shell builtins must be available on the path. If the command fails, the
//     calling module will abort unless `try_` is set to True, in which case
//     the contents of standard error will be returned.
//
//     :param command: the command to execute.
//     :param cwd: the working directory for the command. Defaults to the
//                 calling module's directory.
//     :param env: any environment variables to set when running the command.
//     :param `try_`: when True, the calling module will not be aborted if
//                  the shell command fails.
//
//     :returns: the contents of standard output if `try_` is not truthy and the
//               command succeeds. If `try_` is truthy, output returns
//               (stdout, True) if the command succeeds and (stderr, False)
//               if the command fails.
//     """
//
//starlark:builtin factory=NewOutput,function=Output
func output(thread *starlark.Thread, fn *starlark.Builtin, cmd, cwd string, env starlark.IterableMapping, try bool) (starlark.Value, error) {
	file, options, try, err := command(thread, cmd, cwd, env, try)
	if err != nil {
		return nil, err
	}

	var stdout strings.Builder
	_, stderr := util.Stdio(thread)
	if try {
		stderr = io.Discard
	}
	options = append(options, interp.StdIO(nil, &stdout, stderr))

	if err := run(context.Background(), file, options); err != nil {
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

func command(thread *starlark.Thread, command, cwd string, env starlark.IterableMapping, try bool) (*syntax.File, []interp.RunnerOption, bool, error) {
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

func run(ctx context.Context, file *syntax.File, options []interp.RunnerOption) error {
	runner, err := interp.New(options...)
	if err != nil {
		return err
	}
	return runner.Run(ctx, file)
}
