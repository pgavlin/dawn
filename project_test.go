package dawn

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/otiai10/copy"
	"github.com/pgavlin/dawn/label"
	starlark_os "github.com/pgavlin/dawn/lib/os"
	starlark_sh "github.com/pgavlin/dawn/lib/sh"
	starlark_sha256 "github.com/pgavlin/dawn/lib/sha256"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	starlark_json "go.starlark.net/lib/json"
	"go.starlark.net/starlark"
)

type projectTest struct {
	path     string
	edits    []string
	validate func(t *testing.T, dir string)
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

	options := &LoadOptions{
		Events: DiscardEvents,
		Builtins: starlark.StringDict{
			"json":   starlark_json.Module,
			"os":     starlark_os.Module,
			"sh":     starlark_sh.Module,
			"sha256": starlark_sha256.Module,
		},
	}

	for _, p := range paths {
		err = copy.Copy(p, temp, copy.Options{OnDirExists: func(_, _ string) copy.DirExistsAction {
			return copy.Merge
		}})
		require.NoError(t, err)

		proj, err := Load(temp, options)
		require.NoError(t, err)

		err = proj.Run(def, nil)
		require.NoError(t, err)

		pt.validate(t, temp)
	}
}

func TestSimpleFiles(t *testing.T) {
	pt := projectTest{
		path:  "testdata/simple-files",
		edits: []string{"edit1", "edit2", "edit3"},
		validate: func(t *testing.T, dir string) {
			expected, err := os.ReadFile(filepath.Join(dir, "expected.md"))
			require.NoError(t, err)

			actual, err := os.ReadFile(filepath.Join(dir, "out.md"))
			require.NoError(t, err)

			assert.Equal(t, expected, actual)
		},
	}
	pt.run(t)
}

func TestSimpleTargets(t *testing.T) {
	pt := projectTest{
		path:  "testdata/simple-targets",
		edits: []string{"edit1", "edit2"},
		validate: func(t *testing.T, dir string) {
			expected, err := os.ReadFile(filepath.Join(dir, "expected.md"))
			require.NoError(t, err)

			actual, err := os.ReadFile(filepath.Join(dir, "out.md"))
			require.NoError(t, err)

			assert.Equal(t, expected, actual)
		},
	}
	pt.run(t)
}
