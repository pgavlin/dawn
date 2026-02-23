package dawn

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mitchellh/go-homedir"
	"github.com/pgavlin/dawn/internal/mvs"
	"github.com/pgavlin/dawn/internal/project"
	"github.com/pgavlin/dawn/internal/spell"
	"github.com/pgavlin/dawn/label"
	"github.com/pgavlin/dawn/runner"
	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/fx/v2"
	"github.com/pgavlin/glob"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/pgavlin/starlark-go/syntax"
	"github.com/pgavlin/starlark-go/typecheck"
	"github.com/rjeczalik/notify"
	"github.com/sugawarayuuta/sonnet"
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

	ignore glob.Glob

	configPath   string
	resolver     *mvs.Resolver
	moduleCache  string
	requirements map[string]string // maps project name to project path. local to each module.
	buildList    map[string]string // maps project path to version

	builtins starlark.StringDict
	baseEnv  *typecheck.Env

	always bool
	dryrun bool

	preferIndex  bool
	checkResults []CheckResult
	cyclicErr    error

	flags   map[string]*Flag
	modules map[string]*module
	targets map[string]*runTarget

	runner *runner.Runner
}

// OpenOptions configures a project Open.
type OpenOptions struct {
	Args        []string
	Events      Events
	Builtins    starlark.StringDict
	PreferIndex bool
}

// newProjectForCheck creates a minimally-initialized Project with config loaded.
// It sets up root, ignore patterns, requirements, buildList, and MVS resolver,
// but does not create a runner, temp dirs, or execute any modules.
func newProjectForCheck(root string, events Events) (*Project, error) {
	home, err := homedir.Dir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}
	moduleCache := filepath.Join(home, ".dawn", "modules", "cache")

	proj := &Project{
		root:        root,
		moduleCache: moduleCache,
		events:      events,
		modules:     map[string]*module{},
		targets:     map[string]*runTarget{},
		flags:       map[string]*Flag{},
	}
	if proj.events == nil {
		proj.events = DiscardEvents
	}
	proj.resolver = mvs.NewResolver(moduleCache, mvs.DefaultDialer, resolveEvents{proj.events})
	if err := proj.loadConfig(); err != nil {
		return nil, err
	}
	return proj, nil
}

// Open loads project config and type-checks all modules without evaluating Starlark.
// Type-check errors are non-fatal and available via CheckResults().
func Open(ctx context.Context, root string, options *OpenOptions) (*Project, error) {
	var events Events
	if options != nil {
		events = options.Events
	}

	proj, err := newProjectForCheck(root, events)
	if err != nil {
		return nil, err
	}

	if options != nil {
		proj.args = options.Args
		proj.builtins = options.Builtins
		proj.preferIndex = options.PreferIndex
	}

	results, err := proj.openModules(ctx)
	if err != nil {
		proj.events.OpenDone(nil, err)
		return nil, err
	}
	proj.checkResults = results
	proj.events.OpenDone(results, nil)

	return proj, nil
}

// CheckResults returns the type-check results from the most recent Open or Reload.
func (proj *Project) CheckResults() []CheckResult {
	return proj.checkResults
}

// Load evaluates all Starlark modules in the project. Must be called after Open.
func (proj *Project) Load(ctx context.Context) error {
	proj.work = filepath.Join(proj.root, ".dawn", "build")
	proj.temp = filepath.Join(proj.root, ".dawn", "build", "temp")

	proj.runner = runner.NewRunner(proj, runtime.NumCPU())

	return proj.eval(ctx, proj.preferIndex)
}

func (proj *Project) eval(ctx context.Context, index bool) (err error) {
	defer func() {
		proj.events.LoadDone(err)
	}()

	if index {
		if err := proj.loadIndex(); err == nil {
			return nil
		}
	}

	if err := os.MkdirAll(proj.temp, 0o750); err != nil {
		return err
	}

	// Evaluate all modules discovered during Open, concurrently.
	var wg sync.WaitGroup
	for _, m := range proj.modules {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// loadModule handles claiming (evaluating flag) and dedup
			proj.loadModule(ctx, m.label)
		}()
	}
	wg.Wait()

	for _, m := range proj.modules {
		if m.err != nil {
			return m.err
		}
	}

	if err := proj.link(); err != nil {
		return err
	}

	return proj.saveIndex()
}

