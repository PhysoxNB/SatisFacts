package editor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sort"
	"strings"

	"satisfacts/parser"
)

// OptimizeResult holds statistics from the optimize operation.
type OptimizeResult struct {
	OriginalSize int64
	NewSize      int64
	Savings      int64
	SavingsPct   float64
	TotalObjects int
	LevelStats   []OptimizeLevelStat
}

// OptimizeLevelStat holds per-level statistics.
type OptimizeLevelStat struct {
	Name          string
	ObjectCount   int
	UniqueClasses int
}

// OptimizeSave reorganizes objects by class name within each level for better
// compression. It modifies sd.Decompressed in place and returns statistics.
// The progress callback (if non-nil) is called with (currentLevel, totalLevels, phase)
// before each level. Returning a non-nil error from it aborts the run and that
// error is returned to the caller, which is how cancellation is wired up.
//
// This needs a full second copy of the body in memory. To write the result to a
// file, use OptimizeSaveToFile, which doesn't.
func OptimizeSave(sd *SaveData, progress func(current, total int, phase string) error) (*OptimizeResult, error) {
	// Reordering objects doesn't change the total byte count, so this is the
	// exact final size and the buffer never has to grow.
	var body bytes.Buffer
	body.Grow(len(sd.Decompressed))

	result, err := writeOptimizedBody(sd, &body, progress)
	if err != nil {
		return nil, err
	}
	sd.Decompressed = body.Bytes()
	return result, nil
}

// OptimizeSaveToFile writes the optimized save straight to outputPath, compressing
// as it goes. sd is left unchanged. The optimized body only ever exists as a
// stream, so this needs about half the memory of OptimizeSave plus WriteSave.
func OptimizeSaveToFile(sd *SaveData, outputPath string, progress func(current, total int, phase string) error) (*OptimizeResult, error) {
	var result *OptimizeResult
	err := writeSaveFile(sd, outputPath, func(w io.Writer) error {
		cw := newChunkWriter(w, sd.IsV2, sd.MaxChunkSize)
		var err error
		if result, err = writeOptimizedBody(sd, cw, progress); err != nil {
			return err
		}
		return cw.Close()
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// writeOptimizedBody streams the reordered decompressed body to w.
func writeOptimizedBody(sd *SaveData, w io.Writer, progress func(current, total int, phase string) error) (*OptimizeResult, error) {
	bodyHeaderEnd, levels, trailingStart, trailingEnd, err := ParseBodyLevels(sd.Decompressed, sd.VersionCtx)
	if err != nil {
		return nil, fmt.Errorf("parsing body: %w", err)
	}

	totalObjects := 0
	for _, l := range levels {
		totalObjects += int(l.ObjectCount)
	}

	wpos := 0
	var writeErr error
	write := func(p []byte) {
		if writeErr != nil {
			return
		}
		_, writeErr = w.Write(p)
		wpos += len(p)
	}
	writeUint32 := func(v uint32) {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], v)
		write(b[:])
	}
	writeUint64 := func(v uint64) {
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], v)
		write(b[:])
	}

	// Copy body header as-is, except for the size field it starts with
	writeUint32(uint32(TotalBodyRestSizeFor(sd.Decompressed, sd.VersionCtx)))
	write(sd.Decompressed[4:bodyHeaderEnd])

	levelStats := make([]OptimizeLevelStat, 0, len(levels))

	for li, l := range levels {
		if progress != nil {
			if err := progress(li, len(levels), "Optimizing levels..."); err != nil {
				return nil, err
			}
		}
		shortName := l.Name
		if l.IsPersistent {
			shortName = "Persistent_Level"
		}

		// Copy level name (sublevels only)
		if !l.IsPersistent {
			write(sd.Decompressed[l.NameStart:l.NameEnd])
		}

		if l.ObjectCount == 0 {
			// No objects - copy TOC and data blobs as-is
			write(sd.Decompressed[l.TocLenPos:l.TocDataEnd])
			write(sd.Decompressed[l.DataLenPos:l.DataBlobEnd])
			write(sd.Decompressed[l.FooterStart:l.FooterEnd])
			levelStats = append(levelStats, OptimizeLevelStat{Name: shortName, ObjectCount: 0})
			continue
		}

		// Read what sorting needs straight out of the TOC. Parsing full headers
		// would copy several strings per object, gigabytes on a large save.
		toc := sd.Decompressed[l.TocDataStart:l.TocDataEnd]
		depths, classes, depthsOK := objectSortKeys(toc, l.TocHeaderOffsets, sd.VersionCtx)

		// Create sort indices: sort object indices by nesting depth, then class
		// name. The game drops subobjects stored before their outer object (e.g.
		// FGBlueprintShortcut sorts before its FGPlayerHotbar alphabetically,
		// which wiped blueprint hotbar slots), so outers must always come first.
		indices := make([]int32, l.ObjectCount)
		for i := range indices {
			indices[i] = int32(i)
		}
		if depthsOK {
			sort.SliceStable(indices, func(i, j int) bool {
				a, b := indices[i], indices[j]
				if depths[a] != depths[b] {
					return depths[a] < depths[b]
				}
				return classes[a] < classes[b]
			})
		}

		// Count unique classes for reporting
		classCount := 0
		var prevClass string
		for _, idx := range indices {
			if classes[idx] != prevClass {
				classCount++
				prevClass = classes[idx]
			}
		}

		// Rebuild the TOC blob. Reordering doesn't change its size.
		writeUint64(uint64(len(toc)))
		writeUint32(uint32(l.ObjectCount))
		for _, idx := range indices {
			start := l.TocHeaderOffsets[idx]
			write(toc[start : start+l.TocHeaderSizes[idx]])
		}
		// Copy remaining TOC bytes
		lastIdx := len(l.TocHeaderOffsets) - 1
		write(toc[l.TocHeaderOffsets[lastIdx]+l.TocHeaderSizes[lastIdx]:])

		// Data blob = 8-byte length prefix + 4-byte countEntities + per-object data
		dataBlobLen := 4
		for _, idx := range indices {
			dataBlobLen += l.DataObjSizes[idx]
		}
		writeUint64(uint64(dataBlobLen))
		writeUint32(uint32(l.CountEntities))
		for _, idx := range indices {
			start := l.DataObjOffsets[idx]
			end := start + l.DataObjSizes[idx]
			write(sd.Decompressed[l.DataBlobStart+start : l.DataBlobStart+end])
		}

		// Copy footer
		write(sd.Decompressed[l.FooterStart:l.FooterEnd])

		if writeErr != nil {
			return nil, fmt.Errorf("writing level %q: %w", shortName, writeErr)
		}

		levelStats = append(levelStats, OptimizeLevelStat{
			Name:          shortName,
			ObjectCount:   int(l.ObjectCount),
			UniqueClasses: classCount,
		})
	}

	// Copy trailing bytes
	if trailingEnd > trailingStart {
		write(sd.Decompressed[trailingStart:trailingEnd])
	}
	if writeErr != nil {
		return nil, fmt.Errorf("writing body: %w", writeErr)
	}

	if wpos != len(sd.Decompressed) {
		return nil, fmt.Errorf("optimize: wrote %d bytes but expected %d (body may be silently corrupted)", wpos, len(sd.Decompressed))
	}

	return &OptimizeResult{
		TotalObjects: totalObjects,
		LevelStats:   levelStats,
	}, nil
}

