package extraction

import (
	"strings"
	"sync"
)

// classInfo holds every classification that depends only on an object's class
// name (its "type path").
//
// A large save holds millions of objects but only a few thousand distinct class
// names. The classification helpers are pure functions of that name, and between
// them they run well over a hundred strings.Contains scans plus several
// strings.ToLower allocations. A CPU profile of a 133 MB / 2.2M-object save had
// those scans at ~31% of total CPU and the ToLower calls at a further ~8%,
// purely because every object re-derived answers that only ever depend on its
// class name.
//
// Computing each answer once per distinct class name collapses all of that into
// a single map lookup per object.
type classInfo struct {
	// lower is the lowercased class name, cached so callers don't allocate a
	// fresh one per object.
	lower string

	isBuilding             bool
	isStructure            bool
	isProduction           bool
	isBlueprintOrSubsystem bool
	isInternalSystem       bool
	clockSpeedExcluded     bool
	// retainForPost is the class-name half of shouldRetainForPostExtraction.
	// Callers still have to apply the hasProperties check themselves.
	retainForPost bool

	structureCategory   string
	classifiedStructure string
	mapCategory         uint8

	// Pre-computed strings.Contains results for every substring checked in
	// ProcessObject. Keyed by the substring; value is Contains(lower, sub).
	containsLower map[string]bool
	// Same but for case-sensitive checks against the original typePath.
	containsPath map[string]bool

	// isBeltPipeSupport caches IsBeltPipeSupportItem for processStructure.
	isBeltPipeSupport bool
}

// allLowerSubstrings is the complete set of lowercased substrings that
// ProcessObject checks via strings.Contains(typeLower, ...). Pre-computing
// these once per distinct class name eliminates ~100 * 2.2M string scans.
var allLowerSubstrings = []string{
	"inventorycomponent", "pipepump", "trainstation", "truckstation",
	"freightplatform", "dockingstation", "powerinfo", "powerconnection",
	"resourcenode", "resourcesource", "frackingsatellite", "resourcewell",
	"inventorypotential", "railroadtrack", "station", "platform", "bridge",
	"railroadtrackconnection", "railroadblocksignal", "railroadpathsignal",
	"pathsignal", "train", "locomotive", "wagon", "dockingstationliquid",
	"build_trainstation", "vehiclepath", "node", "network",
	"build_dronestation", "build_freightplatform", "bp_dronetransport",
	"build_conveyorbelt", "lift", "build_pipepump", "bp_fluidtruck",
	"bp_truck", "bp_explorer", "bp_tractor", "bp_golfcart",
	"testa_bp_wb", "cyberwagon", "build_fluidtruckstation",
	"build_truckstation", "bp_train.bp_train", "fgrailroadtimetable",
	"fgtrainstationidentifier", "build_trainstation.build_trainstation",
	"locomotive.bp_locomotive", "freightwagon.bp_freightwagon",
	"build_traindockingstation", "fgwheeledvehicleidentifier",
	"fgwheeledvehicleinfo", "fgsavedwheeledvehiclepath",
	"fgdockingstationidentifier", "fgdockingstationinfo",
	"build_truckstation.build_truckstation", "fgdronestationinfo",
	"build_dronestation.build_dronestation", "conveyorchain",
	"vehiclepathnode", "truck", "drone", "dock", "rail", "switch", "signal",
	"char_spacerabbit", "elevator", "floorstop", "cabin",
	"merger", "programmable", "smart", "splitter",
	"powerpole", "powerpolewall", "wall outlet", "powertower",
	"hypertube", "junction", "entrance", "support", "passthrough",
	"pipehyperstart",
	"belt", "pipe", "pipeline", "conveyor", "pump", "connection",
	"fgpipe", "flowindicator", "industrialtank", "pipestoragetank",
	"powerline", "fgpowercircuit", "statisticssubsystem", "itempickup",
	"stackable", "valve", "railroadendstop",
}

// allPathSubstrings is the set of case-sensitive substrings checked via
// strings.Contains(typePath, ...) in ProcessObject.
var allPathSubstrings = []string{
	"PipeHyper", "TJunction", "FGFactoryConnectionComponent",
	"FGPipeConnectionFactory", "Build_Elevator",
}

var (
	classInfoMu    sync.RWMutex
	classInfoCache = make(map[string]*classInfo)
)

// classInfoFor returns the cached classification for a class name, computing it
// the first time that name is seen.
func classInfoFor(typePath string) *classInfo {
	classInfoMu.RLock()
	ci := classInfoCache[typePath]
	classInfoMu.RUnlock()
	if ci != nil {
		return ci
	}

	lower := strings.ToLower(typePath)
	ci = &classInfo{
		lower:                  lower,
		isBuilding:             computeIsBuilding(lower),
		isStructure:            computeIsStructure(lower),
		isProduction:           computeIsProduction(lower),
		isBlueprintOrSubsystem: computeIsBlueprintOrSubsystem(lower),
		isInternalSystem:       computeIsInternalSystem(lower),
		clockSpeedExcluded:     computeClockSpeedExcluded(lower),
		retainForPost:          computeRetainForPostExtraction(lower),
		structureCategory:      computeStructureCategory(lower),
		mapCategory:            computeMapCategory(lower),
		containsLower:          make(map[string]bool, len(allLowerSubstrings)),
		containsPath:           make(map[string]bool, len(allPathSubstrings)),
	}
	// Depends on the fields above, so it has to come last.
	ci.classifiedStructure = computeClassifyStructure(ci)

	// Pre-compute all substring checks used by ProcessObject.
	for _, sub := range allLowerSubstrings {
		ci.containsLower[sub] = strings.Contains(lower, sub)
	}
	for _, sub := range allPathSubstrings {
		ci.containsPath[sub] = strings.Contains(typePath, sub)
	}

	// Derived flag used by processStructure.
	ci.isBeltPipeSupport = ci.containsLower["conveyor"] ||
		ci.containsLower["pipeline"] ||
		ci.containsLower["pipe"] ||
		ci.containsLower["stackable"] ||
		ci.containsLower["valve"] ||
		ci.containsLower["junction"] ||
		ci.containsLower["pump"] ||
		ci.containsLower["hypertube"]

	classInfoMu.Lock()
	classInfoCache[typePath] = ci
	classInfoMu.Unlock()
	return ci
}

// ClassNameLower returns the lowercased form of a class name, cached.
func ClassNameLower(typePath string) string { return classInfoFor(typePath).lower }

// IsBuilding reports whether a typePath is a production building.
func IsBuilding(typePath string) bool { return classInfoFor(typePath).isBuilding }

// IsStructure reports whether a typePath is a structural buildable.
func IsStructure(typePath string) bool { return classInfoFor(typePath).isStructure }

// IsProduction reports whether a typePath is a transport item: belts, pipes, tanks.
func IsProduction(typePath string) bool { return classInfoFor(typePath).isProduction }

// IsBlueprintOrSubsystem reports whether a typePath is a blueprint proxy or shortcut.
func IsBlueprintOrSubsystem(typePath string) bool {
	return classInfoFor(typePath).isBlueprintOrSubsystem
}

// StructureCategory returns the category name for a structure type.
func StructureCategory(typePath string) string { return classInfoFor(typePath).structureCategory }
