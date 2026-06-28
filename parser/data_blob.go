package parser

import (
	"fmt"
	"log"
	"strings"
)

// Property is a single parsed property: its type plus the decoded value.
type Property struct {
	Type  string
	Value interface{}
}

// SaveObject is one object decoded out of the data blob.
type SaveObject struct {
	Index                          int
	Properties                     map[string]Property
	BinarySize                     int32
	DataStartVPos                  int64
	UseCompleteTagType             bool
	SaveCustomVersion              int32
	ShouldMigrateObjectRefsToPersistent bool
	Header                         *ObjectHeader
}

// Properties we don't care about. We still read the header and skip past the
// value bytes, we just never store them.
var skipProperties = map[string]bool{
	"mPrefabTextElementSaveData": true, "mPrefabIconElementSaveData": true,
	"mFogOfWarRawData": true, "mAllowedItemDescriptors": true,
	"mValidationData": true, "mCachedFeetOffset": true,
	"mSpawnData": true, "mNoiseData": true, "mHeightmapData": true,
	"mUpdatedOnDayNr": true, "mFluidDescriptor": true,
	"mAvailableRecipes": true, "mAvailableItemDescriptors": true,
	"mGlobalPointHistoryValues": true, "mItemsFailedToSink": true,
	"mHasInitialized": true, "mBlueprintName": true, "mLocalBounds": true,
	"mStation": true, "mPairedStation": true, "mBuildingTag": true,
	"mLastInsertedFuelType": true, "mLastEditedBy": true, "mLatestDroneTrips": true,
	"mStationName": true, "mDockingStationStatus": true,
	"mHiddenConnections": true, "mNetworkID": true, "mNetworkNodes": true,
	"mPathNetworkTraversabilityChangelist": true, "mShortcuts": true,
	"mArbitrarySlotSizes": true, "mFluidBox": true, "mAdjustedSizeDiff": true,
	"mIsFullBlast": true, "mDesiredFacingDirection": true,
	"mRailroadTrackConnection": true, "mConnectedTo": true,
	"mSlottedInEquipments": true, "mEquipmentInSlot": true, "mActiveEquipmentIndex": true,
	"mRecipeToActivate": true, "mShortcutIndex": true, "mCustomizationRecipeToActivate": true,
	"mSavedFoliageGridSize": true, "mBuildableSubsystem": true,
	"mLightweightBuildableSubsystem": true, "mBuiltWithRecipe": true, "BuiltBy": true,
	"mCustomizationData": true, "mBlueprintProxy": true,
	"mConveyorChainActor": true, "mSnappedPassthroughs": true,
	"mBlueprintDesigner": true, "mTrackGraphID": true,
	"mTimeSinceStartStopProducing": true, "mLastProductivityMeasurementProduceDuration": true,
	"mLastProductivityMeasurementDuration": true, "mCurrentProductivityMeasurementDuration": true,
	"mHasFuelCached": true, "mCurrentFuelAmount": true, "mTopTransform": true,
	"mPendingPotential": true, "mPipeNetworkID": true,
	// Spline metadata: lengths don't need it.
	"mSplineMetadata": true,
}

// Object classes nobody downstream ever reads properties from.
var skipPropertyTypes = []string{
	"WidgetSign", "Build_Beam_Connector", "Build_Beam_Support", "Build_Beam_Cross",
	"Build_Beam_", "PowerPole", "HyperTubeWallSupport", "ConveyorPoleStackable",
	"PipelineFlowIndicator", "VehiclePathNode", "FoundationPassthrough",
	"BP_ResourceDeposit", "BP_BerryBush", "BP_NutBush",
	"BP_CreatureSpawner", "BP_Shroom", "BP_Crystal", "BP_DeadTree", "BP_Flora",
	"BP_BambooTree", "BP_Tree", "BP_Rock",
}

func ShouldSkipAllProperties(className string) bool {
	if className == "" {
		return false
	}
	if strings.Contains(className, "Cable_Cluster") {
		return false
	}
	for _, pattern := range skipPropertyTypes {
		if strings.Contains(className, pattern) {
			return true
		}
	}
	return false
}

// shouldSkipAllProperties is just the lowercase alias used internally.
func shouldSkipAllProperties(className string) bool {
	return ShouldSkipAllProperties(className)
}

// DataBlobParser pulls per-object property data out of a data blob.
type DataBlobParser struct {
	ctx           *VersionContext
	objectHeaders []ObjectHeader
	skipProps     map[string]bool
	mapMode       bool
}

func NewDataBlobParser(ctx *VersionContext, headers []ObjectHeader, mode string) *DataBlobParser {
	skip := make(map[string]bool)
	for k, v := range skipProperties {
		skip[k] = v
	}
	if mode != "MAP" {
		skip["mSplineMetadata"] = true
	}
	return &DataBlobParser{
		ctx:           ctx,
		objectHeaders: headers,
		skipProps:     skip,
		mapMode:       mode == "MAP",
	}
}

