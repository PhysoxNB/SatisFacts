package extraction

import "strings"

// computeIsBuilding reports whether a lowercased typePath is a production
// building. Call IsBuilding instead; this is the uncached implementation.
func computeIsBuilding(lower string) bool {
	// Subsystem objects (e.g. FGResourceSinkSubsystem) are not buildings.
	if strings.HasPrefix(lower, "/script/") {
		return false
	}

	// Water pumps count, and we check them before the exclusions below so they
	// don't get caught by the "pump" rule.
	if strings.Contains(lower, "waterpump") {
		return true
	}

	// Passthrough attachments aren't buildings.
	if strings.Contains(lower, "passthrough") {
		return false
	}
	// Neither are the supports that hold up belts and pipes.
	if strings.Contains(lower, "conveyorpolewall") || strings.Contains(lower, "pipelinesupportwall") ||
		strings.Contains(lower, "conveyorceilingattachment") || strings.Contains(lower, "conveyorpole") ||
		strings.Contains(lower, "pipelinesupport") || strings.Contains(lower, "stackable") ||
		strings.Contains(lower, "valve") || strings.Contains(lower, "junction") ||
		(strings.Contains(lower, "pump") && strings.Contains(lower, "pipeline")) {
		return false
	}
	// Power poles and switches aren't buildings either.
	if strings.Contains(lower, "powerpole") || strings.Contains(lower, "powerswitch") ||
		strings.Contains(lower, "prioritypowerswitch") {
		return false
	}
	// Skip hypertubes.
	if strings.Contains(lower, "hypertube") {
		return false
	}
	// Fluid buffers (industrial/pipe storage tanks) live in the pipe section, not here.
	if strings.Contains(lower, "fluidbuffer") {
		return false
	}
	// Rail track segments and switch controls are infrastructure, not buildings.
	// Signals (Block/Path) are kept - they're placeable structures.
	if strings.Contains(lower, "railroadtrack") || strings.Contains(lower, "switchcontrol") {
		return false
	}
	// Equipment items (e.g. portable resource miner) are not buildings.
	if strings.Contains(lower, "/equipment/") || strings.HasPrefix(lower, "/game/factorygame/equipment/") {
		return false
	}
	// Blueprint designer's internal storage is not a standalone storage building.
	if strings.Contains(lower, "storageblueprint") {
		return false
	}

	return strings.Contains(lower, "assembler") ||
		strings.Contains(lower, "smelter") ||
		strings.Contains(lower, "foundry") ||
		strings.Contains(lower, "constructor") ||
		strings.Contains(lower, "manufacturer") ||
		strings.Contains(lower, "refinery") ||
		strings.Contains(lower, "blender") ||
		strings.Contains(lower, "packager") ||
		strings.Contains(lower, "miner") ||
		strings.Contains(lower, "oil") ||
		strings.Contains(lower, "waterpump") ||
		strings.Contains(lower, "generator") ||
		strings.Contains(lower, "alienpower") ||
		strings.Contains(lower, "train") ||
		strings.Contains(lower, "station") ||
		strings.Contains(lower, "converter") ||
		strings.Contains(lower, "portal") ||
		strings.Contains(lower, "extractor") ||
		strings.Contains(lower, "accelerator") ||
		strings.Contains(lower, "hadron") ||
		strings.Contains(lower, "collider") ||
		strings.Contains(lower, "quantum") ||
		strings.Contains(lower, "encoder") ||
		strings.Contains(lower, "storage") ||
		strings.Contains(lower, "container") ||
		strings.Contains(lower, "depot") ||
		strings.Contains(lower, "powerstorage") ||
		strings.Contains(lower, "blueprintdesigner") ||
		strings.Contains(lower, "smasher") ||
		strings.Contains(lower, "industrialtank") ||
		strings.Contains(lower, "pipestoragetank") ||
		strings.Contains(lower, "resourcesink") ||
		strings.Contains(lower, "elevator")
}

