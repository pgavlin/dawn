package dawn

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/otiai10/copy"
	"github.com/pgavlin/dawn/diff"
	"github.com/pgavlin/dawn/label"
	starlark_os "github.com/pgavlin/dawn/lib/os"
	starlark_sh "github.com/pgavlin/dawn/lib/sh"
	starlark_json "github.com/pgavlin/starlark-go/lib/json"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readFile(t *testing.T, path string) []byte {
	contents, err := os.ReadFile(path) //nolint:gosec
	require.NoError(t, err)
	return bytes.ReplaceAll(contents, []byte{'\r', '\n'}, []byte{'\n'})
}

type testEvent = map[string]interface{}

type testEvents struct {
	m      sync.Mutex
	events []testEvent
}

func (e *testEvents) Print(label *label.Label, line string) {
	e.event("Print", label, "line", line)
}

func (e *testEvents) RequirementLoading(label *label.Label, version string) {
	e.event("RequirementLoading", label, "version", version)
}

func (e *testEvents) RequirementLoaded(label *label.Label, version string) {
	e.event("RequirementLoaded", label, "version", version)
}

func (e *testEvents) RequirementLoadFailed(label *label.Label, version string, err error) {
	e.event("RequirementLoadFailed", label, "version", version, "err", err)
}

func (e *testEvents) ModuleLoading(label *label.Label) {
	e.event("ModuleLoading", label)
}

func (e *testEvents) ModuleLoaded(label *label.Label) {
	e.event("ModuleLoaded", label)
}

func (e *testEvents) ModuleLoadFailed(label *label.Label, err error) {
	e.event("ModuleLoadFailed", label, "err", err)
}

func (e *testEvents) LoadDone(err error) {
	e.event("LoadDone", nil, "err", err)
}

func (e *testEvents) TargetUpToDate(label *label.Label) {
	e.event("TargetUpToDate", label)
}

func (e *testEvents) TargetEvaluating(label *label.Label, reason string, diff diff.ValueDiff) {
	e.event("TargetEvaluating", label, "reason", reason, "diff", diff)
}

func (e *testEvents) TargetFailed(label *label.Label, err error) {
	e.event("TargetFailed", label, "err", err)
}

func (e *testEvents) TargetSucceeded(label *label.Label, changed bool) {
	e.event("TargetSucceeded", label, "changed", changed)
}

func (e *testEvents) RunDone(err error) {
	e.event("RunDone", nil, "err", err)
}

func (e *testEvents) FileChanged(label *label.Label) {
	e.event("FileChanged", label)
}

func (e *testEvents) event(kind string, label *label.Label, pairs ...interface{}) {
	e.m.Lock()
	defer e.m.Unlock()

	event := testEvent{"kind": kind}
	if label != nil {
		event["label"] = label
	}
	if len(pairs)%2 != 0 {
		panic("oddly-sized pairs")
	}
	for i := 0; i < len(pairs); i += 2 {
		event[pairs[i].(string)] = pairs[i+1]
	}
	e.events = append(e.events, event)
}

type projectTest struct {
	path     string
	edits    []string
	loadErr  string
	runErr   string
	validate func(t *testing.T, dir string, events []testEvent)
}

