Quick Start
===========

System requirements
-------------------

dawn runs anywhere Go is supported, including (but not limited to) macOS, Linux,
and Windows.

Install dawn
------------

The simplest way to install dawn is using the installation script:

.. tabs::

   .. code-tab:: bash macOS/Linux

        curl -fsSL https://get.dawn-build.io | sh

   .. code-tab:: powershell Windows

        @"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -InputFormat None -ExecutionPolicy Bypass -Command "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; iex ((New-Object System.Net.WebClient).DownloadString('https://get.dawn-build.io/install.ps1'))" && SET "PATH=%PATH%;%USERPROFILE%\.dawn\bin"

Alternatively, dawn may be installed manually using `go install`:

.. code-block:: bash

   go install github.com/pgavlin/dawn@latest

Shell completion
----------------

dawn has built-in support for shell completions for Bash, Zsh, fish, and
PowerShell. To enable shell completions, run `dawn completions` and follow the
instructions for your shell. dawn provides completions for subcommands, flags,
and targets.

Create a new project
--------------------

During this quick start, you will create a simple Go_ application. In addition
to dawn, you will need to install Go_.

Once you have installed the prerequisites, create a new directory to hold your
project and run `dawn init`:

..tabs::

    .. code-tab:: bash macOS/Linux

        mkdir hello-dawn
        cd hello-dawn
        dawn init

    .. code-tab:: powershell Windows

        mkdir hello-dawn
        cd hello-dawn
        dawn init

Create a simple Go command-line tool
------------------------------------

Create a new Go module in the current directory:

.. code-block:: bash

   go mod init hello-dawn

Then copy the code below into a file named main.go:

.. code-block:: go

   package main

   import "fmt"

   func main() {
       fmt.Println("Hello, dawn!")
   }

Write your first build target
-----------------------------

Copy the code below into a file named BUILD.dawn:

.. code-block:: python

    @target(sources=["main.go", "go.mod"], generates=["hello-dawn"], default=True)
    def hello_dawn():
        """
        Builds the hello-dawn executable.
        """

        sh.exec("go build -o hello-dawn .")

This code defines a single target, `//:hello_dawn`, that will be run if any of\
the following conditions are true:

- the file main.go changes
- the file go.mod changes
- the file hello-dawn does not exist
- the `hello_dawn` function's environment--i.e. its code and the values of its
  referenced variables--changes

When the target is run, the body of the `hello_dawn` function will execute and
run `go build` to build the `hello-dawn` binary.

Run a build
-----------

Build the project's default target, `hello_dawn`:

.. code-block:: bash

   dawn
   [source://:go.mod] done
   [source://:main.go] done
   [//:hello_dawn] done
   [//:default] done

This will produce an executable named `hello-dawn` in the project's directory.
Running the `hello-dawn` executable will print `Hello, dawn!`:

.. code-block:: bash

   ./hello-dawn
   Hello, dawn!

Now, try building the project's default target again:

.. code-block:: bash

   dawn

This should produce no output, as nothing has changed since the last time the
target was built.

Explore your project
--------------------

dawn provides powerful tools for exploring your project. To see all of your
project's targets, run `dawn list targets`:

.. code-block:: bash

   dawn list targets
   //:default    Builds the hello-dawn executable.
   //:hello_dawn Builds the hello-dawn executable.

To see the sources and targets that the `//:hello_dawn` target depends on,
run `dawn list depends //:hello_dawn`:

.. code-block:: bash

   dawn list depends //:hello_dawn
   source://:go.mod
   source://:main.go

To launch the REPL and interactively explore your project, run `dawn repl`:


.. code-block:: bash

   dawn repl
   >>>

Inside the REPL, call `targets()` to list your project's targets:

.. code-block:: python

   >>> targets()
   [//:default, //:hello_dawn]

Then, call `run("//:hello_dawn")` to build the `//:hello_dawn` target:

.. code-block:: python

   >>> run("//:hello_dawn")
   [//:hello_dawn] done
   >>>

Exit the REPL by pressing Ctrl+D to send an EOF:

.. code-block:: python

   >>> ^D

Make a change
-------------

Change the contents of the file main.go to the following:

.. code-block:: go

   package main

   import "fmt"

   func main() {
       fmt.Println("Hello again, dawn!")
   }

Now, rebuild the project's default target:

.. code-block:: bash

   dawn
   [source://:main.go] done
   [//:hello_dawn] done
   [//:default] done

Because the contents of main.go changed, dawn re-ran the `//:hello_dawn` target
and built a new version of the `hello-dawn` executable. Try running
`hello-dawn`:

.. code-block:: bash

   ./hello-dawn
   Hello again, dawn!

Watch a file for changes
------------------------

In a new terminal, navigate to your project's directory and run `dawn watch`:

.. code-block:: bash

   dawn watch

This command will watch the project's directory and rebuild the default target
any time a file is changed, created, or deleted. Change the contents of main.go
back to the original:

.. code-block:: go

   package main

   import "fmt"

   func main() {
       fmt.Println("Hello, dawn!")
   }

When you save the file, you should see the following output from `dawn watch`:

.. code-block:: bash

   [source://:main.go] done
   [//:hello_dawn] done
   [//:default] done

Because main.go's contents changed, dawn rebuilt the default target.

Next steps
----------

Now that you've explored the basic capabilities of dawn, you can read more about
the :ref:`core concepts <Core Concepts>`, check out the :ref:`module reference <modindex>`,
and get to work authoring your builds. Have fun!

.. _Go: https://golang.org
