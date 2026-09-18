package extraction

import (
	"encoding/binary"
	"math"
	"strings"

	"satisfacts/parser"
)

// Map categories (Uint8, 0-255 range).
const (
	MapCatBuilding     uint8 = 0
	MapCatMiner        uint8 = 1
	MapCatStation      uint8 = 2
	MapCatPowerPole    uint8 = 3
	MapCatBelt         uint8 = 4
	MapCatPipe         uint8 = 5
	MapCatRail         uint8 = 6
	MapCatHypertube    uint8 = 7
	MapCatResourceNode uint8 = 9
	MapCatStructure    uint8 = 10
	MapCatFoundation   uint8 = 11
	MapCatBlueSlug     uint8 = 12
	MapCatYellowSlug   uint8 = 13
	MapCatPurpleSlug   uint8 = 14
	MapCatSomersloop   uint8 = 15
	MapCatMercerSphere uint8 = 16
	MapCatCrashSite    uint8 = 17
	MapCatPowerGen     uint8 = 18
	MapCatPowerStorage uint8 = 19
	MapCatSignal       uint8 = 20
	MapCatSpecial      uint8 = 21
)

// beamThicknessScale controls how thick beams/pillars render on the map relative
// to their actual cross-section. 0.5 = half width. Lower = thinner.
const beamThicknessScale = 0.3

// pillarRenderSize is the fixed half-extent (meters) used to render pillars on
// the map, regardless of their actual footprint. Pillars are vertical so their
// top-down projection is just a small square — a fixed size looks better than
// scaling the full clearance box.
const pillarRenderSize = 0.5

// MapData is the compact map data embedded in the HTML report.
// Objects and polylines are packed as binary and embedded as separate
// gzip-compressed blobs in the HTML (not through the JSON pipeline).
// Coordinate scale: int32 X/Y in 1cm units (covers ±21474km, 1cm precision).
// Z stays int16 at 10cm units (altitude range is small).
type MapData struct {
	// Binary blob: 19 bytes per object [int32 x, int32 y, int16 z, uint8 cat, uint16 typeIdx, int16 yaw, uint16 halfLenOvr, uint16 halfDepthOvr]
	// x,y in 1cm units (int32, covers full world). z in 10cm units (int16, altitude only).
	// cat is category. typeIdx is index into Types.
	// yaw is degrees*10 (0-3599). halfLenOvr is half-length in 1cm units (0 = use type default, overrides halfWidth for beams).
	// All little-endian.
	// Excluded from JSON — embedded as separate gzip-compressed script tag.
	ObjectsBin []byte        `json:"-"`
	Types      []MapTypeInfo `json:"types"`
	// Binary blob with delta-encoded polyline points:
	// Per polyline: uint8 cat, uint16 typeIdx, uint16 pointCount, int16 z (10cm units),
	//   then first point as (int32 x, int32 y) absolute (10cm units),
	//   then remaining points as (int8 dx, int8 dy) deltas from previous point.
	// If a delta doesn't fit in int8 range, it's stored as (int16 dx, int16 dy)
	// with a sentinel: first byte 0x80 means "16-bit delta follows".
	// Excluded from JSON — embedded as separate gzip-compressed script tag.
	PolylinesBin []byte `json:"-"`
	// MinerResources maps object index -> resource class name (e.g. "Desc_IronOre_C")
	// for miner/extractor objects. Used by the map tooltip to show which resource
	// a miner is extracting. Small map (only miners), so JSON is fine.
	MinerResources map[int]string `json:"minerResources,omitempty"`

	// BuildingInfo maps object index -> building details (recipe, status, efficiency, fuel)
	// for production buildings, miners, and generators. Used by the map tooltip.
	BuildingInfo map[int]MapBuildingInfo `json:"buildingInfo,omitempty"`
}

// MapBuildingInfo holds per-building tooltip data for the map.
type MapBuildingInfo struct {
	Recipe     string  `json:"recipe,omitempty"`     // recipe class path (e.g. "Recipe_IronPlate_C")
	Status     string  `json:"status,omitempty"`     // "Running", "Paused", "Idle"
	Efficiency float64 `json:"efficiency,omitempty"` // 0-100, in-game efficiency %
	HasEff     bool    `json:"hasEff,omitempty"`     // true when a productivity measurement exists — show the row even at 0%
	Fuel       string  `json:"fuel,omitempty"`       // fuel class path (e.g. "Desc_Coal_C")
	Name       string  `json:"name,omitempty"`       // custom name (train stations), replaces the type name in the tooltip
}

// MapTypeInfo describes one building/object type for the map.
type MapTypeInfo struct {
	Path      string  `json:"path"`
	Name      string  `json:"name"`
	HalfWidth float64 `json:"hw,omitempty"` // half-width in meters
	HalfDepth float64 `json:"hd,omitempty"` // half-depth in meters
}

