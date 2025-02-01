//go:generate go run ./cmd/dawn-gen-builtins . builtins.go docs/source/modules
package dawn

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/util"
	"github.com/spf13/pflag"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func CurrentModule(thread *starlark.Thread) (*label.Label, bool) {
	m, ok := thread.Local("module").(*module)
	if !ok {
		return nil, false
	}
	return m.label, true
}

// flagValue implements pflag.Value for project flags.
type flagValue struct {
	v      starlark.Value
	thread *starlark.Thread
	type_  starlark.Callable
	set    bool
}

func (a *flagValue) String() string {
	if a.v != nil {
		return a.v.String()
	}
	return ""
}

func (a *flagValue) Set(s string) error {
	v, err := starlark.Call(a.thread, a.type_, starlark.Tuple{starlark.String(s)}, nil)
	if err != nil {
		return err
	}
	a.v, a.set = v, true
	return nil
}

func (a *flagValue) Type() string {
	return a.type_.Name()
}

// def path(label):
//     """
//     Returns the absolute OS path that corresponds to the given label.
//     """
//
//starlark:builtin
func (proj *Project) builtin_path(thread *starlark.Thread, fn *starlark.Builtin, rawlabel string) (starlark.Value, error) {
	m := thread.Local("module").(*module)

	l, err := label.Parse(rawlabel)
	if err != nil {
		return nil, err
	}
	l, err = l.RelativeTo(m.label.Package)
	if err != nil {
		return nil, err
	}

	components := label.Split(l.Package)[1:]
	return starlark.String(filepath.Join(proj.root, filepath.Join(components...), l.Name)), nil
}

// def label(path):
//     """
//     Returns the label that corresponds to the given OS path, if any.
//     """
//
//starlark:builtin
func (proj *Project) builtin_label(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
	m := thread.Local("module").(*module)

	l, err := sourceLabel(m.label.Package, path)
	if err != nil {
		return nil, err
	}
	l.Kind = ""

	components := label.Split(l.Package)[1:]
	if info, err := os.Stat(filepath.Join(proj.root, filepath.Join(components...), l.Name)); err == nil && info.IsDir() {
		l.Package, l.Name = l.Package+"/"+l.Name, ""
	}

	return starlark.String(l.String()), nil
}

var parentDir = string([]rune{'.', '.', os.PathSeparator})

// def contains(path):
//     """
//     Returns the label that corresponds to the given OS path if the path is
//     contained in the current project. If the path is not contained in the
//     current project, contains returns (None, False).
//     """
//
//starlark:builtin
func (proj *Project) builtin_contains(thread *starlark.Thread, fn *starlark.Builtin, path string) (starlark.Value, error) {
	cwd := util.Getwd(thread)
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	if filepath.VolumeName(path) != filepath.VolumeName(proj.root) {
		return starlark.Tuple{starlark.None, starlark.False}, nil
	}

	rel, err := filepath.Rel(proj.root, path)
	if err != nil {
		return nil, err
	}

	if rel == ".." || strings.HasPrefix(rel, parentDir) {
		return starlark.Tuple{starlark.None, starlark.False}, nil
	}

	components := filepath.SplitList(rel)
	return starlark.Tuple{starlark.String("/" + strings.Join(components, "/")), starlark.True}, nil
}

