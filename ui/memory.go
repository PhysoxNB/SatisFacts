package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"

	"satisfacts/parser"
)

// The save tools hold one huge block of live data (the decompressed save) and
// produce little garbage of their own. Go's default GC setting lets the heap grow
// to twice the live data before cleaning up, which on a large save means many
// gigabytes of slack. Scanning that block costs the GC almost nothing, because it
// holds no pointers, so collecting far more often barely affects speed.
const saveToolGCPercent = 10

var (
	gcMu      sync.Mutex
	gcUsers   int
	gcDefault int
)

// lowMemoryGC switches to the tighter GC setting until the returned function is
// called. Calls can overlap: the normal setting returns when the last one ends.
func lowMemoryGC() func() {
	gcMu.Lock()
	defer gcMu.Unlock()
	if gcUsers == 0 {
		gcDefault = debug.SetGCPercent(saveToolGCPercent)
	}
	gcUsers++
	return func() {
		gcMu.Lock()
		defer gcMu.Unlock()
		gcUsers--
		if gcUsers == 0 {
			debug.SetGCPercent(gcDefault)
		}
	}
}

// optimizeMemoryFactor is peak memory as a multiple of the decompressed save size,
// from measurements between 1.15 and 1.2 on saves of 132 to 443 MB, rounded up.
const optimizeMemoryFactor = 1.25

// MemoryCheck compares what Optimize is expected to need with what's free.
type MemoryCheck struct {
	NeededGB    float64 `json:"neededGB"`
	AvailableGB float64 `json:"availableGB"`
	Enough      bool    `json:"enough"`
}

// CheckOptimizeMemory estimates Optimize's memory use from the chunk headers,
// without decompressing anything. Enough stays true if free memory is unknown.
func (a *App) CheckOptimizeMemory(savePath string) (check *MemoryCheck, err error) {
	defer func() {
		if r := recover(); r != nil {
			check, err = nil, fmt.Errorf("could not read %q - the file may be corrupt (details: %v)", savePath, r)
		}
	}()

	file, err := os.Open(savePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	_, _, headerEnd, err := readSaveHeader(file, info.Size())
	if err != nil {
		return nil, err
	}
	body := io.NewSectionReader(file, int64(headerEnd), info.Size()-int64(headerEnd))
	decompressed, err := parser.NewDecompressor(body).TotalUncompressedSize()
	if err != nil {
		return nil, err
	}

	return newMemoryCheck(uint64(float64(decompressed)*optimizeMemoryFactor), availableMemory()), nil
}

func newMemoryCheck(needed, available uint64) *MemoryCheck {
	const gb = 1024 * 1024 * 1024
	return &MemoryCheck{
		NeededGB:    float64(needed) / gb,
		AvailableGB: float64(available) / gb,
		Enough:      available == 0 || needed <= available,
	}
}
