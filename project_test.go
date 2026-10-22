package dawn

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

func (e *testEvents) ModuleOpening(label *label.Label) {
	e.event("ModuleOpening", label)
}

func (e *testEvents) ModuleOpened(label *label.Label) {
	e.event("ModuleOpened", label)
}

func (e *testEvents) ModuleOpenFailed(label *label.Label, err error) {
	e.event("ModuleOpenFailed", label, "err", err)
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

func (e *testEvents) OpenDone(results []CheckResult, err error) {
	e.event("OpenDone", nil, "results", results, "err", err)
}

func (e *testEvents) TargetUpToDate(label *label.Label) {
	e.event("TargetUpToDate", label)
}

func (e *testEvents) TargetWaiting(label *label.Label, dependencies []string) {
	e.event("TargetWaiting", label, "dependencies", dependencies)
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
	options := &OpenOptions{
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

		proj, err := Open(ctx, temp, options)
		if pt.loadErr != "" {
			if err != nil {
				assert.ErrorContains(t, err, pt.loadErr)
				return
			}
			// loadErr might occur during Load
			err = proj.Load(ctx)
			assert.ErrorContains(t, err, pt.loadErr)
			return
		}
		require.NoError(t, err)

		err = proj.Load(ctx)
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

func TestEarlyCutoff(t *testing.T) {
	t.Parallel()

	// //:build_mid copies constant.txt to mid.txt, so editing its source in.txt
	// re-runs it but regenerates mid.txt with identical contents. //:consume's only
	// source is mid.txt, so it must be cut off.
	run, offset := 0, 0
	pt := projectTest{
		path:  "testdata/early-cutoff",
		edits: []string{"edit1", "edit2"},
		validate: func(t *testing.T, _ string, events []testEvent) {
			evts := events[offset:]
			offset = len(events)
			run++

			find := func(kind, target string) testEvent {
				i := slices.IndexFunc(evts, func(e testEvent) bool {
					l, ok := e["label"].(*label.Label)
					return ok && e["kind"].(string) == kind && l.String() == target
				})
				if i == -1 {
					return nil
				}
				return evts[i]
			}

			switch run {
			case 1:
				assert.NotNil(t, find("TargetEvaluating", "//:build_mid"))
				assert.NotNil(t, find("TargetEvaluating", "//:consume"))
			case 2:
				assert.NotNil(t, find("TargetEvaluating", "//:build_mid"))
				assert.Nil(t, find("TargetEvaluating", "//:consume"))
				assert.NotNil(t, find("TargetUpToDate", "//:consume"))

				mid := find("TargetSucceeded", "source://:mid.txt")
				if assert.NotNil(t, mid) {
					assert.False(t, mid["changed"].(bool))
				}
			case 3:
				assert.NotNil(t, find("TargetEvaluating", "//:build_mid"))
				assert.NotNil(t, find("TargetEvaluating", "//:consume"))

				mid := find("TargetSucceeded", "source://:mid.txt")
				if assert.NotNil(t, mid) {
					assert.True(t, mid["changed"].(bool))
				}
			}
		},
	}
	pt.run(t)
}

func TestDependencySetChanges(t *testing.T) {
	t.Parallel()

	temp := t.TempDir()
	writeFile := func(name, contents string) {
		path := filepath.Join(temp, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	}
	removeFile := func(name string) {
		require.NoError(t, os.Remove(filepath.Join(temp, name)))
	}

	writeFile("dawn.toml", "")
	writeFile("BUILD.dawn", `@target(sources=glob("inputs/*.txt"), default=True)
def consume():
    fail_if_requested()
`)
	writeFile("inputs/a.txt", "a\n")

	fail := starlark.NewBuiltin("fail_if_requested", func(_ *starlark.Thread, _ *starlark.Builtin, _ starlark.Tuple, _ []starlark.Tuple) (starlark.Value, error) {
		if _, err := os.Stat(filepath.Join(temp, "fail")); err == nil {
			return nil, assert.AnError
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		return starlark.None, nil
	})
	events := &testEvents{}
	options := &OpenOptions{
		Events:   events,
		Builtins: starlark.StringDict{"fail_if_requested": fail},
	}
	defaultLabel := tryParseLabel(t, "//:default")
	consumeLabel := tryParseLabel(t, "//:consume")

	run := func(wantErr bool) (*Project, []testEvent) {
		offset := len(events.events)
		proj, err := Open(t.Context(), temp, options)
		require.NoError(t, err)
		require.NoError(t, proj.Load(t.Context()))
		err = proj.Run(t.Context(), defaultLabel, nil)
		if wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
		return proj, events.events[offset:]
	}
	find := func(events []testEvent, kind, target string) testEvent {
		i := slices.IndexFunc(events, func(e testEvent) bool {
			l, ok := e["label"].(*label.Label)
			return ok && e["kind"].(string) == kind && l.String() == target
		})
		if i == -1 {
			return nil
		}
		return events[i]
	}
	assertInfo := func(proj *Project, wantDeps []string, rerun bool) {
		info, err := proj.loadTargetInfo(consumeLabel)
		require.NoError(t, err)
		assert.Len(t, info.Dependencies, len(wantDeps))
		for _, dep := range wantDeps {
			assert.Contains(t, info.Dependencies, dep)
		}
		assert.Equal(t, rerun, info.Rerun)
	}

	proj, evts := run(false)
	assert.NotNil(t, find(evts, "TargetEvaluating", "//:consume"))
	assertInfo(proj, []string{"source://inputs:a.txt"}, false)

	_, evts = run(false)
	assert.Nil(t, find(evts, "TargetEvaluating", "//:consume"))

	writeFile("inputs/b.txt", "b\n")
	proj, evts = run(false)
	assert.NotNil(t, find(evts, "TargetEvaluating", "//:consume"))
	assertInfo(proj, []string{"source://inputs:a.txt", "source://inputs:b.txt"}, false)

	_, evts = run(false)
	assert.Nil(t, find(evts, "TargetEvaluating", "//:consume"))

	removeFile("inputs/b.txt")
	proj, evts = run(false)
	assert.NotNil(t, find(evts, "TargetEvaluating", "//:consume"))
	assertInfo(proj, []string{"source://inputs:a.txt"}, false)

	_, evts = run(false)
	assert.Nil(t, find(evts, "TargetEvaluating", "//:consume"))

	// A failed run after an addition must preserve the dependency set from the
	// last successful run.
	writeFile("inputs/b.txt", "b\n")
	writeFile("fail", "")
	proj, evts = run(true)
	assert.NotNil(t, find(evts, "TargetEvaluating", "//:consume"))
	assertInfo(proj, []string{"source://inputs:a.txt"}, true)

	removeFile("fail")
	proj, evts = run(false)
	assert.NotNil(t, find(evts, "TargetEvaluating", "//:consume"))
	assertInfo(proj, []string{"source://inputs:a.txt", "source://inputs:b.txt"}, false)

	// The same rule applies to a failed run after a removal.
	removeFile("inputs/b.txt")
	writeFile("fail", "")
	proj, evts = run(true)
	assert.NotNil(t, find(evts, "TargetEvaluating", "//:consume"))
	assertInfo(proj, []string{"source://inputs:a.txt", "source://inputs:b.txt"}, true)

	removeFile("fail")
	proj, evts = run(false)
	assert.NotNil(t, find(evts, "TargetEvaluating", "//:consume"))
	assertInfo(proj, []string{"source://inputs:a.txt"}, false)
}

func tryParseLabel(t *testing.T, raw string) *label.Label {
	t.Helper()
	l, err := label.Parse(raw)
	require.NoError(t, err)
	return l
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

func TestZeroLengthSource(t *testing.T) {
	t.Parallel()
	pt := projectTest{
		path:     "testdata/zero-length-source",
		validate: func(t *testing.T, _ string, _ []testEvent) {},
	}
	pt.run(t)
}

func TestTypeCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		wantErrors int
		wantMsgs   []string
	}{
		{"clean", "testdata/typecheck-clean", 0, nil},
		{"errors", "testdata/typecheck-errors", 1, []string{"undefined"}},
		{"load", "testdata/typecheck-load", 0, nil},
		{"load-error", "testdata/typecheck-load-error", 1, []string{"undefined"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			temp := t.TempDir()

			path, err := filepath.Abs(tt.path)
			require.NoError(t, err)

			err = copy.Copy(filepath.Join(path, "base"), temp, copy.Options{
				OnDirExists: func(_, _ string) copy.DirExistsAction {
					return copy.Merge
				},
			})
			require.NoError(t, err)

			ctx := t.Context()
			proj, err := Open(ctx, temp, &OpenOptions{})
			require.NoError(t, err)

			results := proj.CheckResults()

			var totalErrors int
			for _, r := range results {
				totalErrors += len(r.Errors)
			}

			assert.Equal(t, tt.wantErrors, totalErrors, "unexpected number of type-check errors")

			for _, msg := range tt.wantMsgs {
				found := false
				for _, r := range results {
					for _, e := range r.Errors {
						if strings.Contains(e.Msg, msg) {
							found = true
							break
						}
					}
					if found {
						break
					}
				}
				assert.True(t, found, "expected error message containing %q", msg)
			}
		})
	}
}
