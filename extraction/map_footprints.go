package extraction

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

// buildingDimsEntry is one entry from building_dimensions.json.
type buildingDimsEntry struct {
	DisplayName    string             `json:"displayName"`
	Dimensions     map[string]float64 `json:"dimensions"`
	Clearance      []clearanceBoxJSON `json:"clearance"`
	AdaptiveLength map[string]float64 `json:"adaptiveLength"`
}

type clearanceBoxJSON struct {
	Min      vec3JSON  `json:"min"`
	Max      vec3JSON  `json:"max"`
	Rotation *quatJSON `json:"rotation,omitempty"`
}

type vec3JSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type quatJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
	W float64 `json:"w"`
}

var buildingDimsLoaded map[string]*buildingDimsEntry

// loadBuildingDims loads building_dimensions.json at init time.
func loadBuildingDims() {
	data, err := dataFilesFS.ReadFile("report/data/building_dimensions.json")
	if err != nil {
		return
	}
	var raw map[string]buildingDimsEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	buildingDimsLoaded = make(map[string]*buildingDimsEntry, len(raw))
	for k, v := range raw {
		vv := v
		buildingDimsLoaded[k] = &vv
	}
}

// init loads building dimensions at package init time.
func init() {
	loadBuildingDims()
}

// resolveFootprints fills in HalfWidth/HalfDepth for each type from building_dimensions.json.
func resolveFootprints(types []MapTypeInfo) {
	for i := range types {
		tp := types[i].Path
		// Skip synthetic types
		if strings.HasPrefix(tp, "__") {
			continue
		}
		hw, hd, ok := footprintForType(tp)
		if ok {
			types[i].HalfWidth = hw
			types[i].HalfDepth = hd
		}
	}
}

// footprintForType resolves the footprint (half-width, half-depth) in meters for a building type.
// Priority: clearance boxes -> dimensions -> hardcoded fallbacks.
func footprintForType(typePath string) (halfWidth, halfDepth float64, ok bool) {
	// Short class name: strip path prefix, then strip "Build_X." prefix
	// Save uses "Build_SmelterMk1.Build_SmelterMk1_C", JSON keys are "Build_SmelterMk1_C"
	shortName := typePath
	if idx := strings.LastIndex(shortName, "/"); idx >= 0 {
		shortName = shortName[idx+1:]
	}
	// Strip "Build_X." -> "Build_X_C"
	if idx := strings.LastIndex(shortName, "."); idx >= 0 {
		shortName = shortName[idx+1:]
	}

	entry := buildingDimsLoaded[shortName]
	if entry == nil {
		// Try full path
		entry = buildingDimsLoaded[typePath]
	}
	if entry == nil {
		// Hardcoded fallbacks
		return hardcodedFootprint(shortName)
	}

	// 1. Try clearance boxes
	if len(entry.Clearance) > 0 {
		minX, maxX, minY, maxY := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
		for _, box := range entry.Clearance {
			bx0, by0, bx1, by1 := box.Min.X, box.Min.Y, box.Max.X, box.Max.Y
			bz0, bz1 := box.Min.Z, box.Max.Z
			if box.Rotation != nil {
				rx2, ry2 := box.Rotation.X*box.Rotation.X, box.Rotation.Y*box.Rotation.Y
				if rx2+ry2 > 0.001 {
					// Non-yaw rotation (e.g. beams with rotation 0.5,0.5,0.5,0.5
					// which swaps Z->Y). Project all 8 3D corners to XY.
					for _, cx := range []float64{bx0, bx1} {
						for _, cy := range []float64{by0, by1} {
							for _, cz := range []float64{bz0, bz1} {
								rx, ry, _ := rotate3DByQuaternion(cx, cy, cz,
									box.Rotation.X, box.Rotation.Y, box.Rotation.Z, box.Rotation.W)
								minX = math.Min(minX, rx)
								maxX = math.Max(maxX, rx)
								minY = math.Min(minY, ry)
								maxY = math.Max(maxY, ry)
							}
						}
					}
				} else {
					// Pure yaw rotation: project 4 XY corners
					corners := [4][2]float64{
						{bx0, by0}, {bx1, by0}, {bx1, by1}, {bx0, by1},
					}
					for _, c := range corners {
						rx, ry := rotateByQuaternion(c[0], c[1], box.Rotation.X, box.Rotation.Y, box.Rotation.Z, box.Rotation.W)
						minX = math.Min(minX, rx)
						maxX = math.Max(maxX, rx)
						minY = math.Min(minY, ry)
						maxY = math.Max(maxY, ry)
					}
				}
			} else {
				minX = math.Min(minX, bx0)
				maxX = math.Max(maxX, bx1)
				minY = math.Min(minY, by0)
				maxY = math.Max(maxY, by1)
			}
		}
		width := maxX - minX
		depth := maxY - minY
		if width > 0 && depth > 0 {
			return width / 2 / 100, depth / 2 / 100, true // cm -> m
		}
	}

	// 2. Try dimensions
	if entry.Dimensions != nil {
		w, hasW := entry.Dimensions["Width"]
		d, hasD := entry.Dimensions["Depth"]
		if !hasW || !hasD {
			// Some entries use different field names
			w, hasW = entry.Dimensions["width"]
			d, hasD = entry.Dimensions["depth"]
		}
		if hasW && hasD && w > 0 && d > 0 {
			return w / 2 / 100, d / 2 / 100, true
		}
	}

	// 3. Hardcoded fallbacks
	return hardcodedFootprint(shortName)
}

