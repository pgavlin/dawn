package sh

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/stretchr/testify/require"
	"mvdan.cc/sh/v3/interp"
)

// TestCommandRelativeCwdResolvesAgainstModuleDir guards against regressing
// the bug where `sh.exec(cwd="...")` with a relative path resolved against
// the process cwd at invocation time instead of the calling module's
// directory. The docstring promises the module-dir anchor for both the
// default branch and the explicit-cwd branch; they must agree.
func TestCommandRelativeCwdResolvesAgainstModuleDir(t *testing.T) {
	moduleDir := t.TempDir()
	subDir := filepath.Join(moduleDir, "sub")
	require.NoError(t, os.Mkdir(subDir, 0o755))

	// Run with the process cwd somewhere unrelated to the module dir, so
	// the buggy behavior (resolving against the process cwd) would either
	// stat the wrong dir or fail outright.
	procDir := t.TempDir()
	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(procDir))
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	thread := &starlark.Thread{}
	util.Chdir(thread, moduleDir)

	_, options, _, err := command(thread, "true", "sub", nil, false)
	require.NoError(t, err)

	runner, err := interp.New(options...)
	require.NoError(t, err)

	// Symlink-resolve both sides: t.TempDir on darwin returns a path under
	// /var/folders/... that interp.Dir's filepath.Abs canonicalizes through
	// /private/var/folders/...
	want, err := filepath.EvalSymlinks(subDir)
	require.NoError(t, err)
	got, err := filepath.EvalSymlinks(runner.Dir)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// TestCommandRelativeCwdMatchesDefault confirms that the explicit-cwd branch
// agrees with the no-cwd branch when the relative path is ".".
func TestCommandRelativeCwdMatchesDefault(t *testing.T) {
	moduleDir := t.TempDir()

	procDir := t.TempDir()
	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(procDir))
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	thread := &starlark.Thread{}
	util.Chdir(thread, moduleDir)

	_, defaultOpts, _, err := command(thread, "true", "", nil, false)
	require.NoError(t, err)
	defaultRunner, err := interp.New(defaultOpts...)
	require.NoError(t, err)

	_, dotOpts, _, err := command(thread, "true", ".", nil, false)
	require.NoError(t, err)
	dotRunner, err := interp.New(dotOpts...)
	require.NoError(t, err)

	defaultDir, err := filepath.EvalSymlinks(defaultRunner.Dir)
	require.NoError(t, err)
	dotDir, err := filepath.EvalSymlinks(dotRunner.Dir)
	require.NoError(t, err)
	require.Equal(t, defaultDir, dotDir)
}

// TestCommandAbsoluteCwdPassesThrough ensures absolute cwd paths are
// preserved exactly, not re-anchored against the module dir.
func TestCommandAbsoluteCwdPassesThrough(t *testing.T) {
	moduleDir := t.TempDir()
	otherDir := t.TempDir()

	thread := &starlark.Thread{}
	util.Chdir(thread, moduleDir)

	_, options, _, err := command(thread, "true", otherDir, nil, false)
	require.NoError(t, err)

	runner, err := interp.New(options...)
	require.NoError(t, err)

	want, err := filepath.EvalSymlinks(otherDir)
	require.NoError(t, err)
	got, err := filepath.EvalSymlinks(runner.Dir)
	require.NoError(t, err)
	require.Equal(t, want, got)
}
