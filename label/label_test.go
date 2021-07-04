package label

import (
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
)

func version(v string) *semver.Version {
	sv, err := semver.ParseTolerant(v)
	if err != nil {
		panic(err)
	}
	return &sv
}

func TestParseLabel(t *testing.T) {
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
			"module@//pkg/path",
			&Label{Module: "module", Package: "//pkg/path"},
		},
		{
			"module@//pkg/path:target",
			&Label{Module: "module", Package: "//pkg/path", Name: "target"},
		},
		{
			"kind:module@//pkg/path:target",
			&Label{Kind: "kind", Module: "module", Package: "//pkg/path", Name: "target"},
		},
		{
			"kind:module+1.2.3@//pkg/path:target",
			&Label{Kind: "kind", Module: "module", Version: version("1.2.3"), Package: "//pkg/path", Name: "target"},
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
			"kind:module+1.2.3@rel-pkg/path:target",
			nil,
		},
		{
			"module@rel-pkg/path",
			nil,
		},
		{
			"module@rel-pkg/path:target",
			nil,
		},
		{
			"module+bad-version@//pkg",
			nil,
		},
		{
			"module+@//pkg",
			nil,
		},
	}
	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
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
