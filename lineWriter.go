package dawn

import (
	"bytes"
	"strings"

	"github.com/pgavlin/dawn/label"
)

type lineWriter struct {
	label  *label.Label
	events Events

	line strings.Builder
}

func newLineWriter(label *label.Label, events Events) *lineWriter {
	return &lineWriter{label: label, events: events}
}

func (l *lineWriter) Write(b []byte) (int, error) {
	w := 0
	for len(b) > 0 {
		newline := bytes.IndexByte(b, '\n')
		if newline == -1 {
			l.line.Write(b)
			w += len(b)
			break
		}
		if l.line.Len() == 0 {
			l.events.Print(l.label, string(b[:newline]))
		} else {
			l.line.Write(b[:newline])
			l.events.Print(l.label, l.line.String())
			l.line.Reset()
		}
		b = b[newline+1:]
		w += newline + 1
	}
	return w, nil
}

func (l *lineWriter) Flush() error {
	if l.line.Len() != 0 {
		l.events.Print(l.label, l.line.String())
	}
	return nil
}
