package extraction

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"satisfacts/parser"
)

// ExtractedData is the running tally we build up while streaming a save.
type ExtractedData struct {
	// Buildings
	Buildings         map[string]int   // typePath -> count
	BuildingInstances map[string]string // instanceName -> typePath
	Blueprints        map[string]int   // typePath -> count

	// Production / Transport
	Belts             map[string]*BeltTypeStats
	Lifts             map[string]*BeltTypeStats
	Pipes             map[string]*PipeTypeStats
	PowerLines        []float64
	Hypertubes        []float64
	Elevators         []float64
	Rails             RailStats
	Trains            TrainStats
	Drones            DroneStats
	Vehicles          VehicleStats
	Mergers           int
	Splitters         int
	SmartSplitters    int
	ProgrammableSplitters int
	PowerPoles        int
	HypertubeJunctions int
	LiftFloorHoles    int
	BeltWallHoles     int
	PipeFloorHoles    int
	BeltHeads         map[string]int
	PipePumps         map[string]int
	ItemsInTransit    map[string]int
	RailComponents    map[string]int
	ElevatorComponents map[string][]string
	Others            map[string]int

	// Power grid
	PowerGrid PowerGridData

	// Structures (populated only for QUICK/DEEP/MAP modes)
	Structures *StructureData

	// Retained objects for post-streaming extraction
	RetainedObjects  []*parser.SaveObject
	RetainedObjectMap map[string]*parser.SaveObject

	// Resolved inventory stacks per instance (avoids retaining raw mInventoryStacks)
	InventoryStacks map[string][]InventoryStack

	BuildingFacts map[string]*BuildingFacts // keyed by instance name

	powerInfoToBuilding map[string]string // PowerInfo instance -> parent building instance

	ResourceNodeFacts map[string]*ResourceNodeFacts

	InventoryPotentialMap map[string]float32 // mCurrentPotential, fallback for nuclear plants

	// Default clock speeds for buildings without properties (100%)
	DefaultClockSpeeds map[string]string // instanceName -> typePath

	// Building connections extracted during streaming (not retained)
	BuildingConnections map[string]BuildingConnection

	// Position lookup for all objects (instanceName -> translation in cm)
	PositionMap map[string][3]float32

	// Vehicle path segments with lengths — calculated during streaming
	VehiclePaths []float64

	// Item pickups (scattered items on the map)
	Pickups []PickupItem

	// Collectibles found in save (not yet collected) — instance names present in save
	CollectiblesFound CollectiblesFoundData

	// Collectibles collected by player — from collectables1 list in TOC blob
	CollectedCollectibles CollectedCollectiblesData

	// Pets (tamed creatures)
	Pets PetData

	// Totals
	TotalProcessed int

	// MAP mode flag — enables per-entry position/rotation extraction
	mapMode bool
}

// PetData tracks tamed creatures (Lizard Doggos etc.)
type PetData struct {
	TamedDoggos int `json:"tamedDoggos"`
	TotalDoggos int `json:"totalDoggos"`
}

type PickupItem struct {
	ItemClass string  `json:"item_class"`
	ItemName  string  `json:"item_name"`
	NumItems  int     `json:"num_items"`
	Position  [3]float32 `json:"position"`
	Collected bool    `json:"collected"`
}

// CollectiblesFoundData tracks collectibles still in the save (not yet picked up).
type CollectiblesFoundData struct {
	PowerSlugsBlue    map[string]bool
	PowerSlugsYellow  map[string]bool
	PowerSlugsPurple  map[string]bool
	Somersloops       map[string]bool
	MercerSpheres     map[string]bool
	CrashSites        map[string]bool // all crash sites still in save
	CrashSiteOpened   map[string]bool // crash sites with mIsOpened=true
	CrashSiteUnopened map[string]bool // crash sites with mIsOpened=false or no mIsOpened
}

// CollectedCollectiblesData tracks collectibles the player has collected,
// from the collectables1 list in each level's TOC blob.
type CollectedCollectiblesData struct {
	PowerSlugsBlue   map[string]bool
	PowerSlugsYellow map[string]bool
	PowerSlugsPurple map[string]bool
	Somersloops      map[string]bool
	MercerSpheres    map[string]bool
	CrashSites       map[string]bool
}

type BeltTypeStats struct {
	Count       int
	TotalLength float64
}

type PipeTypeStats struct {
	Count       int
	TotalLength float64
}

type RailStats struct {
	TotalLength float64
	Count       int
}

type TrainStats struct {
	Locomotives int
	Wagons      int
	Stations    int
}

type DroneStats struct {
	Stations        int
	FreightPlatforms int
}

type VehicleStats struct {
	Trucks    int
	Explorers int
	Tractors  int
	Cykles    int
}

type PowerGridData struct {
	Circuits      []CircuitInfo
	ComponentCount int
	LineCount      int
}

type CircuitInfo struct {
	ID             int32
	ComponentCount int
}

type StructureData struct {
	Details              map[string]map[string]int `json:"details"`
	Totals               map[string]int            `json:"totals"`
	lightweightCounts    map[string]map[string]int // category -> typePath -> count (from lightweight subsystem)
	sublevelCounts       map[string]map[string]int // category -> typePath -> count (individual objects NOT from Persistent_Level)
	persistentLevelTypes map[string]bool           // typePaths that appear as individual Persistent_Level objects
	mapEntries           map[string]map[string][]StructureEntry // category -> typePath -> entries (MAP mode only)
}

type StructureEntry struct {
	InstanceName string       `json:"instanceName,omitempty"`
	TypePath     string       `json:"typePath"`
	Position     []float64    `json:"position"`
	Rotation     *RotationData `json:"rotation,omitempty"`
	Length       float64      `json:"length,omitempty"`
}

