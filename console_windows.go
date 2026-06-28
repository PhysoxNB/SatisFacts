//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// enableWindowsANSI enables virtual terminal processing on Windows 10+ so
// ANSI escape codes are interpreted by the console. On older Windows or if it
// fails, useColor is disabled and we fall back to plain text.
func enableWindowsANSI() {
	const (
		ENABLE_VIRTUAL_TERMINAL_PROCESSING = 0x0004
		STD_OUTPUT_HANDLE                  = ^uintptr(11 - 1) // -11 as uintptr
	)
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getConsoleMode := kernel32.NewProc("GetConsoleMode")
	setConsoleMode := kernel32.NewProc("SetConsoleMode")

	var mode uint32
	r, _, _ := getConsoleMode.Call(STD_OUTPUT_HANDLE, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		useColor = false
		return
	}
	mode |= ENABLE_VIRTUAL_TERMINAL_PROCESSING
	r, _, _ = setConsoleMode.Call(STD_OUTPUT_HANDLE, uintptr(mode))
	if r == 0 {
		useColor = false
	}
}
