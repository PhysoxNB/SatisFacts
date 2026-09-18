package extraction

import (
	"regexp"
	"strings"

	"satisfacts/parser"
)

// ExtractedData is the running tally we build up while streaming a save.
type ExtractedData struct {
	// Buildings
	Buildings         map[string]int    // typePath -> count
	BuildingInstances map[string]string // instanceName -> typePath
	Blueprints        map[string]int    // typePath -> count

	// Production / Transport
	Belts                 map[string]*BeltTypeStats
	Lifts                 map[string]*BeltTypeStats
	Pipes                 map[string]*PipeTypeStats
	PowerLines            []float64
	Hypertubes            []float64
	Elevators             []float64
	Rails                 RailStats
	Trains                TrainStats
	Drones                DroneStats
	Vehicles              VehicleStats
	Mergers               int
	Splitters             int
	SmartSplitters        int
	ProgrammableSplitters int
	PowerPoles            int
	PowerPoleTypes        map[string]int
	HypertubeJunctions    int
	LiftFloorHoles        int
	BeltWallHoles         int
	PipeFloorHoles        int
	BeltHeads             map[string]int
	PipePumps             map[string]int
	ItemsInTransit        map[string]int
	RailComponents        map[string]int
	ElevatorComponents    map[string][]string
	Others                map[string]int

	// Power grid
	PowerGrid PowerGridData

	// Structures (populated for DEEP modes)
	Structures *StructureData

	// Retained objects for post-streaming extraction
	RetainedObjects   []*parser.SaveObject
	RetainedObjectMap map[string]*parser.SaveObject

	// Resolved inventory stacks per instance (avoids retaining raw mInventoryStacks)
	InventoryStacks map[string][]InventoryStack

	// Total slots per inventory (including empty), for fill-percentage calculation
	InventoryTotalSlots map[string]int

	BuildingFacts map[string]*BuildingFacts // keyed by instance name

	powerInfoToBuilding map[string]string // PowerInfo instance -> parent building instance

	ResourceNodeFacts map[string]*ResourceNodeFacts

	InventoryPotentialMap map[string]float32 // mCurrentPotential, fallback for nuclear plants

	// Default clock speeds for buildings without properties (100%)
	DefaultClockSpeeds map[string]string // instanceName -> typePath

	// Building connections extracted during streaming (not retained)
	BuildingConnections map[string]BuildingConnection

	// Vehicle path segments with lengths - calculated during streaming
	VehiclePaths []float64

	// Item pickups (scattered items on the map)
	Pickups []PickupItem

	// Collectible positions retained for the viewer (instanceName -> [x,y,z] in cm).
	// Populated during ProcessObject, survives CleanupAfterStreaming.
	CollectibleLocations map[string][3]float32

	// Map entries for non-building objects (power poles, belts, pipes, etc.)
	// collected during ProcessObject. Used by BuildMapData.
	MapEntries []MapEntry

	// Polyline entries for belts, pipes, rails, hypertubes, power lines.
	PolylineEntries []MapPolylineEntry

	// Power line refs (source/target object refs) for deferred position resolution.
	PowerLineRefs []PowerLineRef

	// ObjectPositions maps instanceName -> [x, y, z] in cm for objects with transforms.
	// Used for resolving power line endpoint positions. Cleared after BuildMapData.
	ObjectPositions map[string][3]float32

	// Collectibles found in save (not yet collected) - instance names present in save
	CollectiblesFound CollectiblesFoundData

	// Collectibles collected by player - from collectables1 list in TOC blob
	CollectedCollectibles CollectedCollectiblesData

	// Pets (tamed creatures)
	Pets PetData

	// Creatures killed by players
	CreatureKills CreatureKillData

	// Consumables consumed (eaten) by players
	ConsumablesConsumed ConsumablesData

	// Logistics: detailed train, vehicle, and drone tracking with names, routes, and warnings
	Logistics LogisticsData

	// Raw logistics objects collected during streaming, linked together in ResolveLogistics
	rawTrains              map[string]*parser.SaveObject // instanceName -> BP_Train
	rawTimetables          map[string]*parser.SaveObject // instanceName -> FGRailroadTimeTable
	rawTrainStationIDs     map[string]*parser.SaveObject // instanceName -> FGTrainStationIdentifier
	rawTrainStations       map[string]*parser.SaveObject // instanceName -> Build_TrainStation
	rawLocomotives         map[string]*parser.SaveObject // instanceName -> BP_Locomotive
	rawWagons              map[string]*parser.SaveObject // instanceName -> BP_FreightWagon
	rawTrainPlatforms      map[string]*parser.SaveObject // instanceName -> Build_TrainDockingStation (freight/liquid platform)
	rawWheeledVehicles     map[string]*parser.SaveObject // instanceName -> BP_Truck/Tractor/Explorer/etc
	rawWheeledVehicleIDs   map[string]*parser.SaveObject // instanceName -> FGWheeledVehicleIdentifier
	rawWheeledVehicleInfos map[string]*parser.SaveObject // instanceName -> FGWheeledVehicleInfo
	rawVehiclePaths        map[string]*parser.SaveObject // instanceName -> FGSavedWheeledVehiclePath
	rawTruckStationIDs     map[string]*parser.SaveObject // instanceName -> FGDockingStationIdentifier
	rawTruckStations       map[string]*parser.SaveObject // instanceName -> Build_TruckStation
	rawDrones              map[string]*parser.SaveObject // instanceName -> BP_DroneTransport
	rawDroneStationInfos   map[string]*parser.SaveObject // instanceName -> FGDroneStationInfo
	rawDroneStations       map[string]*parser.SaveObject // instanceName -> Build_DroneStation

	// droneStationNames caches stationRef -> display name (custom mBuildingTag,
	// or a sequential "Drone Station A/B/C..." fallback), built once on first
	// use by droneStationDisplayNames() instead of being recomputed (with a
	// full sortedKeys scan) on every lookup.
	droneStationNames map[string]string

	// Track graph data - kept around until ResolveLogistics builds the graph
	rawTrackSegments map[string]*TrackSegment // each track piece with its length and endpoints
	rawTrackConns    map[string]*TrackConn    // which track endpoints connect to each other
	rawSignals       map[string]*SignalInfo   // block/path signals for directed graph

	// Totals
	TotalProcessed int
}

