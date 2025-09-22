package label

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input    string
		expected *Label
	}{
		{
			"",
			&Label{},
		},
		{
			":target",
			&Label{Name: "target"},
		},
		{
			"pkg:target",
			&Label{Package: "pkg", Name: "target"},
		},
		{
			"kind:pkg:target",
			&Label{Kind: "kind", Package: "pkg", Name: "target"},
		},
		{
			"pkg",
			&Label{Package: "pkg"},
		},
		{
			"//abs-pkg",
			&Label{Package: "//abs-pkg"},
		},
		{
			"//abs-pkg:target",
			&Label{Package: "//abs-pkg", Name: "target"},
		},
		{
			"kind://abs-pkg:target",
			&Label{Kind: "kind", Package: "//abs-pkg", Name: "target"},
		},
		{
			"//abs-pkg/with/@symbol",
			&Label{Package: "//abs-pkg/with/@symbol"},
		},
		{
			"//abs-pkg/with/@symbol:target",
			&Label{Package: "//abs-pkg/with/@symbol", Name: "target"},
		},
		{
			"kind://abs-pkg/with/@symbol:target",
			&Label{Kind: "kind", Package: "//abs-pkg/with/@symbol", Name: "target"},
		},
		{
			"rel-pkg/path",
			&Label{Package: "rel-pkg/path"},
		},
		{
			"rel-pkg/path:target",
			&Label{Package: "rel-pkg/path", Name: "target"},
		},
		{
			"kind:rel-pkg/path:target",
			&Label{Kind: "kind", Package: "rel-pkg/path", Name: "target"},
		},
		{
			"project//pkg/path",
			&Label{Project: "project", Package: "//pkg/path"},
		},
		{
			"project//pkg/path:target",
			&Label{Project: "project", Package: "//pkg/path", Name: "target"},
		},
		{
			"kind:project//pkg/path:target",
			&Label{Kind: "kind", Project: "project", Package: "//pkg/path", Name: "target"},
		},
		{
			"project@v2//pkg/path",
			&Label{Project: "project@v2", Package: "//pkg/path"},
		},
		{
			"project@v2//pkg/path:target",
			&Label{Project: "project@v2", Package: "//pkg/path", Name: "target"},
		},
		{
			"kind:project@v2//pkg/path:target",
			&Label{Kind: "kind", Project: "project@v2", Package: "//pkg/path", Name: "target"},
		},
		{
			"host/project@v2//pkg/path",
			&Label{Project: "host/project@v2", Package: "//pkg/path"},
		},
		{
			"host/project@v2//pkg/path:target",
			&Label{Project: "host/project@v2", Package: "//pkg/path", Name: "target"},
		},
		{
			"kind:host/project@v2//pkg/path:target",
			&Label{Kind: "kind", Project: "host/project@v2", Package: "//pkg/path", Name: "target"},
		},
		{
			"kind:pkg:with:colons:target",
			nil,
		},
		{
			"./pkg",
			nil,
		},
		{
			"pkg/..",
			nil,
		},
		{
			"/pkg",
			nil,
		},
		{
			"kind://abs-pkg",
			nil,
		},
		{
			"kind:rel-pkg/path",
			nil,
		},
		{
			"kind:module@//pkg/path",
			nil,
		},
		{
			"kind:invalid:project//path",
			nil,
		},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			t.Parallel()
			l, err := Parse(c.input)
			if c.expected == nil {
				assert.Error(t, err)
				return
			}
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, *c.expected, *l)
			assert.Equal(t, c.input, l.String())
		})
	}
}
