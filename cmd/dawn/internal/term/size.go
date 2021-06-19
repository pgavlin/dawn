package term

import (
	"os"

	"golang.org/x/term"
)

func IsTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

func GetSize(f *os.File) (width, height int, err error) {
	return term.GetSize(int(f.Fd()))
}
