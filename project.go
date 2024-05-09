package dawn

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pgavlin/dawn/internal/spell"
	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/runner"
	"github.com/rjeczalik/notify"
	"go.starlark.net/starlark"
)

// An UnknownTargetError is returned by Project.LoadTarget if a referenced target does not exist.
type UnknownTargetError string

func (e UnknownTargetError) Error() string {
	return string(e)
}

// A Project is the runtime representation of a dawn project. It is the primary type used to
// introspect and execute builds.
type Project struct {
	m sync.Mutex

	args   []string
	events Events

	root string
	work string
	temp string

	ignore *regexp.Regexp

	moduleProxy string
	moduleCache string

	builtins starlark.StringDict

	always bool
	dryrun bool

	flags   map[string]*Flag
	modules map[string]*module
	targets map[string]*runTarget
}

type LoadOptions struct {
	Args   []string
	Events Events

	Builtins starlark.StringDict

	PreferIndex bool
}

func (options *LoadOptions) apply(p *Project, preferIndex *bool) {
	if options != nil {
		p.args = options.Args
		p.builtins = options.Builtins
		p.events = options.Events
		*preferIndex = options.PreferIndex
	}
	if p.events == nil {
		p.events = DiscardEvents
	}
}

func Load(root string, options *LoadOptions) (proj *Project, err error) {
	home, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}
	moduleProxy := "file://" + filepath.ToSlash(home) + "/.dawn/modules/proxy"
	moduleCache := filepath.Join(home, ".dawn", "modules", "cache")

	proj = &Project{
		root:        root,
		work:        filepath.Join(root, ".dawn", "build"),
		temp:        filepath.Join(root, ".dawn", "build", "temp"),
		moduleProxy: moduleProxy,
		moduleCache: moduleCache,
		flags:       map[string]*Flag{},
		modules:     map[string]*module{},
		targets:     map[string]*runTarget{},
	}
	preferIndex := false
	options.apply(proj, &preferIndex)

	if err := proj.loadConfig(); err != nil {
		return nil, err
	}

	if err := proj.load(preferIndex); err != nil {
		return nil, err
	}
	return proj, nil
}

func (proj *Project) load(index bool) (err error) {
	defer func() {
		proj.events.LoadDone(err)
	}()

	if index {
		if err := proj.loadIndex(); err == nil {
			return nil
		}
	}

	if err := os.MkdirAll(proj.temp, 0755); err != nil {
		return err
	}

	if err = proj.loadPackage(nil, "//"); err != nil {
		return err
	}
	for _, m := range proj.modules {
		if m.err != nil {
			return m.err
		}
	}

	if err := proj.link(); err != nil {
		return err
	}

	proj.saveIndex()
	return nil
}

func (proj *Project) Reload() (err error) {
	proj.flags = map[string]*Flag{}
	proj.modules = map[string]*module{}
	proj.targets = map[string]*runTarget{}
	return proj.load(false)
}

type RunOptions struct {
	Always bool
	DryRun bool
}

func (opts *RunOptions) apply(proj *Project) {
	if opts == nil {
		proj.always = false
		proj.dryrun = false
		return
	}

	proj.always = opts.Always
	proj.dryrun = opts.DryRun
}

func (proj *Project) Run(label *label.Label, options *RunOptions) error {
	options.apply(proj)

	err := runner.Run(proj, label.String())
	proj.events.RunDone(err)
	return err
}

// LoadTarget implements runner.Host.
func (proj *Project) LoadTarget(rawlabel string) (runner.Target, error) {
	l, err := label.Parse(rawlabel)
	if err != nil {
		return nil, err
	}

	proj.m.Lock()
	defer proj.m.Unlock()

	target, ok := proj.targets[l.String()]
	if !ok {
		return nil, proj.unknownTarget(l.String())
	}
	return target, nil
}

