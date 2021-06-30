package os

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pgavlin/dawn/util"
	"go.starlark.net/starlark"
)

// def exec(command, cwd=None, env=None, try_=None):
//     """
//     Run an executable. If the process fails, the calling module will
//     abort unless `try_` is set to True, in which case the contents of
//     standard error will be returned.
//
//     :param command: a list of strings indicating the executable to run
//                     and its arguments (e.g. `["dawn", "build"]`).
//     :param cwd: the working directory for the command. Defaults to the
//                 calling module's directory.
//     :param env: any environment variables to set when running the command.
//     :param `try_`: when True, the calling module will not be aborted if
//                  the process fails.
//
//     :returns: the contents of standard error if `try_` is set and None
//               otherwise. To capture the process's output, use output.
//     """
//
//starlark:builtin factory=NewExec,function=Exec
func execf(
	thread *starlark.Thread,
	fn *starlark.Builtin,
	cmdV util.StringList,
	cwd string,
	envV starlark.IterableMapping,
	try bool,
) (starlark.Value, error) {
	cmd, err := command(thread, fn, cmdV, cwd, envV)
	if err != nil {
		return nil, err
	}
	cmd.Stdout, cmd.Stderr = util.Stdio(thread)

	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("%v: %w", fn.Name(), err)
	}
	return starlark.None, nil
}

// def output(command, cwd=None, env=None, try_=None):
//     """
//     Run an executable and return its output. If the process fails, the
//     calling module will abort unless `try_` is set to True, in which case
//     the contents of standard error will be returned.
//
//     :param command: a list of strings indicating the executable to run
//                     and its arguments (e.g. `["dawn", "build"]`).
//     :param cwd: the working directory for the command. Defaults to the
//                 calling module's directory.
//     :param env: any environment variables to set when running the command.
//     :param `try_`: when True, the calling module will not be aborted if
//                  the process fails.
//
//     :returns: the contents of standard output if `try_` is not truthy and the
//               process succeeds. If `try_` is truthy, output returns
//               (stdout, True) if the process succeeds and (stderr, False)
//               if the process fails.
//     """
//
//starlark:builtin factory=NewOutput,function=Output
func output(
	thread *starlark.Thread,
	fn *starlark.Builtin,
	cmdV util.StringList,
	cwd string,
	envV starlark.IterableMapping,
	try bool,
) (starlark.Value, error) {
	cmd, err := command(thread, fn, cmdV, cwd, envV)
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

func command(
	thread *starlark.Thread,
	fn *starlark.Builtin,
	command util.StringList,
	cwd string,
	envV starlark.IterableMapping,
) (*exec.Cmd, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("%v: command must have at least one element", fn.Name())
	}

	if cwd == "" {
		cwd = util.Getwd(thread)
	}

	env := os.Environ()
	if envV != nil {
		items := envV.Items()

		pairs := make([]string, 0, len(env)+len(items))
		pairs = append(pairs, env...)
		for _, kvp := range items {
			key, ok := starlark.AsString(kvp[0])
			if !ok {
				key = kvp[0].String()
			}
			value, ok := starlark.AsString(kvp[1])
			if !ok {
				value = kvp[1].String()
			}

			pairs = append(pairs, fmt.Sprintf("%v=%v", key, value))
		}
	}

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = cwd
	cmd.Env = env

	return cmd, nil
}
