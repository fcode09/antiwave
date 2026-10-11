//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// enableVT activa el procesamiento de secuencias ANSI (Virtual Terminal) en la
// consola de Windows, para que el color y el cursor funcionen en cmd/conhost.
func enableVT() {
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return
	}
	_ = windows.SetConsoleMode(h, mode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING)
}
