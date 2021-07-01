##################################
☀️ dawn: Pragmatic Polyglot Builds
##################################

.. meta::
   :description lang=en: Build multi-language software projects without sacrificing productivity.

.. image:: https://readthedocs.org/projects/dawn-build/badge/?version=latest&style=flat
   :target: https://dawn-build.io
   :alt: Read the Docs
.. image:: https://pkg.go.dev/badge/github.com/pgavlin/dawn
   :target: https://pkg.go.dev/github.com/pgavlin/dawn
   :alt: pkg.go.dev
.. image:: https://codecov.io/gh/pgavlin/dawn/branch/master/graph/badge.svg
   :target: https://codecov.io/gh/pgavlin/dawn
   :alt: Code Coverage
.. image:: https://github.com/pgavlin/dawn/workflows/Test/badge.svg
   :target: https://github.com/pgavlin/dawn/actions?query=workflow%3ATest
   :alt: Build Status

dawn_ helps you modernize your build without leaving your existing ecosystems behind.
The IDE integrations, refactoring tools, etc. that you depend on will continue to work,
with many of the benefits of a modern build system:

- Content-based checks for out-of-date targets
- Parallel builds by default
- Build files written in Starlark, a dialect of Python
- Watch mode to automatically rebuild when files change
- Performance profiling for builds
- Cross-platform tooling
- ...and more

Installation
============

The simplest way to install dawn_ is using the installation script:

.. tabs::

   .. code-tab:: bash macOS/Linux

        curl -fsSL https://get.dawn-build.io | sh

   .. code-tab:: powershell Windows

        @"%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -InputFormat None -ExecutionPolicy Bypass -Command "[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; iex ((New-Object System.Net.WebClient).DownloadString('https://get.dawn-build.io/install.ps1'))" && SET "PATH=%PATH%;%USERPROFILE%\.dawn\bin"

Alternatively, dawn_ may be installed manually using `go install`:

.. code-block:: bash

   go install github.com/pgavlin/dawn@latest

.. _dawn: https://dawn-build.io
