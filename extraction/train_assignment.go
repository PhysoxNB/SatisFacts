package extraction

import (
	"satisfacts/parser"
)

// vehicleInfo holds coupling links for a single locomotive or wagon.
// frontRef/backRef are the pathNames from mCoupledVehicleFront/Back -
// they point at the next vehicle in either direction, like a doubly-linked list.
type vehicleInfo struct {
	isLoco   bool
	frontRef string
	backRef  string
}

// resolveTrainVehicles walks the coupling chain (mCoupledVehicleFront/Back)
// from each train's FirstVehicle to count locomotives and wagons.
// The coupling data is parsed from binary special properties on AFGRailroadVehicle.
// Returns a map keyed by FirstVehicle ref -> trainVehicleResult.
type trainVehicleResult struct {
	counts    [2]int // [locoCount, wagonCount]
	wagonRefs []string
}

func (e *ExtractedData) resolveTrainVehicles() map[string]trainVehicleResult {
	vehicles := make(map[string]*vehicleInfo)

	for _, ref := range sortedKeys(e.rawLocomotives) {
		obj := e.rawLocomotives[ref]
		vi := &vehicleInfo{isLoco: true}
		extractCoupling(obj, vi)
		vehicles[ref] = vi
	}
	for _, ref := range sortedKeys(e.rawWagons) {
		obj := e.rawWagons[ref]
		vi := &vehicleInfo{isLoco: false}
		extractCoupling(obj, vi)
		vehicles[ref] = vi
	}

	result := make(map[string]trainVehicleResult)
	for _, trainKey := range sortedKeys(e.rawTrains) {
		train := e.rawTrains[trainKey]
		firstRef := refPathName(train, "FirstVehicle")
		if firstRef == "" {
			continue
		}

		locos, wagons := 0, 0
		var wagonRefs []string
		visited := map[string]bool{}

		// BFS from FirstVehicle, following both front and back coupling links.
		// Coupling direction is relative to vehicle orientation, not chain order,
		// so we must follow both directions.
		queue := []string{firstRef}
		for len(queue) > 0 {
			curRef := queue[0]
			queue = queue[1:]
			if curRef == "" || visited[curRef] {
				continue
			}
			visited[curRef] = true
			vi, ok := vehicles[curRef]
			if !ok {
				continue
			}
			if vi.isLoco {
				locos++
			} else {
				wagons++
				wagonRefs = append(wagonRefs, curRef)
			}
			queue = append(queue, vi.frontRef, vi.backRef)
		}

		result[firstRef] = trainVehicleResult{
			counts:    [2]int{locos, wagons},
			wagonRefs: wagonRefs,
		}
	}
	return result
}

// extractCoupling reads mCoupledVehicleFront/Back from the __special__ property
// that parseRailroadVehicleSpecial stashes on the object. If the property is
// missing (e.g. older save format), the refs stay empty and the vehicle is
// effectively isolated in the BFS.
func extractCoupling(obj *parser.SaveObject, vi *vehicleInfo) {
	special, ok := obj.Properties["__special__"]
	if !ok {
		return
	}
	sp, ok := special.Value.(map[string]interface{})
	if !ok {
		return
	}
	if front, ok := sp["mCoupledVehicleFront"].(map[string]string); ok {
		vi.frontRef = front["pathName"]
	}
	if back, ok := sp["mCoupledVehicleBack"].(map[string]string); ok {
		vi.backRef = back["pathName"]
	}
}
