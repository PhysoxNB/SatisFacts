package extraction

import (
	"strings"

	"satisfacts/parser"
)

// PostExtractionData is the second-pass output from retained objects.
type PostExtractionData struct {
	ClockSpeeds       map[string]float32
	ProductionBoosts  map[string]float32
	BuildingTypes     map[string]string
	BuildingRecipes   map[string]string
	BuildingInventories map[string]BuildingInventoryRefs
	InventoryContents map[string]int
	NuclearWaste      NuclearWasteData
	GeneratorPower    GeneratorPowerData
	BuildingConnections map[string]BuildingConnection
	PowerConsumption  PowerConsumptionData
	APABuildingCount  int
	APAFueledCount    int
	MinerPurities     map[string]string
	MinerResources    map[string]string
	GeothermalPurities map[string]string
	MinerResourceRefs map[string]string
	FrackingFluidTypes map[string]string
	FrackingPurities   map[string]string
	Collectibles       CollectiblesData
}

type CollectiblesData struct {
	Tapes        []string
	DropPods     int
	TotalPickups int
}

type BuildingInventoryRefs struct {
	InputRef  string
	OutputRef string
}

type InventoryStack struct {
	Item      string
	Count     float64
	StackSize int
	// raw item descriptor / unconverted count for nuclear waste, fuel, storage
	rawClass string
	numItems int
}

type ResolvedInventory struct {
	InputStacks  []InventoryStack
	OutputStacks []InventoryStack
}

type NuclearWasteData struct {
	TotalWaste       int
	TotalStoredWaste int
	WasteContainers  int
	Plants           []NuclearPlant
}

type NuclearPlant struct {
	Instance   string
	Waste      int
	Fuel       int
	Supplemental int
	FuelType   string
	ClockSpeed float32
}

type GeneratorPowerData struct {
	TotalCapacityMW     float64
	ActiveCapacityMW    float64
	StandbyCapacityMW   float64
	TheoreticalMaxMW    float64
	TotalGenerators     int
	ActiveGenerators    int
	StandbyGenerators   int
	Generators          []GeneratorEntry
}

type GeneratorEntry struct {
	InstanceName    string
	Type            string
	TypePath        string
	Status          string
	ActualMW        float64
	MaxMW           float64
	TheoreticalMaxMW float64
	Reason          string
	OutputPercent   int
	FuelType        string
	PowerShardCount int
	Location        *ConsumerLocation
}

type BuildingConnection struct {
	Inputs  []string
	Outputs []string
}

type PowerConsumptionData struct {
	TotalTheoreticalMW float64
	TotalActualMW      float64
	ActiveBuildings    int
	PausedBuildings    int
	Consumers          []PowerConsumerEntry
}

type PowerConsumerEntry struct {
	InstanceName string
	Type         string
	TypePath     string
	Status       string
	Reason       string
	ActualMW     float64
	MaxMW        float64
	IsBlocked    bool
	Location     *ConsumerLocation
	ProducedItem string
}

type ConsumerLocation struct {
	X string
	Y string
	Z string
}

// NewPostExtractionData returns a PostExtractionData with all its maps made.
func NewPostExtractionData() *PostExtractionData {
	return &PostExtractionData{
		ClockSpeeds:        make(map[string]float32),
		ProductionBoosts:   make(map[string]float32),
		BuildingTypes:      make(map[string]string),
		BuildingRecipes:    make(map[string]string),
		BuildingInventories: make(map[string]BuildingInventoryRefs),
		InventoryContents:  make(map[string]int),
		MinerPurities:      make(map[string]string),
		MinerResources:     make(map[string]string),
		GeothermalPurities: make(map[string]string),
		MinerResourceRefs:  make(map[string]string),
		FrackingFluidTypes: make(map[string]string),
		FrackingPurities:   make(map[string]string),
		BuildingConnections: make(map[string]BuildingConnection),
	}
}