// NewExtractedData returns an ExtractedData with all its maps ready to go.
func NewExtractedData(extractStructures bool) *ExtractedData {
	e := &ExtractedData{
		Buildings:             make(map[string]int),
		BuildingInstances:     make(map[string]string),
		Blueprints:            make(map[string]int),
		Belts:                 make(map[string]*BeltTypeStats),
		Lifts:                 make(map[string]*BeltTypeStats),
		Pipes:                 make(map[string]*PipeTypeStats),
		BeltHeads:             make(map[string]int),
		PipePumps:             make(map[string]int),
		ItemsInTransit:        make(map[string]int),
		RailComponents:        make(map[string]int),
		ElevatorComponents:    make(map[string][]string),
		Others:                make(map[string]int),
		PowerPoleTypes:        make(map[string]int),
		RetainedObjectMap:     make(map[string]*parser.SaveObject),
		ObjectPositions:       make(map[string][3]float32),
		InventoryStacks:       make(map[string][]InventoryStack),
		InventoryTotalSlots:   make(map[string]int),
		BuildingFacts:         make(map[string]*BuildingFacts),
		powerInfoToBuilding:   make(map[string]string),
		ResourceNodeFacts:     make(map[string]*ResourceNodeFacts),
		InventoryPotentialMap: make(map[string]float32),
		DefaultClockSpeeds:    make(map[string]string),
		BuildingConnections:   make(map[string]BuildingConnection),
		CollectibleLocations:  make(map[string][3]float32),
		CollectiblesFound: CollectiblesFoundData{
			PowerSlugsBlue:    make(map[string]bool),
			PowerSlugsYellow:  make(map[string]bool),
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
		CreatureKills: CreatureKillData{
			ByType: make(map[string]int),
		},
		ConsumablesConsumed: ConsumablesData{
			ByType: make(map[string]int),
		},
		rawTrains:              make(map[string]*parser.SaveObject),
		rawTimetables:          make(map[string]*parser.SaveObject),
		rawTrainStationIDs:     make(map[string]*parser.SaveObject),
		rawTrainStations:       make(map[string]*parser.SaveObject),
		rawLocomotives:         make(map[string]*parser.SaveObject),
		rawWagons:              make(map[string]*parser.SaveObject),
		rawTrainPlatforms:      make(map[string]*parser.SaveObject),
		rawWheeledVehicles:     make(map[string]*parser.SaveObject),
		rawWheeledVehicleIDs:   make(map[string]*parser.SaveObject),
		rawWheeledVehicleInfos: make(map[string]*parser.SaveObject),
		rawVehiclePaths:        make(map[string]*parser.SaveObject),
		rawTruckStationIDs:     make(map[string]*parser.SaveObject),
		rawTruckStations:       make(map[string]*parser.SaveObject),
		rawDrones:              make(map[string]*parser.SaveObject),
		rawDroneStationInfos:   make(map[string]*parser.SaveObject),
		rawDroneStations:       make(map[string]*parser.SaveObject),
		rawTrackSegments:       make(map[string]*TrackSegment),
		rawTrackConns:          make(map[string]*TrackConn),
		rawSignals:             make(map[string]*SignalInfo),
	}
	if extractStructures {
		e.Structures = &StructureData{
			Details:              make(map[string]map[string]int),
			Totals:               make(map[string]int),
			lightweightCounts:    make(map[string]map[string]int),
			sublevelCounts:       make(map[string]map[string]int),
			persistentLevelTypes: make(map[string]bool),
		}
	}
	return e
}

var beltMkRegex = regexp.MustCompile(`Build_ConveyorBelt(Mk\d+)_C`)
var liftMkRegex = regexp.MustCompile(`Build_ConveyorLift(Mk\d+)_C`)
var pipePumpRegex = regexp.MustCompile(`Build_PipePump(Mk\d+)?_C`)

// shouldRetainForPostExtraction decides whether to keep an object for the second pass.
func shouldRetainForPostExtraction(typePath string, hasProperties bool) bool {
	if !hasProperties {
		return false
	}
	return classInfoFor(typePath).retainForPost
}

// computeRetainForPostExtraction is the class-name half of
// shouldRetainForPostExtraction. Cached via classInfo.
func computeRetainForPostExtraction(typeLower string) bool {
	// Already handled during streaming, no need to keep it.
	if strings.Contains(typeLower, "fglightweightbuildable") {
		return false
	}

	// Check before isInternalSystem - FGCentralStorageSubsystem would match "fgcentral".
	if strings.Contains(typeLower, "subsystem") ||
		strings.Contains(typeLower, "gamestate") ||
		strings.Contains(typeLower, "phasemanager") ||
		strings.Contains(typeLower, "schematicmanager") ||
		strings.Contains(typeLower, "researchmanager") ||
		strings.Contains(typeLower, "tutorialintro") ||
		strings.Contains(typeLower, "gamemode") ||
		strings.Contains(typeLower, "char_player") ||
		strings.Contains(typeLower, "playerstate") {
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
	"mCurrentPotential":       true,
	"mPendingPotential":       true,
	"mCurrentProductionBoost": true,
	// Recipe
	"mCurrentRecipe": true,
	// Inventory refs
	"mInputInventory":     true,
	"mOutputInventory":    true,
	"mInventoryPotential": true,
	"mFuelInventory":      true,
	// Power
	"mPowerInfo":                  true,
	"mIsProducing":                true,
	"mIsProductionPaused":         true,
	"mProductivityMonitorEnabled": true,
	"mCurrentProductivityMeasurementProduceDuration": true,
	// Extractor
	"mExtractableResource":    true,
	"mCurrentExtractProgress": true,
	// Nuclear
	"mWasteLeftFromCurrentFuel":  true,
	"mCurrentFuelAmount":         true,
	"mCurrentSupplementalAmount": true,
	"mCurrentFuelClass":          true,
	// APA
	"mHasFuelCached": true,
	// Inventory stacks
	"mInventoryStacks": true,
	// Connections
	"mConnectedComponent": true,
	// Power info
	"mTargetConsumption":         true,
	"mDynamicProductionCapacity": true,
	"mBaseProduction":            true,
	// Power connection
	"mWires": true,
	// Power storage
	"mPowerStore": true,
	// Subsystems / collectibles
	"mUnlockedTapes":          true,
	"mLootedDropPods":         true,
	"mDestroyedPickups":       true,
	"mStoredItems":            true,
	"mCurrentPointLevels":     true,
	"mNumResourceSinkCoupons": true,
	// Game state / managers
	"mGamePhaseManager":      true,
	"mSchematicManager":      true,
	"mResearchManager":       true,
	"mResourceSinkSubsystem": true,
	"mCurrentGamePhase":      true,
	"mTargetGamePhase":       true,
	"mPurchasedSchematics":   true,
	"mLastActiveSchematic":   true,
	"mUnlockedResearchTrees": true,
	"mIsActivated":           true,
	"mLastUsedHardDriveID":   true,
	// Custom map settings
	"mNodePuritySettings":       true,
	"mNodeRandomization":        true,
	"mNodeRandomizationSeed":    true,
	"mPartsCostMultiplier":      true,
	"mSpacePartsCostMultiplier": true,
	"mEnergyCostMultiplier":     true,
	// World info
	"mTradingPostLevel":     true,
	"mStartingPointTagName": true,
	// Player info
	"mCachedPlayerName":          true,
	"mOwnedPawn":                 true,
	"mNumObservedInventorySlots": true,
	"mVisitedAreas":              true,
	"mPlayedMessages":            true,
	// Resource nodes
	"mPurityOverride":        true,
	"mResourceClassOverride": true,
	"mResourceType":          true,
	// Special (lightweight buildable, conveyor chain - usually not retained but kept for safety)
	"__special__": true,
}

// CleanupAfterStreaming frees maps only needed during ProcessObject.
func (e *ExtractedData) CleanupAfterStreaming() {
	e.powerInfoToBuilding = nil
	e.ObjectPositions = nil
	e.PowerLineRefs = nil
}

// CleanupAfterPostExtraction frees maps only needed by RunPostExtraction.
func (e *ExtractedData) CleanupAfterPostExtraction() {
	e.DefaultClockSpeeds = nil
	e.BuildingInstances = nil
}
