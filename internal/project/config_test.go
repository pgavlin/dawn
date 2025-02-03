package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTripConfigFile(t *testing.T) {
	in := filepath.Join("testdata", "conf.toml")
	config, err := LoadConfigFile(in)
	require.NoError(t, err)

	out := filepath.Join(t.TempDir(), "conf.toml")
	err = WriteConfigFile(out, config)
	require.NoError(t, err)

	expected, err := os.ReadFile(in)
	require.NoError(t, err)

	actual, err := os.ReadFile(out)
	require.NoError(t, err)

	assert.Equal(t, string(expected), string(actual))
}
