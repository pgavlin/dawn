:code:`os`
=================


   Provides a platform-independent interface to host operating system
   functionality. Functions in this package expect and return host paths.
   

.. py:module:: os



:py:mod:`path`

       The path module provides functions to manipulate host paths.
       




.. py:function:: environ()

   Returns a mapping object where keys and values are strings that represent
   the process environment. This mapping is captured at startup time.
   

.. py:function:: look_path(file)

   Search for an executable named file in the directories named by
   the PATH environment variable. If file contains a slash, it is
   tried directly and the PATH is not consulted. Otherwise, on
   success, the result is an absolute path.

   :param file: the name of the executable to find

   :returns: the absolute path to file if found or None if not found.
   

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
   

.. py:function:: output(command, cwd=None, env=None, try_=None)

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
   

.. py:function:: mkdir(path, mode=None)

   Create a directory named path with numeric mode mode.
   

.. py:function:: makedirs(path, mode=None)

   Recursive directory creation function. Like mkdir(), but makes all
   intermediate-level directories needed to contain the leaf directory.
   


