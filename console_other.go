//go:build !windows

package main

// enableWindowsANSI is a no-op on non-Windows platforms. The real
// implementation is in console_windows.go (Windows-only build).
func enableWindowsANSI() {}