// RunPostExtraction is the second pass over retained objects.
func RunPostExtraction(buildingFacts map[string]*BuildingFacts, retained []*parser.SaveObject, objectMap map[string]*parser.SaveObject, resourceNodeFacts map[string]*ResourceNodeFacts, inventoryPotentialMap map[string]float32, defaultClockSpeeds map[string]string, buildingConnections map[string]BuildingConnection, inventoryStacks map[string][]InventoryStack) *PostExtractionData {
	result := NewPostExtractionData()

	// Connections were already collected during streaming.
	result.BuildingConnections = buildingConnections

	// Clock speeds from BuildingFacts.
	result.ClockSpeeds, result.ProductionBoosts, result.BuildingTypes = ExtractClockSpeeds(buildingFacts)

	// Fold in the buildings we didn't retain, at a default 100%.
	for instanceName, typePath := range defaultClockSpeeds {
		if _, exists := result.ClockSpeeds[instanceName]; !exists {
			result.ClockSpeeds[instanceName] = 100
			result.BuildingTypes[instanceName] = typePath
		}
	}

	// Walk every BuildingFacts entry for recipes, inventories, nuclear, APA.
	var apaInstances []string

	for instanceName, bf := range buildingFacts {
		typePath := bf.TypePath
		typeLower := strings.ToLower(typePath)

		// APA buildings
		if strings.Contains(typeLower, "alienpowerbuilding") || strings.Contains(typeLower, "build_alienpowerbuilding") {
			result.APABuildingCount++
			apaInstances = append(apaInstances, instanceName)
		}

		// Recipe
		if bf.CurrentRecipe != "" {
			result.BuildingRecipes[instanceName] = bf.CurrentRecipe
		}

		// Building inventory references
		if bf.InputInventory != "" || bf.OutputInventory != "" {
			result.BuildingInventories[instanceName] = BuildingInventoryRefs{
				InputRef:  bf.InputInventory,
				OutputRef: bf.OutputInventory,
			}
		} else if bf.InventoryPotential != "" {
			result.BuildingInventories[instanceName] = BuildingInventoryRefs{
				OutputRef: bf.InventoryPotential,
			}
		}

		// Miner resource references
		if strings.Contains(typeLower, "miner") && !strings.Contains(typeLower, "portable") {
			if bf.ExtractableResource != "" {
				result.MinerResourceRefs[instanceName] = bf.ExtractableResource
			}
		}

		// Nuclear plants
		if strings.Contains(typeLower, "generatornuclear") || strings.Contains(typeLower, "build_generatornuclear") {
			extractNuclearPlant(bf, instanceName, &result.NuclearWaste, inventoryPotentialMap)
		}
	}

	// Central storage items — still from retained subsystem objects.
	for _, obj := range retained {
		if obj == nil || obj.Header == nil {
			continue
		}
		typeLower := strings.ToLower(obj.Header.ClassName)
		if strings.Contains(typeLower, "centralstoragesubsystem") {
			extractCentralStorageItems(obj, result.InventoryContents, &result.NuclearWaste)
		}
	}

	// Figure out which APA buildings are actually fueled.
	for _, apaInstance := range apaInstances {
		bf, ok := buildingFacts[apaInstance]
		if !ok {
			continue
		}
		isFueled := false

		// mHasFuelCached is the most reliable signal.
		if bf.HasFuelCached {
			isFueled = true
		}

		// Otherwise check mCurrentFuelClass for an alien power matrix.
		if !isFueled {
			fuelLower := strings.ToLower(bf.CurrentFuelClass)
			if strings.Contains(fuelLower, "alienpower") || strings.Contains(fuelLower, "matrix") {
				isFueled = true
			}
		}

		// Last resort: mCurrentPotential above 1.0 means it's running.
		if !isFueled && bf.HasCurrentPotential && bf.CurrentPotential > 1.0 {
			isFueled = true
		}

		if isFueled {
			result.APAFueledCount++
		}
	}

	// Resolve miner resources and purities from the resource node refs.
	for inst, refPath := range result.MinerResourceRefs {
		if rf, ok := resourceNodeFacts[refPath]; ok && rf != nil {
			// 1.2+ saves may carry an override on the node itself.
			if rf.ResourceClassOverride != "" {
				if m := descNameRegex.FindStringSubmatch(rf.ResourceClassOverride); m != nil {
					result.MinerResources[inst] = m[1]
				}
			}
			// Purity override, present from 1.0 on.
			if rf.PurityOverride != "" {
				purity := rf.PurityOverride
				if strings.HasPrefix(strings.ToUpper(purity), "RP_") {
					purity = purity[3:]
				}
				result.MinerPurities[inst] = strings.ToLower(purity)
			}
		}
		// Pre-1.0 saves: fall back to our resource_nodes.json table.
		if result.MinerResources[inst] == "" || result.MinerPurities[inst] == "" {
			if nodeData, ok2 := resourceNodesLoaded[refPath]; ok2 {
				if result.MinerResources[inst] == "" {
					result.MinerResources[inst] = nodeData.Type
				}
				if result.MinerPurities[inst] == "" {
					result.MinerPurities[inst] = nodeData.Purity
				}
			}
		}
	}

	// Nuclear waste from resolved inventory stacks
	for _, stacks := range inventoryStacks {
		accumulateNuclearWasteFromStacks(stacks, &result.NuclearWaste)
	}

	// Generator and power consumption data.
	result.GeneratorPower = ExtractGeneratorPower(result.BuildingTypes, buildingFacts, resourceNodeFacts, inventoryStacks)

	// Power consumption needs the resolved inventories to figure out status.
	resolvedInventories := BuildResolvedInventories(result.BuildingInventories, inventoryStacks)
	extractorProgress := BuildExtractorProgress(buildingFacts)
	result.PowerConsumption = ExtractPowerConsumption(
		buildingFacts, resourceNodeFacts,
		resolvedInventories,
		result.BuildingRecipes,
		result.ClockSpeeds,
		result.ProductionBoosts,
		extractorProgress,
	)

	// Collectibles and storage contents.
	result.Collectibles = ExtractCollectibles(retained)
	resolveStorageInventories(inventoryStacks, buildingFacts, result.InventoryContents)

	return result
}