// computeIsBlueprintOrSubsystem reports whether a lowercased typePath is a
// blueprint proxy or shortcut. Call IsBlueprintOrSubsystem instead.
func computeIsBlueprintOrSubsystem(lower string) bool {
	if strings.Contains(lower, "blueprintproxy") || strings.Contains(lower, "blueprintshortcut") {
		return !strings.Contains(lower, "storage")
	}
	return false
}

// computeIsProduction reports whether a lowercased typePath is a transport
// item: belts, pipes, tanks. Call IsProduction instead.
func computeIsProduction(lower string) bool {
	if strings.Contains(lower, "wall") {
		return false
	}
	return strings.Contains(lower, "conveyor") ||
		strings.Contains(lower, "pipe") ||
		strings.Contains(lower, "industrialtank") ||
		strings.Contains(lower, "pipestoragetank")
}

// computeIsStructure reports whether a lowercased typePath is a structural
// buildable. On top of the obvious shapes it also catches barriers, gates,
// roofs, fans and vents. Call IsStructure instead.
func computeIsStructure(lower string) bool {
	// Subsystem objects (e.g. FUR_Subsystem, LongerBeams_Subsystem) are not structures.
	if strings.Contains(lower, "_subsystem") {
		return false
	}
	if strings.Contains(lower, "foundationpassthrough") {
		return false
	}
	// Infrastructure (belts, pipes, power) is transport, not structure.
	if strings.Contains(lower, "conveyorpole") || strings.Contains(lower, "conveyorceiling") ||
		strings.Contains(lower, "pipelinesupport") || strings.Contains(lower, "stackable") ||
		strings.Contains(lower, "valve") || strings.Contains(lower, "junction") ||
		strings.Contains(lower, "hypertube") ||
		strings.Contains(lower, "powerpole") || strings.Contains(lower, "powerswitch") ||
		strings.Contains(lower, "prioritypowerswitch") ||
		(strings.Contains(lower, "pump") && !strings.Contains(lower, "waterpump") && !strings.Contains(lower, "oilpump")) {
		return false
	}
	return strings.Contains(lower, "foundation") ||
		strings.Contains(lower, "wall") ||
		strings.Contains(lower, "pillar") ||
		strings.Contains(lower, "beam") ||
		strings.Contains(lower, "truss") ||
		strings.Contains(lower, "ramp") ||
		strings.Contains(lower, "quarterpipe") ||
		strings.Contains(lower, "walkway") ||
		strings.Contains(lower, "catwalk") ||
		strings.Contains(lower, "railing") ||
		strings.Contains(lower, "fence") ||
		strings.Contains(lower, "ladder") ||
		strings.Contains(lower, "decoration") ||
		strings.Contains(lower, "statue") ||
		strings.Contains(lower, "monument") ||
		strings.Contains(lower, "barrier") ||
		strings.Contains(lower, "gate") ||
		strings.Contains(lower, "door") ||
		strings.Contains(lower, "roof") ||
		strings.Contains(lower, "fan") ||
		(strings.Contains(lower, "vent") && !strings.Contains(lower, "inventory"))
}