// def parse_flag(name, default=None, type=None, choices=None, required=None, help=None):
//     """
//     Defines and parses a new project flag in the current package.
//
//     :param name: the name of the flag.
//     :param default: the default value for the flag, if any.
//     :param type: the type to which the flag's argument should be converted. Defaults to str.
//     :param choices: the valid values for the flag. Defaults to any value.
//     :param required: True if the flag must be set.
//     :param help: the help string for the flag.
//
//     :returns: the flag's value.
//     """
//
//starlark:builtin
func (proj *Project) builtin_parse_flag(
	thread *starlark.Thread,
	fn *starlark.Builtin,
	name string,
	default_ starlark.Value, //??
	type_ starlark.Callable, //??
	choices []starlark.Value, //??
	required bool, //??
	help string, //??
) (starlark.Value, error) {
	if name == "" {
		return nil, fmt.Errorf("%v: name must not be empty", fn.Name())
	}

	m := thread.Local("module").(*module)

	l, err := label.New("arg", "", m.label.Package, name)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", fn.Name(), err)
	}

	components := append(label.Split(l.Package)[1:], name)
	name = strings.Join(components, ".")

	if type_ == nil {
		type_ = starlark.Universe["str"].(starlark.Callable)
	}

	stringChoices := make([]string, len(choices))
	for i, c := range choices {
		stringChoices[i] = c.String()
	}

	stringDefault := ""
	if default_ != nil {
		stringDefault = default_.String()
	}

	flag := &Flag{
		Name:     name,
		Default:  stringDefault,
		FlagType: type_.Name(),
		Choices:  stringChoices,
		Required: required,
		Help:     help,
	}

	proj.m.Lock()
	if _, ok := proj.flags[name]; ok {
		proj.m.Unlock()
		return nil, fmt.Errorf("%v: duplicate flag %v", fn.Name(), name)
	}
	proj.flags[name] = flag
	proj.m.Unlock()

	flagValue := flagValue{v: default_, thread: thread, type_: type_}
	set := pflag.NewFlagSet(name, pflag.ContinueOnError)
	set.Var(&flagValue, name, help)
	if type_.Name() == "bool" {
		set.Lookup(name).NoOptDefVal = "False"
	}

	if err := set.Parse(proj.args); err != nil {
		return nil, fmt.Errorf("%v: %w", fn.Name(), err)
	}

	if required && !flagValue.set {
		return nil, fmt.Errorf("%v: missing required flag --%s", fn.Name(), name)
	}

	if len(choices) > 0 {
		valid := false
		for _, choice := range choices {
			if eq, _ := starlark.Equal(flagValue.v, choice); eq {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("%v: invalid value for flag --%v", fn.Name(), name)
		}
	}

	if flagValue.v == nil {
		flagValue.v = starlark.None
	}
	flag.Value = flagValue.v
	return flag.Value, nil
}

func (proj *Project) builtin_targetDecorator(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) == 1 {
		if function, decorator := args[0].(*starlark.Function); decorator {
			return proj.builtin_target(thread, fn, function.Name(), starlark.Tuple{}, nil, nil, function, false, false, "")
		}
	}

	return proj.starlark_builtin_target(thread, fn, args, kwargs)
}

