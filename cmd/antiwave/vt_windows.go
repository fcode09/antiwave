//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT enables ANSI (Virtual Terminal) processing on the Windows console,
// so color and cursor sequences work in cmd/conhost.
func enableVT() {
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