// ExtractCollectibles pulls tapes, drop pods and pickup counts from subsystem objects.
func ExtractCollectibles(retained []*parser.SaveObject) CollectiblesData {
	data := CollectiblesData{}

	for _, obj := range retained {
		if obj == nil || obj.Header == nil {
			continue
		}
		typeLower := strings.ToLower(obj.Header.ClassName)

		// UnlockSubsystem holds mUnlockedTapes.
		if strings.Contains(typeLower, "unlocksubsystem") {
			tapes, ok := GetPropArray(obj, "mUnlockedTapes")
			if !ok {
				continue
			}
			for _, tape := range tapes {
				// Each tape is an ObjectProperty (levelName/pathName).
				if ref, ok := tape.(map[string]string); ok {
					pathName := ref["pathName"]
					if pathName == "" {
						continue
					}
					// e.g. "/Game/.../Tape_X.Tape_X_C" -> "Tape_X"
					parts := strings.Split(pathName, "/")
					tapeName := parts[len(parts)-1]
					tapeName = strings.TrimSuffix(tapeName, "_C")
					if idx := strings.Index(tapeName, "."); idx > 0 {
						tapeName = tapeName[:idx]
					}
					data.Tapes = append(data.Tapes, tapeName)
				}
			}
		}

		// ScannableSubsystem has mLootedDropPods and mDestroyedPickups.
		if strings.Contains(typeLower, "scannable") {
			if dropPods, ok := GetPropArray(obj, "mLootedDropPods"); ok {
				data.DropPods = len(dropPods)
			}
			if pickups, ok := GetPropArray(obj, "mDestroyedPickups"); ok {
				data.TotalPickups = len(pickups)
			}
		}
	}

	return data
}

func extractNuclearPlant(bf *BuildingFacts, instanceName string, waste *NuclearWasteData, inventoryPotentialMap map[string]float32) {
	wasteLeft := bf.WasteLeftFromCurrentFuel
	fuel := bf.CurrentFuelAmount
	supplemental := bf.CurrentSupplementalAmount
	clockSpeed := bf.CurrentPotential
	fuelClassRef := bf.CurrentFuelClass

	fuelType := "Unknown"
	if fuelClassRef != "" {
		if strings.Contains(fuelClassRef, "NuclearFuelRod") {
			fuelType = "Uranium"
		} else if strings.Contains(fuelClassRef, "PlutoniumFuelRod") {
			fuelType = "Plutonium"
		} else if strings.Contains(strings.ToLower(fuelClassRef), "ficsonium") {
			fuelType = "Ficsonium"
		}
	}

	if fuelClassRef == "" {
		return
	}

	// Ficsonium plants don't set mCurrentPotential; use InventoryPotential instead.
	if clockSpeed == 0 {
		if bf.InventoryPotential != "" {
			if v, exists := inventoryPotentialMap[bf.InventoryPotential]; exists && v > 0 {
				clockSpeed = v
			}
		}
	}
	// Still nothing? Default to 100%.
	if clockSpeed == 0 {
		clockSpeed = 1.0
	}

	plantWaste := int(wasteLeft)
	if fuelType == "Ficsonium" {
		plantWaste = 0
	}

	waste.Plants = append(waste.Plants, NuclearPlant{
		Instance:   instanceName,
		Waste:      plantWaste,
		Fuel:       int(fuel),
		Supplemental: int(supplemental),
		FuelType:   fuelType,
		ClockSpeed: clockSpeed * 100,
	})
	waste.TotalWaste += plantWaste
}

