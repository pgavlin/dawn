package dawn

import (
	"strings"
	"testing"

	"github.com/pgavlin/dawn/label"
	"github.com/stretchr/testify/assert"
)

type lwTestEvents struct {
	discardEventsT

	lines strings.Builder
}

func (e *lwTestEvents) Print(_ *label.Label, line string) {
	e.lines.WriteString(line)
	e.lines.WriteByte('\n')
}

func TestLineWriter(t *testing.T) {
	cases := []struct {
		writes   []string
		expected string
	}{
		{
			[]string{"here's\nsome ", "text", " with multiple\n", "lines"},
			"here's\nsome text with multiple\nlines\n",
		},
		{
			[]string{"here's text without newlines"},
			"here's text without newlines\n",
		},
		{
			[]string{"here's\n", "simple\n", "text\n"},
			"here's\nsimple\ntext\n",
		},
		{
			[]string{"here's\nsimple\ntext\n"},
			"here's\nsimple\ntext\n",
		},
		{
			[]string{"many\n\n\n\n\nblank lines"},
			"many\n\n\n\n\nblank lines\n",
		},
	}
	for _, c := range cases {
		t.Run(c.expected, func(t *testing.T) {
			var events lwTestEvents

			w := newLineWriter(nil, &events)
			for _, t := range c.writes {
				w.Write([]byte(t))
			}
			w.Flush()

			assert.Equal(t, c.expected, events.lines.String())
		})
	}
}