type RotationData struct {
	Pitch float64 `json:"pitch"`
	Yaw   float64 `json:"yaw"`
	Roll  float64 `json:"roll"`
}

// NewExtractedData returns an ExtractedData with all its maps ready to go.
func NewExtractedData(extractStructures bool) *ExtractedData {
	return NewExtractedDataWithMode(extractStructures, false)
}

func NewExtractedDataWithMode(extractStructures bool, mapMode bool) *ExtractedData {
	e := &ExtractedData{
		Buildings:         make(map[string]int),
		BuildingInstances: make(map[string]string),
		Blueprints:        make(map[string]int),
		Belts:             make(map[string]*BeltTypeStats),
		Lifts:             make(map[string]*BeltTypeStats),
		Pipes:             make(map[string]*PipeTypeStats),
		BeltHeads:         make(map[string]int),
		PipePumps:         make(map[string]int),
		ItemsInTransit:    make(map[string]int),
		RailComponents:    make(map[string]int),
		ElevatorComponents: make(map[string][]string),
		Others:            make(map[string]int),
		RetainedObjectMap: make(map[string]*parser.SaveObject),
		InventoryStacks:   make(map[string][]InventoryStack),
		BuildingFacts:     make(map[string]*BuildingFacts),
		powerInfoToBuilding: make(map[string]string),
		ResourceNodeFacts: make(map[string]*ResourceNodeFacts),
		InventoryPotentialMap: make(map[string]float32),
		DefaultClockSpeeds: make(map[string]string),
		BuildingConnections: make(map[string]BuildingConnection),
		PositionMap:       make(map[string][3]float32),
		CollectiblesFound: CollectiblesFoundData{
			PowerSlugsBlue:   make(map[string]bool),
			PowerSlugsYellow: make(map[string]bool),
			PowerSlugsPurple:  make(map[string]bool),
			Somersloops:       make(map[string]bool),
			MercerSpheres:     make(map[string]bool),
			CrashSites:        make(map[string]bool),
			CrashSiteOpened:   make(map[string]bool),
			CrashSiteUnopened: make(map[string]bool),
		},
		CollectedCollectibles: CollectedCollectiblesData{
			PowerSlugsBlue:   make(map[string]bool),
			PowerSlugsYellow: make(map[string]bool),
			PowerSlugsPurple: make(map[string]bool),
			Somersloops:      make(map[string]bool),
			MercerSpheres:    make(map[string]bool),
			CrashSites:       make(map[string]bool),
		},
		mapMode:           mapMode,
	}
	if extractStructures {
		e.Structures = &StructureData{
			Details:              make(map[string]map[string]int),
			Totals:               make(map[string]int),
			lightweightCounts:    make(map[string]map[string]int),
			sublevelCounts:       make(map[string]map[string]int),
			persistentLevelTypes: make(map[string]bool),
		}
		if mapMode {
			e.Structures.mapEntries = make(map[string]map[string][]StructureEntry)
		}
	}
	return e
}

var beltMkRegex = regexp.MustCompile(`Build_ConveyorBelt(Mk\d+)_C`)
var liftMkRegex = regexp.MustCompile(`Build_ConveyorLift(Mk\d+)_C`)
var pipePumpRegex = regexp.MustCompile(`Build_PipePump(Mk\d+)?_C`)

// shouldRetainForPostExtraction decides whether to keep an object for the second pass.
func shouldRetainForPostExtraction(typePath, typeLower string, hasProperties bool) bool {
	if !hasProperties {
		return false
	}

	// Already handled during streaming, no need to keep it.
	if strings.Contains(typeLower, "fglightweightbuildable") {
		return false
	}

	// Check before isInternalSystem — FGCentralStorageSubsystem would match "fgcentral".
	if strings.Contains(typeLower, "subsystem") ||
		strings.Contains(typeLower, "gamestate") ||
		strings.Contains(typeLower, "phasemanager") ||
		strings.Contains(typeLower, "schematicmanager") ||
		strings.Contains(typeLower, "researchmanager") {
		return true
	}

	// Buildings, power info, connections, and resource nodes are all
	// resolved into BuildingFacts/ResourceNodeFacts during streaming.
	if strings.Contains(typeLower, "inventorycomponent") {
		return false
	}

	return false
}

// Properties kept on retained objects (rest is stripped during streaming).
var essentialPostExtractionProps = map[string]bool{
	// Clock speed / production
	"mCurrentPotential": true,
	"mCurrentProductionBoost": true,
	// Recipe
	"mCurrentRecipe": true,
	// Inventory refs
	"mInputInventory": true,
	"mOutputInventory": true,
	"mInventoryPotential": true,
	"mFuelInventory": true,
	// Power
	"mPowerInfo": true,
	"mIsProducing": true,
	"mIsProductionPaused": true,
	"mProductivityMonitorEnabled": true,
	"mCurrentProductivityMeasurementProduceDuration": true,
	// Extractor
	"mExtractableResource": true,
	"mCurrentExtractProgress": true,
	// Nuclear
	"mWasteLeftFromCurrentFuel": true,
	"mCurrentFuelAmount": true,
	"mCurrentSupplementalAmount": true,
	"mCurrentFuelClass": true,
	// APA
	"mHasFuelCached": true,
	// Inventory stacks
	"mInventoryStacks": true,
	// Connections
	"mConnectedComponent": true,
	// Power info
	"mTargetConsumption": true,
	"mDynamicProductionCapacity": true,
	"mBaseProduction": true,
	// Power connection
	"mWires": true,
	// Power storage
	"mPowerStore": true,
	// Subsystems / collectibles
	"mUnlockedTapes": true,
	"mLootedDropPods": true,
	"mDestroyedPickups": true,
	"mStoredItems": true,
	"mCurrentPointLevels": true,
	"mNumResourceSinkCoupons": true,
	// Game state / managers
	"mGamePhaseManager": true,
	"mSchematicManager": true,
	"mResearchManager": true,
	"mResourceSinkSubsystem": true,
	"mCurrentGamePhase": true,
	"mTargetGamePhase": true,
	"mPurchasedSchematics": true,
	"mLastActiveSchematic": true,
	"mUnlockedResearchTrees": true,
	"mIsActivated": true,
	"mLastUsedHardDriveID": true,
	// Custom map settings
	"mNodePuritySettings": true,
	"mNodeRandomization": true,
	"mNodeRandomizationSeed": true,
	"mPartsCostMultiplier": true,
	"mSpacePartsCostMultiplier": true,
	"mPowerConsumptionMultiplier": true,
	// Resource nodes
	"mPurityOverride": true,
	"mResourceClassOverride": true,
	"mResourceType": true,
	// Special (lightweight buildable, conveyor chain — usually not retained but kept for safety)
	"__special__": true,
}