func (proj *Project) Watch(label *label.Label) error {
	events := make(chan notify.EventInfo, 1000)
	eventsDone := make(chan struct{})
	go func() {
		builds := make(chan struct{})
		buildsDone := make(chan struct{})

		go func() {
			for range builds {
				if err := proj.Reload(); err != nil {
					// Project's load events are responsible for logging the error.
					continue
				}

				// Project's run events are responsible for logging the error.
				proj.Run(label, nil)
			}
			close(buildsDone)
		}()

		dirty := false
		rate := time.NewTicker(500 * time.Millisecond)
		for {
			select {
			case event, ok := <-events:
				if !ok {
					close(builds)
					<-buildsDone

					close(eventsDone)
					return
				}

				rel, err := filepath.Rel(proj.root, event.Path())
				if err != nil {
					continue
				}

				if !strings.HasPrefix(event.Path(), proj.work) && !proj.ignored(rel) {
					dirty = true

					label, err := sourceLabel("//", rel)
					if err != nil {
						continue
					}
					proj.events.FileChanged(label)
				}

			case <-rate.C:
				if dirty {
					select {
					case builds <- struct{}{}:
						dirty = false
					default:
						// Loop around
					}
				}
			}
		}
	}()

	if err := notify.Watch(filepath.Join(proj.root, "..."), events, notify.All); err != nil {
		close(events)
		<-eventsDone
		return err
	}

	// Never terminates
	<-eventsDone

	return nil
}

func (proj *Project) REPLEnv(stdout io.Writer, pkg *label.Label) (thread *starlark.Thread, globals starlark.StringDict) {
	m := &module{label: &label.Label{Kind: "module", Package: pkg.Package, Name: "<stdin>"}}
	thread, globals, _ = m.env(proj)
	thread.Print = func(_ *starlark.Thread, msg string) {
		fmt.Fprintln(stdout, msg)
	}
	globals["get_target"] = proj.newBuiltin_get_target()
	globals["flags"] = proj.newBuiltin_flags()
	globals["targets"] = proj.newBuiltin_targets()
	globals["sources"] = proj.newBuiltin_sources()
	globals["run"] = proj.newBuiltin_run()
	return thread, globals
}

func (proj *Project) GC() error {
	// collect all of the info paths referenced by this project
	paths := map[string]struct{}{}

	markPath := func(p string) {
		for {
			paths[p] = struct{}{}
			dir := filepath.Dir(p)
			if dir == proj.root || dir == p {
				break
			}
			p = dir
		}
	}

	markPath(filepath.Join(proj.work, "index.json"))
	for _, t := range proj.targets {
		markPath(proj.targetInfoPath(t.target.Label()))
	}

	return filepath.WalkDir(proj.work, func(path string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return fs.SkipDir
		}
		if err != nil {
			return err
		}

		if _, ok := paths[path]; !ok {
			err := os.RemoveAll(path)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	})
}

func (proj *Project) Flag(name string) (*Flag, error) {
	proj.m.Lock()
	defer proj.m.Unlock()

	f, ok := proj.flags[name]
	if !ok {
		return nil, fmt.Errorf("unknown flag %v", name)
	}
	return f, nil
}

func (proj *Project) Flags() []*Flag {
	proj.m.Lock()
	defer proj.m.Unlock()

	flags := make([]*Flag, 0, len(proj.flags))
	for _, flag := range proj.flags {
		flags = append(flags, flag)
	}
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
	return flags
}

func (proj *Project) Target(label *label.Label) (Target, error) {
	proj.m.Lock()
	defer proj.m.Unlock()

	t, ok := proj.targets[label.String()]
	if !ok {
		return nil, proj.unknownTarget(label.String())
	}
	return t.target, nil
}

func (proj *Project) Targets() []Target {
	proj.m.Lock()
	defer proj.m.Unlock()

	var targets []Target
	for _, target := range proj.targets {
		if IsTarget(target.target.Label()) || len(target.target.dependencies()) != 0 {
			targets = append(targets, target.target)
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Label().String() < targets[j].Label().String()
	})
	return targets
}

func (proj *Project) Sources() []string {
	proj.m.Lock()
	defer proj.m.Unlock()

	var paths []string
	for _, target := range proj.targets {
		label := target.target.Label()
		if IsSource(label) {
			paths = append(paths, target.target.(*sourceFile).path)
		}
	}
	sort.Strings(paths)
	return paths
}