// computeStructureCategory returns the category name for a lowercased structure
// type. Call StructureCategory instead.
func computeStructureCategory(lower string) string {
	if strings.Contains(lower, "foundationpassthrough") {
		return "other"
	}
	if strings.Contains(lower, "bp_gaspillar") {
		return "other"
	}
	if strings.Contains(lower, "pillar") {
		return "pillars"
	}
	if strings.Contains(lower, "roof") {
		return "roofs"
	}
	if strings.Contains(lower, "walkway") || strings.Contains(lower, "catwalk") {
		return "walkways"
	}
	if strings.Contains(lower, "gate") || strings.Contains(lower, "door") {
		return "walls"
	}
	if strings.Contains(lower, "foundation") || strings.Contains(lower, "ramp") || strings.Contains(lower, "quarterpipe") {
		return "foundations"
	}
	if strings.Contains(lower, "wall") {
		return "walls"
	}
	if strings.Contains(lower, "beam") || strings.Contains(lower, "truss") {
		return "beams"
	}
	if strings.Contains(lower, "railing") || strings.Contains(lower, "fence") || strings.Contains(lower, "ladder") ||
		strings.Contains(lower, "barrier") || strings.Contains(lower, "fan") ||
		strings.Contains(lower, "vent") {
		return "attachments"
	}
	if strings.Contains(lower, "decoration") || strings.Contains(lower, "statue") || strings.Contains(lower, "monument") {
		return "statues"
	}
	if strings.Contains(lower, "blueprintdesigner") || strings.Contains(lower, "blueprint_designer") {
		return "other"
	}
	if strings.Contains(lower, "sign") && !strings.Contains(lower, "blueprint") {
		return "signs"
	}
	return "other"
}

// computeMapCategory classifies a lowercased typePath into a map category for
// BuildMapData's "buildings from BuildingFacts" pass. Call
// classInfoFor(typePath).mapCategory instead; this is the uncached
// implementation - it used to run inline in BuildMapData's per-instance loop
// (once per building instance, not once per distinct class name).
func computeMapCategory(lower string) uint8 {
	switch {
	case strings.Contains(lower, "miner") || strings.Contains(lower, "extractor") || strings.Contains(lower, "waterpump") || strings.Contains(lower, "oilpump") || strings.Contains(lower, "frackingsatellite") || strings.Contains(lower, "frackingsmasher"):
		return MapCatMiner
	case strings.Contains(lower, "trainstation") || strings.Contains(lower, "trainplatform") || strings.Contains(lower, "truckstation") || strings.Contains(lower, "dronestation") || strings.Contains(lower, "dockingstation"):
		return MapCatStation
	case strings.Contains(lower, "generator") || strings.Contains(lower, "biomass") || strings.Contains(lower, "alienpower"):
		return MapCatPowerGen
	case strings.Contains(lower, "powerstorage") || strings.Contains(lower, "powerswitch") || strings.Contains(lower, "prioritypowerswitch"):
		return MapCatPowerStorage
	case strings.Contains(lower, "railroadblocksignal") || strings.Contains(lower, "railroadpathsignal"):
		return MapCatSignal
	case strings.Contains(lower, "elevator") || strings.Contains(lower, "resourcesink") || strings.Contains(lower, "blueprintdesigner") || strings.Contains(lower, "storage") || strings.Contains(lower, "container") || strings.Contains(lower, "depot") || strings.Contains(lower, "industrialtank") || strings.Contains(lower, "pipestoragetank"):
		return MapCatSpecial
	case strings.Contains(lower, "pipelinepump") || strings.Contains(lower, "pipepump"):
		return MapCatPipe
	case strings.Contains(lower, "pipelinejunction") || strings.Contains(lower, "valve"):
		return MapCatPipe
	case strings.Contains(lower, "pipehyperstart") || (strings.Contains(lower, "hypertube") && strings.Contains(lower, "junction")):
		return MapCatHypertube
	case strings.Contains(lower, "railroadendstop"):
		return MapCatRail
	default:
		return MapCatBuilding
	}
}

// IsBeltPipeSupportItem reports whether a type is one of the belt/pipe supports.
func IsBeltPipeSupportItem(typeLower string) bool {
	return strings.Contains(typeLower, "conveyor") ||
		strings.Contains(typeLower, "pipeline") ||
		strings.Contains(typeLower, "pipe") ||
		strings.Contains(typeLower, "stackable") ||
		strings.Contains(typeLower, "valve") ||
		strings.Contains(typeLower, "junction") ||
		strings.Contains(typeLower, "pump") ||
		strings.Contains(typeLower, "hypertubeentrance") ||
		strings.Contains(typeLower, "hypertubesupport") ||
		strings.Contains(typeLower, "hypertube")
}
