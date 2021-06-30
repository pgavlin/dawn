Core Concepts
=============

The Starlark language
---------------------

dawn uses an enhanced version of the `Starlark language`_ to describe builds.
Starlark was originally developed for the `Bazel`_ build system, and as such has
a number of features that are particularly valuable in this domain:

- **Deterministic evaluation.** Executing the same code twice will give the same
  results.
- **Parallel evaluation.** Modules can be loaded in parallel. To guarantee
  thread-safe execution, shared data is logically immutable.
- **Python-like.** Python is a widely used language. Keeping the language similar
  to Python can reduce the learning curve and make the semantics more obvious to
  users.

In addition to the features described by the `Starlark specification`_, dawn
adds support for `function decorators`_ and `interpolated strings`_. The former
are the primary method for denoting functions that represent build targets. The
latter provide a simpler, more readable way of constructing strings, which is
particularly useful for constructing shell commands.

In addition to the core language, dawn provides a number of useful builtin
modules to aid in authoring :ref:`build targets <Targets>`. These modules
are documented in the :ref:`module index <modindex>`.

Labels
------

Each dawn :ref:`module <Modules>`, :ref:`target <Targets>`, and :ref:`source <Sources>`
is assigned a *label*. The label is derived from the kind of the entity being
labeled, the :ref:`package <Packages>` that contains the entity, and the
name of the entity. dawn's labels are similar to those used by `Bazel`_.

The general syntax for a label is:

`[kind:][[module[+version]@][//]package][:name]`

Spefic examples of labels are:

- `modules://:BUILD.dawn`, which refers to the :ref:`module <Modules>` named
  `BUILD.dawn` in the root :ref:`package <Packages>` of the containing
  :ref:`project <Projects>`
- `modules://.dawn:go_sources.dawn`, which refers to the :ref:`module <Modules>`
  named `go_sources.dawn` in the `.dawn` :ref:`package <Packages>` of the
  containing :ref:`project <Projects>`
- `target://:dawn`, which refers to the :ref:`build target <Targets>` named
  `dawn` in the root :ref:`package <Packages>` of the containing
  :ref:`project <Projects>`
- `target://docs:site`, which refers to the :ref:`build target <Targets>` named
  `site` in the `docs` :ref:`package <Packages>` of the containing
  :ref:`project <Projects>`
- `source://:project.go`, which refers to the :ref:`source file <Sources>` named
  `project.go` in the root :ref:`package <Packages>` of the containing
  :ref:`project <Projects>`
- `source://lib/sh:exec.go`, which refers to the :ref:`source file <Sources>`
  named `exec.go` inside the `lib/sh` :ref:`package <Packages>`  of the
  containing :ref:`project <Projects>`

Under most circumstances, the *kind* portion of a label will be detected from
the context in which the label is used and may be omitted. For example, the
`module:` kind need not be specified inside of a `load statement`_,
and the `target:` kind need not be specified when invoking the CLI to build a
target or specifying a target's dependencies.

Projects
--------

A dawn *project* is a tree of :ref:`packages <Packages>`, each of which is
composed of :ref:`modules <Modules>` that define :ref:`build targets <Targets>`.
and/or implement shared utility functions. The root of a project is demarcated
by a `.dawnconfig` file. When a project is loaded, each of its constituent
:ref:`packages <Packages>` is loaded in parallel.

Packages
^^^^^^^^

A dawn *package* is a directory inside of a :ref:`project <Projects>` that
contains a `BUILD.dawn` file. The `BUILD.dawn` file serves as the package's root
:ref:`module <Modules>`. If any of a package's :ref:`targets <Targets>` are
defined in other :ref:`modules <Modules>`, its `BUILD.dawn` file must explicitly
load those :ref:`modules <Modules>` using appropriate `load statements`_.

Modules
^^^^^^^

A dawn *module* is a file that contains :ref:`Starlark <The Starlark language>`
code that defines dawn :ref:`build targets <Targets>` and/or exports shared
functionality. Modules are loaded in parallel, and circular dependencies between
modules are not allowed. Modules are referenced by their :ref:`label <Labels>`,
e.g. `//path/to/package:module_file` or `module+version@//path/to/package:module_file`
in the case of an :ref:`external module <External Modules>`.

External Modules
""""""""""""""""

.. note:: Although the dawn CLI supports external modules, there is not yet a module server. Until such a server exists, this section is purely informational.

A dawn module may be *external*, meaning that it is hosted externally and must
be fetched as part of the module loading process. External modules are intended
to provide abstractions and shared functionality (e.g. utility functions to
define :ref:`build targets <Targets>` for a particular language ecosystem), and
should not themselves define :ref:`build targets <Targets>`.

Targets
-------

A dawn *target* is the core unit of work in a :ref:`project <Projects>`. Each
target is represented by a :ref:`Starlark <The Starlark language>` function
that has been annotated with the :py:func:`globals.target` decorator:

.. code-block:: python

    @target(sources=["foo"], deps=[":bar"], generates=["baz"])
    def my_target():
        sh.exec("cat foo >baz")

The set of targets and :ref:`sources <Sources>` that a target depends on form
the target's *inputs*, and the set of files it generates (if any) form its
*outputs*. If a target *T* generates a file that is a :ref:`source <Sources>`
for another target, *T* is automatically added to the file's set of
dependencies. It is an error for multiple targets to generate the same file. If
a target does not generate any files--or if the set of files it generates is not
known at the time the target is defined--it must be explicitly named by its
dependents.

When a target is built, its body is only executed if its dependencies are
have changed with respect to the last time the target was successfully run. A
target is considered to have changed each time it successfully runs. A
dependency on a :ref:`source file <Sources>` is only considered to have changed
if the file's contents have changed since the last time the target was
successfully run. A target's dependencies are always built before the target
itself, and it is an error for targets to have cyclic dependencies.

Sources
^^^^^^^

Each :ref:`target <Targets>` in a dawn :ref:`project <Projects>` may depend on
zero or more *sources*. A *source* refers to a file on disk that is contained
within a :ref:`project <Projects>`. A source is considered to have changed if
and only if its contents differ from the last time the source was used by a
:ref:`target <Targets>`. If a source is present in a :ref:`target <Targets>`'s
list of generated files, that :ref:`target <Targets>` is automatically added
as a dependency of the source. It is an error for multiple :ref:`targets <Targets>`
to generate the same source.

.. _Starlark language: https://github.com/bazelbuild/starlark
.. _Bazel: https://bazel.build
.. _Starlark specification: https://github.com/bazelbuild/starlark/blob/master/spec.md
.. _function decorators: https://www.python.org/dev/peps/pep-0318
.. _interpolated strings: https://www.python.org/dev/peps/pep-0498
.. _load statement: https://github.com/bazelbuild/starlark/blob/master/spec.md#load-statements
.. _load statements: https://github.com/bazelbuild/starlark/blob/master/spec.md#load-statements