func (pt *projectTest) run(t *testing.T) {
	def, err := label.Parse("//:default")
	require.NoError(t, err)

	temp := t.TempDir()

	t.Logf("temp dir: %v", temp)

	path, err := filepath.Abs(pt.path)
	require.NoError(t, err)

	paths := []string{filepath.Join(path, "base")}
	for _, edit := range pt.edits {
		paths = append(paths, filepath.Join(path, edit))
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cancelBuiltin := starlark.NewBuiltin("cancel", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		cancel()
		return starlark.None, nil
	})

	events := &testEvents{}
	options := &LoadOptions{
		Events: events,
		Builtins: starlark.StringDict{
			"cancel": cancelBuiltin,
			"json":   starlark_json.Module,
			"os":     starlark_os.Module,
			"sh":     starlark_sh.Module,
		},
	}

	for _, p := range paths {
		err = copy.Copy(p, temp, copy.Options{OnDirExists: func(_, _ string) copy.DirExistsAction {
			return copy.Merge
		}})
		require.NoError(t, err)

		proj, err := Load(ctx, temp, options)
		if pt.loadErr != "" {
			assert.ErrorContains(t, err, pt.loadErr)
			return
		}
		require.NoError(t, err)

		err = proj.GC()
		require.NoError(t, err)

		err = proj.Run(ctx, def, nil)
		if pt.runErr != "" {
			assert.ErrorContains(t, err, pt.runErr)
			return
		}
		require.NoError(t, err)

		pt.validate(t, temp, events.events)

		repl := filepath.Join(temp, "repl.dawn")
		if _, err := os.Stat(repl); err == nil {
			thread, globals := proj.REPLEnv(io.Discard, &label.Label{Package: def.Package})
			_, err := starlark.ExecFile(thread, repl, nil, globals)
			require.NoError(t, err)
		}
	}
}

func TestSimpleFiles(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:  "testdata/simple-files",
		edits: []string{"edit1", "edit2", "edit3"},
		validate: func(t *testing.T, dir string, _ []testEvent) {
			expected := readFile(t, filepath.Join(dir, "expected.md"))
			actual := readFile(t, filepath.Join(dir, "out.md"))
			assert.Equal(t, expected, actual)
		},
	}
	pt.run(t)
}

func TestSimpleTargets(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:  "testdata/simple-targets",
		edits: []string{"edit1", "edit2", "edit3"},
		validate: func(t *testing.T, dir string, _ []testEvent) {
			expected := readFile(t, filepath.Join(dir, "expected.md"))
			actual := readFile(t, filepath.Join(dir, "out.md"))
			assert.Equal(t, expected, actual)
		},
	}
	pt.run(t)
}

func TestTargetDiffs(t *testing.T) {
	t.Parallel()
	dirs := []string{"constants", "functions", "names", "predeclared", "universal", "globals", "freevars"}
	for _, dir := range dirs {
		pt := projectTest{
			path:  "testdata/target-diffs/" + dir,
			edits: []string{"edit1"},
			validate: func(t *testing.T, _ string, events []testEvent) {
				evaluated := false
				for _, e := range events {
					if e["kind"].(string) == "TargetEvaluating" {
						evaluated = true
						break
					}
				}
				assert.True(t, evaluated)
			},
		}
		pt.run(t)
	}
}

func TestLocalModules(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:     "testdata/local-modules",
		validate: func(t *testing.T, _ string, _ []testEvent) {},
	}
	pt.run(t)
}

func TestCyclicModules(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:    "testdata/cyclic-modules",
		loadErr: "cyclic dependency",
	}
	pt.run(t)
}

func TestCyclicTargets(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:   "testdata/cyclic-targets",
		runErr: "dependencies failed",
		validate: func(t *testing.T, _ string, events []testEvent) {
			i := slices.IndexFunc(events, func(e testEvent) bool {
				return e["kind"].(string) == "TargetFailed"
			})
			require.NotEqual(t, -1, i)
			assert.ErrorContains(t, events[i]["err"].(error), "cyclic dependency")
		},
	}
	pt.run(t)
}

func TestCyclicGenerate(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:     "testdata/cyclic-generates",
		validate: func(t *testing.T, _ string, _ []testEvent) {},
	}
	pt.run(t)
}

func TestBuiltins(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:     "testdata/builtins",
		validate: func(t *testing.T, _ string, _ []testEvent) {},
	}
	pt.run(t)
}

func TestCancelLoad(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:    "testdata/cancel-load",
		loadErr: "context canceled",
	}
	pt.run(t)
}

func TestCancelRun(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:   "testdata/cancel-run",
		runErr: "context canceled",
	}
	pt.run(t)
}

func TestInvalidTargetName(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:    "testdata/invalid-target-name",
		loadErr: "invalid name",
	}
	pt.run(t)
}
