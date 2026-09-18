package extraction

import (
	"satisfacts/parser"
	"strings"
)

// stripLastComponent removes the last ".ComponentName" suffix from an
// instance name. E.g. "PersistentLevel.Build_X_123.PowerInfo" -> "PersistentLevel.Build_X_123".
// Returns the original string if there is no dot.
func stripLastComponent(instanceName string) string {
	idx := strings.LastIndex(instanceName, ".")
	if idx <= 0 {
		return instanceName
	}
	return instanceName[:idx]
}

// BuildingFacts holds extracted building data for post-extraction, replacing
// the need to retain raw *SaveObject pointers.
type BuildingFacts struct {
	TypePath     string
	Translation  [3]float32
	Rotation     [4]float32 // quaternion (x, y, z, w)
	HasTransform bool
	HasRotation  bool

	// Clock speed / production boost
	CurrentPotential          float32
	HasCurrentPotential       bool
	PendingPotential          float32
	HasPendingPotential       bool
	CurrentProductionBoost    float32
	HasCurrentProductionBoost bool

	// Recipe (ObjectRef pathName)
	CurrentRecipe string

	// Inventory refs (ObjectRef pathNames)
	InputInventory     string
	OutputInventory    string
	InventoryPotential string
	FuelInventory      string

	// Power
	PowerInfoRef           string
	IsProducing            bool
	HasIsProducing         bool
	IsProductionPaused     bool
	HasProductivityMonitor bool
	HasProductionDuration  bool

	// Productivity measurement (in-game efficiency)
	LastProductivityProduceDuration float32
	LastProductivityDuration        float32
	HasLastProductivity             bool

	// Extractor
	ExtractableResource    string
	CurrentExtractProgress float32
	HasExtractProgress     bool

	// Geothermal
	VariablePowerCycleOffset float32
	HasVariablePowerOffset   bool

	// Nuclear
	WasteLeftFromCurrentFuel  float32
	CurrentFuelAmount         float32
	CurrentSupplementalAmount float32
	CurrentFuelClass          string

	// APA
	HasFuelCached bool

	// Power storage
	PowerStore    float32
	HasPowerStore bool

	// Power info component fields (inlined from FGPowerInfoComponent)
	TargetConsumption            float32
	HasTargetConsumption         bool
	DynamicProductionCapacity    float32
	HasDynamicProductionCapacity bool
	BaseProduction               float32
	HasBaseProduction            bool
	PowerInfoPotential           float32
	HasPowerInfoPotential        bool

	// Power connection component fields
	WireCount int

	// Which power circuit this building ended up on. Filled in after
	// streaming, see ResolvePowerCircuits. Needed because train stations and
	// freight platforms are powered off the third rail instead of a wire, so
	// WireCount is always 0 on them and tells you nothing.
	CircuitID           int32
	HasCircuit          bool
	CircuitHasGenerator bool

	// InventoryPotential fallback (for Ficsonium nuclear plants)
	InventoryPotentialValue    float32
	HasInventoryPotentialValue bool
}

// ResourceNodeFacts covers ResourceNode, ResourceSource, FrackingSatellite, ResourceWell.
type ResourceNodeFacts struct {
	ResourceClassOverride string
	PurityOverride        string
}

func populateResourceNodeFacts(obj *parser.SaveObject) *ResourceNodeFacts {
	rf := &ResourceNodeFacts{}
	rf.ResourceClassOverride, _ = GetPropObjectRefPathName(obj, "mResourceClassOverride")
	rf.PurityOverride, _ = GetPropString(obj, "mPurityOverride")
	return rf
}

// populateBuildingFacts extracts post-extraction-relevant properties into a BuildingFacts.
func populateBuildingFacts(obj *parser.SaveObject) *BuildingFacts {
	bf := &BuildingFacts{}

	if obj.Header != nil {
		bf.TypePath = obj.Header.ClassName
		if obj.Header.NeedTransform {
			bf.Translation = obj.Header.Transform.Translation
			bf.HasTransform = true
			bf.Rotation = obj.Header.Transform.Rotation
			bf.HasRotation = true
		}
	}

	// Clock speed / production boost
	if v, ok := GetPropFloat32(obj, "mCurrentPotential"); ok {
		bf.CurrentPotential = v
		bf.HasCurrentPotential = true
	}
	if v, ok := GetPropFloat32(obj, "mPendingPotential"); ok {
		bf.PendingPotential = v
		bf.HasPendingPotential = true
	}
	if v, ok := GetPropFloat32(obj, "mCurrentProductionBoost"); ok {
		bf.CurrentProductionBoost = v
		bf.HasCurrentProductionBoost = true
	}

	// Recipe
	bf.CurrentRecipe, _ = GetPropObjectRefPathName(obj, "mCurrentRecipe")

	// Inventory refs
	bf.InputInventory, _ = GetPropObjectRefPathName(obj, "mInputInventory")
	bf.OutputInventory, _ = GetPropObjectRefPathName(obj, "mOutputInventory")
	bf.InventoryPotential, _ = GetPropObjectRefPathName(obj, "mInventoryPotential")
	bf.FuelInventory, _ = GetPropObjectRefPathName(obj, "mFuelInventory")

	// Power
	bf.PowerInfoRef, _ = GetPropObjectRefPathName(obj, "mPowerInfo")
	if v, ok := GetPropBool(obj, "mIsProducing"); ok {
		bf.IsProducing = v
		bf.HasIsProducing = true
	}
	if v, ok := GetPropBool(obj, "mIsProductionPaused"); ok {
		bf.IsProductionPaused = v
	}
	if _, ok := obj.Properties["mProductivityMonitorEnabled"]; ok {
		bf.HasProductivityMonitor = true
	}
	if _, ok := obj.Properties["mCurrentProductivityMeasurementProduceDuration"]; ok {
		bf.HasProductionDuration = true
	}

	// Productivity measurement (in-game efficiency)
	if v, ok := GetPropFloat32(obj, "mLastProductivityMeasurementProduceDuration"); ok {
		bf.LastProductivityProduceDuration = v
		bf.HasLastProductivity = true
	}
	if v, ok := GetPropFloat32(obj, "mLastProductivityMeasurementDuration"); ok {
		bf.LastProductivityDuration = v
	}

	// Extractor
	bf.ExtractableResource, _ = GetPropObjectRefPathName(obj, "mExtractableResource")
	if v, ok := GetPropFloat32(obj, "mCurrentExtractProgress"); ok {
		bf.CurrentExtractProgress = v
		bf.HasExtractProgress = true
	}

	// Geothermal fluctuation phase offset
	if v, ok := GetPropFloat32(obj, "mVariablePowerProductionCycleOffset"); ok {
		bf.VariablePowerCycleOffset = v
		bf.HasVariablePowerOffset = true
	}

	// Nuclear
	bf.WasteLeftFromCurrentFuel, _ = GetPropFloat32(obj, "mWasteLeftFromCurrentFuel")
	bf.CurrentFuelAmount, _ = GetPropFloat32(obj, "mCurrentFuelAmount")
	bf.CurrentSupplementalAmount, _ = GetPropFloat32(obj, "mCurrentSupplementalAmount")
	bf.CurrentFuelClass, _ = GetPropObjectRefPathName(obj, "mCurrentFuelClass")

	// APA
	if v, ok := GetPropBool(obj, "mHasFuelCached"); ok {
		bf.HasFuelCached = v
	}

	// Power storage
	if v, ok := GetPropFloat32(obj, "mPowerStore"); ok {
		bf.PowerStore = v
		bf.HasPowerStore = true
	}

	return bf
}
