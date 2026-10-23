package dawn

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/pgavlin/starlark-go/starlark"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestModule(t *testing.T, source string) (*Project, *module) {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".dawnconfig"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "BUILD.dawn"), []byte(source), 0o600))

	proj, err := Open(t.Context(), root, nil)
	require.NoError(t, err)
	require.Len(t, proj.modules, 1)

	var mod *module
	for _, m := range proj.modules {
		mod = m
	}
	return proj, mod
}

func TestStructConstructor(t *testing.T) {
	proj, mod := openTestModule(t, `value = struct(foo="bar")
field = value.foo
`)

	// The same source accepted by dawn check must also evaluate successfully.
	assert.Empty(t, proj.CheckResults())
	require.NoError(t, proj.Load(t.Context()))

	value, ok := mod.data["value"]
	require.True(t, ok)
	attrs, ok := value.(starlark.HasAttrs)
	require.True(t, ok)
	foo, err := attrs.Attr("foo")
	require.NoError(t, err)
	assert.Equal(t, starlark.String("bar"), foo)
	assert.Equal(t, starlark.String("bar"), mod.data["field"])
}

func TestModuleConstructorIsUnavailable(t *testing.T) {
	proj, _ := openTestModule(t, `value = module(name="example")
`)

	var checkMessages []string
	for _, result := range proj.CheckResults() {
		for _, err := range result.Errors {
			checkMessages = append(checkMessages, err.Msg)
		}
	}
	require.NotEmpty(t, checkMessages)
	assert.True(t, slices.ContainsFunc(checkMessages, func(msg string) bool {
		return msg == "undefined: module"
	}), "check errors: %v", checkMessages)

	err := proj.Load(t.Context())
	require.Error(t, err)
	assert.ErrorContains(t, err, "undefined: module")
}

func TestDawnVersionRequirement(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "dawn.toml"), []byte("[dawn]\nversion = 'v0.2.0'\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "BUILD.dawn"), nil, 0o600))

	_, err := Open(t.Context(), root, &OpenOptions{DawnVersion: "v0.1.0"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "requires Dawn v0.2.0 or newer")

	_, err = Open(t.Context(), root, &OpenOptions{DawnVersion: "v0.2.0"})
	require.NoError(t, err)
}

func TestPublicConstructorEnvironmentsAgree(t *testing.T) {
	proj, mod := openTestModule(t, "")
	_, runtimeEnv, err := mod.env(proj)
	require.NoError(t, err)
	typeEnv := proj.BaseEnv()

	var runtimeConstructors, typeConstructors []string
	for _, name := range []string{"struct", "module"} {
		if _, ok := runtimeEnv[name]; ok {
			runtimeConstructors = append(runtimeConstructors, name)
		}
		if _, ok := typeEnv.Predeclared[name]; ok {
			typeConstructors = append(typeConstructors, name)
		}
	}

	assert.Equal(t, []string{"struct"}, runtimeConstructors)
	assert.Equal(t, runtimeConstructors, typeConstructors)
	assert.True(t, IsUniversal("struct"))
	assert.False(t, IsUniversal("module"))
}
