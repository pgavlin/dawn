package os

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pgavlin/dawn/util"
	"github.com/pgavlin/starlark-go/starlark"
	"github.com/stretchr/testify/require"
)

// TestCommandRelativeCwdResolvesAgainstModuleDir guards against the same
// class of bug as the sh.exec case: a relative `cwd` argument must be
// anchored against the calling module's directory, not the process cwd.
// exec.Cmd.Dir treats a relative path as relative to the parent process's
// cwd; the docstring promises the module dir.
func TestCommandRelativeCwdResolvesAgainstModuleDir(t *testing.T) {
	moduleDir := t.TempDir()
	subDir := filepath.Join(moduleDir, "sub")
	require.NoError(t, os.Mkdir(subDir, 0o755))

	procDir := t.TempDir()
	origWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(procDir))
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	thread := &starlark.Thread{}
	util.Chdir(thread, moduleDir)

	fn := starlark.NewBuiltin("exec", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	cmd, err := command(thread, fn, util.StringList{"true"}, "sub", nil)
	require.NoError(t, err)

	want, err := filepath.EvalSymlinks(subDir)
	require.NoError(t, err)
	got, err := filepath.EvalSymlinks(cmd.Dir)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// TestCommandAbsoluteCwdPassesThrough ensures absolute cwd paths are
// preserved unchanged.
func TestCommandAbsoluteCwdPassesThrough(t *testing.T) {
	moduleDir := t.TempDir()
	otherDir := t.TempDir()

	thread := &starlark.Thread{}
	util.Chdir(thread, moduleDir)

	fn := starlark.NewBuiltin("exec", func(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
		return starlark.None, nil
	})

	cmd, err := command(thread, fn, util.StringList{"true"}, otherDir, nil)
	require.NoError(t, err)

	require.Equal(t, otherDir, cmd.Dir)
}
