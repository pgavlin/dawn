package dawn

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/otiai10/copy"
	"github.com/pgavlin/dawn/diff"
	"github.com/pgavlin/dawn/label"
	starlark_os "github.com/pgavlin/dawn/lib/os"
	starlark_sh "github.com/pgavlin/dawn/lib/sh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	starlark_json "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
)

func readFile(t *testing.T, path string) []byte {
	contents, err := os.ReadFile(path)
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

	temp, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	t.Logf("temp dir: %v", temp)

	path, err := filepath.Abs(pt.path)
	require.NoError(t, err)

	paths := []string{filepath.Join(path, "base")}
	for _, edit := range pt.edits {
		paths = append(paths, filepath.Join(path, edit))
	}

	events := &testEvents{}
	options := &LoadOptions{
		Events: events,
		Builtins: starlark.StringDict{
			"json": starlark_json.Module,
			"os":   starlark_os.Module,
			"sh":   starlark_sh.Module,
		},
	}

	for _, p := range paths {
		err = copy.Copy(p, temp, copy.Options{OnDirExists: func(_, _ string) copy.DirExistsAction {
			return copy.Merge
		}})
		require.NoError(t, err)

		proj, err := Load(temp, options)
		if pt.loadErr != "" {
			assert.ErrorContains(t, err, pt.loadErr)
			return
		}
		require.NoError(t, err)

		err = proj.Run(def, nil)
		if pt.runErr != "" {
			assert.ErrorContains(t, err, pt.runErr)
			return
		}
		require.NoError(t, err)

		pt.validate(t, temp, events.events)
	}
}

func TestSimpleFiles(t *testing.T) {
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
	pt := projectTest{
		path:  "testdata/simple-targets",
		edits: []string{"edit1", "edit2"},
		validate: func(t *testing.T, dir string, _ []testEvent) {
			expected := readFile(t, filepath.Join(dir, "expected.md"))
			actual := readFile(t, filepath.Join(dir, "out.md"))
			assert.Equal(t, expected, actual)
		},
	}
	pt.run(t)
}

func TestTargetDiffs(t *testing.T) {
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
	pt := projectTest{
		path:     "testdata/local-modules",
		validate: func(t *testing.T, _ string, _ []testEvent) {},
	}
	pt.run(t)
}

func TestCyclicModules(t *testing.T) {
	pt := projectTest{
		path:    "testdata/cyclic-modules",
		loadErr: "cyclic dependency",
	}
	pt.run(t)
}

func TestBuiltins(t *testing.T) {
	pt := projectTest{
		path:     "testdata/builtins",
		validate: func(t *testing.T, _ string, _ []testEvent) {},
	}
	pt.run(t)
}