// CleanupAfterStreaming frees maps only needed during ProcessObject.
func (e *ExtractedData) CleanupAfterStreaming() {
	e.powerInfoToBuilding = nil
	e.PositionMap = nil
}

// CleanupAfterPostExtraction frees maps only needed by RunPostExtraction.
func (e *ExtractedData) CleanupAfterPostExtraction() {
	e.DefaultClockSpeeds = nil
	e.BuildingInstances = nil
}

// ProcessObject folds one streamed object into the running tally.
func (e *ExtractedData) ProcessObject(obj *parser.SaveObject, extractStructures bool) {
	e.TotalProcessed++

	if obj.Header == nil {
		return
	}

	typePath := obj.Header.ClassName
	typeLower := strings.ToLower(typePath)
	instanceName := obj.Header.Reference.PathName

	// If this is a known world collectible that's still in the save, it hasn't
	// been picked up yet.
	if collectiblesWorldLoaded != nil && instanceName != "" {
		ref := instanceName
		if _, ok := collectiblesWorldLoaded.PowerSlugs.Blue[ref]; ok {
			e.CollectiblesFound.PowerSlugsBlue[ref] = true
		} else if _, ok := collectiblesWorldLoaded.PowerSlugs.Yellow[ref]; ok {
			e.CollectiblesFound.PowerSlugsYellow[ref] = true
		} else if _, ok := collectiblesWorldLoaded.PowerSlugs.Purple[ref]; ok {
			e.CollectiblesFound.PowerSlugsPurple[ref] = true
		} else if _, ok := collectiblesWorldLoaded.Somersloops[ref]; ok {
			e.CollectiblesFound.Somersloops[ref] = true
		} else if _, ok := collectiblesWorldLoaded.MercerSpheres[ref]; ok {
			e.CollectiblesFound.MercerSpheres[ref] = true
		} else if _, ok := collectiblesWorldLoaded.CrashSites[ref]; ok {
			e.CollectiblesFound.CrashSites[ref] = true
			// Opened/looted means the hard drive's already been grabbed.
			opened := false
			if v, ok := GetPropBool(obj, "mHasBeenOpened"); ok && v {
				opened = true
			}
			if v, ok := GetPropBool(obj, "mHasBeenLooted"); ok && v {
				opened = true
			}
			if opened {
				e.CollectiblesFound.CrashSiteOpened[ref] = true
			} else {
				e.CollectiblesFound.CrashSiteUnopened[ref] = true
			}
		}
	}

	// Populate position map for all objects with transforms
	if instanceName != "" && obj.Header.NeedTransform {
		e.PositionMap[instanceName] = obj.Header.Transform.Translation
	}

	// Resolve inventory stacks to compact form now, discard raw object later.
	if strings.Contains(typeLower, "inventorycomponent") {
		if raw, ok := GetPropArray(obj, "mInventoryStacks"); ok {
			e.InventoryStacks[instanceName] = parseInventoryStacks(raw)
		}
	}

	// BuildingFacts for buildings and power-consuming logistics
	isBuildingType := (IsBuilding(typePath) && !isInternalSystem(typeLower)) ||
		strings.Contains(typeLower, "pipepump") ||
		strings.Contains(typeLower, "trainstation") ||
		strings.Contains(typeLower, "truckstation") ||
		strings.Contains(typeLower, "freightplatform") ||
		strings.Contains(typeLower, "dockingstation")

	if isBuildingType && instanceName != "" {
		bf := populateBuildingFacts(obj)
		e.BuildingFacts[instanceName] = bf
		// Map PowerInfo ref to building for later inlining
		if bf.PowerInfoRef != "" {
			e.powerInfoToBuilding[bf.PowerInfoRef] = instanceName
		}
		// Buildings without mCurrentPotential default to 100% clock speed.
		if !bf.HasCurrentPotential && !isDefaultClockSpeedExcluded(typeLower) {
			e.DefaultClockSpeeds[instanceName] = typePath
		}
	}

	// Inline power info fields onto the parent building's BuildingFacts
	if strings.Contains(typeLower, "powerinfo") && !strings.Contains(typeLower, "powerconnection") && instanceName != "" {
		if buildingInst, ok := e.powerInfoToBuilding[instanceName]; ok {
			if bf, ok2 := e.BuildingFacts[buildingInst]; ok2 && bf != nil {
				if v, ok2 := GetPropFloat32(obj, "mTargetConsumption"); ok2 {
					bf.TargetConsumption = v
					bf.HasTargetConsumption = true
				}
				if v, ok2 := GetPropFloat32(obj, "mDynamicProductionCapacity"); ok2 {
					bf.DynamicProductionCapacity = v
					bf.HasDynamicProductionCapacity = true
				}
				if v, ok2 := GetPropFloat32(obj, "mBaseProduction"); ok2 {
					bf.BaseProduction = v
					bf.HasBaseProduction = true
				}
				if v, ok2 := GetPropFloat32(obj, "mCurrentPotential"); ok2 {
					bf.PowerInfoPotential = v
					bf.HasPowerInfoPotential = true
				}
			}
		}
	}

	// Power connection: strip ".PowerConnection" suffix to find parent building
	if strings.Contains(typeLower, "powerconnection") && instanceName != "" {
		if bf, ok := e.BuildingFacts[stripLastComponent(instanceName)]; ok && bf != nil {
			if wires, ok2 := GetPropArray(obj, "mWires"); ok2 {
				bf.WireCount = len(wires)
			}
		}
	}

	// Populate ResourceNodeFacts for resource nodes / sources / fracking.
	if (strings.Contains(typeLower, "resourcenode") ||
		strings.Contains(typeLower, "resourcesource") ||
		strings.Contains(typeLower, "frackingsatellite") ||
		strings.Contains(typeLower, "resourcewell")) && instanceName != "" {
		e.ResourceNodeFacts[instanceName] = populateResourceNodeFacts(obj)
	}

	// Capture InventoryPotential mCurrentPotential for nuclear plant fallback.
	if strings.Contains(typeLower, "inventorypotential") && instanceName != "" {
		if v, ok := GetPropFloat32(obj, "mCurrentPotential"); ok {
			e.InventoryPotentialMap[instanceName] = v
		}
	}
	// Retain only what the second pass needs
	retained := false
	if shouldRetainForPostExtraction(typePath, typeLower, len(obj.Properties) > 0) {
		e.RetainedObjects = append(e.RetainedObjects, obj)
		if instanceName != "" {
			e.RetainedObjectMap[instanceName] = obj
		}
		retained = true
	}

	// === Hypertubes ===
	if strings.Contains(typePath, "PipeHyper") && !strings.Contains(typePath, "TJunction") {
		if splineProp, ok := obj.Properties["mSplineData"]; ok {
			length := CalculateSplineLength(splineProp)
			if length > 0 {
				e.Hypertubes = append(e.Hypertubes, length)
			}
			delete(obj.Properties, "mSplineData") // free memory
		}
	}

	// === Rails ===
	if strings.Contains(typeLower, "railroadtrack") {
		if splineProp, ok := obj.Properties["mSplineData"]; ok {
			length := CalculateSplineLength(splineProp)
			if length > 0 {
				e.Rails.TotalLength += length
				e.Rails.Count++
			}
			delete(obj.Properties, "mSplineData") // free memory
		}
	}

	// === Trains ===
	if strings.Contains(typeLower, "train") {
		if strings.Contains(typeLower, "locomotive") {
			e.Trains.Locomotives++
		} else if strings.Contains(typeLower, "wagon") {
			e.Trains.Wagons++
		}
		if strings.Contains(typeLower, "station") {
			e.Trains.Stations++
		}
	}

	// === Vehicle Paths (1.2+) ===
	if strings.Contains(typeLower, "vehiclepath") && !strings.Contains(typeLower, "node") && !strings.Contains(typeLower, "network") {
		if splineProp, ok := obj.Properties["mSplinePoints"]; ok {
			length := CalculateSplineLength(splineProp)
			if length > 0 {
				e.VehiclePaths = append(e.VehiclePaths, length)
			}
			delete(obj.Properties, "mSplinePoints") // free memory
		}
	}

	// === Drones ===
	if strings.Contains(typeLower, "build_dronestation") {
		e.Drones.Stations++
	}
	if strings.Contains(typeLower, "build_freightplatform") {
		e.Drones.FreightPlatforms++
	}

	// === Belt heads ===
	if strings.Contains(typeLower, "build_conveyorbelt") && !strings.Contains(typeLower, "lift") {
		if m := beltMkRegex.FindStringSubmatch(typePath); m != nil {
			beltType := fmt.Sprintf("/Game/FactoryGame/Buildable/Factory/ConveyorBelt%s/Build_ConveyorBelt%s.Build_ConveyorBelt%s_C", m[1], m[1], m[1])
			e.BeltHeads[beltType]++
		}
	}

	// === Pipe pumps ===
	if strings.Contains(typeLower, "build_pipepump") {
		if m := pipePumpRegex.FindStringSubmatch(typePath); m != nil {
			var pumpType string
			if m[1] != "" {
				pumpType = "PipelineMk" + m[1]
			} else {
				pumpType = "Pipeline"
			}
			e.PipePumps[pumpType]++
		}
	}

	// === Vehicles ===
	if strings.Contains(typeLower, "build_truck") && !strings.Contains(typeLower, "station") {
		e.Vehicles.Trucks++
	}
	if strings.Contains(typeLower, "build_explorer") && !strings.Contains(typeLower, "station") {
		e.Vehicles.Explorers++
	}
	if strings.Contains(typeLower, "build_tractor") && !strings.Contains(typeLower, "station") {
		e.Vehicles.Tractors++
	}
	if strings.Contains(typeLower, "build_cykle") {
		e.Vehicles.Cykles++
	}

	// Structures (must be before instanceName check — many structures have empty instanceName)
	if extractStructures && e.Structures != nil {
		e.processStructure(obj, typePath, typeLower, instanceName)
	}

	// Lightweight buildable subsystem — packed arrays of structures
	if extractStructures && e.Structures != nil {
		if special, ok := obj.Properties["__special__"]; ok {
			if special.Type == "Special" {
				if sp, ok := special.Value.(map[string]interface{}); ok {
					if spType, ok := sp["type"].(string); ok && spType == "BuildableSubsystemSpecialProperties" {
						e.processLightweightBuildables(sp)
					}
				}
			}
		}
	}

	// ConveyorChainActor — extract belt/lift lengths from chain data
	if strings.Contains(typeLower, "conveyorchain") {
		if special, ok := obj.Properties["__special__"]; ok {
			if special.Type == "Special" {
				if sp, ok := special.Value.(map[string]interface{}); ok {
					if spType, ok := sp["type"].(string); ok && spType == "ConveyorChainActorSpecialProperties" {
						e.processConveyorChain(sp)
					}
				}
			}
		}
	}

	// === Connection components (extracted during streaming, not retained) ===
	if instanceName != "" {
		if strings.Contains(typePath, "FGFactoryConnectionComponent") {
			extractBeltConnection(obj, instanceName, e.BuildingConnections)
		} else if strings.Contains(typePath, "FGPipeConnectionFactory") {
			extractPipeConnection(obj, instanceName, e.BuildingConnections)
		}
	}

	// === Objects with instanceName ===
	if instanceName != "" {
		// Buildings
		if IsBuilding(typePath) {
		if isInternalSystem(typeLower) {
			// Skip internal game systems
		} else if strings.Contains(typeLower, "vehiclepathnode") {
			// Skip
		} else if strings.Contains(typeLower, "truck") || strings.Contains(typeLower, "drone") ||
			(strings.Contains(typeLower, "dock") && !strings.Contains(typeLower, "train")) {
			e.Others[typePath]++
		} else {
			if strings.Contains(typeLower, "train") || strings.Contains(typeLower, "rail") ||
				strings.Contains(typeLower, "switch") || strings.Contains(typeLower, "signal") {
				if !strings.Contains(typeLower, "truck") && !strings.Contains(typeLower, "drone") {
					e.RailComponents[typePath]++
				}
			}
			e.Buildings[typePath]++
			e.BuildingInstances[instanceName] = typePath
		}
	}

	// Blueprints and subsystems
	if IsBlueprintOrSubsystem(typePath) {
		e.Blueprints[typePath]++
	}

	// Pets — count Lizard Doggos (Char_SpaceRabbit) and track tamed ones
	if strings.Contains(typeLower, "char_spacerabbit") {
		e.Pets.TotalDoggos++
		if v, ok := GetPropBool(obj, "mTamed"); ok && v {
			e.Pets.TamedDoggos++
		}
	}

	// Elevators
	if strings.Contains(typeLower, "elevator") {
		if strings.Contains(typePath, "Build_Elevator") && !strings.Contains(typeLower, "floorstop") && !strings.Contains(typeLower, "cabin") {
			length := ExtractLength(obj)
			e.Elevators = append(e.Elevators, length)
		} else if strings.Contains(typeLower, "floorstop") || strings.Contains(typeLower, "cabin") {
			e.ElevatorComponents[typePath] = append(e.ElevatorComponents[typePath], instanceName)
		}
	}

	// Mergers and splitters
	if strings.Contains(typeLower, "merger") {
		e.Mergers++
	} else if strings.Contains(typeLower, "programmable") {
		e.ProgrammableSplitters++
	} else if strings.Contains(typeLower, "smart") {
		e.SmartSplitters++
	} else if strings.Contains(typeLower, "splitter") {
		e.Splitters++
	}

	// Power poles
	if strings.Contains(typeLower, "powerpole") || strings.Contains(typeLower, "powerpolewall") ||
		strings.Contains(typeLower, "wall outlet") || strings.Contains(typeLower, "powertower") {
		e.PowerPoles++
	}

	// Hypertube junctions/entrances
	if strings.Contains(typeLower, "hypertube") && (strings.Contains(typeLower, "junction") ||
		strings.Contains(typeLower, "entrance") || strings.Contains(typeLower, "support")) {
		e.HypertubeJunctions++
	}

	// Passthroughs
	if strings.Contains(typeLower, "passthrough") {
		if strings.Contains(typeLower, "lift") {
			e.LiftFloorHoles++
		} else if strings.Contains(typeLower, "belt") {
			e.BeltWallHoles++
		} else if strings.Contains(typeLower, "pipe") || strings.Contains(typeLower, "pipeline") {
			e.PipeFloorHoles++
		}
	}

	// Production (pipes, tanks)
	if IsProduction(typePath) {
		length := ExtractLength(obj)
		// Free spline data after length extraction
		delete(obj.Properties, "mSplineData")
		if strings.Contains(typeLower, "conveyor") && (strings.Contains(typeLower, "belt") || strings.Contains(typeLower, "lift")) {
			// skip — belts/lifts handled via ConveyorChainActor
		} else if strings.Contains(typeLower, "pipeline") && !strings.Contains(typeLower, "pump") &&
			!strings.Contains(typeLower, "support") && !strings.Contains(typeLower, "junction") &&
			!strings.Contains(typeLower, "connection") && !strings.Contains(typeLower, "fgpipe") &&
			!strings.Contains(typeLower, "flowindicator") {
			if _, ok := e.Pipes[typePath]; !ok {
				e.Pipes[typePath] = &PipeTypeStats{}
			}
			e.Pipes[typePath].Count++
			e.Pipes[typePath].TotalLength += length
		} else if strings.Contains(typeLower, "industrialtank") || strings.Contains(typeLower, "pipestoragetank") {
			// Fluid tanks — counted as buildings, not pipe segments
			e.Others[typePath]++
		}
	} else if strings.Contains(typeLower, "powerline") {
		// Power lines
		if v, ok := GetPropFloat32(obj, "mCachedLength"); ok && v > 0 {
			e.PowerLines = append(e.PowerLines, float64(v)/100)
		}
		e.PowerGrid.LineCount++
	} else if strings.Contains(typeLower, "fgpowercircuit") {
		// Power circuits
		if v, ok := GetPropInt32(obj, "mCircuitID"); ok {
			componentCount := 0
			if arr, ok2 := GetPropArray(obj, "mComponents"); ok2 {
				componentCount = len(arr)
			}
			e.PowerGrid.Circuits = append(e.PowerGrid.Circuits, CircuitInfo{
				ID:             v,
				ComponentCount: componentCount,
			})
		}
	} else if strings.Contains(typeLower, "fgpowerconnectioncomponent") {
		e.PowerGrid.ComponentCount++
	}
	} // end if instanceName != ""

	// Extract item pickups (scattered items on the map)
	if strings.Contains(typeLower, "itempickup") && obj.Header.NeedTransform {
		if pickup, ok := extractPickupItem(obj); ok {
			e.Pickups = append(e.Pickups, pickup)
		}
	}

	// Strip non-essential props from retained objects
	if retained && obj.Properties != nil {
		for key := range obj.Properties {
			if !essentialPostExtractionProps[key] {
				delete(obj.Properties, key)
			}
		}
	}
}

