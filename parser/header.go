package parser

import "fmt"

// SaveHeaderType constants
const (
	HeaderInitialVersion          = 0
	HeaderPrepareForLoadingMaps   = 1
	HeaderAddedSessionId          = 2
	HeaderAddedPlayDuration       = 3
	HeaderSessionIDStringAndSave  = 4
	HeaderAddedSessionVisibility  = 5
	HeaderLookAtTheComment        = 6 // checksum field
	HeaderUE425EngineUpdate       = 7 // fEditorObjectVersion
	HeaderAddedModdingParams      = 8
	HeaderUE426EngineUpdate       = 9
	HeaderAddedSaveIdentifier     = 10
	HeaderAddedWorldPartition     = 11
	HeaderAddedSaveModChecksum    = 12
	HeaderAddedIsCreativeMode     = 13
	HeaderAddedSaveName           = 14
)

// SaveVersion constants
const (
	SaveSerializeObjectFlags              = 41
	SaveUnrealEngine5                     = 37
	SaveIntroducedWorldPartition          = 38
	SaveSerializePerStreamableLevelTOC    = 51
	SaveSerializeDataPackageAndCustomVer  = 53
)

// SaveHeader holds the parsed save-file header.
type SaveHeader struct {
	SaveHeaderType    int32
	SaveVersion       int32
	BuildVersion      int32
	SaveName          string
	MapName           string
	MapOptions        string
	SessionName       string
	PlayDurationSeconds int32
	SaveDateTime      string // Unix milliseconds as string
	SessionVisibility uint8
	FEditorObjectVersion int32
	RawModMetadataString string
	IsModdedSave      int32
	SaveIdentifier    string
	PartitionEnabledFlag int32
	CreativeModeEnabled int32
	ExtraField        int32
}

// VersionContext carries the version numbers we branch on while parsing.
type VersionContext struct {
	HeaderVersion  int32
	SaveVersion    int32
	BuildVersion   int32
	LevelVersion   int32
	ObjectVersion  int32
	PackageFileVerUE5 int32
}

func NewVersionContext(header *SaveHeader) *VersionContext {
	return &VersionContext{
		HeaderVersion:  header.SaveHeaderType,
		SaveVersion:    header.SaveVersion,
		BuildVersion:   header.BuildVersion,
		LevelVersion:   header.SaveVersion, // starts equal to the save version
		ObjectVersion:  header.SaveVersion,
		PackageFileVerUE5: 0,
	}
}

func (c *VersionContext) HeaderIsAtLeast(v int32) bool { return c.HeaderVersion >= v }
func (c *VersionContext) SaveIsAtLeast(v int32) bool   { return c.SaveVersion >= v }
func (c *VersionContext) LevelIsAtLeast(v int32) bool  { return c.LevelVersion >= v }
func (c *VersionContext) HasObjectFlags() bool          { return c.SaveIsAtLeast(SaveSerializeObjectFlags) }

// Clone makes a shallow copy.
func (c *VersionContext) Clone() *VersionContext {
	cp := *c
	return &cp
}

// ParseHeader reads and sanity-checks the save header off a BinaryReader.
func ParseHeader(r *BinaryReader) (*SaveHeader, *VersionContext, error) {
	header := &SaveHeader{}

	header.SaveHeaderType = r.ReadInt32()
	header.SaveVersion = r.ReadInt32()
	header.BuildVersion = r.ReadInt32()

	// Sanity-check the version fields before trusting anything else.
	if header.SaveHeaderType < 0 || header.SaveHeaderType > 20 {
		return nil, nil, fmt.Errorf("invalid saveHeaderType: %d", header.SaveHeaderType)
	}
	if header.SaveVersion < 0 || header.SaveVersion > 200 {
		return nil, nil, fmt.Errorf("invalid saveVersion: %d", header.SaveVersion)
	}

	ctx := NewVersionContext(header)

	// Save name (version >= 14)
	if ctx.HeaderIsAtLeast(HeaderAddedSaveName) {
		header.SaveName = r.ReadString()
	}

	// Always present
	header.MapName = r.ReadString()
	header.MapOptions = r.ReadString()
	header.SessionName = r.ReadString()
	header.PlayDurationSeconds = r.ReadInt32()

	// SaveDateTime comes in as Windows FILETIME ticks; convert to Unix ms.
	epochTicks := int64(621355968000000000)
	rawTicks := r.ReadInt64()
	unixMs := (rawTicks - epochTicks) / 10000
	header.SaveDateTime = fmt.Sprintf("%d", unixMs)

	header.SessionVisibility = r.ReadUInt8()

	// Editor object version (version >= 7)
	if ctx.HeaderIsAtLeast(HeaderUE425EngineUpdate) {
		header.FEditorObjectVersion = r.ReadInt32()
	}

	// Modding params (version >= 8)
	if ctx.HeaderIsAtLeast(HeaderAddedModdingParams) {
		header.RawModMetadataString = r.ReadString()
		header.IsModdedSave = r.ReadInt32()
	}

	// Save identifier (version >= 10)
	if ctx.HeaderIsAtLeast(HeaderAddedSaveIdentifier) {
		header.SaveIdentifier = r.ReadString()
	}

	// World partition (version >= 11)
	if ctx.HeaderIsAtLeast(HeaderAddedWorldPartition) {
		header.PartitionEnabledFlag = r.ReadInt32()
	}

	// Quirk: v13 puts the creative-mode flag before the checksum.
	if header.SaveHeaderType == 13 {
		header.CreativeModeEnabled = r.ReadInt32()
	}

	// Checksum (version >= 6)
	if ctx.HeaderIsAtLeast(HeaderLookAtTheComment) {
		r.ReadBytes(16) // 16-byte consistency hash
	}

	// ...and v13 also tacks an extra int32 on after the checksum.
	if header.SaveHeaderType == 13 {
		header.ExtraField = r.ReadInt32()
	}

	// Creative mode (version >= 7), except v13 which we already handled.
	if ctx.HeaderIsAtLeast(HeaderAddedIsCreativeMode) && header.SaveHeaderType != 13 {
		header.CreativeModeEnabled = r.ReadInt32()
	}

	// v14 has one more int32 here.
	if header.SaveHeaderType == 14 {
		header.ExtraField = r.ReadInt32()
	}

	return header, ctx, nil
}
