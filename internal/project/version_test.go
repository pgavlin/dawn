package project

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanPath(t *testing.T) {
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