// ProcessCollectables1 processes collected collectible refs from a level's TOC blob.
func (e *ExtractedData) ProcessCollectables1(refs []parser.ObjectRef) {
	if collectiblesWorldLoaded == nil || len(refs) == 0 {
		return
	}
	for _, ref := range refs {
		pathName := ref.PathName
		if pathName == "" {
			continue
		}
		if _, ok := collectiblesWorldLoaded.PowerSlugs.Blue[pathName]; ok {
			e.CollectedCollectibles.PowerSlugsBlue[pathName] = true
		} else if _, ok := collectiblesWorldLoaded.PowerSlugs.Yellow[pathName]; ok {
			e.CollectedCollectibles.PowerSlugsYellow[pathName] = true
		} else if _, ok := collectiblesWorldLoaded.PowerSlugs.Purple[pathName]; ok {
			e.CollectedCollectibles.PowerSlugsPurple[pathName] = true
		} else if _, ok := collectiblesWorldLoaded.Somersloops[pathName]; ok {
			e.CollectedCollectibles.Somersloops[pathName] = true
		} else if _, ok := collectiblesWorldLoaded.MercerSpheres[pathName]; ok {
			e.CollectedCollectibles.MercerSpheres[pathName] = true
		} else if _, ok := collectiblesWorldLoaded.CrashSites[pathName]; ok {
			e.CollectedCollectibles.CrashSites[pathName] = true
		}
	}
}

