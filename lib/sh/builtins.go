// Code generated by dawn-gen-builtins; DO NOT EDIT.

package sh


import (
	
	starlark "go.starlark.net/starlark"
	
)



func NewExec() *starlark.Builtin {
	const doc = `
   Execute a shell command. The command must be a valid POSIX Shell, Bash,
   or mksh command. Any commands that are not shell builtins must be
   available on the path. If the command fails, the calling module will
   abort unless `+"`"+`try_`+"`"+` is set to True, in which case the contents of
   standard error will be returned.

   :param command: the command to execute.
   :param cwd: the working directory for the command. Defaults to the
               calling module's directory.
   :param env: any environment variables to set when running the command.
   :param `+"`"+`try_`+"`"+`: when True, the calling module will not be aborted if
                the shell command fails.

   :returns: the contents of standard error if `+"`"+`try_`+"`"+` is set and None
             otherwise. To capture the command's output, use output.
   `
	return starlark.NewBuiltin("exec", Exec).WithDoc(doc)
}

func Exec(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		cmd string
		
		cwd string
		
		env starlark.IterableMapping
		
		try bool
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "command", &cmd, "cwd??", &cwd, "env??", &env, "try_??", &try); err != nil {
		return nil, err
	}
	
	return exec(thread, fn, cmd, cwd, env, try)
}

func NewOutput() *starlark.Builtin {
	const doc = `
   Execute a shell command and capture its output. The command must be a
   valid POSIX Shell, Bash, or mksh command. Any commands that are not
   shell builtins must be available on the path. If the command fails, the
   calling module will abort unless `+"`"+`try_`+"`"+` is set to True, in which case
   the contents of standard error will be returned.

   :param command: the command to execute.
   :param cwd: the working directory for the command. Defaults to the
               calling module's directory.
   :param env: any environment variables to set when running the command.
   :param `+"`"+`try_`+"`"+`: when True, the calling module will not be aborted if
                the shell command fails.

   :returns: the contents of standard output if `+"`"+`try_`+"`"+` is not truthy and the
             command succeeds. If `+"`"+`try_`+"`"+` is truthy, output returns
             (stdout, True) if the command succeeds and (stderr, False)
             if the command fails.
   `
	return starlark.NewBuiltin("output", Output).WithDoc(doc)
}

func Output(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	
	var (
		
		cmd string
		
		cwd string
		
		env starlark.IterableMapping
		
		try bool
		
	)
	if err := starlark.UnpackArgs(fn.Name(), args, kwargs, "command", &cmd, "cwd??", &cwd, "env??", &env, "try_??", &try); err != nil {
		return nil, err
	}
	
	return output(thread, fn, cmd, cwd, env, try)
}
