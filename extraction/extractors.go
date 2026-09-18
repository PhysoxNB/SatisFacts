package extraction

import (
	"math"
	"strings"

	"satisfacts/parser"
)

// ExtractPosition returns [x, y, z] from an object's transform, or nil.
func ExtractPosition(obj *parser.SaveObject) []float32 {
	if obj.Header != nil && obj.Header.NeedTransform {
		t := obj.Header.Transform
		return []float32{t.Translation[0], t.Translation[1], t.Translation[2]}
	}
	return nil
}

// ExtractMapLocation returns {x, y, altitude} in meters from centimeter save coords.
// Save: pos[0]=X(E/W), pos[1]=Y(N/S), pos[2]=Z(altitude). Map: X=E/W, Y=N/S.
func ExtractMapLocation(obj *parser.SaveObject) (x, y, altitude int, ok bool) {
	pos := ExtractPosition(obj)
	if pos == nil || len(pos) != 3 {
		return 0, 0, 0, false
	}
	return int(math.Round(float64(pos[0]) / 100)),
		int(math.Round(float64(pos[1]) / 100)),
		int(math.Round(float64(pos[2]) / 100)),
		true
}

// ExtractRotation returns pitch, yaw, roll in degrees from an object's transform.
func ExtractRotation(obj *parser.SaveObject) (pitch, yaw, roll float32, ok bool) {
	if obj.Header != nil && obj.Header.NeedTransform {
		r := obj.Header.Transform.Rotation
		return r[0], r[1], r[2], true
	}
	return 0, 0, 0, false
}

// ExtractLength extracts length in meters from various property sources.
func ExtractLength(obj *parser.SaveObject) float64 {
	props := obj.Properties
	if props == nil {
		return 0
	}

	// Elevators: mHeight, a float32 in cm.
	if p, ok := props["mHeight"]; ok {
		if v, ok := p.Value.(float32); ok {
			return float64(v) / 100
		}
	}

	// mCachedLength, also a float32 in cm.
	if p, ok := props["mCachedLength"]; ok {
		if v, ok := p.Value.(float32); ok {
			return float64(v) / 100
		}
	}

	// mSplineData: an array of structs, each with a Location.
	if p, ok := props["mSplineData"]; ok {
		return splineLengthFromProperty(p)
	}

	// Last resort, mLength.
	if p, ok := props["mLength"]; ok {
		if v, ok := p.Value.(float32); ok {
			return float64(v) / 100
		}
	}

	return 0
}

// ExtractSplinePoints returns spline points as [x,y,z] slices from mSplineData.
func ExtractSplinePoints(obj *parser.SaveObject) [][]float32 {
	props := obj.Properties
	if props == nil {
		return nil
	}
	p, ok := props["mSplineData"]
	if !ok {
		return nil
	}
	arr, ok := p.Value.([]interface{})
	if !ok {
		return nil
	}
	var points [][]float32
	for _, item := range arr {
		structProps, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		locRaw, ok := structProps["Location"]
		if !ok {
			continue
		}
		// StructProperty is wrapped as {"type": "Vector", "value": {"x":..., "y":..., "z":...}}
		locWrapper, ok := locRaw.(map[string]interface{})
		if !ok {
			continue
		}
		locValue, ok := locWrapper["value"].(map[string]float64)
		if !ok {
			continue
		}
		points = append(points, []float32{float32(locValue["x"]), float32(locValue["y"]), float32(locValue["z"])})
	}
	return points
}

// CalculateSplineLength computes total length in meters from an mSplineData property.
func CalculateSplineLength(splineProp parser.Property) float64 {
	return splineLengthFromProperty(splineProp)
}

// splineItems unwraps mSplineData into its list of points. The parser hands
// this back in two different shapes depending on the property, so both are
// handled here.
func splineItems(p parser.Property) []interface{} {
	if v, ok := p.Value.([]interface{}); ok {
		return v
	}
	if m, ok := p.Value.(map[string]interface{}); ok {
		if items, ok := m["items"].([]interface{}); ok {
			return items
		}
	}
	return nil
}

