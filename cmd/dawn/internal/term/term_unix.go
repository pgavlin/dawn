// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package term

import (
	"fmt"
	"io"
)

func CursorUp(w io.Writer) {
	fmt.Fprintf(w, "\x1b[A")
}

func ClearLine(w io.Writer, _ int) {
	fmt.Fprint(w, "\r\x1b[K")
}
