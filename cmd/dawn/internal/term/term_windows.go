package term

import (
	"bytes"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

func CursorUp(w io.Writer) {
	stdout := windows.Handle(os.Stdout.Fd())

	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(stdout, &info); err != nil {
		return
	}

	if coords := info.CursorPosition; coords.Y > 0 {
		coords.Y--
		windows.SetConsoleCursorPosition(stdout, coords)
	}
}

func ClearLine(w io.Writer, _ int) {
	stdout := windows.Handle(os.Stdout.Fd())

	var info windows.ConsoleScreenBufferInfo
	if err := windows.GetConsoleScreenBufferInfo(stdout, &info); err != nil {
		return
	}

	coords := info.CursorPosition
	if coords.X > 0 {
		coords.X = 0
		if err := windows.SetConsoleCursorPosition(stdout, coords); err != nil {
			return
		}
	}

	if _, err := windows.Write(stdout, bytes.Repeat([]byte{' '}, int(info.Size.X))); err != nil {
		return
	}

	windows.SetConsoleCursorPosition(stdout, coords)
}
