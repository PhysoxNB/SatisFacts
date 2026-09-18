package extraction

import (
	"embed"
	"encoding/json"
)

//go:embed report/data/*.json
var dataFilesFS embed.FS

// PowerDataFile holds power_data.json content.
type PowerDataFile struct {
	Generators         map[string]PowerGeneratorData `json:"generators"`
	Consumers          map[string]PowerConsumerData  `json:"consumers"`
	PowerStorage       PowerStorageData              `json:"power_storage"`
	ClockSpeedFormulas map[string]interface{}        `json:"clock_speed_formulas"`
}

type PowerGeneratorData struct {
	BaseMW            interface{} `json:"base_mw"`
	Overclockable     bool        `json:"overclockable"`
	ClockSpeedFormula string      `json:"clock_speed_formula"`
	FuelTypes         []string    `json:"fuel_types"`
}

type PowerConsumerData struct {
	BaseMW float64 `json:"base_mw"`
}

type PowerStorageData struct {
	CapacityMWH     float64 `json:"capacity_mwh"`
	MaxChargeRateMW float64 `json:"max_charge_rate_mw"`
}

// MinersDataFile holds miners_data.json content.
type MinersDataFile map[string]MinerData

type MinerData struct {
	BaseRate         int                `json:"base_rate"`
	PurityMultiplier map[string]float64 `json:"purity_multiplier"`
}

// StorageDataFile holds storage_data.json content.
type StorageDataFile struct {
	Containers       map[string]ContainerData `json:"containers"`
	DimensionalDepot map[string]interface{}   `json:"dimensional_depot"`
}

type ContainerData struct {
	Slots   int `json:"slots"`
	Inputs  int `json:"inputs"`
	Outputs int `json:"outputs"`
}

// CollectiblePositionsData holds all world collectibles with their static positions,
// extracted from a fully-explored save. Used for:
// - Identifying which save objects are collectibles (key lookup)
// - Categorizing collected items from the TOC blob
// - Providing position data for remaining collectibles (with save positions as primary)
type CollectiblePositionsData struct {
	PowerSlugs    PowerSlugPositions  `json:"power_slugs"`
	Somersloops   map[string]Position `json:"somersloops"`
	MercerSpheres map[string]Position `json:"mercer_spheres"`
	CrashSites    map[string]Position `json:"crash_sites"`
}

type PowerSlugPositions struct {
	Blue   map[string]Position `json:"blue"`
	Yellow map[string]Position `json:"yellow"`
	Purple map[string]Position `json:"purple"`
}

type Position struct {
	X int `json:"X"`
	Y int `json:"Y"`
	Z int `json:"Z"`
}

// ResourceWellsDataFile holds resource_wells.json content.
// Keys are full instance paths (e.g. Persistent_Level:PersistentLevel.BP_FrackingSatellite14).
type ResourceWellsDataFile map[string]ResourceWellData

type ResourceWellData struct {
	Type   string `json:"type"`
	Purity string `json:"purity"`
	Core   string `json:"core"`
}

// ItemDisplayNamesDataFile holds item_display_names.json content.
type ItemDisplayNamesDataFile map[string]string

// Parsed data files, loaded once at startup.
var (
	powerDataLoaded            *PowerDataFile
	minersDataLoaded           MinersDataFile
	storageDataLoaded          *StorageDataFile
	collectiblePositionsLoaded *CollectiblePositionsData
	resourceWellsLoaded        ResourceWellsDataFile
	itemDisplayNamesLoaded     ItemDisplayNamesDataFile
	itemDisplayNamesDeLoaded   ItemDisplayNamesDataFile
)

// LoadDataFiles loads embedded JSON data files. First batch is required, rest are optional.
func LoadDataFiles(dataDir string) error {
	if err := loadEmbeddedJSON("report/data/power_data.json", &powerDataLoaded); err != nil {
		return err
	}
	if err := loadEmbeddedJSON("report/data/miners_data.json", &minersDataLoaded); err != nil {
		return err
	}
	if err := loadEmbeddedJSON("report/data/storage_data.json", &storageDataLoaded); err != nil {
		return err
	}

	// Optional from here on.
	_ = loadEmbeddedJSON("report/data/resource_wells.json", &resourceWellsLoaded)
	_ = loadEmbeddedJSON("report/data/item_display_names.json", &itemDisplayNamesLoaded)
	_ = loadEmbeddedJSON("report/data/item_display_names_de.json", &itemDisplayNamesDeLoaded)
	_ = loadEmbeddedJSON("report/data/collectible_positions.json", &collectiblePositionsLoaded)

	return nil
}

func loadEmbeddedJSON(path string, target interface{}) error {
	data, err := dataFilesFS.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// IsCollectiblePositionsLoaded returns true if collectible_positions.json was successfully loaded.
func IsCollectiblePositionsLoaded() bool {
	return collectiblePositionsLoaded != nil
}

// GetCollectibleTotals returns the total counts from the collectible positions data.
func GetCollectibleTotals() (blue, yellow, purple, sloops, spheres, crashSites int) {
	if collectiblePositionsLoaded == nil {
		return
	}
	blue = len(collectiblePositionsLoaded.PowerSlugs.Blue)
	yellow = len(collectiblePositionsLoaded.PowerSlugs.Yellow)
	purple = len(collectiblePositionsLoaded.PowerSlugs.Purple)
	sloops = len(collectiblePositionsLoaded.Somersloops)
	spheres = len(collectiblePositionsLoaded.MercerSpheres)
	crashSites = len(collectiblePositionsLoaded.CrashSites)
	return
}