// projectedFootprintForInstance computes the top-down silhouette of a tilted
// instance as an oriented bounding box. It projects all 8 corners of the 3D
// clearance box through the instance's quaternion, derives the on-map heading
// from the projected local X axis, then measures the corner extents in that
// rotated frame. Returns (halfWidth, halfDepth in meters, yaw in degrees*10,
// offX, offY in cm, ok). The offset shifts the drawn rectangle to the actual
// centroid of the projected box, which may differ from the actor position when
// the box is tilted.
//
// An axis-aligned box would discard the object's orientation and render every
// tilted structure screen-aligned; measuring in the rotated frame keeps the
// silhouette pointing the right way, which is the rectangle equivalent of the
// convex-hull polygon a polygon-capable renderer would emit.
func projectedFootprintForInstance(typePath string, qx, qy, qz, qw float32) (float64, float64, int16, float64, float64, bool) {
	shortName := typePath
	if idx := strings.LastIndex(shortName, "/"); idx >= 0 {
		shortName = shortName[idx+1:]
	}
	if idx := strings.LastIndex(shortName, "."); idx >= 0 {
		shortName = shortName[idx+1:]
	}
	entry := buildingDimsLoaded[shortName]
	if entry == nil {
		entry = buildingDimsLoaded[typePath]
	}
	if entry == nil || len(entry.Clearance) == 0 {
		return 0, 0, 0, 0, 0, false
	}

	fqx, fqy, fqz, fqw := float64(qx), float64(qy), float64(qz), float64(qw)

	// Heading: the map-plane direction of the box's longest local axis, mapped
	// through the box's own rotation and then the instance's. Using the longest
	// axis rather than always X keeps the heading meaningful for boxes whose
	// rotation permutes axes. If that axis points straight up the object is
	// stood on end, so try the remaining axes longest-first before giving up.
	ext := [3]float64{}
	for _, box := range entry.Clearance {
		ext[0] = math.Max(ext[0], box.Max.X-box.Min.X)
		ext[1] = math.Max(ext[1], box.Max.Y-box.Min.Y)
		ext[2] = math.Max(ext[2], box.Max.Z-box.Min.Z)
	}
	order := [3]int{0, 1, 2}
	sort.SliceStable(order[:], func(a, b int) bool { return ext[order[a]] > ext[order[b]] })

	axX, axY := 1.0, 0.0
	for _, axis := range order {
		var vx, vy, vz float64
		switch axis {
		case 0:
			vx = 1
		case 1:
			vy = 1
		default:
			vz = 1
		}
		if box := entry.Clearance[0]; box.Rotation != nil {
			vx, vy, vz = rotate3DByQuaternion(vx, vy, vz,
				box.Rotation.X, box.Rotation.Y, box.Rotation.Z, box.Rotation.W)
		}
		px, py, _ := rotate3DByQuaternion(vx, vy, vz, fqx, fqy, fqz, fqw)
		if math.Hypot(px, py) >= 0.01 {
			axX, axY = px, py
			break
		}
	}
	yawRad := math.Atan2(axY, axX)
	cosY, sinY := math.Cos(yawRad), math.Sin(yawRad)

	// Extents of the projected corners measured along the heading (u) and
	// across it (v). A clearance box may carry its own RelativeTransform
	// rotation, which is part of the box's definition and must be applied
	// before the instance's rotation.
	minU, maxU := math.Inf(1), math.Inf(-1)
	minV, maxV := math.Inf(1), math.Inf(-1)
	for _, box := range entry.Clearance {
		for _, cx := range []float64{box.Min.X, box.Max.X} {
			for _, cy := range []float64{box.Min.Y, box.Max.Y} {
				for _, cz := range []float64{box.Min.Z, box.Max.Z} {
					bx, by, bz := cx, cy, cz
					if box.Rotation != nil {
						bx, by, bz = rotate3DByQuaternion(bx, by, bz,
							box.Rotation.X, box.Rotation.Y, box.Rotation.Z, box.Rotation.W)
					}
					rx, ry, _ := rotate3DByQuaternion(bx, by, bz, fqx, fqy, fqz, fqw)
					u := rx*cosY + ry*sinY
					v := -rx*sinY + ry*cosY
					minU = math.Min(minU, u)
					maxU = math.Max(maxU, u)
					minV = math.Min(minV, v)
					maxV = math.Max(maxV, v)
				}
			}
		}
	}

	width := maxU - minU
	depth := maxV - minV
	if width <= 0 || depth <= 0 {
		return 0, 0, 0, 0, 0, false
	}

	// Center offset: the OBB may not be symmetric around the actor position
	// because projecting a tilted 3D box onto the map plane shifts the centroid.
	// Compute the center of the projected extents in the (u,v) frame, then
	// rotate back to world XY to get the offset in cm.
	centerU := (minU + maxU) / 2
	centerV := (minV + maxV) / 2
	offX := centerU*cosY - centerV*sinY
	offY := centerU*sinY + centerV*cosY

	yawDeg := yawRad * 180.0 / math.Pi
	if yawDeg < 0 {
		yawDeg += 360
	}
	return width / 2 / 100, depth / 2 / 100, int16(math.Round(yawDeg * 10)), offX, offY, true // cm -> m for hw/hd; offset in cm
}