// rotateByQuat rotates v by the quaternion q (x, y, z, w) using
// v' = v + 2 * cross(q.xyz, cross(q.xyz, v) + q.w*v).
func rotateByQuat(v [3]float64, q [4]float32) [3]float64 {
	qx, qy, qz, qw := float64(q[0]), float64(q[1]), float64(q[2]), float64(q[3])

	// t = cross(q.xyz, v) + q.w * v
	tx := qy*v[2] - qz*v[1] + qw*v[0]
	ty := qz*v[0] - qx*v[2] + qw*v[1]
	tz := qx*v[1] - qy*v[0] + qw*v[2]

	// v' = v + 2 * cross(q.xyz, t)
	return [3]float64{
		v[0] + 2*(qy*tz-qz*ty),
		v[1] + 2*(qz*tx-qx*tz),
		v[2] + 2*(qx*ty-qy*tx),
	}
}

// SplineEndpointsWorld returns the world positions, in meters, of the first and
// last point of a spline.
//
// Spline points are stored in the actor's local space, so they have to be
// rotated by the actor's rotation and offset by its translation before they
// mean anything on the map. Without that, both ends of a track piece look like
// they're at the actor origin, which is exactly the tie that made station
// snapping ambiguous.
func SplineEndpointsWorld(splineProp parser.Property, tf parser.Transform3f, hasTransform bool) (first, last [3]float64, ok bool) {
	arr := splineItems(splineProp)
	if len(arr) < 2 {
		return first, last, false
	}
	firstMap, ok1 := arr[0].(map[string]interface{})
	lastMap, ok2 := arr[len(arr)-1].(map[string]interface{})
	if !ok1 || !ok2 {
		return first, last, false
	}
	locFirst, ok1 := extractVec3(firstMap["Location"])
	locLast, ok2 := extractVec3(lastMap["Location"])
	if !ok1 || !ok2 {
		return first, last, false
	}

	toWorld := func(local [3]float64) [3]float64 {
		if !hasTransform {
			return [3]float64{local[0] / 100, local[1] / 100, local[2] / 100}
		}
		r := rotateByQuat(local, tf.Rotation)
		return [3]float64{
			(r[0] + float64(tf.Translation[0])) / 100,
			(r[1] + float64(tf.Translation[1])) / 100,
			(r[2] + float64(tf.Translation[2])) / 100,
		}
	}
	return toWorld(locFirst), toWorld(locLast), true
}

// SplineCP is a single spline control point in world space (XY only).
// Tangents are direction vectors in world meters, already rotated by the
// actor's quaternion but without translation. The frontend converts these
// to Bezier control points: CP1 = Pos + LeaveTan/3, CP2 = NextPos - NextArriveTan/3.
type SplineCP struct {
	Pos       [2]float64
	LeaveTan  [2]float64
	ArriveTan [2]float64
}

// SplineControlPointsWorld extracts raw spline control points with tangents
// in world space (XY only), plus the average Z. The frontend performs Bezier
// subdivision using the tangents
func SplineControlPointsWorld(splineProp parser.Property, tf parser.Transform3f, hasTransform bool) ([]SplineCP, float64) {
	arr := splineItems(splineProp)
	if len(arr) < 2 {
		return nil, 0
	}

	toWorldPos := func(local [3]float64) [2]float64 {
		if !hasTransform {
			return [2]float64{local[0] / 100, local[1] / 100}
		}
		r := rotateByQuat(local, tf.Rotation)
		return [2]float64{
			(r[0] + float64(tf.Translation[0])) / 100,
			(r[1] + float64(tf.Translation[1])) / 100,
		}
	}

	toWorldTan := func(local [3]float64) [2]float64 {
		if !hasTransform {
			return [2]float64{local[0] / 100, local[1] / 100}
		}
		r := rotateByQuat(local, tf.Rotation)
		return [2]float64{r[0] / 100, r[1] / 100}
	}

	toWorldZ := func(local [3]float64) float64 {
		if !hasTransform {
			return local[2] / 100
		}
		r := rotateByQuat(local, tf.Rotation)
		return (r[2] + float64(tf.Translation[2])) / 100
	}

	type splinePoint struct {
		loc       [3]float64
		leaveTan  [3]float64
		arriveTan [3]float64
	}

	var pts []splinePoint
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		loc, ok := extractVec3(m["Location"])
		if !ok {
			continue
		}
		leaveTan, _ := extractVec3(m["LeaveTangent"])
		arriveTan, _ := extractVec3(m["ArriveTangent"])
		pts = append(pts, splinePoint{loc: loc, leaveTan: leaveTan, arriveTan: arriveTan})
	}

	if len(pts) < 2 {
		return nil, 0
	}

	cps := make([]SplineCP, len(pts))
	var zSum float64
	for i, p := range pts {
		cps[i] = SplineCP{
			Pos:       toWorldPos(p.loc),
			LeaveTan:  toWorldTan(p.leaveTan),
			ArriveTan: toWorldTan(p.arriveTan),
		}
		zSum += toWorldZ(p.loc)
	}

	return cps, zSum / float64(len(pts))
}

