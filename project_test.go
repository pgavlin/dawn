package dawn

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/otiai10/copy"
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
		validate: func(t *testing.T, dir string) {
			expected := readFile(t, filepath.Join(dir, "expected.md"))
			actual := readFile(t, filepath.Join(dir, "out.md"))
			assert.Equal(t, expected, actual)
		},
	}
	pt.run(t)
}
