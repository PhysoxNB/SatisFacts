package extraction

import "strings"

// Anything that can actually put power into a circuit. If a circuit has none
// of these on it, nothing attached to it is running.
var circuitPowerSourceMarkers = []string{
	"generator",          // every generator type
	"alienpowerbuilding", // APA
	"powerstorage",       // can discharge back into the circuit
}

// ResolvePowerCircuits tags every building with the circuit it sits on and
// whether that circuit has anything generating.
//
// The reason this exists: train stations and freight platforms get their power
// down the third rail, so checking WireCount on them tells you nothing (it's
// always 0). The game has already solved the network though - connected track
// ends up sharing a circuit - so we can just look at which circuit a station
// landed in and whether there's a generator on it.
//
// Needs to run after streaming and before the power pass.
func (e *ExtractedData) ResolvePowerCircuits() {
	if len(e.PowerGrid.Circuits) == 0 || len(e.BuildingFacts) == 0 {
		return
	}

	for _, c := range e.PowerGrid.Circuits {
		if len(c.Components) == 0 {
			continue
		}

		hasSource := false
		for _, comp := range c.Components {
			low := strings.ToLower(comp)
			for _, marker := range circuitPowerSourceMarkers {
				if strings.Contains(low, marker) {
					hasSource = true
					break
				}
			}
			if hasSource {
				break
			}
		}

		for _, comp := range c.Components {
			// Entries look like "<building>.PowerConnection", so drop the last
			// segment to get back to the building. Things that aren't owned by a
			// building (ThirdRail4 and friends) just won't match anything.
			owner := stripLastComponent(comp)
			bf, ok := e.BuildingFacts[owner]
			if !ok || bf == nil {
				continue
			}
			bf.CircuitID = c.ID
			bf.HasCircuit = true
			// A building can have more than one connection. If any of them is on
			// a live circuit, it's powered.
			if hasSource {
				bf.CircuitHasGenerator = true
			}
		}
	}
}