// NOTE: proj.m must be held!
func (proj *Project) unknownTarget(label string) error {
	var targets []string
	for _, target := range proj.targets {
		l := target.target.Label()
		if IsTarget(l) {
			targets = append(targets, l.String())
		}
	}
	if len(targets) == 0 {
		return UnknownTargetError(fmt.Sprintf("unknown target %v", label))
	}

	nearest := spell.Nearest(label, targets)
	if nearest != "" {
		return UnknownTargetError(fmt.Sprintf("unknown target %v; did you mean %v?", label, nearest))
	}
	return UnknownTargetError(fmt.Sprintf("unknown target %v", label))
}

type targetInfo struct {
	Doc          string            `json:"doc,omitempty"`
	Dependencies map[string]string `json:"dependencies,omitempty"`
	Data         string            `json:"stamp,omitempty"`
	Rerun        bool              `json:"rerun,omitempty"`
}

func (proj *Project) targetInfoPath(l *label.Label) string {
	kind := l.Kind
	if kind == "" {
		kind = "target"
	}
	target := l.Name
	if target == "" {
		target = "BUILD.dawn"
	}

	targetPath := url.PathEscape(l.Package[2:] + "/" + target)
	return filepath.Join(proj.work, kind+"s", targetPath)
}

func (proj *Project) loadTargetInfo(label *label.Label) (targetInfo, error) {
	path := proj.targetInfoPath(label)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return targetInfo{}, nil
		}
		return targetInfo{}, err
	}
	defer f.Close()

	var info targetInfo
	if err := json.NewDecoder(f).Decode(&info); err != nil {
		return targetInfo{}, err
	}
	return info, nil
}

func (proj *Project) saveTargetInfo(label *label.Label, info targetInfo) error {
	path := proj.targetInfoPath(label)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	f, err := os.CreateTemp(proj.temp, "")
	if err != nil {
		return err
	}
	tempName := f.Name()

	if err = json.NewEncoder(f).Encode(info); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}

	return os.Rename(tempName, path)
}

func (proj *Project) ignored(path string) bool {
	return proj.ignore != nil && proj.ignore.MatchString(path)
}

func (proj *Project) loadPackage(wg *sync.WaitGroup, path string) error {
	if proj.ignored(path[2:]) {
		return nil
	}

	if wg == nil {
		wg = &sync.WaitGroup{}
		defer wg.Wait()
	}

	dir := filepath.Join(proj.root, path[2:])

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		switch {
		case e.IsDir():
			if e.Name() != ".dawn" {
				pkg, _ := label.Join(path, e.Name())
				if err := proj.loadPackage(wg, pkg); err != nil {
					return err
				}
			}
		case e.Name() == "BUILD.dawn":
			wg.Add(1)
			go func() {
				proj.loadModule(nil, &label.Label{Kind: "module", Package: path, Name: "BUILD.dawn"})
				wg.Done()
			}()
		}
	}

	return nil
}

func (proj *Project) loadModule(waiter *module, label *label.Label) (starlark.StringDict, error) {
	proj.m.Lock()
	if m, ok := proj.modules[label.String()]; ok {
		proj.m.Unlock()

		if waiter != nil {
			waiter.setLoading(m)
			defer waiter.setLoading(nil)
		}

		return m.wait(waiter)
	}

	m := &module{label: label, out: newLineWriter(label, proj.events)}
	m.cond = sync.NewCond(&m.m)
	proj.modules[label.String()] = m
	proj.m.Unlock()

	if waiter != nil {
		waiter.setLoading(m)
		defer waiter.setLoading(nil)
	}

	return m.load(proj)
}

func (proj *Project) loadFunction(m *module, l *label.Label, dependencies, sources, generates []string, fn starlark.Callable, always bool, docs string) (*function, error) {
	if docs == "" {
		if hasdoc, ok := fn.(starlark.HasDoc); ok {
			docs = hasdoc.Doc()
		}
	}

	rawlabel := l.String()
	proj.m.Lock()
	if _, ok := proj.targets[rawlabel]; ok {
		proj.m.Unlock()
		return nil, fmt.Errorf("duplicate target %v", rawlabel)
	}
	f := &function{
		proj:     proj,
		module:   m,
		label:    l,
		deps:     dependencies,
		sources:  sources,
		gens:     generates,
		docs:     docs,
		function: fn,
		always:   always,
		out:      newLineWriter(l, proj.events),
	}
	proj.targets[rawlabel] = &runTarget{target: f}
	proj.m.Unlock()

	if err := f.load(); err != nil {
		return nil, err
	}
	return f, nil
}

