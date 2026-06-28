package parser

import (
	"fmt"
	"log"
)

// Body is the top-level metadata we pull out of the body structure.
type Body struct {
	TotalSize      int32
	SublevelCount  int32
	LevelCount     int32
}

// ObjectRef is an FObjectReferenceDisc
type ObjectRef struct {
	LevelName string
	PathName  string
}

// Transform3f holds rotation, position and scale, all float32.
type Transform3f struct {
	Rotation    [4]float32 // x, y, z, w (quaternion)
	Translation [3]float32 // x, y, z (position in cm)
	Scale       [3]float32 // x, y, z
}

// ObjectHeader is one object's header straight out of the TOC blob.
type ObjectHeader struct {
	Type           string // "Actor" or "Object"
	ClassName      string
	Reference      ObjectRef
	ObjectFlags    uint32 // if SaveVersion >= 49
	NeedTransform  bool
	Transform      Transform3f
	WasPlacedInLevel bool
	OuterPathName  string // only for Object type
}

// Level bundles a level's TOC headers with its data-blob info.
type Level struct {
	Name             string
	IsPersistent     bool
	TOCBlobLength    int64
	DataBlobLength   int64
	ObjectHeaderCount int32
	ObjectHeaders    []ObjectHeader
	SaveVersion      int32
	// Collectables1 is the list of collected/dismantled collectible
	// references embedded in the TOC blob after the object headers.
	// These are ObjectReferences to actors that were picked up by the
	// player (power slugs, somersloops, mercer spheres, crash sites).
	Collectables1    []ObjectRef
}

// BodyParser walks the decompressed save body.
type BodyParser struct {
	reader *StreamingReader
	ctx    *VersionContext
}

func NewBodyParser(reader *StreamingReader, ctx *VersionContext) *BodyParser {
	return &BodyParser{reader: reader, ctx: ctx}
}

// ParseBody reads the body's leading fields and returns the metadata.
func (b *BodyParser) ParseBody() (*Body, error) {
	body := &Body{}

	// 1. totalBodyRestSize (int32)
	totalSize, err := b.reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("reading totalSize: %w", err)
	}
	body.TotalSize = totalSize

	// 2. UE5 zero (int32), only from version 37 on.
	if b.ctx.SaveIsAtLeast(SaveUnrealEngine5) {
		ue5zero, err := b.reader.ReadInt32()
		if err != nil {
			return nil, fmt.Errorf("reading UE5 zero: %w", err)
		}
		_ = ue5zero
	}

	// 3. FSaveObjectVersionData, only from version 53 on.
	if b.ctx.SaveIsAtLeast(SaveSerializeDataPackageAndCustomVer) {
		if err := b.readFSaveObjectVersionData(); err != nil {
			return nil, fmt.Errorf("reading version data: %w", err)
		}
	}

	// 4. SaveBodyValidation, only from version 38 on.
	if b.ctx.SaveIsAtLeast(SaveIntroducedWorldPartition) {
		if err := b.readSaveBodyValidation(); err != nil {
			return nil, fmt.Errorf("reading body validation: %w", err)
		}
	}

	// 5. sublevelCount (int32)
	sublevelCount, err := b.reader.ReadInt32()
	if err != nil {
		return nil, fmt.Errorf("reading sublevelCount: %w", err)
	}
	body.SublevelCount = sublevelCount
	body.LevelCount = sublevelCount + 1

	return body, nil
}

