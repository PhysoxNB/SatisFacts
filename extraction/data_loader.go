package extraction

import (
	"encoding/json"
	"embed"
)

//go:embed data/*.json
var dataFilesFS embed.FS

// PowerDataFile holds power_data.json content.
type PowerDataFile struct {
	Generators         map[string]PowerGeneratorData `json:"generators"`
	Consumers          map[string]PowerConsumerData  `json:"consumers"`
	PowerStorage       PowerStorageData              `json:"power_storage"`
	ClockSpeedFormulas map[string]interface{}        `json:"clock_speed_formulas"`
}

type PowerGeneratorData struct {
	BaseMW          interface{} `json:"base_mw"`
	Overclockable   bool        `json:"overclockable"`
	ClockSpeedFormula string    `json:"clock_speed_formula"`
	FuelTypes       []string    `json:"fuel_types"`
}

type PowerConsumerData struct {
	BaseMW float64 `json:"base_mw"`
}

type PowerStorageData struct {
	CapacityMWH    float64 `json:"capacity_mwh"`
	MaxChargeRateMW float64 `json:"max_charge_rate_mw"`
}

// MinersDataFile holds miners_data.json content.
type MinersDataFile map[string]MinerData

type MinerData struct {
	BaseRate            int                       `json:"base_rate"`
	PurityMultiplier    map[string]float64        `json:"purity_multiplier"`
}

// StorageDataFile holds storage_data.json content.
type StorageDataFile struct {
	Containers        map[string]ContainerData `json:"containers"`
	DimensionalDepot  map[string]interface{}   `json:"dimensional_depot"`
}

type ContainerData struct {
	Slots   int `json:"slots"`
	Inputs  int `json:"inputs"`
	Outputs int `json:"outputs"`
}

// MapDataFile holds map_data.json content.
type MapDataFile struct {
	Foundations FoundationData `json:"foundations"`
}

type FoundationData struct {
	WidthM   int `json:"width_m"`
	LengthM  int `json:"length_m"`
	AreaM2   int `json:"area_m2"`
}

// CollectiblesWorldDataFile holds all collectibles on a fresh map, keyed by "LevelName:PathName".
// Diffed against what's still in the save to get collected vs remaining.
type CollectiblesWorldDataFile struct {
	PowerSlugs    PowerSlugsWorldData `json:"power_slugs"`
	Somersloops   map[string]bool     `json:"somersloops"`
	MercerSpheres map[string]bool     `json:"mercer_spheres"`
	CrashSites    map[string]bool     `json:"crash_sites"`
}

type PowerSlugsWorldData struct {
	Blue   map[string]bool `json:"blue"`
	Yellow map[string]bool `json:"yellow"`
	Purple map[string]bool `json:"purple"`
}

// ResourceNodesDataFile holds resource_nodes.json content.
type ResourceNodesDataFile map[string]ResourceNodeData

type ResourceNodeData struct {
	Type    string `json:"type"`
	Purity  string `json:"purity"`
}

// SignDisplayNamesDataFile holds sign_display_names.json content.
type SignDisplayNamesDataFile map[string]string

// ItemDisplayNamesDataFile holds item_display_names.json content.
type ItemDisplayNamesDataFile map[string]string

// Parsed data files, loaded once at startup.
var (
	powerDataLoaded        *PowerDataFile
	minersDataLoaded       MinersDataFile
	storageDataLoaded      *StorageDataFile
	mapDataLoaded          *MapDataFile
	collectiblesWorldLoaded    *CollectiblesWorldDataFile
	resourceNodesLoaded        ResourceNodesDataFile
	signDisplayNamesLoaded     SignDisplayNamesDataFile
	itemDisplayNamesLoaded     ItemDisplayNamesDataFile
)

// LoadDataFiles loads embedded JSON data files. First batch is required, rest are optional.
func LoadDataFiles(dataDir string) error {
	if err := loadEmbeddedJSON("data/power_data.json", &powerDataLoaded); err != nil {
		return err
	}
	if err := loadEmbeddedJSON("data/miners_data.json", &minersDataLoaded); err != nil {
		return err
	}
	if err := loadEmbeddedJSON("data/storage_data.json", &storageDataLoaded); err != nil {
		return err
	}
	if err := loadEmbeddedJSON("data/map_data.json", &mapDataLoaded); err != nil {
		return err
	}

	// Optional from here on.
	_ = loadEmbeddedJSON("data/resource_nodes.json", &resourceNodesLoaded)
	_ = loadEmbeddedJSON("data/sign_display_names.json", &signDisplayNamesLoaded)
	_ = loadEmbeddedJSON("data/item_display_names.json", &itemDisplayNamesLoaded)
	_ = loadEmbeddedJSON("data/collectibles_world.json", &collectiblesWorldLoaded)

	return nil
}

func loadEmbeddedJSON(path string, target interface{}) error {
	data, err := dataFilesFS.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// IsCollectiblesWorldLoaded returns true if collectibles_world.json was successfully loaded.
func IsCollectiblesWorldLoaded() bool {
	return collectiblesWorldLoaded != nil
}

// GetSignDisplayNames returns the sign display names map, or nil if not loaded.
func GetSignDisplayNames() map[string]string {
	return signDisplayNamesLoaded
}

// GetCollectiblesWorldTotals returns the total counts from the world data.
func GetCollectiblesWorldTotals() (blue, yellow, purple, sloops, spheres, crashSites int) {
	if collectiblesWorldLoaded == nil {
		return
	}
	blue = len(collectiblesWorldLoaded.PowerSlugs.Blue)
	yellow = len(collectiblesWorldLoaded.PowerSlugs.Yellow)
	purple = len(collectiblesWorldLoaded.PowerSlugs.Purple)
	sloops = len(collectiblesWorldLoaded.Somersloops)
	spheres = len(collectiblesWorldLoaded.MercerSpheres)
	crashSites = len(collectiblesWorldLoaded.CrashSites)
	return
}