func (proj *Project) loadSourceFile(l *label.Label) (*sourceFile, error) {
	components := label.Split(l.Package)[1:]
	path := filepath.Join(proj.root, filepath.Join(components...), l.Name)

	rawlabel := l.String()
	proj.m.Lock()
	if f, ok := proj.targets[rawlabel]; ok {
		proj.m.Unlock()

		return f.target.(*sourceFile), nil
	}
	f := &sourceFile{
		proj:  proj,
		label: l,
		path:  path,
	}
	proj.targets[rawlabel] = &runTarget{target: f}
	proj.m.Unlock()

	if err := f.load(); err != nil {
		return nil, err
	}
	return f, nil
}

// link adds dependencies between targets that generate source files and the source files themselves.
func (proj *Project) link() error {
	for _, t := range proj.targets {
		for _, g := range t.target.generates() {
			g = g[len(proj.root)+1:]

			label, err := sourceLabel("//", g)
			if err != nil {
				return err
			}
			f, ok := proj.targets[label.String()]
			if !ok {
				continue
			}
			generated := f.target.(*sourceFile)
			if generated.generator != nil {
				return fmt.Errorf("multiple generators for %v: %v, %v", label, t.target.Label(), generated.generator)
			}
			generated.generator = t.target.Label()
		}
	}
	return nil
}

func equalStringMaps(x, y map[string]string) bool {
	if len(x) != len(y) {
		return false
	}
	for k, vx := range x {
		if vy, ok := y[k]; !ok || vx != vy {
			return false
		}
	}
	for k := range y {
		if _, ok := x[k]; !ok {
			return false
		}
	}
	return true
}

func IsTarget(l *label.Label) bool {
	return l.Kind == ""
}

func IsSource(l *label.Label) bool {
	return l.Kind == "source"
}

// def globals():
//     # Per-module globals
//
//     @attribute
//     def host():
//         """
//         Provides information about the host on which dawn is running.
//         """
//
//     @attribute
//     def package():
//         """
//         The name of the package that contains the executing Starlark module.
//         """
//
//     @constructor
//     def Cache():
//         """
//         A Cache provides a simple, concurrency-safe string -> value map
//         for use by dawn programs.
//         """
//
//         @method("*cache.once")
//         def once():
//             pass
//
//     @constructor
//     def Target():
//         """
//         A Target represents a dawn build target. Targets are created using the
//         target function.
//         """
//
//         @attribute
//         def label():
//             """
//             The target's label.
//             """
//
//         @attribute
//         def always():
//             """
//             True if the target should always be considered out-of-date.
//             """
//
//         @attribute
//         def env():
//             """
//             The target's environment (its globals, defaults, and free variables).
//             """
//
//         @attribute
//         def dependencies():
//             """
//             A list of the target's dependencies as stringified labels.
//             """
//
//         @attribute
//         def sources():
//             """
//             A list of the target's sources as absolute host paths.
//             """
//
//         @attribute
//         def generates():
//             """
//             A list of the files generated by the target as absolute host paths.
//             """
//
//     @function("*Project.builtin_path")
//     def path():
//         pass
//
//     @function("*Project.builtin_label")
//     def label():
//         pass
//
//     @function("*Project.builtin_contains")
//     def contains():
//         pass
//
//     @function("*Project.builtin_parse_flag")
//     def parse_flag():
//         pass
//
//     @function("*Project.builtin_target")
//     def target():
//         pass
//
//     @function("*Project.builtin_glob")
//     def glob():
//         pass
//
//     @function("*Project.builtin_fail")
//     def fail():
//         pass
//
//     # REPL methods
//
//     @function("*Project.builtin_get_target")
//     def get_target():
//         pass
//
//     @function("*Project.builtin_flags")
//     def flags():
//         pass
//
//     @function("*Project.builtin_targets")
//     def targets():
//         pass
//
//     @function("*Project.builtin_sources")
//     def sources():
//         pass
//
//     @function("*Project.builtin_run")
//     def run():
//         pass
//
//starlark:module
type globalModule int