// def target(name=None, deps=None, sources=None, generates=None, function=None, default=None, always=None, docs=None):
//     """
//     Defines a new build target in the current package. Typically used as a
//     decorator, in which case the decorated function is treated as the value
//     of the function parameter.
//
//     :param name: the name of the target.
//     :param deps: the target's dependencies. Must be a sequence whose elements
//                  are either labels or other build targets.
//     :param sources: the target's source files. Must be a sequence of strings.
//                     Each string will be interpreted relative to the package's
//                     directory (if the path is relative) or project root (if
//                     the path is absolute).
//     :param generates: any files generated by the targets. Must be a sequence of
//                       strings. Paths are interpreted identically to those in
//                       the sources parameter.
//     :param function: the target's callback function. If this parameter is None,
//                      target returns a decorator function rather than a target.
//     :param default: True if the target is its package's default target.
//     :param always: True if the target should always be considered out-of-date.
//     :param docs: the docs for the target. Normally picked up from the
//                  function's docstring.
//
//     :returns: the new build target object or a decorator if function is None.
//     """
//
//starlark:builtin
func (proj *Project) builtin_target(
	thread *starlark.Thread,
	fn *starlark.Builtin,
	name string,
	deps starlark.Sequence,
	sources util.StringList,
	generates util.StringList,
	function *starlark.Function,
	default_ bool,
	always bool,
	docs string,
) (starlark.Value, error) {
	// If the function is nil, treat this as a decorator. Otherwise, create a new target.
	if function == nil {
		return starlark.NewBuiltin("target", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
			var function *starlark.Function
			if err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 1, &function); err != nil {
				return nil, err
			}
			return proj.builtin_target(thread, fn, name, deps, sources, generates, function, default_, always, docs)
		}), nil
	}

	if name == "" {
		name = function.Name()
	}

	m := thread.Local("module").(*module)

	// Process deps.
	var dependencies []string
	var dep starlark.Value
	if deps != nil {
		it := deps.Iterate()
		defer it.Done()
		for it.Next(&dep) {
			var deplabel *label.Label
			switch dep := dep.(type) {
			case starlark.String:
				l, err := label.Parse(string(dep))
				if err != nil {
					return nil, err
				}
				l, err = l.RelativeTo(m.label.Package)
				if err != nil {
					return nil, err
				}
				deplabel = l
			case Target:
				deplabel = dep.Label()
			default:
				return nil, fmt.Errorf("%v: dependency is a %s, not a string or target", fn.Name(), dep.Type())
			}
			dependencies = append(dependencies, deplabel.String())
		}
	}

	var sourcePaths []string
	for _, s := range sources {
		label, err := sourceLabel(m.label.Package, s)
		if err != nil {
			return nil, err
		}
		f, err := proj.loadSourceFile(label)
		if err != nil {
			return nil, err
		}
		sourcePaths, dependencies = append(sourcePaths, f.path), append(dependencies, label.String())
	}

	// Process gens.
	var gens []string
	for _, g := range generates {
		path, err := repoSourcePath(m.label.Package, g)
		if err != nil {
			return nil, err
		}
		components := strings.Split(path, "/")
		path = filepath.Join(proj.root, filepath.Join(components...))
		gens = append(gens, path)
	}

	l := &label.Label{
		Package: m.label.Package,
		Name:    name,
	}
	f, err := proj.loadFunction(m, l, dependencies, sourcePaths, gens, function, always, docs)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", fn.Name(), err)
	}

	if default_ {
		defaultLabel := &label.Label{
			Package: m.label.Package,
			Name:    "default",
		}
		if _, err = proj.loadFunction(m, defaultLabel, []string{l.String()}, nil, nil, builtin_default(function.Doc()), false, ""); err != nil {
			return nil, err
		}
	}

	return f, nil
}