// SplinePointsWorld subdivides Hermite spline segments into polyline points.
// Deprecated: prefer SplineControlPointsWorld for accurate curve rendering.
func SplinePointsWorld(splineProp parser.Property, tf parser.Transform3f, hasTransform bool) [][2]float64 {
	arr := splineItems(splineProp)
	if len(arr) < 2 {
		return nil
	}

	toWorld := func(local [3]float64) [2]float64 {
		if !hasTransform {
			return [2]float64{local[0] / 100, local[1] / 100}
		}
		r := rotateByQuat(local, tf.Rotation)
		return [2]float64{
			(r[0] + float64(tf.Translation[0])) / 100,
			(r[1] + float64(tf.Translation[1])) / 100,
		}
	}

	// Extract all spline points with their tangents
	type splinePoint struct {
		loc       [3]float64
		leaveTan  [3]float64
		arriveTan [3]float64
	}

	var pts []splinePoint
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		loc, ok := extractVec3(m["Location"])
		if !ok {
			continue
		}
		leaveTan, _ := extractVec3(m["LeaveTangent"])
		arriveTan, _ := extractVec3(m["ArriveTangent"])
		pts = append(pts, splinePoint{loc: loc, leaveTan: leaveTan, arriveTan: arriveTan})
	}

	if len(pts) < 2 {
		return nil
	}

	// Subdivide each Hermite segment adaptively.
	// Straight segments (tangents parallel to segment direction) get just 2 points.
	// Curved segments get subdivisions proportional to their length.
	var points [][2]float64
	for i := 0; i < len(pts)-1; i++ {
		p0 := pts[i].loc
		t0 := pts[i].leaveTan
		p1 := pts[i+1].loc
		t1 := pts[i+1].arriveTan

		segLen := math.Sqrt(
			(p1[0]-p0[0])*(p1[0]-p0[0]) +
				(p1[1]-p0[1])*(p1[1]-p0[1]) +
				(p1[2]-p0[2])*(p1[2]-p0[2]))

		// Check if segment is straight: tangents parallel to (p1-p0).
		// For Hermite curves, the path is straight as long as tangents are parallel
		// to the segment direction — magnitude doesn't affect shape, only parameterization.
		dx, dy, dz := p1[0]-p0[0], p1[1]-p0[1], p1[2]-p0[2]
		straight := false
		if segLen > 0.001 {
			t0Len := math.Sqrt(t0[0]*t0[0] + t0[1]*t0[1] + t0[2]*t0[2])
			t1Len := math.Sqrt(t1[0]*t1[0] + t1[1]*t1[1] + t1[2]*t1[2])
			if t0Len > 0.001 && t1Len > 0.001 {
				dot0 := (t0[0]*dx + t0[1]*dy + t0[2]*dz) / (t0Len * segLen)
				dot1 := (t1[0]*dx + t1[1]*dy + t1[2]*dz) / (t1Len * segLen)
				if dot0 > 0.99 && dot1 > 0.99 {
					straight = true
				}
			}
		}

		if straight {
			// Just add start point; end point added by next segment or final point
			points = append(points, toWorld(p0))
			continue
		}

		// Adaptive subdivisions: 2 for short segments, up to 12 for long ones
		subdivs := 4
		if segLen > 200 {
			subdivs = 8
		}
		if segLen > 800 {
			subdivs = 12
		}

		// Hermite interpolation: H(t) = h00*P0 + h10*T0 + h01*P1 + h11*T1
		// h00 = 2t³-3t²+1, h10 = t³-2t²+t, h01 = -2t³+3t², h11 = t³-t²
		for s := 0; s < subdivs; s++ {
			t := float64(s) / float64(subdivs)
			t2 := t * t
			t3 := t2 * t
			h00 := 2*t3 - 3*t2 + 1
			h10 := t3 - 2*t2 + t
			h01 := -2*t3 + 3*t2
			h11 := t3 - t2

			p := [3]float64{
				h00*p0[0] + h10*t0[0] + h01*p1[0] + h11*t1[0],
				h00*p0[1] + h10*t0[1] + h01*p1[1] + h11*t1[1],
				h00*p0[2] + h10*t0[2] + h01*p1[2] + h11*t1[2],
			}
			points = append(points, toWorld(p))
		}
	}
	// Add the final point
	points = append(points, toWorld(pts[len(pts)-1].loc))
	return points
}

