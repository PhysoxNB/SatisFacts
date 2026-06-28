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

func splineLengthFromProperty(p parser.Property) float64 {
	// mSplineData is an ArrayProperty with value: map[string]interface{}{"type":"StructProperty", "items": [...]}
	var arr []interface{}
	if v, ok := p.Value.([]interface{}); ok {
		arr = v
	} else if m, ok := p.Value.(map[string]interface{}); ok {
		if items, ok := m["items"].([]interface{}); ok {
			arr = items
		}
	}
	if len(arr) < 2 {
		return 0
	}
	var totalLength float64
	for i := 0; i < len(arr)-1; i++ {
		p1, ok1 := arr[i].(map[string]interface{})
		p2, ok2 := arr[i+1].(map[string]interface{})
		if !ok1 || !ok2 {
			continue
		}
		loc1, ok1 := extractVec3(p1["Location"])
		loc2, ok2 := extractVec3(p2["Location"])
		if !ok1 || !ok2 {
			continue
		}
		dx := loc2[0] - loc1[0]
		dy := loc2[1] - loc1[1]
		dz := loc2[2] - loc1[2]
		totalLength += math.Sqrt(dx*dx + dy*dy + dz*dz)
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

// CleanTypeName extracts a short display name from a type path.
func CleanTypeName(typePath string) string {
	shortName := typePath
	if idx := strings.LastIndex(shortName, "/"); idx >= 0 {
		shortName = shortName[idx+1:]
	}
	shortName = strings.TrimSuffix(shortName, "_C")
	if idx := strings.LastIndex(shortName, "."); idx >= 0 {
		suffix := shortName[idx+1:]
		prefix := shortName[:idx]
		if strings.HasPrefix(suffix, prefix) {
			shortName = suffix
		} else {
			shortName = suffix
		}
	}
	if strings.HasPrefix(shortName, "Build_") {
		shortName = shortName[6:]
	}
	return shortName
}

// FormatDisplayName converts a type path to a human-readable name.
func FormatDisplayName(typePath string) string {
	shortName := CleanTypeName(typePath)
	if strings.HasPrefix(shortName, "BP_") {
		shortName = shortName[3:]
	}
	if strings.HasPrefix(shortName, "FG") {
		shortName = shortName[2:]
	}
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