func (e *ExtractedData) processLightweightBuildables(sp map[string]interface{}) {
	typeCounts, ok := sp["typeCounts"].(map[string]int)
	if !ok {
		return
	}
	typeEntries, hasEntries := sp["typeEntries"].(map[string][]map[string]interface{})

	for typePath, count := range typeCounts {
		typeLower := strings.ToLower(typePath)
		if isInternalSystem(typeLower) {
			continue
		}
		category := classifyStructure(typePath, typeLower)
		if category == "" {
			continue
		}

		// Store lightweight counts separately — applied in post-streaming fixup
		if e.Structures.lightweightCounts[category] == nil {
			e.Structures.lightweightCounts[category] = make(map[string]int)
		}
		e.Structures.lightweightCounts[category][typePath] = count

		// Store map entries from lightweight buildables for MAP mode
		if e.mapMode && e.Structures.mapEntries != nil && hasEntries {
			entries, ok := typeEntries[typePath]
			if !ok {
				continue
			}
			if e.Structures.mapEntries[category] == nil {
				e.Structures.mapEntries[category] = make(map[string][]StructureEntry)
			}
			for _, lwEntry := range entries {
				entry := StructureEntry{
					TypePath: typePath,
				}
				if pos, ok := lwEntry["position"].([]float64); ok && len(pos) >= 3 {
					entry.Position = []float64{pos[0], pos[1], pos[2]}
				}
				if rot, ok := lwEntry["rotation"].(map[string]float64); ok {
					entry.Rotation = quaternionToEuler(rot["x"], rot["y"], rot["z"], rot["w"])
				}
				e.Structures.mapEntries[category][typePath] = append(e.Structures.mapEntries[category][typePath], entry)
			}
		}
	}
}

