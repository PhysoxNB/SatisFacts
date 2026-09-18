package main

import (
	"syscall"
	"unsafe"
)

// memoryStatusEx mirrors the Win32 MEMORYSTATUSEX structure.
type memoryStatusEx struct {
	length               uint32
	memoryLoad           uint32
	totalPhys            uint64
	availPhys            uint64
	totalPageFile        uint64
	availPageFile        uint64
	totalVirtual         uint64
	availVirtual         uint64
	availExtendedVirtual uint64
}

// availableMemory returns the free physical memory in bytes, or 0 if unknown.
func availableMemory() uint64 {
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx")
	status := memoryStatusEx{}
	status.length = uint32(unsafe.Sizeof(status))
	if ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&status))); ok == 0 {
		return 0
	}
	return status.availPhys
}