// MapEntry is collected during ProcessObject for non-building objects.
type MapEntry struct {
	X, Y, Z        float32 // cm
	Qx, Qy, Qz, Qw float32 // quaternion rotation
	HasRot         bool
	Cat            uint8
	TypePath       string
	BeamLength     float32 // cm, 0 = not a beam/adaptive structure
}

// MapPolylineEntry is collected during ProcessObject for belts, pipes, etc.
type MapPolylineEntry struct {
	Cat      uint8
	TypePath string
	Points   [][2]float64 // world meters [x, y]
	Tangents [][4]float64 // per-point [leaveTanX, leaveTanY, arriveTanX, arriveTanY] in world meters; nil for straight lines
	Z        float64      // average Z (meters) for z-layer filtering
}

// PowerLineRef stores source/target object refs for deferred position resolution.
type PowerLineRef struct {
	TypePath string
	SrcPath  string
	TgtPath  string
}

// AddMapEntry collects a map entry during ProcessObject.
func (e *ExtractedData) AddMapEntry(obj *parser.SaveObject, cat uint8) {
	e.AddMapEntryWithType(obj, cat, obj.Header.ClassName)
}

// AddMapEntryWithType is like AddMapEntry but allows overriding the type path.
// Used to merge lift-variant splitters/mergers into their base type for the
// map type list.
func (e *ExtractedData) AddMapEntryWithType(obj *parser.SaveObject, cat uint8, typePath string) {
	if obj.Header == nil || !obj.Header.NeedTransform {
		return
	}
	t := obj.Header.Transform
	entry := MapEntry{
		X:        t.Translation[0],
		Y:        t.Translation[1],
		Z:        t.Translation[2],
		Cat:      cat,
		TypePath: typePath,
	}
	// Check if rotation is non-identity
	if t.Rotation[3] != 1 || t.Rotation[0] != 0 || t.Rotation[1] != 0 || t.Rotation[2] != 0 {
		entry.Qx = t.Rotation[0]
		entry.Qy = t.Rotation[1]
		entry.Qz = t.Rotation[2]
		entry.Qw = t.Rotation[3]
		entry.HasRot = true
	}
	e.MapEntries = append(e.MapEntries, entry)
}

// resolveMinerResource determines which resource a miner/extractor is extracting.
// Returns a Desc_X_C class name (e.g. "Desc_IronOre_C") or "" if unknown.
// Uses the same resolution chain as post-extraction: ResourceNodeFacts override
// (1.2+ saves) -> resource_wells.json -> resource_node_positions.json -> output
// inventory inference.
func resolveMinerResource(bf *BuildingFacts, ext *ExtractedData, resourceNodeLookup map[string]string) string {
	if bf.ExtractableResource != "" {
		// 1.2+ saves: check ResourceNodeFacts for override
		if rf, ok := ext.ResourceNodeFacts[bf.ExtractableResource]; ok && rf != nil {
			if rf.ResourceClassOverride != "" {
				if m := descNameRegex.FindStringSubmatch(rf.ResourceClassOverride); m != nil {
					return resourceClassName(m[1])
				}
			}
		}
		// Fall back to resource_wells.json (fracking satellites)
		if wt, _, ok := lookupResourceWell(bf.ExtractableResource); ok {
			return resourceClassName(wt)
		}
		// Fall back to resource_node_positions.json (regular nodes)
		if rt, ok := resourceNodeLookup[bf.ExtractableResource]; ok {
			return rt
		}
	}
	// Final fallback: infer from output inventory
	return inferResourceFromOutputInventory(bf, ext.InventoryStacks)
}

// buildMapBuildingInfo extracts tooltip-relevant info from BuildingFacts.
func buildMapBuildingInfo(bf *BuildingFacts, cat uint8, ext *ExtractedData) MapBuildingInfo {
	var info MapBuildingInfo

	// Recipe (production buildings only — miners/generators don't have mCurrentRecipe)
	if cat == MapCatBuilding && bf.CurrentRecipe != "" {
		info.Recipe = bf.CurrentRecipe
	}

	// Status: derive simple running state from BuildingFacts
	if bf.IsProductionPaused {
		info.Status = "Paused"
	} else if bf.HasIsProducing {
		if bf.IsProducing {
			info.Status = "Running"
		} else {
			info.Status = "Idle"
		}
	} else if cat == MapCatPowerGen && bf.HasDynamicProductionCapacity && bf.DynamicProductionCapacity > 0 {
		info.Status = "Running"
	} else {
		// Newly placed buildings that never ran have no status properties —
		// the game writes mIsProducing only after the building's first tick.
		info.Status = "Idle"
	}

	// Efficiency from productivity measurement (same as in-game UI).
	// HasEff marks that a measurement exists so the tooltip can show 0%
	// for idle buildings (in-game also shows 0% efficiency when idle).
	if bf.HasLastProductivity && bf.LastProductivityDuration > 0 {
		info.Efficiency = float64(bf.LastProductivityProduceDuration) / float64(bf.LastProductivityDuration) * 100
		info.HasEff = true
		if info.Efficiency < 0 {
			info.Efficiency = 0
		}
		if info.Efficiency > 100 {
			info.Efficiency = 100
		}
	}

	// Fuel for generators — nuclear plants have mCurrentFuelClass, other generators
	// (coal, biomass, fuel) store fuel in mFuelInventory / mInputInventory.
	// Resolve to the full item class path so the frontend displayName() works.
	if cat == MapCatPowerGen {
		if bf.CurrentFuelClass != "" {
			info.Fuel = bf.CurrentFuelClass
		} else {
			// Look up the fuel inventory, then the input inventory, and take
			// the first non-empty stack's raw class path.
			for _, invRef := range []string{bf.FuelInventory, bf.InputInventory} {
				if invRef == "" {
					continue
				}
				if stacks, ok := ext.InventoryStacks[invRef]; ok {
					for _, st := range stacks {
						if st.rawClass != "" {
							info.Fuel = st.rawClass
							break
						}
					}
				}
				if info.Fuel != "" {
					break
				}
			}
		}
	}

	return info
}

