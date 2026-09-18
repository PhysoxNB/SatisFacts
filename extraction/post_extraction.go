package extraction

import (
	"strings"

	"satisfacts/parser"
)

// resourceClassName converts an internal resource name (e.g. "OreGold",
// "NitrogenGas") to a Desc_ class name (e.g. "Desc_OreGold_C") that the
// viewer's displayName() function can look up in itemDisplayNames.
func resourceClassName(name string) string {
	if name == "" {
		return ""
	}
	return "Desc_" + name + "_C"
}

// PostExtractionData is the second-pass output from retained objects.
type PostExtractionData struct {
	ClockSpeeds             map[string]float32
	ProductionBoosts        map[string]float32
	BuildingTypes           map[string]string
	BuildingRecipes         map[string]string
	BuildingInventories     map[string]BuildingInventoryRefs
	InventoryContents       map[string]int
	NuclearWaste            NuclearWasteData
	GeneratorPower          GeneratorPowerData
	BuildingConnections     map[string]BuildingConnection
	PowerConsumption        PowerConsumptionData
	APABuildingCount        int
	APAFueledCount          int
	MinerPurities           map[string]string
	MinerResources          map[string]string
	GeothermalPurities      map[string]string
	MinerResourceRefs       map[string]string
	FrackingFluidTypes      map[string]string
	FrackingPurities        map[string]string
	Collectibles            CollectiblesData
	StorageContainerDetails []StorageContainerDetail
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

type StorageItemEntry struct {
	Item  string `json:"item"`
	Count int    `json:"count"`
}

type StorageContainerDetail struct {
	Type     string             `json:"type"`
	Location *[3]float32        `json:"location"`
	Items    []StorageItemEntry `json:"items"`
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
	Instance     string
	Waste        int
	Fuel         int
	Supplemental int
	FuelType     string
	ClockSpeed   float32
	Location     *[3]float32
}

type GeneratorPowerData struct {
	TotalCapacityMW   float64
	ActiveCapacityMW  float64
	StandbyCapacityMW float64
	TheoreticalMaxMW  float64
	TotalGenerators   int
	ActiveGenerators  int
	StandbyGenerators int
	Generators        []GeneratorEntry
}

type GeneratorEntry struct {
	InstanceName             string `json:"-"`
	Type                     string `json:"-"`
	TypePath                 string
	Status                   string
	ActualMW                 float64
	MaxMW                    float64
	TheoreticalMaxMW         float64
	Reason                   string
	OutputPercent            int
	FuelType                 string
	PowerShardCount          int
	Location                 *ConsumerLocation
	VariablePowerCycleOffset float32
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
	InstanceName string `json:"-"`
	Type         string `json:"-"`
	TypePath     string
	Status       string
	Reason       string
	ActualMW     float64
	MaxMW        float64
	IsBlocked    bool
	Location     *ConsumerLocation
	ProducedItem string
	Productivity float64 `json:",omitempty"`
}

type ConsumerLocation struct {
	X int
	Y int
	Z int
}

// NewPostExtractionData returns a PostExtractionData with all its maps made.
func NewPostExtractionData() *PostExtractionData {
	return &PostExtractionData{
		ClockSpeeds:         make(map[string]float32),
		ProductionBoosts:    make(map[string]float32),
		BuildingTypes:       make(map[string]string),
		BuildingRecipes:     make(map[string]string),
		BuildingInventories: make(map[string]BuildingInventoryRefs),
		InventoryContents:   make(map[string]int),
		MinerPurities:       make(map[string]string),
		MinerResources:      make(map[string]string),
		GeothermalPurities:  make(map[string]string),
		MinerResourceRefs:   make(map[string]string),
		FrackingFluidTypes:  make(map[string]string),
		FrackingPurities:    make(map[string]string),
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

	// Sort keys so nuclear plants and APA instances come out in the same order
	// on every run.
	for _, instanceName := range sortedKeys(buildingFacts) {
		bf := buildingFacts[instanceName]
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

		// Oil pumps and water pumps also extract from a resource node but
		// don't have "miner" in their class name, so they need to be matched
		// explicitly here to get their purity and resource resolved.
		if (strings.Contains(typeLower, "miner") && !strings.Contains(typeLower, "portable")) ||
			strings.Contains(typeLower, "oilpump") || strings.Contains(typeLower, "waterpump") {
			if bf.ExtractableResource != "" {
				result.MinerResourceRefs[instanceName] = bf.ExtractableResource
			}
		}

		// Fracking extractor resource references
		if strings.Contains(typeLower, "frackingextractor") {
			if bf.ExtractableResource != "" {
				result.MinerResourceRefs[instanceName] = bf.ExtractableResource
			}
		}

		// Nuclear plants
		if strings.Contains(typeLower, "generatornuclear") || strings.Contains(typeLower, "build_generatornuclear") {
			extractNuclearPlant(bf, instanceName, &result.NuclearWaste, inventoryPotentialMap)
		}
	}

	// Central storage items - still from retained subsystem objects.
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
		isFracking := false
		if bt, ok := result.BuildingTypes[inst]; ok {
			isFracking = strings.Contains(strings.ToLower(bt), "frackingextractor")
		}

		if rf, ok := resourceNodeFacts[refPath]; ok && rf != nil {
			// 1.2+ saves may carry an override on the node itself.
			if rf.ResourceClassOverride != "" {
				if m := descNameRegex.FindStringSubmatch(rf.ResourceClassOverride); m != nil {
					if isFracking {
						result.FrackingFluidTypes[inst] = resourceClassName(m[1])
					} else {
						result.MinerResources[inst] = resourceClassName(m[1])
					}
				}
			}
			// Purity override, present from 1.2 on.
			if rf.PurityOverride != "" {
				purity := rf.PurityOverride
				if strings.HasPrefix(strings.ToUpper(purity), "RP_") {
					purity = purity[3:]
				}
				purity = strings.ToLower(purity)
				// RefinedPower mod misspells "impure" as "inpure".
				if purity == "inpure" {
					purity = "impure"
				}
				if isFracking {
					result.FrackingPurities[inst] = purity
				} else {
					result.MinerPurities[inst] = purity
				}
			}
		}
		// Fall back to resource_wells.json for saves where the
		// node/satellite has no mResourceClassOverride or mPurityOverride.
		if isFracking {
			if result.FrackingFluidTypes[inst] == "" || result.FrackingPurities[inst] == "" {
				if wt, wp, ok2 := lookupResourceWell(refPath); ok2 {
					if result.FrackingFluidTypes[inst] == "" {
						result.FrackingFluidTypes[inst] = resourceClassName(wt)
					}
					if result.FrackingPurities[inst] == "" {
						result.FrackingPurities[inst] = wp
					}
				}
			}
		} else {
			if result.MinerResources[inst] == "" || result.MinerPurities[inst] == "" {
				if wt, wp, ok2 := lookupResourceWell(refPath); ok2 {
					if result.MinerResources[inst] == "" {
						result.MinerResources[inst] = resourceClassName(wt)
					}
					if result.MinerPurities[inst] == "" {
						result.MinerPurities[inst] = wp
					}
				}
			}
		}
		// Final fallback: infer the resource from the extractor's output
		// inventory.  Some pre-1.2 saves reference resource nodes that are
		// missing from resource_wells.json (e.g. BP_ResourceNode11), and the
		// node itself carries no override properties. The output inventory
		// always contains the item being extracted, so we can recover the
		// resource class from it.
		//
		// We read bf.OutputInventory directly (not result.BuildingInventories)
		// because BuildingInventories may alias InventoryPotential as OutputRef
		// for buildings without mOutputInventory (e.g. water extractors), which
		// would give us Power Shards instead of the extracted resource.
		bf := buildingFacts[inst]
		if isFracking {
			if result.FrackingFluidTypes[inst] == "" {
				if rc := inferResourceFromOutputInventory(bf, inventoryStacks); rc != "" {
					result.FrackingFluidTypes[inst] = rc
				}
			}
		} else {
			if result.MinerResources[inst] == "" {
				if rc := inferResourceFromOutputInventory(bf, inventoryStacks); rc != "" {
					result.MinerResources[inst] = rc
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
		buildingConnections,
	)

	// Collectibles and storage contents.
	result.Collectibles = ExtractCollectibles(retained)
	resolveStorageInventories(inventoryStacks, buildingFacts, result.InventoryContents, &result.StorageContainerDetails)

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