// SplineAvgZWorld returns the average Z (meters) of all spline points in world space.
func SplineAvgZWorld(splineProp parser.Property, tf parser.Transform3f, hasTransform bool) float64 {
	arr := splineItems(splineProp)
	if len(arr) == 0 {
		return 0
	}
	toWorldZ := func(local [3]float64) float64 {
		if !hasTransform {
			return local[2] / 100
		}
		r := rotateByQuat(local, tf.Rotation)
		return (r[2] + float64(tf.Translation[2])) / 100
	}
	var sum float64
	var n int
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		loc, ok := extractVec3(m["Location"])
		if !ok {
			continue
		}
		sum += toWorldZ(loc)
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

func splineLengthFromProperty(p parser.Property) float64 {
	arr := splineItems(p)
	if len(arr) < 2 {
		return 0
	}

	// Extract all spline points with their tangents.
	type splinePoint struct {
		loc       [3]float64
		leaveTan  [3]float64
		arriveTan [3]float64
	}
	var pts []splinePoint
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		loc, ok := extractVec3(m["Location"])
		if !ok {
			continue
		}
		leaveTan, _ := extractVec3(m["LeaveTangent"])
		arriveTan, _ := extractVec3(m["ArriveTangent"])
		pts = append(pts, splinePoint{loc: loc, leaveTan: leaveTan, arriveTan: arriveTan})
	}
	if len(pts) < 2 {
		return 0
	}

	var totalLength float64
	for i := 0; i < len(pts)-1; i++ {
		p0 := pts[i].loc
		t0 := pts[i].leaveTan
		p1 := pts[i+1].loc
		t1 := pts[i+1].arriveTan

		dx, dy, dz := p1[0]-p0[0], p1[1]-p0[1], p1[2]-p0[2]
		segLen := math.Sqrt(dx*dx + dy*dy + dz*dz)

		// Check if segment is straight: tangents parallel to (p1-p0).
		straight := false
		if segLen > 0.001 {
			t0Len := math.Sqrt(t0[0]*t0[0] + t0[1]*t0[1] + t0[2]*t0[2])
			t1Len := math.Sqrt(t1[0]*t1[0] + t1[1]*t1[1] + t1[2]*t1[2])
			if t0Len > 0.001 && t1Len > 0.001 {
				dot0 := (t0[0]*dx + t0[1]*dy + t0[2]*dz) / (t0Len * segLen)
				dot1 := (t1[0]*dx + t1[1]*dy + t1[2]*dz) / (t1Len * segLen)
				if dot0 > 0.99 && dot1 > 0.99 {
					straight = true
				}
			}
		}

		if straight {
			totalLength += segLen
			continue
		}

		// Hermite interpolation with enough subdivisions for accurate arc length.
		// H(t) = h00*P0 + h10*T0 + h01*P1 + h11*T1
		const subdivs = 32
		var prev [3]float64 = p0
		for s := 1; s <= subdivs; s++ {
			t := float64(s) / float64(subdivs)
			t2 := t * t
			t3 := t2 * t
			h00 := 2*t3 - 3*t2 + 1
			h10 := t3 - 2*t2 + t
			h01 := -2*t3 + 3*t2
			h11 := t3 - t2

			pt := [3]float64{
				h00*p0[0] + h10*t0[0] + h01*p1[0] + h11*t1[0],
				h00*p0[1] + h10*t0[1] + h01*p1[1] + h11*t1[1],
				h00*p0[2] + h10*t0[2] + h01*p1[2] + h11*t1[2],
			}
			px, py, pz := pt[0]-prev[0], pt[1]-prev[1], pt[2]-prev[2]
			totalLength += math.Sqrt(px*px + py*py + pz*pz)
			prev = pt
		}
	}
	return totalLength / 100
}

