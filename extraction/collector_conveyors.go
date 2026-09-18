package extraction

import (
	"fmt"
	"strings"
)

func (e *ExtractedData) processConveyorChain(sp map[string]interface{}) {
	beltsInChain, ok := sp["beltsInChain"].([]interface{})
	if !ok {
		return
	}

	// Items in transit - use raw class path as key so the frontend can
	// translate via displayName() based on the selected language.
	if items, ok := sp["items"].([]interface{}); ok {
		for _, itemRaw := range items {
			item, ok := itemRaw.(map[string]interface{})
			if !ok {
				continue
			}
			if itemRef, ok := item["item"].(map[string]string); ok {
				itemPath := itemRef["pathName"]
				if itemPath != "" {
					e.ItemsInTransit[itemPath]++
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