// objectSortKeys reads each object's nesting depth (see objectDepths, whose rules
// it follows) and class name directly from a level's TOC blob. Class names repeat
// heavily, so each distinct one is only stored once.
func objectSortKeys(toc []byte, offsets []int, ctx *parser.VersionContext) (depths []int32, classes []string, ok bool) {
	var (
		dot   = []byte(".")
		colon = []byte(":")
	)
	r := parser.NewBinaryReader(toc)

	// readFString returns a string's content without copying it. UTF-16 strings
	// are rare in a TOC, so those just take the normal, copying path.
	readFString := func() []byte {
		start := r.Position()
		length := r.ReadInt32()
		if length < 0 {
			r.SetPosition(start)
			return []byte(r.ReadString())
		}
		raw := r.ReadBytes(int(length))
		if i := bytes.IndexByte(raw, 0); i >= 0 {
			raw = raw[:i]
		}
		return raw
	}
	depthOf := func(path []byte) int32 {
		return int32(bytes.Count(path, dot) + bytes.Count(path, colon))
	}

	depths = make([]int32, len(offsets))
	classes = make([]string, len(offsets))
	known := make(map[string]string)
	ok = true
	for i, offset := range offsets {
		r.SetPosition(offset)
		isActor := r.ReadInt32() != 0

		class := readFString()
		name, seen := known[string(class)] // no allocation for the lookup
		if !seen {
			name = string(class)
			known[name] = name
		}
		classes[i] = name

		readFString() // level name
		depths[i] = depthOf(readFString())
		if isActor {
			continue
		}
		if ctx.SaveVersion >= 49 {
			r.ReadUInt32() // object flags
		}
		if outer := readFString(); len(outer) > 0 && depthOf(outer) >= depths[i] {
			ok = false
		}
	}
	return depths, classes, ok
}

// IsOptimizedOrder reports whether a level's objects are already in the order
// OptimizeSave would write them, so optimizing again would gain nothing. A level
// OptimizeSave would leave alone (see objectDepths) counts as optimized too.
func IsOptimizedOrder(headers []parser.ObjectHeader) bool {
	depths, ok := objectDepths(headers)
	if !ok {
		return true
	}
	for i := 1; i < len(headers); i++ {
		if depths[i] < depths[i-1] {
			return false
		}
		if depths[i] == depths[i-1] && headers[i].ClassName < headers[i-1].ClassName {
			return false
		}
	}
	return true
}

// objectDepths returns each object's nesting depth, derived from its path name:
// a subobject's path is its outer's path plus a '.' or ':' separated name, so
// counting separators gives a depth that is always greater than the outer's.
// Returns ok=false if any object breaks that rule; the caller then keeps the
// level's original (game-written) order rather than risk reordering it.
func objectDepths(headers []parser.ObjectHeader) ([]int32, bool) {
	depth := func(path string) int32 {
		return int32(strings.Count(path, ".") + strings.Count(path, ":"))
	}
	depths := make([]int32, len(headers))
	for i, h := range headers {
		depths[i] = depth(h.Reference.PathName)
		if h.OuterPathName != "" && depth(h.OuterPathName) >= depths[i] {
			return nil, false
		}
	}
	return depths, true
}