func accumulateNuclearWasteFromStacks(stacks []InventoryStack, waste *NuclearWasteData) {
	for _, st := range stacks {
		if st.rawClass == "" {
			continue
		}
		if strings.Contains(strings.ToLower(st.rawClass), "nuclearwaste") {
			if st.numItems > 0 {
				waste.TotalStoredWaste += st.numItems
				waste.WasteContainers++
			}
		}
	}
}

func extractCentralStorageItems(obj *parser.SaveObject, inventoryContents map[string]int, waste *NuclearWasteData) {
	storedItems, ok := GetPropArray(obj, "mStoredItems")
	if !ok {
		return
	}
	for _, item := range storedItems {
		structProps, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for nuclear waste
		itemPath := getItemClassPathFromEntry(structProps)
		if strings.Contains(strings.ToLower(itemPath), "nuclearwaste") {
			amount := getAmountFromEntry(structProps)
			if amount > 0 {
				waste.TotalStoredWaste += amount
				waste.WasteContainers++
			}
		}

		// Add to inventory contents
		itemName := cleanItemName(itemPath)
		if itemName == "" {
			continue
		}
		amount := getAmountFromEntry(structProps)
		if amount > 0 {
			inventoryContents[itemName] += amount
		}
	}
}

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
		"hadroncollider",  // Particle Accelerator — nitrogen gas input
		"generatorfuel",   // Fuel generator — fuel input
		"generatornuclear", // Nuclear power plant — water input
		"converter",       // Converter — fluid input
		"blender",         // Blender — fluid input
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

// resolveStorageInventories tallies items in storage containers from resolved inventory stacks.
func resolveStorageInventories(inventoryStacks map[string][]InventoryStack, buildingFacts map[string]*BuildingFacts, inventoryContents map[string]int) {
	for instanceName, stacks := range inventoryStacks {
		// Find the parent building.
		if !strings.Contains(instanceName, ".") {
			continue
		}
		parts := strings.Split(instanceName, ".")
		parentInstance := strings.Join(parts[:len(parts)-1], ".")
		parentBf, ok := buildingFacts[parentInstance]
		if !ok {
			continue
		}
		parentTypeLower := strings.ToLower(parentBf.TypePath)

		// Only count storage containers, not production buildings.
		isStorage := strings.Contains(parentTypeLower, "storagecontainer") ||
			strings.Contains(parentTypeLower, "industrialstorage") ||
			strings.Contains(parentTypeLower, "centralstorage") ||
			strings.Contains(parentTypeLower, "storageintegrated") ||
			strings.Contains(parentTypeLower, "crate") ||
			strings.Contains(parentTypeLower, "containerwall") ||
			strings.Contains(parentTypeLower, "playerinventory") ||
			strings.Contains(parentTypeLower, "storage")

		isProduction := strings.Contains(parentTypeLower, "conveyor") ||
			strings.Contains(parentTypeLower, "belt") ||
			strings.Contains(parentTypeLower, "splitter") ||
			strings.Contains(parentTypeLower, "merger") ||
			strings.Contains(parentTypeLower, "lift") ||
			strings.Contains(parentTypeLower, "pipe") ||
			strings.Contains(parentTypeLower, "pump") ||
			strings.Contains(parentTypeLower, "valve") ||
			strings.Contains(parentTypeLower, "manufacturer") ||
			strings.Contains(parentTypeLower, "constructor") ||
			strings.Contains(parentTypeLower, "smelter") ||
			strings.Contains(parentTypeLower, "refinery") ||
			strings.Contains(parentTypeLower, "generator") ||
			strings.Contains(parentTypeLower, "drone") ||
			strings.Contains(parentTypeLower, "train") ||
			strings.Contains(parentTypeLower, "vehicle")

		if !isStorage || isProduction {
			continue
		}

		for _, st := range stacks {
			if st.Item == "" {
				continue
			}
			if st.numItems > 0 {
				inventoryContents[st.Item] += st.numItems
			}
		}
	}
}