func isEmptyBuildingInfo(info MapBuildingInfo) bool {
	return info.Recipe == "" && info.Status == "" && !info.HasEff && info.Fuel == "" && info.Name == ""
}

// BuildMapData assembles compact map data from all sources after streaming.
// Called before CleanupAfterStreaming.
func BuildMapData(ext *ExtractedData) *MapData {
	typeLookup := make(map[string]int) // typePath -> index
	types := []MapTypeInfo{}

	getTypeIdx := func(typePath string) int {
		if idx, ok := typeLookup[typePath]; ok {
			return idx
		}
		idx := len(types)
		typeLookup[typePath] = idx
		types = append(types, MapTypeInfo{
			Path: typePath,
			Name: mapDisplayNameForType(typePath),
		})
		return idx
	}

	var objBuf []byte
	var objCount int
	appendObject := func(xCm, yCm, zCm float64, cat uint8, typeIdx int, yaw int16, halfLenOvr uint16, halfDepthOvr uint16) int {
		// Skip objects at world origin (0,0) — these are phantom objects
		// (e.g. FGTrainStationIdentifier) with default transforms, not real buildings.
		if xCm == 0 && yCm == 0 {
			return -1
		}
		objBuf = binary.LittleEndian.AppendUint32(objBuf, uint32(cmToInt32(xCm)))
		objBuf = binary.LittleEndian.AppendUint32(objBuf, uint32(cmToInt32(yCm)))
		objBuf = binary.LittleEndian.AppendUint16(objBuf, uint16(cmToInt16(zCm)))
		objBuf = append(objBuf, cat)
		objBuf = binary.LittleEndian.AppendUint16(objBuf, uint16(typeIdx))
		objBuf = binary.LittleEndian.AppendUint16(objBuf, uint16(yaw))
		objBuf = binary.LittleEndian.AppendUint16(objBuf, halfLenOvr)
		objBuf = binary.LittleEndian.AppendUint16(objBuf, halfDepthOvr)
		idx := objCount
		objCount++
		return idx
	}

	// Build resource node lookup from static JSON for miner resource resolution.
	// Loaded early so the building loop can resolve miner resources.
	resourceNodeEntries := loadResourceNodeEntries()
	resourceNodeLookup := make(map[string]string) // instanceName -> Desc_X_C
	for _, rn := range resourceNodeEntries {
		resourceNodeLookup[rn.InstanceName] = rn.ResourceType
	}

	minerResources := make(map[int]string)
	buildingInfo := make(map[int]MapBuildingInfo)

	stationIDKeys := sortedKeys(ext.rawTrainStationIDs)

	// 1. Buildings from BuildingFacts
	for instanceName, bf := range ext.BuildingFacts {
		if !bf.HasTransform {
			continue
		}
		// Classified once per distinct class name (classInfoFor is a shared
		// cache keyed by typePath), not once per building instance - this
		// loop can run over hundreds of thousands of buildings sharing only
		// a few thousand distinct class names.
		cat := classInfoFor(bf.TypePath).mapCategory
		typeIdx := getTypeIdx(bf.TypePath)
		yaw := int16(0)
		if bf.HasRotation {
			yaw = quaternionToYaw10(bf.Rotation[0], bf.Rotation[1], bf.Rotation[2], bf.Rotation[3])
		}
		idx := appendObject(float64(bf.Translation[0]), float64(bf.Translation[1]), float64(bf.Translation[2]), cat, typeIdx, yaw, 0, 0)
		if idx >= 0 && cat == MapCatMiner {
			if res := resolveMinerResource(bf, ext, resourceNodeLookup); res != "" {
				minerResources[idx] = res
			}
		}
		// Collect tooltip info for production buildings, miners, and generators
		if idx >= 0 && (cat == MapCatBuilding || cat == MapCatMiner || cat == MapCatPowerGen) {
			if info := buildMapBuildingInfo(bf, cat, ext); !isEmptyBuildingInfo(info) {
				buildingInfo[idx] = info
			}
		}
		// Train station names for the tooltip
		if idx >= 0 && cat == MapCatStation {
			if _, ok := ext.rawTrainStations[instanceName]; ok {
				if name := ext.trainStationName(instanceName, stationIDKeys); name != "" {
					buildingInfo[idx] = MapBuildingInfo{Name: name}
				}
			}
		}
	}

	// Collect hypertube polyline endpoints for entrance snapping.
	// Entrances (Build_PipeHyperStart) have an actor center offset from where
	// tubes actually connect. Snap them to the nearest tube endpoint so they
	// appear exactly at the gaps between tube segments.
	htEndpoints := [][2]float64{}
	for _, pe := range ext.PolylineEntries {
		if pe.Cat != MapCatHypertube || len(pe.Points) < 2 {
			continue
		}
		htEndpoints = append(htEndpoints, pe.Points[0])
		htEndpoints = append(htEndpoints, pe.Points[len(pe.Points)-1])
	}

	// 2. Non-building entries collected during ProcessObject
	for _, entry := range ext.MapEntries {
		typeIdx := getTypeIdx(entry.TypePath)
		yaw := int16(0)
		var halfLenOvr uint16
		var halfDepthOvr uint16
		// Snap hypertube entrances to nearest tube endpoint
		isHtEntrance := entry.Cat == MapCatHypertube && strings.Contains(strings.ToLower(entry.TypePath), "pipehyperstart")
		if isHtEntrance && len(htEndpoints) > 0 {
			ex := float64(entry.X) / 100.0
			ey := float64(entry.Y) / 100.0
			bestD := math.MaxFloat64
			bestIdx := -1
			for i, ep := range htEndpoints {
				dx := ep[0] - ex
				dy := ep[1] - ey
				d := dx*dx + dy*dy
				if d < bestD {
					bestD = d
					bestIdx = i
				}
			}
			if bestIdx >= 0 {
				entry.X = float32(htEndpoints[bestIdx][0] * 100)
				entry.Y = float32(htEndpoints[bestIdx][1] * 100)
			}
			// Entrances have no footprint data — give them a fixed 1m half-size
			halfLenOvr = cmToUint16(100)
			halfDepthOvr = cmToUint16(100)
		}
		beamX, beamY := float64(entry.X), float64(entry.Y)
		if entry.BeamLength > 0 && entry.HasRot {
			var dirX, dirY float64
			yaw, dirX, dirY = quaternionToBeamYaw10(entry.Qx, entry.Qy, entry.Qz, entry.Qw)
			if dirX != 0 || dirY != 0 {
				// dirX/dirY are the beam's unit X axis projected onto the map plane,
				// so their magnitude is cos(pitch): a pitched beam covers less ground
				// than its 3D length. Both the drawn length and the centre offset must
				// use that foreshortened value, or pitched beams overshoot their
				// foundation.
				halfLenCm := float64(entry.BeamLength) / 2
				xyLen := math.Hypot(dirX, dirY)
				halfLenOvr = cmToUint16(halfLenCm * xyLen)
				// Shrink beam cross-section so beams don't dominate the map at full width.
				_, hd, ok := footprintForType(entry.TypePath)
				if ok && hd > 0 {
					halfDepthOvr = cmToUint16(hd * 100 * beamThicknessScale)
				}
				// Beams extend from 0 to length in local X, so the actor position is at
				// one end. Offset the center by length/2 along the beam direction.
				beamX += dirX * halfLenCm
				beamY += dirY * halfLenCm
			} else {
				// Vertical beam (pillar): render as small fixed marker.
				halfLenOvr = cmToUint16(pillarRenderSize * 100)
				halfDepthOvr = halfLenOvr
			}
		} else if entry.HasRot {
			isPillar := strings.Contains(strings.ToLower(entry.TypePath), "pillar")
			// A tilted object's map silhouette is the projection of its 3D clearance
			// box, which is neither the type's local footprint nor axis-aligned. Take
			// the oriented bounding box of that projection: it carries both the
			// on-map heading and the extents measured in that same frame, so the
			// renderer's rotate-then-draw produces the correct outline.
			qx2 := float64(entry.Qx) * float64(entry.Qx)
			qy2 := float64(entry.Qy) * float64(entry.Qy)
			if qx2+qy2 > 0.001 {
				phw, phd, pyaw, offX, offY, ok := projectedFootprintForInstance(entry.TypePath, entry.Qx, entry.Qy, entry.Qz, entry.Qw)
				if ok && phw > 0 && phd > 0 {
					if isPillar {
						// For pillars: keep the longer dimension (length) full so
						// horizontal pillars connect. Scale only the shorter
						// dimension (thickness). If roughly square (vertical
						// pillar), use a fixed small size instead.
						aspect := phw / phd
						if aspect > 0.8 && aspect < 1.25 {
							// Vertical pillar — small fixed marker
							phw = pillarRenderSize
							phd = pillarRenderSize
						} else if phw >= phd {
							// Length is in X, thickness in Y
							phd *= beamThicknessScale
						} else {
							// Length is in Y, thickness in X
							phw *= beamThicknessScale
						}
					}
					halfLenOvr = cmToUint16(phw * 100)
					halfDepthOvr = cmToUint16(phd * 100)
					yaw = pyaw
					beamX += offX
					beamY += offY
				} else {
					yaw = quaternionToYaw10(entry.Qx, entry.Qy, entry.Qz, entry.Qw)
				}
			} else {
				// Pure yaw rotation: the type's local footprint is already correct.
				yaw = quaternionToYaw10(entry.Qx, entry.Qy, entry.Qz, entry.Qw)
			}
		}
		appendObject(beamX, beamY, float64(entry.Z), entry.Cat, typeIdx, yaw, halfLenOvr, halfDepthOvr)
	}

	// 3. Collectibles — sub-categorized by type (cats 12-17)
	// Skip collected/dismantled collectibles. Use save positions when available,
	// fall back to static reference positions from collectible_positions.json.
	// All six kinds differ only in which reference map they come from, which
	// collected-set marks them gone, and which category/type they get, so they
	// share one loop. Every kind renders at the same fixed 40cm half-extent.
	if collectiblePositionsLoaded != nil {
		cp := collectiblePositionsLoaded
		cc := ext.CollectedCollectibles
		collectibleKinds := []struct {
			positions map[string]Position
			collected map[string]bool
			cat       uint8
			typePath  string
		}{
			{cp.PowerSlugs.Blue, cc.PowerSlugsBlue, MapCatBlueSlug, "__collectible_blue_slug__"},
			{cp.PowerSlugs.Yellow, cc.PowerSlugsYellow, MapCatYellowSlug, "__collectible_yellow_slug__"},
			{cp.PowerSlugs.Purple, cc.PowerSlugsPurple, MapCatPurpleSlug, "__collectible_purple_slug__"},
			{cp.Somersloops, cc.Somersloops, MapCatSomersloop, "__collectible_somersloop__"},
			{cp.MercerSpheres, cc.MercerSpheres, MapCatMercerSphere, "__collectible_mercer_sphere__"},
			{cp.CrashSites, cc.CrashSites, MapCatCrashSite, "__collectible_crash_site__"},
		}
		for _, kind := range collectibleKinds {
			typeIdx := getTypeIdx(kind.typePath)
			for ref, pos := range kind.positions {
				if kind.collected[ref] {
					continue
				}
				var x, y, z float64
				if savePos, ok := ext.CollectibleLocations[ref]; ok {
					x, y, z = float64(savePos[0]), float64(savePos[1]), float64(savePos[2])
				} else {
					x, y, z = float64(pos.X)/100, float64(pos.Y)/100, float64(pos.Z)/100
				}
				appendObject(x, y, z, kind.cat, typeIdx, 0, 40, 40)
			}
		}
	}

	// 4. Resource nodes from static JSON — per-type typeIdx, purity in yaw (0=impure, 1=normal, 2=pure)
	// Map Desc_* paths to __resource_*__ synthetic paths for frontend i18n + color lookup.
	resourceDescMap := map[string]string{
		"Desc_OreIron_C":     "__resource_iron_ore__",
		"Desc_Stone_C":       "__resource_stone__",
		"Desc_Coal_C":        "__resource_coal__",
		"Desc_Water_C":       "__resource_water__",
		"Desc_OreCopper_C":   "__resource_copper_ore__",
		"Desc_LiquidOil_C":   "__resource_crude_oil__",
		"Desc_NitrogenGas_C": "__resource_nitrogen_gas__",
		"Desc_Geyser_C":      "__resource_geyser__",
		"Desc_SAM_C":         "__resource_sam__",
		"Desc_OreGold_C":     "__resource_caterium_ore__",
		"Desc_RawQuartz_C":   "__resource_quartz__",
		"Desc_OreBauxite_C":  "__resource_bauxite__",
		"Desc_Sulfur_C":      "__resource_sulfur__",
		"Desc_OreUranium_C":  "__resource_uranium_ore__",
	}
	// Build set of resource nodes that have an extractor built on them.
	occupiedNodes := make(map[string]bool)
	for _, bf := range ext.BuildingFacts {
		if bf.ExtractableResource != "" {
			occupiedNodes[bf.ExtractableResource] = true
		}
	}
	resourceNodes := resourceNodeEntries
	for _, rn := range resourceNodes {
		if occupiedNodes[rn.InstanceName] {
			continue
		}
		// Start with static JSON defaults, then override from save data if available.
		// 1.2+ saves with randomized nodes carry mResourceClassOverride and mPurityOverride
		// on each BP_ResourceNode / BP_FrackingSatellite. Default saves have empty overrides.
		resourceType := rn.ResourceType
		purity := rn.Purity
		if rf, ok := ext.ResourceNodeFacts[rn.InstanceName]; ok && rf != nil {
			if rf.ResourceClassOverride != "" {
				// Extract Desc_X_C from path like /Game/.../Desc_Coal.Desc_Coal_C
				if idx := strings.LastIndex(rf.ResourceClassOverride, "."); idx >= 0 {
					resourceType = rf.ResourceClassOverride[idx+1:]
				} else {
					resourceType = rf.ResourceClassOverride
				}
			}
			if rf.PurityOverride != "" {
				purity = rf.PurityOverride
			}
		}
		syntheticPath := resourceDescMap[resourceType]
		if syntheticPath == "" {
			syntheticPath = "__resource_unknown__"
		}
		typeIdx := getTypeIdx(syntheticPath)
		var yaw int16
		switch strings.ToUpper(purity) {
		case "RP_IMPURE", "IMPURE":
			yaw = 0
		case "RP_NORMAL", "NORMAL":
			yaw = 1
		case "RP_PURE", "PURE":
			yaw = 2
		default:
			yaw = 0
		}
		appendObject(rn.Position[0], rn.Position[1], rn.Position[2], MapCatResourceNode, typeIdx, yaw, 300, 300)
	}

	// 5. Rails from rawTrackSegments are already drawn as polylines —
	// skip redundant point entries that create stray squares with mismatched Z.

	// Build spatial grid of building centers (in meters) for polyline endpoint snapping.
	// Belts/pipes start at their own transform position, not the building edge,
	// causing a visible gap. Snapping endpoints to nearby building centers fills
	// the gap — the building rectangle drawn on top hides the overshoot.
	// Also includes point objects (lifts, splitters, mergers, pumps, junctions)
	// so belt/pipe endpoints snap toward them too.
	const gridCellM = 20.0
	const snapRadiusM = 15.0
	type gridKey struct{ x, y int }
	buildingGrid := make(map[gridKey][][2]float64)
	for _, bf := range ext.BuildingFacts {
		if !bf.HasTransform {
			continue
		}
		bx := float64(bf.Translation[0]) / 100.0 // cm -> meters
		by := float64(bf.Translation[1]) / 100.0
		key := gridKey{int(math.Floor(bx / gridCellM)), int(math.Floor(by / gridCellM))}
		buildingGrid[key] = append(buildingGrid[key], [2]float64{bx, by})
	}
	for _, entry := range ext.MapEntries {
		if entry.Cat != MapCatBelt && entry.Cat != MapCatPipe && entry.Cat != MapCatHypertube {
			continue
		}
		bx := float64(entry.X) / 100.0
		by := float64(entry.Y) / 100.0
		key := gridKey{int(math.Floor(bx / gridCellM)), int(math.Floor(by / gridCellM))}
		buildingGrid[key] = append(buildingGrid[key], [2]float64{bx, by})
	}
	findNearestBuilding := func(x, y float64) (float64, float64, bool) {
		gx := int(math.Floor(x / gridCellM))
		gy := int(math.Floor(y / gridCellM))
		bestDist := snapRadiusM * snapRadiusM
		bestX, bestY := 0.0, 0.0
		found := false
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				if pts, ok := buildingGrid[gridKey{gx + dx, gy + dy}]; ok {
					for _, bp := range pts {
						ddx := bp[0] - x
						ddy := bp[1] - y
						d2 := ddx*ddx + ddy*ddy
						if d2 < bestDist {
							bestDist = d2
							bestX = bp[0]
							bestY = bp[1]
							found = true
						}
					}
				}
			}
		}
		return bestX, bestY, found
	}

	// Belt and pipe pieces are chained end to end — on a large save ~97% of
	// endpoints already sit on another piece's endpoint, most of them exactly.
	// Extending those would push each piece into its neighbour (up to 2×
	// snapExtendM of overlap) and, because a stub runs straight along the end
	// tangent while the chain keeps curving, would visibly cut corners on curved
	// runs. Only genuinely free ends — the ones that terminate at a machine —
	// have a gap worth filling, so index every belt/pipe endpoint to tell them
	// apart.
	const chainJointM = 2.5
	endpointGrid := make(map[gridKey][][2]float64)
	for _, pe := range ext.PolylineEntries {
		if pe.Cat != MapCatBelt && pe.Cat != MapCatPipe {
			continue
		}
		if len(pe.Points) < 2 {
			continue
		}
		for _, p := range [][2]float64{pe.Points[0], pe.Points[len(pe.Points)-1]} {
			key := gridKey{int(math.Floor(p[0] / gridCellM)), int(math.Floor(p[1] / gridCellM))}
			endpointGrid[key] = append(endpointGrid[key], p)
		}
	}
	// isChainJoint reports whether p is an interior joint of a belt/pipe chain.
	// p is itself in the index, so it always finds one match; two or more means a
	// different piece ends at the same place. Coincident endpoints are the common
	// case, so this counts rather than comparing identity.
	isChainJoint := func(p [2]float64) bool {
		gx := int(math.Floor(p[0] / gridCellM))
		gy := int(math.Floor(p[1] / gridCellM))
		n := 0
		for dx := -1; dx <= 1; dx++ {
			for dy := -1; dy <= 1; dy++ {
				for _, q := range endpointGrid[gridKey{gx + dx, gy + dy}] {
					if math.Hypot(q[0]-p[0], q[1]-p[1]) <= chainJointM {
						n++
						if n >= 2 {
							return true
						}
					}
				}
			}
		}
		return false
	}

	// Resolve footprints for all types
	resolveFootprints(types)

	// Build polylines as binary blob.
	// For very large saves, simplify to start/end points to keep size manageable.
	// No more skipping belts/pipes entirely — binary encoding is compact enough.
	//
	// The budget is expressed in *rendered* line segments, not stored points:
	// tangent-mode polylines are subdivided by the frontend, where a span becomes
	// one segment when flat and up to 24 when genuinely curved. Each rendered
	// segment costs ~28 bytes of GPU instance data, so 3M segments is ~84 MB.
	const maxRenderedSegments = 3_000_000
	// Conservative average spans -> segments expansion for tangent-mode polylines.
	// Measured ~1.7 on a large save; 3 leaves headroom for curve-heavy factories.
	const tangentExpansionEstimate = 3

	estRenderedSegments := 0
	for _, pe := range ext.PolylineEntries {
		if len(pe.Points) < 2 {
			continue
		}
		spans := len(pe.Points) - 1
		if pe.Tangents != nil && len(pe.Tangents) == len(pe.Points) {
			spans *= tangentExpansionEstimate
		}
		estRenderedSegments += spans
	}
	simplify := estRenderedSegments > maxRenderedSegments

	var polyBuf []byte
	appendPolyline := func(cat uint8, typeIdx int, pts [][2]float64, tangents [][4]float64, z float64) {
		hasTan := tangents != nil && len(tangents) == len(pts)
		polyBuf = append(polyBuf, cat)
		polyBuf = binary.LittleEndian.AppendUint16(polyBuf, uint16(typeIdx))
		ptCount := uint16(len(pts))
		if hasTan {
			ptCount |= 0x8000 // flag: has tangents
		}
		polyBuf = binary.LittleEndian.AppendUint16(polyBuf, ptCount)
		// Z as int16 in 10cm units (same scale as object positions).
		// Clamped — an unchecked cast would wrap a >3276.7m altitude to the
		// opposite extreme and hide the polyline under the Z filter.
		polyBuf = binary.LittleEndian.AppendUint16(polyBuf, uint16(metersToInt16(z)))
		if hasTan {
			// Tangent mode: each point is int32 x + int32 y + 4×float16 tangents = 16 bytes
			for i := range pts {
				polyBuf = binary.LittleEndian.AppendUint32(polyBuf, uint32(metersToInt32(pts[i][0])))
				polyBuf = binary.LittleEndian.AppendUint32(polyBuf, uint32(metersToInt32(pts[i][1])))
				polyBuf = binary.LittleEndian.AppendUint16(polyBuf, float16(tangents[i][0]))
				polyBuf = binary.LittleEndian.AppendUint16(polyBuf, float16(tangents[i][1]))
				polyBuf = binary.LittleEndian.AppendUint16(polyBuf, float16(tangents[i][2]))
				polyBuf = binary.LittleEndian.AppendUint16(polyBuf, float16(tangents[i][3]))
			}
			return
		}
		// No tangents: delta-encoded format (for power lines, etc.)
		// First point: absolute int32 (X/Y can span full world)
		prevX := metersToInt32(pts[0][0])
		prevY := metersToInt32(pts[0][1])
		polyBuf = binary.LittleEndian.AppendUint32(polyBuf, uint32(prevX))
		polyBuf = binary.LittleEndian.AppendUint32(polyBuf, uint32(prevY))
		// Subsequent points: delta-encoded
		for i := 1; i < len(pts); i++ {
			curX := metersToInt32(pts[i][0])
			curY := metersToInt32(pts[i][1])
			dx := curX - prevX
			dy := curY - prevY
			if dx >= -127 && dx <= 127 && dy >= -127 && dy <= 127 {
				// int8 delta: 2 bytes
				polyBuf = append(polyBuf, byte(int8(dx)))
				polyBuf = append(polyBuf, byte(int8(dy)))
			} else if dx >= -32768 && dx <= 32767 && dy >= -32768 && dy <= 32767 {
				// 16-bit delta: sentinel + 4 bytes
				polyBuf = append(polyBuf, 0x80) // sentinel: means 16-bit delta follows
				polyBuf = append(polyBuf, 0x80)
				polyBuf = binary.LittleEndian.AppendUint16(polyBuf, uint16(int16(dx)))
				polyBuf = binary.LittleEndian.AppendUint16(polyBuf, uint16(int16(dy)))
			} else {
				// 32-bit delta: sentinel + 8 bytes (for deltas exceeding int16 range)
				polyBuf = append(polyBuf, 0x80, 0x81)
				polyBuf = binary.LittleEndian.AppendUint32(polyBuf, uint32(dx))
				polyBuf = binary.LittleEndian.AppendUint32(polyBuf, uint32(dy))
			}
			prevX = curX
			prevY = curY
		}
	}

	for _, pe := range ext.PolylineEntries {
		if len(pe.Points) < 2 {
			continue
		}
		// Skip polylines at world origin — phantom objects with default transforms
		if pe.Points[0][0] == 0 && pe.Points[0][1] == 0 {
			continue
		}
		ti := getTypeIdx(pe.TypePath)

		// Polylines with tangents (belts, pipes, rails, hypertubes) keep the raw
		// control points from the save file; the frontend renders them as Bezier
		// curves using the tangents.
		if pe.Tangents != nil && len(pe.Tangents) == len(pe.Points) {
			if simplify {
				// Over budget: drop the tangents so the frontend cannot expand
				// this polyline, keeping the control points as a straight chord
				// chain. Shape is coarser but every belt still shows up.
				appendPolyline(pe.Cat, ti, simplifyPoints(pe.Points), nil, pe.Z)
				continue
			}
			pts, tans := pe.Points, pe.Tangents
			// Belts and pipes stop short of the building they feed, leaving a
			// visible gap. Rails end at station footprints and hypertube
			// endpoints are deliberately offset from their entrances, so neither
			// wants this.
			if pe.Cat == MapCatBelt || pe.Cat == MapCatPipe {
				pts, tans = snapTangentEnds(pts, tans, findNearestBuilding, isChainJoint)
			}
			appendPolyline(pe.Cat, ti, pts, tans, pe.Z)
			continue
		}

		// If over cap, simplify to just start + end points
		ptsToUse := pe.Points
		if simplify {
			ptsToUse = simplifyPoints(pe.Points)
		}
		// Snap first and last points toward nearby building centers to fill gaps.
		// Move only a few meters in — just enough to enter the building rectangle.
		// Building rectangles drawn on top hide the overshoot inside buildings.
		// The endpoints are replaced in a copy — writing into ptsToUse would
		// mutate pe.Points whenever simplify is off.
		if len(ptsToUse) >= 2 {
			lastIdx := len(ptsToUse) - 1
			start, hasStart := buildingStub(ptsToUse[0], findNearestBuilding)
			end, hasEnd := buildingStub(ptsToUse[lastIdx], findNearestBuilding)
			if hasStart || hasEnd {
				snapped := make([][2]float64, len(ptsToUse))
				copy(snapped, ptsToUse)
				if hasStart {
					snapped[0] = start
				}
				if hasEnd {
					snapped[lastIdx] = end
				}
				ptsToUse = snapped
			}
		}
		appendPolyline(pe.Cat, ti, ptsToUse, nil, pe.Z)
	}

	// Resolve power line refs to positions
	if len(ext.PowerLineRefs) > 0 {
		// Build position lookup: instanceName -> [x, y, z] in cm
		posLookup := make(map[string][3]float64)
		for inst, pos := range ext.ObjectPositions {
			posLookup[inst] = [3]float64{
				float64(pos[0]),
				float64(pos[1]),
				float64(pos[2]),
			}
		}

		// stripLastComponent removes the last ".Component" part from an instance name
		stripLastComponent := func(s string) string {
			idx := strings.LastIndex(s, ".")
			if idx < 0 {
				return s
			}
			return s[:idx]
		}

		powerLineTypeIdx := getTypeIdx("/Game/FactoryGame/Buildable/Factory/PowerLine/Build_PowerLine.Build_PowerLine_C")
		for _, plr := range ext.PowerLineRefs {
			srcInst := stripLastComponent(plr.SrcPath)
			tgtInst := stripLastComponent(plr.TgtPath)
			srcPos, srcOk := posLookup[srcInst]
			tgtPos, tgtOk := posLookup[tgtInst]
			if !srcOk || !tgtOk {
				continue
			}
			polyZ := (srcPos[2] + tgtPos[2]) / 200
			appendPolyline(MapCatPowerPole, powerLineTypeIdx, [][2]float64{
				{srcPos[0] / 100, srcPos[1] / 100},
				{tgtPos[0] / 100, tgtPos[1] / 100},
			}, nil, polyZ)
		}
	}

	return &MapData{
		ObjectsBin:     objBuf,
		Types:          types,
		PolylinesBin:   polyBuf,
		MinerResources: minerResources,
		BuildingInfo:   buildingInfo,
	}
}
