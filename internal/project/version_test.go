package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanPath(t *testing.T) {
	t.Parallel()
	cases := [][2]string{
		{"foo", "foo"},
		{"./foo/bar/../baz", "foo/baz"},
		{"foo@v0", "foo"},
		{"foo@v1", "foo"},
		{"foo@v2", "foo@v2"},
		{"foo@bar/baz", "foo@bar/baz"},
		{"foo@bar/baz@v0", "foo@bar/baz"},
	}
	for _, c := range cases {
		assert.Equal(t, c[1], CleanPath(c[0]))
	}
}

func TestCheckDawnVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		current  string
		required string
		wantErr  string
	}{
		{name: "unspecified", current: "v0.1.0"},
		{name: "development", current: "development", required: "v99.0.0"},
		{name: "equal", current: "v0.2.0", required: "v0.2.0"},
		{name: "newer", current: "v0.3.0", required: "v0.2.0"},
		{name: "version without prefix", current: "0.3.0", required: "v0.2.0"},
		{name: "too old", current: "v0.1.0", required: "v0.2.0", wantErr: "requires Dawn v0.2.0 or newer"},
		{name: "invalid current", current: "invalid", required: "v0.2.0", wantErr: "invalid current Dawn version"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := CheckDawnVersion(tt.current, tt.required)
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidDawnVersion(t *testing.T) {
	t.Parallel()

	_, err := LoadConfigBytes([]byte("[dawn]\nversion = '0.2.0'\n"))
	require.Error(t, err)
	assert.ErrorContains(t, err, `invalid Dawn version "0.2.0"`)
}