// getItemPathFromStack extracts the item descriptor path from an InventoryStack element.
func getItemPathFromStack(props map[string]interface{}) string {
	// Item -> StructProperty(InventoryItem) -> itemClass.pathName
	if itemRaw, ok := props["Item"]; ok {
		if itemWrapper, ok := itemRaw.(map[string]interface{}); ok {
			// StructProperty is wrapped as {"type": ..., "value": ...}.
			if itemValue, ok := itemWrapper["value"]; ok {
				if itemStruct, ok := itemValue.(map[string]interface{}); ok {
					if itemClass, ok := itemStruct["itemClass"]; ok {
						if ref, ok := itemClass.(map[string]string); ok {
							return ref["pathName"]
						}
					}
				}
			}
		}
	}
	// Fallback: ItemClass -> ObjectProperty -> pathName.
	if itemClassRaw, ok := props["ItemClass"]; ok {
		if ref, ok := itemClassRaw.(map[string]string); ok {
			return ref["pathName"]
		}
		// ...or wrapped in a StructProperty.
		if wrapper, ok := itemClassRaw.(map[string]interface{}); ok {
			if val, ok := wrapper["value"]; ok {
				if ref, ok := val.(map[string]string); ok {
					return ref["pathName"]
				}
			}
		}
	}
	return ""
}

func getItemClassPathFromEntry(props map[string]interface{}) string {
	if itemClassRaw, ok := props["ItemClass"]; ok {
		if ref, ok := itemClassRaw.(map[string]string); ok {
			return ref["pathName"]
		}
		if wrapper, ok := itemClassRaw.(map[string]interface{}); ok {
			if val, ok := wrapper["value"]; ok {
				if ref, ok := val.(map[string]string); ok {
					return ref["pathName"]
				}
			}
		}
	}
	return ""
}

func getNumItemsFromStack(props map[string]interface{}) int {
	if numRaw, ok := props["NumItems"]; ok {
		switch v := numRaw.(type) {
		case int32:
			return int(v)
		case float32:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

func getAmountFromEntry(props map[string]interface{}) int {
	for _, key := range []string{"Amount", "amount"} {
		if amountRaw, ok := props[key]; ok {
			switch v := amountRaw.(type) {
			case int32:
				return int(v)
			case float32:
				return int(v)
			case float64:
				return int(v)
			}
		}
	}
	return 0
}

// cleanItemName turns a descriptor path into a readable item name.
// Prefers item_display_names.json, falls back to trimming prefixes/suffixes.
func cleanItemName(itemPath string) string {
	if itemPath == "" {
		return ""
	}
	// Pull the class name off the end of the path.
	className := itemPath
	if idx := strings.LastIndex(className, "/"); idx >= 0 {
		className = className[idx+1:]
	}
	// e.g. "Desc_IronPlate.Desc_IronPlate_C" -> "Desc_IronPlate_C"
	if idx := strings.Index(className, "."); idx >= 0 {
		className = className[idx+1:]
	}

	// Try the display names table first.
	if itemDisplayNamesLoaded != nil {
		if displayName, ok := itemDisplayNamesLoaded[className]; ok {
			return displayName
		}
	}

	// No mapping, so trim the usual prefixes/suffixes.
	name := strings.TrimSuffix(className, "_C")
	name = strings.TrimPrefix(name, "Desc_")
	// Handle a stray "Desc" prefix followed by a capital letter.
	if strings.HasPrefix(name, "Desc") && len(name) > 4 {
		if name[4] >= 'A' && name[4] <= 'Z' {
			name = name[4:]
		} else if name[4] >= 'a' && name[4] <= 'z' {
			name = "C" + name[4:]
		}
	}
	// Remove BP_ItemDescriptor prefix
	name = strings.TrimPrefix(name, "BP_ItemDescriptor")
	// Remove BP_EquipmentDescriptor prefix
	name = strings.TrimPrefix(name, "BP_EquipmentDescriptor")
	return name
}