// processConveyorChain pulls belt/lift counts and lengths out of a
// ConveyorChainActor's special properties.
func (e *ExtractedData) processConveyorChain(sp map[string]interface{}) {
	beltsInChain, ok := sp["beltsInChain"].([]interface{})
	if !ok {
		return
	}

	// Items in transit
	if items, ok := sp["items"].([]interface{}); ok {
		for _, itemRaw := range items {
			item, ok := itemRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if itemRef, ok := item["item"].(map[string]string); ok {
				pathName := itemRef["pathName"]
				if pathName != "" {
					// Item name is the bit after the last dot.
					itemName := pathName
					if idx := strings.LastIndex(itemName, "."); idx >= 0 {
						itemName = itemName[idx+1:]
					}
					e.ItemsInTransit[itemName]++
				}
			}
		}
	}

	// Process each belt in chain for counts and lengths
	for _, beltRaw := range beltsInChain {
		belt, ok := beltRaw.(map[string]interface{})
		if !ok {
			continue
		}
		beltRef, ok := belt["beltRef"].(map[string]string)
		if !ok {
			continue
		}
		beltPath := beltRef["pathName"]
		if beltPath == "" {
			continue
		}

		startsAtLength, _ := belt["startsAtLength"].(float32)
		endsAtLength, _ := belt["endsAtLength"].(float32)
		segmentLength := float64(endsAtLength - startsAtLength)

		beltLower := strings.ToLower(beltPath)

		if strings.Contains(beltLower, "conveyorbelt") {
			if m := beltMkRegex.FindStringSubmatch(beltPath); m != nil {
				beltType := fmt.Sprintf("/Game/FactoryGame/Buildable/Factory/ConveyorBelt%s/Build_ConveyorBelt%s.Build_ConveyorBelt%s_C", m[1], m[1], m[1])
				if e.Belts[beltType] == nil {
					e.Belts[beltType] = &BeltTypeStats{}
				}
				e.Belts[beltType].Count++
				e.Belts[beltType].TotalLength += segmentLength / 100 // cm to m
			}
		} else if strings.Contains(beltLower, "conveyorlift") {
			if m := liftMkRegex.FindStringSubmatch(beltPath); m != nil {
				liftType := fmt.Sprintf("/Game/FactoryGame/Buildable/Factory/ConveyorLift%s/Build_ConveyorLift%s.Build_ConveyorLift%s_C", m[1], m[1], m[1])
				if e.Lifts[liftType] == nil {
					e.Lifts[liftType] = &BeltTypeStats{}
				}
				e.Lifts[liftType].Count++
				e.Lifts[liftType].TotalLength += segmentLength / 100
			}
		}
	}
}

