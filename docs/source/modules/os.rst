:code:`os`
=================


   Provides a platform-independent interface to host operating system
   functionality. Functions in this package expect and return host paths.
   

.. py:module:: os




.. py:function:: exec(command, cwd=None, env=None, try_=None)

   Run an executable. If the process fails, the calling module will
   abort unless `try_` is set to True, in which case the contents of
   standard error will be returned.

   :param command: a list of strings indicating the executable to run
                   and its arguments (e.g. `["dawn", "build"]`).
   :param cwd: the working directory for the command. Defaults to the
               calling module's directory.
   :param env: any environment variables to set when running the command.
   :param `try_`: when True, the calling module will not be aborted if
                the process fails.

   :returns: the contents of standard error if `try_` is set and None
             otherwise. To capture the process's output, use output.
   

.. py:function:: exec(command, cwd=None, env=None, try_=None)

   Run an executable and return its output. If the process fails, the
   calling module will abort unless `try_` is set to True, in which case
   the contents of standard error will be returned.

   :param command: a list of strings indicating the executable to run
                   and its arguments (e.g. `["dawn", "build"]`).
   :param cwd: the working directory for the command. Defaults to the
               calling module's directory.
   :param env: any environment variables to set when running the command.
   :param `try_`: when True, the calling module will not be aborted if
                the process fails.

   :returns: the contents of standard output if `try_` is not truthy and the
             process succeeds. If `try_` is truthy, output returns
             (stdout, True) if the process succeeds and (stderr, False)
             if the process fails.
   

.. py:function:: exists(path)

   Returns true if a file exists at the given path.
   

.. py:function:: getcwd()

   Returns the current OS working directory. This is typically the path of
   the directory containg the root module on the callstack.
   

.. py:function:: glob(include, exclude=None)

   Return a list of paths rooted in the current directory that match the
   given include and exclude patterns.

   - `*` matches any number of non-path-separator characters
   - `**` matches any number of any characters
   - `?` matches a single character

   :param include: the patterns to include.
   :param exclude: the patterns to exclude.

   :returns: the matched paths
   


