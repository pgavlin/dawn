:code:`sh`
=================


    The sh module provides functions for executing POSIX Shell, Bash, and
    mksh commands. The implementation uses the `mvdan.cc/sh`_ interpreter
    instead of relying on the system shell, and therefore provides a
    consistent experience across all platforms (including Windows).

    .. _mvdan.cc/sh: https://github.com/mvdan/sh
    

.. py:module:: sh




.. py:function:: exec(command, cwd=None, env=None, try_=None)

    Execute a shell command. The command must be a valid POSIX Shell, Bash,
    or mksh command. Any commands that are not shell builtins must be
    available on the path. If the command fails, the calling module will
    abort unless `try_` is set to True, in which case the contents of
    standard error will be returned.

    :param command: the command to execute.
    :param cwd: the working directory for the command. Defaults to the
                calling module's directory.
    :param env: any environment variables to set when running the command.
    :param `try_`: when True, the calling module will not be aborted if
                 the shell command fails.

    :returns: the contents of standard error if `try_` is set and None
              otherwise. To capture the command's output, use output.
    

.. py:function:: output(command, cwd=None, env=None, try_=None)

    Execute a shell command and return its output. The command must be a
    valid POSIX Shell, Bash, or mksh command. Any commands that are not
    shell builtins must be available on the path. If the command fails, the
    calling module will abort unless `try_` is set to True, in which case
    the contents of standard error will be returned.

    :param command: the command to execute.
    :param cwd: the working directory for the command. Defaults to the
                calling module's directory.
    :param env: any environment variables to set when running the command.
    :param `try_`: when True, the calling module will not be aborted if
                 the shell command fails.

    :returns: the contents of standard output if `try_` is not truthy and the
              command succeeds. If `try_` is truthy, output returns
              (stdout, True) if the command succeeds and (stderr, False)
              if the command fails.
    