// ApplyLightweightFixup merges lightweight subsystem counts with individual
// sublevel counts to avoid double-counting. Call after streaming finishes.
func (e *ExtractedData) ApplyLightweightFixup() {
	if e.Structures == nil {
		return
	}
	// For types in the lightweight subsystem, use lightweight count + sublevel individual count
	for category, typeCounts := range e.Structures.lightweightCounts {
		if e.Structures.Details[category] == nil {
			e.Structures.Details[category] = make(map[string]int)
		}
		for typePath, lwCount := range typeCounts {
			individualCount := e.Structures.Details[category][typePath]
			sublevelCount := 0
			if e.Structures.sublevelCounts[category] != nil {
				sublevelCount = e.Structures.sublevelCounts[category][typePath]
			}
			e.Structures.Details[category][typePath] = lwCount + individualCount + sublevelCount
		}
	}

	// Recalculate totals from details
	for cat := range e.Structures.Totals {
		e.Structures.Totals[cat] = 0
	}
	for category, types := range e.Structures.Details {
		for typePath, count := range types {
			typeLower := strings.ToLower(typePath)
			if category == "attachments" && IsBeltPipeSupportItem(typeLower) {
				continue
			}
			e.Structures.Totals[category] += count
		}
	}
}

// GetMapStructures returns per-entry structure data for MAP mode.
func (e *ExtractedData) GetMapStructures() map[string]map[string][]StructureEntry {
	if e.Structures == nil {
		return nil
	}
	return e.Structures.mapEntries
}

func (e *ExtractedData) processStructure(obj *parser.SaveObject, typePath, typeLower, instanceName string) {
	category := classifyStructure(typePath, typeLower)
	if category == "" {
		return
	}

	if isInternalSystem(typeLower) {
		return
	}

	// Sublevel objects aren't in the lightweight subsystem, so track them separately.
	levelName := ""
	if obj.Header != nil {
		levelName = obj.Header.Reference.LevelName
	}
	isPersistent := strings.EqualFold(levelName, "Persistent_Level")
	if isPersistent {
		e.Structures.persistentLevelTypes[typePath] = true
	}

	if category == "hypertubes" {
		length := ExtractLength(obj)
		if length > 0 {
			e.Hypertubes = append(e.Hypertubes, length)
		}
		e.addStructure(category, typePath, instanceName, obj)
		if !isPersistent {
			if e.Structures.sublevelCounts[category] == nil {
				e.Structures.sublevelCounts[category] = make(map[string]int)
			}
			e.Structures.sublevelCounts[category][typePath]++
		}
	} else {
		e.addStructure(category, typePath, instanceName, obj)
		if !isPersistent {
			if e.Structures.sublevelCounts[category] == nil {
				e.Structures.sublevelCounts[category] = make(map[string]int)
			}
			e.Structures.sublevelCounts[category][typePath]++
		}
	}
	// Only skip totals for attachments that are belt/pipe support items
	if category == "attachments" && IsBeltPipeSupportItem(typeLower) {
		// Skip — shown in Production dropdowns
	} else {
		e.Structures.Totals[category]++
	}
}

// classifyStructure returns the structure category for a type path, or "".
func classifyStructure(typePath, typeLower string) string {
	if strings.Contains(typeLower, "foundationpassthrough") || strings.Contains(typeLower, "bp_gaspillar") {
		return ""
	}
	if strings.Contains(typeLower, "blueprintdesigner") || strings.Contains(typeLower, "blueprint_designer") {
		return ""
	}
	if strings.Contains(typeLower, "fglightweightbuildable") {
		return ""
	}

	// Priority cases first; these don't go through IsStructure.
	if strings.Contains(typeLower, "beam") || strings.Contains(typeLower, "truss") {
		return "beams"
	}
	// Lights, including control panels, but not anything power-line/pole/switch.
	if (strings.Contains(typeLower, "light") || strings.Contains(typeLower, "lamp") ||
		strings.Contains(typeLower, "floodlight") || strings.Contains(typeLower, "streetlight") ||
		strings.Contains(typeLower, "controlpanel")) &&
		!strings.Contains(typeLower, "powerline") && !strings.Contains(typeLower, "powerpole") &&
		!strings.Contains(typeLower, "powerswitch") && !strings.Contains(typeLower, "prioritypowerswitch") {
		return "lights"
	}
	// Power switches
	if strings.Contains(typeLower, "powerswitch") || strings.Contains(typeLower, "prioritypowerswitch") {
		return "powerSwitches"
	}
	// Signs (not signal, not pole, not blueprintdesigner)
	if (strings.Contains(typeLower, "sign") || strings.Contains(typeLower, "billboard") ||
		strings.Contains(typeLower, "widgetsign")) && !strings.Contains(typeLower, "signal") &&
		!strings.Contains(typeLower, "pole") && !strings.Contains(typeLower, "blueprintdesigner") {
		return "signs"
	}

	// Everything else goes through IsStructure + StructureCategory.
	if !IsStructure(typePath) {
		return ""
	}
	cat := StructureCategory(typePath)
	if cat == "other" {
		return ""
	}
	return cat
}