func (proj *Project) Reload(ctx context.Context) (err error) {
	proj.flags = map[string]*Flag{}
	proj.modules = map[string]*module{}
	proj.targets = map[string]*runTarget{}

	results, err := proj.openModules(ctx)
	if err != nil {
		proj.events.OpenDone(results, err)
		return err
	}
	proj.checkResults = results
	proj.events.OpenDone(results, nil)

	return proj.eval(ctx, false)
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

func (proj *Project) Run(ctx context.Context, label *label.Label, options *RunOptions) error {
	options.apply(proj)

	err := proj.runner.Run(ctx, label.String())
	proj.events.RunDone(err)
	return err
}

func (proj *Project) Metrics() (running, waiting int) {
	return proj.runner.Metrics()
}

// LoadTarget implements runner.Host.
func (proj *Project) LoadTarget(_ context.Context, rawlabel string) (runner.Target, error) {
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

func (proj *Project) Watch(ctx context.Context, label *label.Label) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	events := make(chan notify.EventInfo, 1000)
	eventsDone := make(chan struct{})
	go func() {
		builds := make(chan struct{})
		buildsDone := make(chan struct{})

		go func() {
			for range builds {
				if err := proj.Reload(ctx); err != nil {
					// Project's load events are responsible for logging the error.
					continue
				}

				// Project's run events are responsible for logging the error.
				_ = proj.Run(ctx, label, nil)
			}
			close(buildsDone)
		}()

		defer func() {
			close(builds)
			<-buildsDone

			close(eventsDone)
		}()

		dirty := false
		rate := time.NewTicker(500 * time.Millisecond)
		for {
			select {
			case event, ok := <-events:
				if !ok {
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

			case <-ctx.Done():
				close(builds)
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
	return ctx.Err()
}

func (proj *Project) REPLEnv(stdout io.Writer, pkg *label.Label) (thread *starlark.Thread, globals starlark.StringDict) {
	m := &module{label: &label.Label{Kind: "module", Package: pkg.Package, Name: "<stdin>"}}
	thread, globals, _ = m.env(proj)
	util.SetStdio(thread, stdout, stdout)
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
	markPath(filepath.Join(proj.work, "temp"))
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

	return slices.SortedFunc(maps.Values(proj.flags), func(a, b *Flag) int { return cmp.Compare(a.Name, b.Name) })
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

	return slices.SortedFunc(
		fx.FMap(maps.Values(proj.targets), func(target *runTarget) (Target, bool) {
			ok := IsTarget(target.target.Label()) || len(target.target.dependencies()) != 0
			return target.target, ok
		}),
		func(a, b Target) int { return cmp.Compare(a.Label().String(), b.Label().String()) },
	)
}

func (proj *Project) Sources() []string {
	proj.m.Lock()
	defer proj.m.Unlock()

	return slices.Sorted(fx.FMap(maps.Values(proj.targets), func(target *runTarget) (string, bool) {
		if IsSource(target.target.Label()) {
			return target.target.(*sourceFile).path, true
		}
		return "", false
	}))
}

// NOTE: proj.m must be held!
func (proj *Project) unknownTarget(label string) error {
	targets := slices.Collect(fx.FMap(maps.Values(proj.targets), func(target *runTarget) (string, bool) {
		l := target.target.Label()
		return l.String(), IsTarget(l)
	}))
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
	Label        string            `json:"label,omitempty"`
	Doc          string            `json:"doc,omitempty"`
	Pos          string            `json:"pos,omitempty"`
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

	pathSum := sha256.Sum256([]byte(l.Package[2:] + "/" + target))
	return filepath.Join(proj.work, kind+"s", hex.EncodeToString(pathSum[:]))
}

func (proj *Project) loadTargetInfo(label *label.Label) (targetInfo, error) {
	path := proj.targetInfoPath(label)

	//nolint:gosec
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return targetInfo{}, nil
		}
		return targetInfo{}, err
	}
	defer f.Close()

	var info targetInfo
	if err := sonnet.NewDecoder(f).Decode(&info); err != nil {
		return targetInfo{}, err
	}
	if info.Label != label.String() {
		return targetInfo{}, fmt.Errorf("internal error: label mismatch: expected %v, not %v at path %v", label, info.Label, path)
	}
	return info, nil
}

func (proj *Project) saveTargetInfo(label *label.Label, info targetInfo) error {
	path := proj.targetInfoPath(label)

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}

	f, err := os.CreateTemp(proj.temp, "")
	if err != nil {
		return err
	}
	tempName := f.Name()

	info.Label = label.String()
	if err = sonnet.NewEncoder(f).Encode(info); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}

	return os.Rename(tempName, path)
}

func (proj *Project) ignored(path string) bool {
	path = filepath.ToSlash(path)
	return proj.ignore != nil && proj.ignore.MatchPath(path)
}

func (proj *Project) walkPackages(root, rel string, fn func(pkg string)) error {
	dir := filepath.Join(root, rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	pkg := "//"
	if rel != "." {
		pkg = "//" + filepath.ToSlash(rel)
	}

	if slices.ContainsFunc(entries, func(e os.DirEntry) bool { return e.Name() == "BUILD.dawn" }) {
		fn(pkg)
	}

	for _, e := range entries {
		if !e.IsDir() || e.Name() == ".dawn" {
			continue
		}
		childRel := filepath.Join(rel, e.Name())
		if proj.ignored(childRel) {
			continue
		}
		if err := proj.walkPackages(root, childRel, fn); err != nil {
			return err
		}
	}
	return nil
}

func (proj *Project) loadModule(ctx context.Context, l *label.Label) (starlark.StringDict, error) {
	proj.m.Lock()
	m, exists := proj.modules[l.String()]
	if exists && (m.loaded || m.evaluating) {
		proj.m.Unlock()
		return m.wait()
	}
	if !exists {
		m = &module{label: l, out: newLineWriter(l, proj.events)}
		m.cond = sync.NewCond(&m.m)
		proj.modules[l.String()] = m
	}
	m.evaluating = true
	proj.m.Unlock()

	return m.load(ctx, proj)
}

func (proj *Project) loadFunction(m *module, l *label.Label, dependencies, sources, generates []string, fn starlark.Callable, always bool, docs string, pos *syntax.Position) (*function, error) {
	if docs == "" {
		if hasdoc, ok := fn.(starlark.HasDoc); ok {
			docs = hasdoc.Doc()
		}
	}
	if pos == nil {
		if hasPosition, ok := fn.(interface{ Position() syntax.Position }); ok {
			p := hasPosition.Position()
			pos = &p
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
		label:    l.Copy(),
		deps:     dependencies,
		sources:  sources,
		gens:     generates,
		docs:     docs,
		pos:      pos,
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
		label: l.Copy(),
		path:  path,
	}
	proj.targets[rawlabel] = &runTarget{target: f}
	proj.m.Unlock()

	if err := f.load(); err != nil {
		return nil, err
	}
	return f, nil
}

func (proj *Project) setGenerator(relPath string, generator *label.Label) error {
	label, err := sourceLabel("//", relPath)
	if err != nil {
		return err
	}
	if f, ok := proj.targets[label.String()]; ok {
		generated := f.target.(*sourceFile)
		if generated.generator != nil && generated.generator != generator {
			return fmt.Errorf("multiple generators for %v: %v, %v", label, generator, generated.generator)
		}
		generated.generator = generator
	}
	return nil
}

// link adds dependencies between targets that generate source files and the source files themselves.
func (proj *Project) link() error {
	var sources fs.FS
	for _, t := range proj.targets {
		var include []string
		for _, g := range t.target.generates() {
			g = g[len(proj.root)+1:]

			if hasMeta := strings.ContainsAny(g, "*?[\\"); hasMeta {
				include = append(include, g)
				continue
			}

			if err := proj.setGenerator(g, t.target.Label()); err != nil {
				return err
			}
		}

		if len(include) != 0 {
			if sources == nil {
				sources = newSourceFS(proj.targets)
			}

			// TODO: validate earlier, strongly type, etc.
			gens, err := glob.New(include, nil)
			if err != nil {
				return fmt.Errorf("invalid generator patterns for target %v: %w", t.target.Label(), err)
			}
			for relPath := range gens.Match(sources, ".", false) {
				if err := proj.setGenerator(relPath, t.target.Label()); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type sourceNode struct {
	name     string
	children map[string]*sourceNode
}

func (n *sourceNode) ModTime() time.Time         { panic("unimplemented") }
func (n *sourceNode) Mode() fs.FileMode          { panic("unimplemented") }
func (n *sourceNode) Size() int64                { panic("unimplemented") }
func (n *sourceNode) Sys() any                   { panic("unimplemented") }
func (n *sourceNode) Info() (fs.FileInfo, error) { panic("unimplemented") }
func (n *sourceNode) Type() fs.FileMode          { panic("unimplemented") }

func (n *sourceNode) IsDir() bool {
	return len(n.children) != 0
}

func (n *sourceNode) Name() string {
	return n.name
}

type sourceFS struct {
	root *sourceNode
}

var (
	_ = fs.ReadDirFS((*sourceFS)(nil))
	_ = fs.StatFS((*sourceFS)(nil))
)

func newSourceFS(targets map[string]*runTarget) *sourceFS {
	fs := &sourceFS{root: &sourceNode{name: "."}}
	for _, t := range targets {
		if sf, ok := t.target.(*sourceFile); ok {
			relPath := path.Join(label.Split(sf.label.Package)[1:]...) + "/" + sf.label.Name
			fs.create(relPath)
		}
	}
	return fs
}

func (s *sourceFS) get(p string) *sourceNode {
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return s.root
	}
	dir, base := path.Split(p)
	dirn := s.get(dir)
	if dirn == nil {
		return nil
	}
	return dirn.children[base]
}

func (s *sourceFS) create(p string) *sourceNode {
	p = path.Clean(p)
	if p == "." || p == "/" || p == ".." || strings.HasPrefix(p, "../") {
		return s.root
	}
	dir, base := path.Split(p)
	dirn := s.create(dir)
	if n, ok := dirn.children[base]; ok {
		return n
	}
	if dirn.children == nil {
		dirn.children = map[string]*sourceNode{}
	}
	n := &sourceNode{name: base}
	dirn.children[base] = n
	return n
}

func (*sourceFS) Open(name string) (fs.File, error) { panic("unimplemented") }

func (s *sourceFS) Stat(name string) (fs.FileInfo, error) {
	n := s.get(name)
	if n == nil {
		return nil, fs.ErrNotExist
	}
	return n, nil
}

func (s *sourceFS) ReadDir(name string) ([]fs.DirEntry, error) {
	n := s.get(name)
	if n == nil {
		return nil, fs.ErrNotExist
	}
	return slices.Collect(fx.Map(maps.Values(n.children), func(n *sourceNode) fs.DirEntry { return n })), nil
}

func IsTarget(l *label.Label) bool {
	return l.Kind == ""
}

func IsSource(l *label.Label) bool {
	return l.Kind == "source" || l.Kind == "shadow"
}

type resolveEvents struct {
	events Events
}

func (r resolveEvents) ProjectLoading(req project.RequirementConfig) {
	r.events.RequirementLoading(&label.Label{Kind: "project", Project: req.Path}, req.Version)
}

func (r resolveEvents) ProjectLoaded(req project.RequirementConfig) {
	r.events.RequirementLoaded(&label.Label{Kind: "project", Project: req.Path}, req.Version)
}

func (r resolveEvents) ProjectLoadFailed(req project.RequirementConfig, err error) {
	r.events.RequirementLoadFailed(&label.Label{Kind: "project", Project: req.Path}, req.Version, err)
}

// starlark
//
//	def globals():
//	    # Per-module globals
//
//	    @attribute
//	    def host():
//	        """
//	        Provides information about the host on which dawn is running.
//	        """
//
//	    @attribute
//	    def package():
//	        """
//	        The name of the package that contains the executing Starlark module.
//	        """
//
//	    @constructor
//	    def Cache():
//	        """
//	        A Cache provides a simple, concurrency-safe string -> value map
//	        for use by dawn programs.
//	        """
//
//	        @method("*cache.once")
//	        def once():
//	            pass
//
//	    @constructor
//	    def Target():
//	        """
//	        A Target represents a dawn build target. Targets are created using the
//	        target function.
//	        """
//
//	        @attribute
//	        def label():
//	            """
//	            The target's label.
//	            """
//
//	        @attribute
//	        def always():
//	            """
//	            True if the target should always be considered out-of-date.
//	            """
//
//	        @attribute
//	        def env():
//	            """
//	            The target's environment (its globals, defaults, and free variables).
//	            """
//
//	        @attribute
//	        def dependencies():
//	            """
//	            A list of the target's dependencies as stringified labels.
//	            """
//
//	        @attribute
//	        def sources():
//	            """
//	            A list of the target's sources as absolute host paths.
//	            """
//
//	        @attribute
//	        def generates():
//	            """
//	            A list of the files generated by the target as absolute host paths.
//	            """
//
//	    @function("*Project.builtin_path")
//	    def path():
//	        pass
//
//	    @function("*Project.builtin_label")
//	    def label():
//	        pass
//
//	    @function("*Project.builtin_contains")
//	    def contains():
//	        pass
//
//	    @function("*Project.builtin_parse_flag")
//	    def parse_flag():
//	        pass
//
//	    @function("*Project.builtin_target")
//	    def target():
//	        pass
//
//	    @function("*Project.builtin_glob")
//	    def glob():
//	        pass
//
//	    @function("*Project.builtin_fail")
//	    def fail():
//	        pass
//
//	    # REPL methods
//
//	    @function("*Project.builtin_get_target")
//	    def get_target():
//	        pass
//
//	    @function("*Project.builtin_flags")
//	    def flags():
//	        pass
//
//	    @function("*Project.builtin_targets")
//	    def targets():
//	        pass
//
//	    @function("*Project.builtin_sources")
//	    def sources():
//	        pass
//
//	    @function("*Project.builtin_run")
//	    def run():
//	        pass
//
//starlark:module
type globalModule int
