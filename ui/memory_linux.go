package main

import (
	"bytes"
	"fmt"
	"os"
)

// availableMemory returns the memory programs can use without swapping, in bytes,
// or 0 if unknown.
func availableMemory() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	return parseMemAvailable(data)
}

func parseMemAvailable(meminfo []byte) uint64 {
	for _, line := range bytes.Split(meminfo, []byte("\n")) {
		var kb uint64
		if n, _ := fmt.Sscanf(string(line), "MemAvailable: %d kB", &kb); n == 1 {
			return kb * 1024
		}
	}
	return 0
}