// readFSaveObjectVersionData walks the version block (version >= 53). We don't
// keep most of it, just the UE5 package version.
func (b *BodyParser) readFSaveObjectVersionData() error {
	// saveObjectVersionDataVersion
	if _, err := b.reader.ReadUInt32(); err != nil {
		return err
	}
	// packageFileVersion: ue4Version + ue5Version
	ue4Ver, err := b.reader.ReadInt32()
	if err != nil {
		return err
	}
	ue5Ver, err := b.reader.ReadInt32()
	if err != nil {
		return err
	}
	_ = ue4Ver
	b.ctx.PackageFileVerUE5 = ue5Ver

	// licenceVersion
	if _, err := b.reader.ReadInt32(); err != nil {
		return err
	}
	// engineVersion: major(u16) + minor(u16) + patch(u16) + changelist(u32) + branch(FString)
	if _, err := b.reader.ReadUInt16(); err != nil {
		return err
	}
	if _, err := b.reader.ReadUInt16(); err != nil {
		return err
	}
	if _, err := b.reader.ReadUInt16(); err != nil {
		return err
	}
	if _, err := b.reader.ReadUInt32(); err != nil {
		return err
	}
	if _, err := b.reader.ReadString(); err != nil {
		return err
	}
	// customVersionContainer: count + versions
	count, err := b.reader.ReadInt32()
	if err != nil {
		return err
	}
	for i := int32(0); i < count; i++ {
		// GUID: 4 x uint32
		for j := 0; j < 4; j++ {
			if _, err := b.reader.ReadUInt32(); err != nil {
				return err
			}
		}
		// version
		if _, err := b.reader.ReadInt32(); err != nil {
			return err
		}
	}
	return nil
}

// readSaveBodyValidation skips past the world-partition validation grid
// (version >= 38). We only need to consume it, not store it.
func (b *BodyParser) readSaveBodyValidation() error {
	count, err := b.reader.ReadInt32()
	if err != nil {
		return err
	}
	for i := int32(0); i < count; i++ {
		if _, err := b.reader.ReadString(); err != nil {
			return err
		}
		if _, err := b.reader.ReadInt32(); err != nil { // cellSize
			return err
		}
		if _, err := b.reader.ReadUInt32(); err != nil { // gridHash
			return err
		}
		childrenCount, err := b.reader.ReadUInt32()
		if err != nil {
			return err
		}
		for j := uint32(0); j < childrenCount; j++ {
			if _, err := b.reader.ReadString(); err != nil {
				return err
			}
			if _, err := b.reader.ReadUInt32(); err != nil { // cellHash
				return err
			}
		}
	}
	return nil
}

// StreamLevels walks every level (the sublevels plus the persistent one). For
// each one it parses the TOC blob for object headers, then calls onDataBlob
// with the reader sitting right at the data blob. Your callback has to consume
// exactly dataBlobLength bytes.
func (b *BodyParser) StreamLevels(sublevelCount int32, onDataBlob func(sr *StreamingReader, dataBlobLength int64, level *Level) error) ([]*Level, error) {
	var levels []*Level

	// Sublevels
	for i := int32(0); i < sublevelCount; i++ {
		level, err := b.readLevelStreaming(false, onDataBlob)
		if err != nil {
			return nil, fmt.Errorf("sublevel %d: %w", i, err)
		}
		levels = append(levels, level)
	}

	// Persistent level comes last and has no name field.
	persistentLevel, err := b.readLevelStreaming(true, onDataBlob)
	if err != nil {
		return nil, fmt.Errorf("persistent level: %w", err)
	}
	levels = append(levels, persistentLevel)

	return levels, nil
}

