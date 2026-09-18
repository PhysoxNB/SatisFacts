package extraction

import (
	"strings"

	"satisfacts/parser"
)

// extractBeltConnection records input/output belt connections for a building.
func extractBeltConnection(obj *parser.SaveObject, instanceName string, connections map[string]BuildingConnection) {
	parts := strings.Split(instanceName, ".")
	if len(parts) < 3 {
		return
	}
	buildingInstance := parts[0] + "." + parts[1]
	connName := parts[2]
	connLower := strings.ToLower(connName)

	isInput := strings.Contains(connLower, "input")
	isOutput := strings.Contains(connLower, "output")

	// Only care about input or output connections.
	if !isInput && !isOutput {
		return
	}

	// Make an entry even if nothing's connected yet.
	conn, ok := connections[buildingInstance]
	if !ok {
		conn = BuildingConnection{Inputs: []string{}, Outputs: []string{}}
	}

	connectedComponent, _ := GetPropObjectRefPathName(obj, "mConnectedComponent")
	if connectedComponent != "" {
		connType := classifyConnection(connectedComponent)
		if connType != "" {
			if isInput {
				conn.Inputs = append(conn.Inputs, connType)
			} else {
				conn.Outputs = append(conn.Outputs, connType)
			}
		}
	}

	connections[buildingInstance] = conn
}

// extractPipeConnection records pipe connections for a building.
func extractPipeConnection(obj *parser.SaveObject, instanceName string, connections map[string]BuildingConnection) {
	parts := strings.Split(instanceName, ".")
	if len(parts) < 3 {
		return
	}
	buildingInstance := parts[0] + "." + parts[1]
	connName := parts[2]
	connLower := strings.ToLower(connName)

	isInput := strings.Contains(connLower, "input")
	isOutput := strings.Contains(connLower, "output")

	// Always create an entry, even with nothing connected.
	conn, ok := connections[buildingInstance]
	if !ok {
		conn = BuildingConnection{Inputs: []string{}, Outputs: []string{}}
	}

	connectedComponent, _ := GetPropObjectRefPathName(obj, "mConnectedComponent")
	if connectedComponent != "" {
		connType := classifyConnection(connectedComponent)
		if connType != "" {
			if isInput {
				conn.Inputs = append(conn.Inputs, connType)
			} else if isOutput {
				conn.Outputs = append(conn.Outputs, connType)
			} else {
				// No direction in the name, so fall back to the building type.
				if isPipeInputBuilding(buildingInstance) {
					conn.Inputs = append(conn.Inputs, connType)
				} else {
					conn.Outputs = append(conn.Outputs, connType)
				}
			}
		}
	}

	connections[buildingInstance] = conn
}

// isPipeInputBuilding reports whether a building's pipe connection is an input.
// Used when the connection name doesn't include "input" or "output".
func isPipeInputBuilding(buildingInstance string) bool {
	lower := strings.ToLower(buildingInstance)
	pipeInputBuildings := []string{
		"hadroncollider",   // Particle Accelerator - nitrogen gas input
		"generatorfuel",    // Fuel generator - fuel input
		"generatornuclear", // Nuclear power plant - water input
		"converter",        // Converter - fluid input
		"blender",          // Blender - fluid input
	}
	for _, b := range pipeInputBuildings {
		if strings.Contains(lower, b) {
			return true
		}
	}
	return false
}

func classifyConnection(refPath string) string {
	lower := strings.ToLower(refPath)
	if strings.Contains(lower, "conveyorbelt") {
		if mk := extractMkNumber(refPath, "conveyorbelt"); mk != "" {
			return "Belt Mk" + mk
		}
		return "Belt"
	}
	if strings.Contains(lower, "conveyorlift") {
		if mk := extractMkNumber(refPath, "conveyorlift"); mk != "" {
			return "Lift Mk" + mk
		}
		return "Lift"
	}
	if strings.Contains(lower, "pipeline") {
		if mk := extractMkNumber(refPath, "pipeline"); mk != "" {
			return "Pipe Mk" + mk
		}
		return "Pipe Mk1"
	}
	return ""
}

func extractMkNumber(path, typeKeyword string) string {
	idx := strings.Index(strings.ToLower(path), typeKeyword)
	if idx < 0 {
		return ""
	}
	rest := path[idx+len(typeKeyword):]
	// Find Mk followed by digits
	mkIdx := strings.Index(strings.ToLower(rest), "mk")
	if mkIdx < 0 {
		return ""
	}
	rest = rest[mkIdx+2:]
	var digits string
	for _, r := range rest {
		if r >= '0' && r <= '9' {
			digits += string(r)
		} else {
			break
		}
	}
	return digits
}