// def glob(include, exclude=None):
//     """
//     Return a list of paths relative to the calling module's directory that match
//     the given include and exclude patterns. Typically passed to the sources parameter
//     of target.
//
//     - `*` matches any number of non-path-separator characters
//     - `**` matches any number of any characters
//     - `?` matches a single character
//
//     :param include: the patterns to include.
//     :param exclude: the patterns to exclude.
//
//     :returns: the matched paths
//     """
//
//starlark:builtin
func (proj *Project) builtin_glob(
	thread *starlark.Thread,
	fn *starlark.Builtin,
	include util.StringList,
	exclude util.StringList,
) (starlark.Value, error) {
	includeRE, err := util.CompileGlobs([]string(include))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}

	excludeRE, err := util.CompileGlobs([]string(exclude))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", fn.Name(), err)
	}

	m := thread.Local("module").(*module)
	dir := filepath.Dir(m.path)

	sources := starlark.NewList(nil)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		path = path[len(dir):]
		if len(path) == 0 {
			return nil
		}
		path = filepath.ToSlash(path)

		if path == "/.dawn/build" {
			return fs.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		path = path[1:]
		if includeRE.MatchString(path) && !excludeRE.MatchString(path) {
			sources.Append(starlark.String(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sources, nil
}

// def run(label_or_target, always=None, dry_run=None, callback=None):
//     """
//     Builds a target.
//
//     :param label_or_target: the label or target to run.
//     :param always: True if all targets should be considered out-of-date.
//     :param dry_run: True if the targets to run should be displayed but not run.
//     :param callback: a callback that receives build events. If absent,
//                      events will be displayed using the default renderer.
//     """
//
//starlark:builtin
func (proj *Project) builtin_run(
	thread *starlark.Thread,
	fn *starlark.Builtin,
	labelOrTarget starlark.Value,
	always bool,
	dryRun bool,
	callback starlark.Callable,
) (_ starlark.Value, err error) {
	m := thread.Local("module").(*module)

	var l *label.Label
	switch labelOrTarget := labelOrTarget.(type) {
	case starlark.String:
		l, err = label.Parse(string(labelOrTarget))
		if err != nil {
			return nil, err
		}
		l, err = l.RelativeTo(m.label.Package)
		if err != nil {
			return nil, err
		}
	case Target:
		l = labelOrTarget.Label()
	default:
		return nil, fmt.Errorf("%v: label_or_target must be a string or a target", fn.Name())
	}

	if callback != nil {
		events := &runEvents{
			c:        make(chan starlark.Value),
			callback: callback,
			done:     make(chan bool),
		}
		go events.process(thread)
		defer events.Close()

		currentEvents := proj.events
		proj.events = events
		defer func() {
			proj.events = currentEvents
		}()
	}

	options := RunOptions{
		Always: always,
		DryRun: dryRun,
	}
	return starlark.None, proj.Run(l, &options)
}

// def get_target(label):
//     """
//     Gets the target with the given label, if it exists.
//
//     :param: label: the target's label.
//
//     :returns: the target with the given label.
//     """
//
//starlark:builtin
func (proj *Project) builtin_get_target(
	thread *starlark.Thread,
	fn *starlark.Builtin,
	rawlabel string,
) (starlark.Value, error) {
	m := thread.Local("module").(*module)

	label, err := label.Parse(rawlabel)
	if err != nil {
		return nil, err
	}
	label, err = label.RelativeTo(m.label.Package)
	if err != nil {
		return nil, err
	}

	t, err := proj.Target(label)
	if err != nil {
		return nil, fmt.Errorf("%v: %w", fn.Name(), err)
	}
	return t, nil
}

const flagsDoc = `
	Lists the project's flags.
`

// def flags():
//     """
//     Lists the project's flags.
//     """
//
//starlark:builtin
func (proj *Project) builtin_flags(thread *starlark.Thread, fn *starlark.Builtin) (starlark.Value, error) {
	proj.m.Lock()
	defer proj.m.Unlock()

	var flags []starlark.Value
	for _, flag := range proj.flags {
		flags = append(flags, flag)
	}
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].(*Flag).Name < flags[j].(*Flag).Name
	})
	return starlark.NewList(flags), nil
}

// def targets():
//     """
//     Lists the project's targets.
//     """
//
//starlark:builtin
func (proj *Project) builtin_targets(thread *starlark.Thread, fn *starlark.Builtin) (starlark.Value, error) {
	proj.m.Lock()
	defer proj.m.Unlock()

	var targets []starlark.Value
	for _, target := range proj.targets {
		label := target.target.Label()
		if IsTarget(label) {
			targets = append(targets, target.target)
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].(Target).Label().String() < targets[j].(Target).Label().String()
	})
	return starlark.NewList(targets), nil
}

// def sources():
//     """
//     Lists the project's sources.
//     """
//
//starlark:builtin
func (proj *Project) builtin_sources(thread *starlark.Thread, fn *starlark.Builtin) (starlark.Value, error) {
	paths := proj.Sources()
	return util.StringList(paths).List(), nil
}

func builtin_default(doc string) *starlark.Builtin {
	return starlark.NewBuiltin("pass", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := starlark.UnpackPositionalArgs(fn.Name(), args, kwargs, 0); err != nil {
			return nil, err
		}
		return starlark.None, nil
	}).WithDoc(doc)
}

type failError string

func (err failError) Error() string {
	return string(err)
}

// def fail(message):
//     """
//     Fails the calling target with the given message.
//     """
//
//starlark:builtin
func (proj *Project) builtin_fail(thread *starlark.Thread, fn *starlark.Builtin, message string) (starlark.Value, error) {
	return starlark.None, failError(message)
}

var builtin_host = starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
	"arch": starlark.String(runtime.GOARCH),
	"os":   starlark.String(runtime.GOOS),
})