// readLevelStreaming reads one level: parse its TOC headers, then hand the data
// blob off to the callback.
func (b *BodyParser) readLevelStreaming(isPersistent bool, onDataBlob func(sr *StreamingReader, dataBlobLength int64, level *Level) error) (*Level, error) {
	level := &Level{
		IsPersistent: isPersistent,
	}

	if !isPersistent {
		name, err := b.reader.ReadString()
		if err != nil {
			return nil, fmt.Errorf("reading level name: %w", err)
		}
		level.Name = name
	} else {
		level.Name = "Persistent_Level"
	}
	// TOC Blob: int64 length + data
	tocLen, err := b.reader.ReadInt64()
	if err != nil {
		return nil, fmt.Errorf("reading TOC blob length: %w", err)
	}
	level.TOCBlobLength = tocLen

	tocData, err := b.reader.ReadBytes(int(tocLen))
	if err != nil {
		return nil, fmt.Errorf("reading TOC blob data: %w", err)
	}

	// Parse the TOC headers now while we have the bytes.
	tocReader := NewBinaryReader(tocData)
	objCount := tocReader.ReadInt32()
	level.ObjectHeaderCount = objCount

	if objCount > 0 {
		headers, err := ParseObjectHeaders(tocReader, b.ctx, int(objCount))
		if err != nil {
			return nil, fmt.Errorf("parsing object headers: %w", err)
		}
		level.ObjectHeaders = headers
	}

	// For the persistent level, the TOC blob has a persistent flag and
	// "Persistent_Level" string after the object headers.
	if isPersistent && tocReader.Remaining() >= 4 {
		persistentFlag := tocReader.ReadInt32()
		if persistentFlag != 0 && tocReader.Remaining() >= 4 {
			// Read and discard the "Persistent_Level" string.
			tocReader.ReadString()
		}
	}

	// Collectables #1: if there are remaining bytes in the TOC blob,
	// they contain the list of collected/dismantled collectibles.
	if tocReader.Remaining() >= 4 {
		collectablesCount := tocReader.ReadInt32()
		if collectablesCount > 0 && collectablesCount <= 1000000 {
			collectables := make([]ObjectRef, 0, collectablesCount)
			for i := int32(0); i < collectablesCount; i++ {
				ln := tocReader.ReadString()
				pn := tocReader.ReadString()
				collectables = append(collectables, ObjectRef{LevelName: ln, PathName: pn})
			}
			level.Collectables1 = collectables
		}
	}
	// tocData can be collected from here on.

	// Data blob: int64 length followed by the data, streamed via the callback.
	dataLen, err := b.reader.ReadInt64()
	if err != nil {
		return nil, fmt.Errorf("reading data blob length: %w", err)
	}
	level.DataBlobLength = dataLen

	if dataLen > 0 && onDataBlob != nil {
		if err := onDataBlob(b.reader, dataLen, level); err != nil {
			return nil, fmt.Errorf("data blob callback: %w", err)
		}
	} else {
		if err := b.reader.Skip(int(dataLen)); err != nil {
			return nil, fmt.Errorf("skipping data blob: %w", err)
		}
	}

	// And the footer after it.
	if err := b.readLevelFooter(level, isPersistent); err != nil {
		return nil, fmt.Errorf("reading level footer: %w", err)
	}

	return level, nil
}