func (e *ExtractedData) addStructure(category, typePath, instanceName string, obj *parser.SaveObject) {
	if e.Structures.Details[category] == nil {
		e.Structures.Details[category] = make(map[string]int)
	}
	e.Structures.Details[category][typePath]++

	// Store map entry with position/rotation for MAP mode
	if e.mapMode && e.Structures.mapEntries != nil && obj != nil && obj.Header != nil {
		entry := StructureEntry{
			TypePath: typePath,
		}
		if instanceName != "" {
			entry.InstanceName = instanceName
		}
		t := obj.Header.Transform
		entry.Position = []float64{float64(t.Translation[0]), float64(t.Translation[1]), float64(t.Translation[2])}
		entry.Rotation = quaternionToEuler(float64(t.Rotation[0]), float64(t.Rotation[1]), float64(t.Rotation[2]), float64(t.Rotation[3]))
		if e.Structures.mapEntries[category] == nil {
			e.Structures.mapEntries[category] = make(map[string][]StructureEntry)
		}
		e.Structures.mapEntries[category][typePath] = append(e.Structures.mapEntries[category][typePath], entry)
	}
}

func quaternionToEuler(x, y, z, w float64) *RotationData {
	// Quaternion -> Euler (pitch/yaw/roll in degrees), using Unreal's ZYX order.
	sinp := 2 * (w*y - z*x)
	if sinp >= 1 {
		sinp = 1
	} else if sinp <= -1 {
		sinp = -1
	}
	pitch := math.Asin(sinp)

	sinr_cosp := 2 * (w*x + y*z)
	cosr_cosp := 1 - 2*(x*x+y*y)
	roll := math.Atan2(sinr_cosp, cosr_cosp)

	siny_cosp := 2 * (w*z + x*y)
	cosy_cosp := 1 - 2*(y*y+z*z)
	yaw := math.Atan2(siny_cosp, cosy_cosp)

	return &RotationData{
		Pitch: pitch * 180 / math.Pi,
		Yaw:   yaw * 180 / math.Pi,
		Roll:  roll * 180 / math.Pi,
	}
}

func isInternalSystem(typeLower string) bool {
	return strings.Contains(typeLower, "fgtrain") || strings.Contains(typeLower, "fgcentral") ||
		strings.Contains(typeLower, "fgdocking") || strings.Contains(typeLower, "fgdrone") ||
		strings.Contains(typeLower, "fglightweightbuildable") ||
		strings.Contains(typeLower, "/bp_") || strings.Contains(typeLower, "bp_")
}

// isDefaultClockSpeedExcluded mirrors isExcludedBuilding in clock_speed.go, plus
// generators (those get their clock speed handled in post-extraction).
func isDefaultClockSpeedExcluded(typeLower string) bool {
	excluded := []string{"storage", "conveyor", "belt", "pipe", "splitter", "merger", "lift",
		"wall", "door", "foundation", "ramp", "walkway", "dock", "table", "shelf",
		"lamp", "sign", "beacon", "train", "vehicle", "drone", "hub", "elevator",
		"powerpole", "powertower", "powerswitch", "powerline",
		"workbench", "workshop", "mam",
		"beam", "lookout", "streetlight", "snow", "candycane", "snowman", "xmasstree",
		"tank", "powerstorage", "blueprintdesigner", "depot", "centralstorage",
		"portal", "sink", "switch", "pole", "equip_", "truckstation",
		// Generators handled separately during clock speed extraction.
		"generator", "biomass", "nuclear", "geothermal"}
	for _, e := range excluded {
		if strings.Contains(typeLower, e) {
			return true
		}
	}
	return false
}

// extractPickupItem reads item class, count and position from an FGItemPickup_Spawnable.
func extractPickupItem(obj *parser.SaveObject) (PickupItem, bool) {
	prop, ok := obj.Properties["mPickupItems"]
	if !ok {
		return PickupItem{}, false
	}

	// Unwrap StructProperty -> InventoryStack -> value.
	outerMap, ok := prop.Value.(map[string]interface{})
	if !ok {
		return PickupItem{}, false
	}
	innerValue, ok := outerMap["value"].(map[string]interface{})
	if !ok {
		return PickupItem{}, false
	}

	// Extract item class path using existing helper
	itemClass := getItemPathFromStack(innerValue)
	if itemClass == "" {
		return PickupItem{}, false
	}

	// Extract NumItems
	numItems := getNumItemsFromStack(innerValue)
	if numItems <= 0 {
		return PickupItem{}, false
	}

	// Extract short item name from path
	itemName := shortenItemName(itemClass)

	// Check if collected (mHasBeenLooted or mPickupState)
	collected := false
	if looted, ok := GetPropBool(obj, "mHasBeenLooted"); ok && looted {
		collected = true
	}

	return PickupItem{
		ItemClass: itemClass,
		ItemName:  itemName,
		NumItems:  numItems,
		Position:  obj.Header.Transform.Translation,
		Collected: collected,
	}, true
}

// shortenItemName extracts a readable name from an item class path.
func shortenItemName(itemClass string) string {
	parts := strings.Split(itemClass, "/")
	last := parts[len(parts)-1]
	// Last segment looks like "Desc_Rotor.Desc_Rotor_C"; keep the part after the dot.
	if idx := strings.Index(last, "."); idx > 0 {
		last = last[idx+1:]
	}

	// Look up in item_display_names.json first
	if itemDisplayNamesLoaded != nil {
		if displayName, ok := itemDisplayNamesLoaded[last]; ok {
			return displayName
		}
	}

	// No mapping, so fall back to trimming the usual prefixes/suffixes.
	last = strings.TrimPrefix(last, "Desc_")
	last = strings.TrimPrefix(last, "BP_EquipmentDescriptor")
	last = strings.TrimSuffix(last, "_C")
	return last
}