// hardcodedFootprint returns known footprints for buildings missing from Docs.json.
//
// Docs.json carries no clearance or Width/Depth for a number of point-like
// supports, poles and wall mounts, so they would otherwise fall through to the
// viewer's generic fallback and all render at the same arbitrary size. The
// values below are taken from each class's documented sibling, which shares its
// mesh footprint:
//
//	Build_HyperPoleStackable_C       x±50    y±120  -> 0.50 × 1.20
//	Build_PipeSupportStackable_C     x±50    y±120  -> 0.50 × 1.20
//	Build_ConveyorPoleStackable_C    x±50    y±100  -> 0.50 × 1.00
//	Build_PipelineSupportWallHole_C  x±46.8  y±100  -> 0.47 × 1.00
func hardcodedFootprint(shortName string) (float64, float64, bool) {
	switch shortName {
	case "Build_Elevator_C":
		return 4.0, 4.0, true // 8×8m
	case "Build_FloodlightWall_C":
		return 0.3, 0.15, true // 0.6×0.3m

	// Free-standing supports/poles — sibling: *Stackable variants
	case "Build_PipeHyperSupport_C", "Build_PipelineSupport_C":
		return 0.5, 1.2, true
	case "Build_ConveyorPole_C":
		return 0.5, 1.0, true

	// Wall-mounted supports and pass-through holes — sibling:
	// Build_PipelineSupportWallHole_C
	case "Build_HyperTubeWallSupport_C", "Build_HyperTubeWallHole_C",
		"Build_PipelineSupportWall_C", "Build_ConveyorPoleWall_C":
		return 0.47, 1.0, true
	}
	return 0, 0, false
}