// readLevelFooter reads the save version and the collectables/destroyed-actors
// lists that follow the data blob.
func (b *BodyParser) readLevelFooter(level *Level, isPersistent bool) error {
	// Per-level save version (>= 51, sublevels only).
	if !isPersistent && b.ctx.SaveIsAtLeast(SaveSerializePerStreamableLevelTOC) {
		sv, err := b.reader.ReadInt32()
		if err != nil {
			return err
		}
		level.SaveVersion = sv
		b.ctx.LevelVersion = sv
	}

	if isPersistent {
		// Persistent level carries the destroyed-actors map.
		mapCount, err := b.reader.ReadInt32()
		if err != nil {
			return err
		}
		if mapCount < 0 || mapCount > 1000000 {
			log.Printf("Warning: Suspicious destroyed actors count: %d", mapCount)
		} else {
			for i := int32(0); i < mapCount; i++ {
				if _, err := b.reader.ReadString(); err != nil {
					return err
				}
				arrayCount, err := b.reader.ReadInt32()
				if err != nil {
					return err
				}
				if arrayCount < 0 || arrayCount > 100000 {
					continue
				}
				for j := int32(0); j < arrayCount; j++ {
					if _, err := b.readObjectRef(); err != nil {
						return err
					}
				}
			}
		}
	} else {
		// Sublevels carry collectables instead.
		collectablesCount, err := b.reader.ReadInt32()
		if err != nil {
			return err
		}
		if collectablesCount >= 0 && collectablesCount <= 1000000 {
			for i := int32(0); i < collectablesCount; i++ {
				if _, err := b.readObjectRef(); err != nil {
					return err
				}
			}
		}

		// Version data again on >= 53.
		if b.ctx.SaveIsAtLeast(SaveSerializeDataPackageAndCustomVer) {
			shouldSerialize, err := b.reader.ReadInt32()
			if err != nil {
				return err
			}
			if shouldSerialize >= 1 {
				if err := b.readFSaveObjectVersionData(); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (b *BodyParser) readObjectRef() (ObjectRef, error) {
	levelName, err := b.reader.ReadString()
	if err != nil {
		return ObjectRef{}, err
	}
	pathName, err := b.reader.ReadString()
	if err != nil {
		return ObjectRef{}, err
	}
	return ObjectRef{LevelName: levelName, PathName: pathName}, nil
}

// ParseObjectHeaders reads every object header out of a TOC blob.
func ParseObjectHeaders(r *BinaryReader, ctx *VersionContext, count int) ([]ObjectHeader, error) {
	headers := make([]ObjectHeader, 0, count)

	for i := 0; i < count; i++ {
		isActor := r.ReadInt32() != 0

		if isActor {
			h, err := parseActorHeader(r, ctx)
			if err != nil {
				return nil, err
			}
			headers = append(headers, h)
		} else {
			h, err := parseObjectHeader(r, ctx)
			if err != nil {
				return nil, err
			}
			headers = append(headers, h)
		}
	}

	return headers, nil
}

// ParseObjectHeaderOffsets is like ParseObjectHeaders but, instead of the
// headers themselves, returns where each one starts and how big it is within
// the TOC blob (measured after the countEntities field).
func ParseObjectHeaderOffsets(r *BinaryReader, ctx *VersionContext, count int) ([]int, []int, error) {
	offsets := make([]int, 0, count)
	sizes := make([]int, 0, count)

	for i := 0; i < count; i++ {
		start := r.Position()

		isActor := r.ReadInt32() != 0
		if isActor {
			if _, err := parseActorHeader(r, ctx); err != nil {
				return nil, nil, err
			}
		} else {
			if _, err := parseObjectHeader(r, ctx); err != nil {
				return nil, nil, err
			}
		}

		end := r.Position()
		offsets = append(offsets, start)
		sizes = append(sizes, end-start)
	}

	return offsets, sizes, nil
}

func parseObjectBaseSaveHeader(r *BinaryReader, ctx *VersionContext) (string, ObjectRef, uint32, error) {
	className := r.ReadString()
	ref := ObjectRef{
		LevelName: r.ReadString(),
		PathName:  r.ReadString(),
	}
	var flags uint32
	if ctx.SaveVersion >= 49 {
		flags = r.ReadUInt32()
	}
	return className, ref, flags, nil
}

func parseActorHeader(r *BinaryReader, ctx *VersionContext) (ObjectHeader, error) {
	className, ref, flags, err := parseObjectBaseSaveHeader(r, ctx)
	if err != nil {
		return ObjectHeader{}, err
	}

	needTransform := r.ReadInt32() != 0
	transform := parseTransform3f(r)
	wasPlaced := r.ReadInt32() != 0

	return ObjectHeader{
		Type:             "Actor",
		ClassName:        className,
		Reference:        ref,
		ObjectFlags:      flags,
		NeedTransform:    needTransform,
		Transform:        transform,
		WasPlacedInLevel: wasPlaced,
	}, nil
}

func parseObjectHeader(r *BinaryReader, ctx *VersionContext) (ObjectHeader, error) {
	className, ref, flags, err := parseObjectBaseSaveHeader(r, ctx)
	if err != nil {
		return ObjectHeader{}, err
	}
	outerPath := r.ReadString()

	return ObjectHeader{
		Type:          "Object",
		ClassName:     className,
		Reference:     ref,
		ObjectFlags:   flags,
		OuterPathName: outerPath,
	}, nil
}

func parseTransform3f(r *BinaryReader) Transform3f {
	return Transform3f{
		Rotation: [4]float32{
			r.ReadFloat32(), r.ReadFloat32(), r.ReadFloat32(), r.ReadFloat32(),
		},
		Translation: [3]float32{
			r.ReadFloat32(), r.ReadFloat32(), r.ReadFloat32(),
		},
		Scale: [3]float32{
			r.ReadFloat32(), r.ReadFloat32(), r.ReadFloat32(),
		},
	}
}
