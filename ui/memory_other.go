//go:build !linux && !windows

package main

// availableMemory returns 0 (unknown) on platforms without a simple way to ask.
func availableMemory() uint64 {
	return 0
}
