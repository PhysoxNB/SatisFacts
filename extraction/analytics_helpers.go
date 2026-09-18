package extraction

import (
	"math"
	"regexp"
	"strings"
)

// classNameFromPath extracts the class name from a type path.
// e.g. "/MinerMk5/Build_MinerMk5.Build_MinerMk5_C" -> "Build_MinerMk5_C"
func classNameFromPath(typePath string) string {
	name := typePath
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	if idx := strings.Index(name, "."); idx >= 0 {
		name = name[:idx]
	}
	return name
}

// pumpMkRe detects modded pumps with Mk numbers (e.g. WaterPumpMk4, OilPumpMk3).
// Vanilla pumps don't have Mk in their name.
var pumpMkRe = regexp.MustCompile(`(?i)(WaterPump|OilPump)Mk\d`)

// triangleWaveFactor computes the triangle wave multiplier for a given
// effective time t (in seconds, period = 60s).
// f(t) = 2*|t/60 - floor(t/60 + 0.5)| + 0.5, ranging from 0.5 to 1.5.
func triangleWaveFactor(t float64) float64 {
	return 2*math.Abs(t/60-math.Floor(t/60+0.5)) + 0.5
}

// geothermalTargets are the known average MW values for each purity.
var geothermalTargets = []float64{100, 200, 400}

// computeGeothermalPhaseShift finds the phase shift (0–60s) that makes the
// inferred average MW values cluster closest to the known targets (100/200/400).
// The mVariablePowerProductionCycleOffset is the generator's build-time phase;
// the effective cycle position at save time is (offset + shift) mod 60.
// We brute-force search 0–60 in 0.01s steps to find the shift that minimizes
// the sum of squared distances from each avgMW to its nearest target.
func computeGeothermalPhaseShift(gens []struct{ mw, offset float64 }) float64 {
	if len(gens) == 0 {
		return 0
	}
	bestShift := 0.0
	bestErr := math.MaxFloat64
	for i := 0; i < 6000; i++ {
		s := float64(i) / 100.0
		totalErr := 0.0
		for _, g := range gens {
			et := math.Mod(g.offset+s, 60)
			tri := triangleWaveFactor(et)
			if tri < 0.01 {
				tri = 0.01
			}
			avg := g.mw / tri
			minDist := math.Abs(avg - geothermalTargets[0])
			for _, t := range geothermalTargets[1:] {
				if d := math.Abs(avg - t); d < minDist {
					minDist = d
				}
			}
			totalErr += minDist * minDist
		}
		if totalErr < bestErr {
			bestErr = totalErr
			bestShift = s
		}
	}
	return bestShift
}

// classifyGeothermalPurity determines the purity of a geothermal generator
// given its actual MW, cycle offset, and the computed phase shift.
func classifyGeothermalPurity(actualMW float64, cycleOffset float32, phaseShift float64) string {
	if actualMW <= 0 {
		return "normal" // default for standby/disabled
	}
	et := math.Mod(float64(cycleOffset)+phaseShift, 60)
	tri := triangleWaveFactor(et)
	if tri < 0.01 {
		tri = 0.01
	}
	avgMW := actualMW / tri
	switch {
	case avgMW < 150:
		return "impure"
	case avgMW < 300:
		return "normal"
	default:
		return "pure"
	}
}

// beltThroughput returns items/min for a conveyor belt type path.
func beltThroughput(typePath string) float64 {
	lower := strings.ToLower(typePath)
	switch {
	case strings.Contains(lower, "mk1"):
		return 60
	case strings.Contains(lower, "mk2"):
		return 120
	case strings.Contains(lower, "mk3"):
		return 270
	case strings.Contains(lower, "mk4"):
		return 480
	case strings.Contains(lower, "mk5"):
		return 780
	case strings.Contains(lower, "mk6"):
		return 1200
	default:
		return 60 // default to Mk1
	}
}

// outputThroughput computes the total throughput of connected belts/lifts or pipes.
// The outputs slice already contains classified labels like "Belt Mk3", "Lift Mk2", "Pipe Mk1".
// Returns (throughput, label).
func outputThroughput(outputs []string, isFluid bool) (float64, string) {
	if len(outputs) == 0 {
		return 0, "None"
	}
	total := 0.0
	labels := []string{}
	for _, label := range outputs {
		lower := strings.ToLower(label)
		if isFluid {
			if strings.Contains(lower, "pipe") {
				if strings.Contains(lower, "mk2") {
					total += 600
				} else {
					total += 300
				}
				labels = append(labels, label)
			}
		} else {
			if strings.Contains(lower, "belt") || strings.Contains(lower, "lift") {
				total += beltThroughput(label)
				labels = append(labels, label)
			}
		}
	}
	if len(labels) == 0 {
		return 0, "None"
	}
	return total, strings.Join(labels, ", ")
}

func isProductionBuilding(typeLower string) bool {
	return strings.Contains(typeLower, "miner") || strings.Contains(typeLower, "smelter") ||
		strings.Contains(typeLower, "constructor") || strings.Contains(typeLower, "assembler") ||
		strings.Contains(typeLower, "manufacturer") || strings.Contains(typeLower, "refinery") ||
		strings.Contains(typeLower, "blender") || strings.Contains(typeLower, "foundry") ||
		strings.Contains(typeLower, "packager") || strings.Contains(typeLower, "hadron") ||
		strings.Contains(typeLower, "quantum") || strings.Contains(typeLower, "converter") ||
		strings.Contains(typeLower, "waterpump") || strings.Contains(typeLower, "waterextractor") ||
		strings.Contains(typeLower, "oilextractor") || strings.Contains(typeLower, "oilpump") ||
		strings.Contains(typeLower, "fracking") || strings.Contains(typeLower, "resourcewell") ||
		strings.Contains(typeLower, "particleaccelerator")
}

func getBuildingName(v interface{}) string {
	m, ok := v.(map[string]interface{})
	if !ok {
		return ""
	}
	typePath, _ := m["type"].(string)
	parts := strings.Split(typePath, "/")
	last := parts[len(parts)-1]
	className := strings.Split(last, ".")[0]
	className = strings.Replace(className, "Build_", "", 1)
	className = strings.TrimSuffix(className, "_C")
	return className
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case map[string]interface{}:
		// Geothermal map like {impure: 100, normal: 200, pure: 400}; return normal.
		if n, ok := val["normal"].(float64); ok {
			return n
		}
		if n, ok := val["normal"].(int); ok {
			return float64(n)
		}
	}
	return 0
}