// extractVec3 extracts x,y,z from a StructProperty value (Vector).
// Handles both map[string]float64 and map[string]interface{} value types.
func extractVec3(val interface{}) ([3]float64, bool) {
	// StructProperty stores: map[string]interface{}{"type": "Vector", "value": map[string]float64{"x":..., "y":..., "z":...}}
	m, ok := val.(map[string]interface{})
	if !ok {
		return [3]float64{}, false
	}
	inner, ok := m["value"]
	if !ok {
		// Maybe it's directly the value map
		inner = val
	}
	// Try map[string]float64 first
	if coords, ok := inner.(map[string]float64); ok {
		return [3]float64{coords["x"], coords["y"], coords["z"]}, true
	}
	// Try map[string]interface{}
	if coords, ok := inner.(map[string]interface{}); ok {
		x, _ := coords["x"].(float64)
		y, _ := coords["y"].(float64)
		z, _ := coords["z"].(float64)
		return [3]float64{x, y, z}, true
	}
	return [3]float64{}, false
}

// GetPropFloat32 reads a float32 property, returning ok=false if it's missing
// or the wrong type. DoubleProperty (float64) values are downcast.
func GetPropFloat32(obj *parser.SaveObject, name string) (float32, bool) {
	if obj.Properties == nil {
		return 0, false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return 0, false
	}
	if v, ok := p.Value.(float32); ok {
		return v, true
	}
	if v, ok := p.Value.(float64); ok {
		return float32(v), true
	}
	return 0, false
}

// GetPropInt32 retrieves an int32 property value.
func GetPropInt32(obj *parser.SaveObject, name string) (int32, bool) {
	if obj.Properties == nil {
		return 0, false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return 0, false
	}
	v, ok := p.Value.(int32)
	return v, ok
}

// GetPropBool retrieves a bool property value.
func GetPropBool(obj *parser.SaveObject, name string) (bool, bool) {
	if obj.Properties == nil {
		return false, false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return false, false
	}
	v, ok := p.Value.(bool)
	return v, ok
}

// GetPropString retrieves a string property value.
func GetPropString(obj *parser.SaveObject, name string) (string, bool) {
	if obj.Properties == nil {
		return "", false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return "", false
	}
	v, ok := p.Value.(string)
	return v, ok
}

// GetPropObjectRef retrieves an object reference (levelName, pathName) from an ObjectProperty.
func GetPropObjectRef(obj *parser.SaveObject, name string) (levelName, pathName string, ok bool) {
	if obj.Properties == nil {
		return "", "", false
	}
	p, ok2 := obj.Properties[name]
	if !ok2 {
		return "", "", false
	}
	// Normal ObjectProperty: stored as map[string]string.
	if ref, ok3 := p.Value.(map[string]string); ok3 {
		return ref["levelName"], ref["pathName"], true
	}
	// Struct-wrapped reference: map[string]parser.Property.
	if ref, ok3 := p.Value.(map[string]parser.Property); ok3 {
		ln, _ := ref["levelName"].Value.(string)
		pn, _ := ref["pathName"].Value.(string)
		return ln, pn, true
	}
	return "", "", false
}

// GetPropObjectRefPathName retrieves just the pathName from an ObjectProperty.
func GetPropObjectRefPathName(obj *parser.SaveObject, name string) (string, bool) {
	_, pn, ok := GetPropObjectRef(obj, name)
	return pn, ok
}