// isCompletePropertyTagType reports whether this save uses the newer property
// tag format.
func (d *DataBlobParser) isCompletePropertyTagType() bool {
	return d.ctx.ObjectVersion >= 53 && d.ctx.PackageFileVerUE5 >= 1012
}

// StreamObjects reads objects one at a time off the StreamingReader and fires
// onObject for each one it decodes.
func (d *DataBlobParser) StreamObjects(
	sr *StreamingReader,
	dataBlobLength int64,
	objectCount int,
	onObject func(obj *SaveObject, index int),
) (int, error) {
	// Global header is just countEntities (int32).
	countEntities, err := sr.ReadInt32()
	if err != nil {
		return 0, fmt.Errorf("reading countEntities: %w", err)
	}
	if int(countEntities) != len(d.objectHeaders) {
		log.Printf("  Warning: countEntities %d != objectHeaders.length %d", countEntities, len(d.objectHeaders))
	}

	endPos := sr.Position() + dataBlobLength - 4 // -4: we already read countEntities
	parsed := 0

	for i := 0; i < objectCount; i++ {
		if sr.Position() >= endPos {
			break
		}

		// Per-object header.
		var saveCustomVersion int32
		if d.ctx.LevelVersion >= 38 {
			scv, err := sr.ReadInt32()
			if err != nil {
				return parsed, err
			}
			saveCustomVersion = scv
		}
		d.ctx.ObjectVersion = saveCustomVersion

		var shouldMigrate bool
		if d.ctx.LevelVersion >= 38 {
			v, err := sr.ReadInt32()
			if err != nil {
				return parsed, err
			}
			shouldMigrate = v >= 1
		}

		binarySize, err := sr.ReadInt32()
		if err != nil {
			return parsed, err
		}
		dataStartVPos := sr.Position()

		// Slurp the object's bytes into a small buffer we can parse from.
		objBuffer, err := sr.ReadBytes(int(binarySize))
		if err != nil {
			return parsed, err
		}

		// Version data sits right after the object bytes on >= 53.
		if d.ctx.ObjectVersion >= 53 {
			shouldSerialize, err := sr.ReadInt32()
			if err != nil {
				return parsed, err
			}
			if shouldSerialize == 1 {
				if err := d.readFSaveObjectVersionDataStream(sr); err != nil {
					return parsed, err
				}
			}
			// otherwise we just keep the level's PackageFileVerUE5
		}

		// Now decode the properties out of that buffer.
		objReader := NewBinaryReader(objBuffer)
		properties := make(map[string]Property)

		var className string
		if i < len(d.objectHeaders) {
			className = d.objectHeaders[i].ClassName
		}

		if !shouldSkipAllProperties(className) {
			var header *ObjectHeader
			if i < len(d.objectHeaders) {
				header = &d.objectHeaders[i]
			}
			props, err := d.parseObjectProperties(objReader, int(binarySize), header)
			if err != nil {
				_ = err
			} else {
				properties = props
			}
		}

		obj := &SaveObject{
			Index:                          i,
			Properties:                     properties,
			BinarySize:                     binarySize,
			DataStartVPos:                  dataStartVPos,
			UseCompleteTagType:             d.isCompletePropertyTagType(),
			SaveCustomVersion:              saveCustomVersion,
			ShouldMigrateObjectRefsToPersistent: shouldMigrate,
		}
		if i < len(d.objectHeaders) {
			hdr := d.objectHeaders[i]
			obj.Header = &hdr
		}

		onObject(obj, i)
		parsed++
	}

	return parsed, nil
}

// readFSaveObjectVersionDataStream is the StreamingReader twin of
// BodyParser.readFSaveObjectVersionData.
func (d *DataBlobParser) readFSaveObjectVersionDataStream(sr *StreamingReader) error {
	// saveObjectVersionDataVersion
	if _, err := sr.ReadUInt32(); err != nil {
		return err
	}
	// packageFileVersion: ue4Version + ue5Version
	if _, err := sr.ReadInt32(); err != nil {
		return err
	}
	ue5Ver, err := sr.ReadInt32()
	if err != nil {
		return err
	}
	d.ctx.PackageFileVerUE5 = ue5Ver

	// licenceVersion
	if _, err := sr.ReadInt32(); err != nil {
		return err
	}
	// engineVersion: major(u16) + minor(u16) + patch(u16) + changelist(u32) + branch(FString)
	if _, err := sr.ReadUInt16(); err != nil {
		return err
	}
	if _, err := sr.ReadUInt16(); err != nil {
		return err
	}
	if _, err := sr.ReadUInt16(); err != nil {
		return err
	}
	if _, err := sr.ReadUInt32(); err != nil {
		return err
	}
	if _, err := sr.ReadString(); err != nil {
		return err
	}
	// customVersionContainer
	count, err := sr.ReadInt32()
	if err != nil {
		return err
	}
	for i := int32(0); i < count; i++ {
		for j := 0; j < 4; j++ {
			if _, err := sr.ReadUInt32(); err != nil {
				return err
			}
		}
		if _, err := sr.ReadInt32(); err != nil {
			return err
		}
	}
	return nil
}