// GetPropText reads a TextProperty and returns the culture-invariant string.
// Returns ("", true) if the property exists but has no text value (e.g. an unnamed station).
func GetPropText(obj *parser.SaveObject, name string) (string, bool) {
	if obj.Properties == nil {
		return "", false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return "", false
	}
	m, ok := p.Value.(map[string]interface{})
	if !ok {
		return "", false
	}
	if has, ok := m["hasCultureInvariantString"]; ok {
		if b, ok := has.(bool); ok && b {
			if v, ok := m["value"].(string); ok {
				return v, true
			}
		}
	}
	return "", true // property exists but has no text value
}

// GetPropEnum reads an EnumProperty and returns its value as a string.
func GetPropEnum(obj *parser.SaveObject, name string) (string, bool) {
	if obj.Properties == nil {
		return "", false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return "", false
	}
	v, ok := p.Value.(string)
	return v, ok
}

// GetPropArray reads an array property as []interface{}. ArrayProperty values
// come wrapped as map[string]interface{}{"type":..., "count":..., "items": [...]}.
func GetPropArray(obj *parser.SaveObject, name string) ([]interface{}, bool) {
	if obj.Properties == nil {
		return nil, false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return nil, false
	}
	// Direct []interface{} (rare)
	if arr, ok := p.Value.([]interface{}); ok {
		return arr, true
	}
	// Wrapped in map with "items" key
	if m, ok := p.Value.(map[string]interface{}); ok {
		if items, ok := m["items"].([]interface{}); ok {
			return items, true
		}
		// Check "values" alias
		if vals, ok := m["values"].([]interface{}); ok {
			return vals, true
		}
	}
	return nil, false
}

// GetPropMap reads a MapProperty and returns it as a map[string]interface{}.
// The parser wraps MapProperty values with "type" and "entries" keys.
func GetPropMap(obj *parser.SaveObject, name string) (map[string]interface{}, bool) {
	if obj.Properties == nil {
		return nil, false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return nil, false
	}
	if m, ok := p.Value.(map[string]interface{}); ok {
		return m, true
	}
	return nil, false
}

// GetPropStruct reads a StructProperty and returns its inner value as a map.
func GetPropStruct(obj *parser.SaveObject, name string) (interface{}, bool) {
	if obj.Properties == nil {
		return nil, false
	}
	p, ok := obj.Properties[name]
	if !ok {
		return nil, false
	}
	if m, ok := p.Value.(map[string]interface{}); ok {
		// StructProperty wraps as {"type": "...", "value": {...}}
		if inner, ok := m["value"].(map[string]interface{}); ok {
			return inner, true
		}
		return m, true
	}
	return nil, false
}

// CleanTypeName extracts a short display name from a type path.
func CleanTypeName(typePath string) string {
	shortName := typePath
	if idx := strings.LastIndex(shortName, "/"); idx >= 0 {
		shortName = shortName[idx+1:]
	}
	shortName = strings.TrimSuffix(shortName, "_C")
	if idx := strings.LastIndex(shortName, "."); idx >= 0 {
		shortName = shortName[idx+1:]
	}
	shortName = strings.TrimPrefix(shortName, "Build_")
	return shortName
}

// FormatDisplayName converts a type path to a human-readable name.
func FormatDisplayName(typePath string) string {
	shortName := CleanTypeName(typePath)
	shortName = strings.TrimPrefix(shortName, "BP_")
	shortName = strings.TrimPrefix(shortName, "FG")
	formatted := strings.ReplaceAll(shortName, "_", " ")
	// Split camelCase into words.
	formatted = insertSpaceBeforeCapital(formatted)
	// ...but don't split "Mk1" into "Mk 1".
	formatted = strings.ReplaceAll(formatted, "Mk ", "Mk")
	formatted = strings.TrimSpace(formatted)
	return formatted
}

func insertSpaceBeforeCapital(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' && s[i-1] >= 'a' && s[i-1] <= 'z' {
			sb.WriteByte(' ')
		}
		sb.WriteRune(r)
	}
	return sb.String()
}
