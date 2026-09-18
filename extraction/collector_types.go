package extraction

// PetData tracks tamed creatures (Lizard Doggos etc.)
type PetData struct {
	TamedDoggos int `json:"tamedDoggos"`
	TotalDoggos int `json:"totalDoggos"`
}

// CreatureKillData tracks creatures killed by players, by creature type.
type CreatureKillData struct {
	ByType map[string]int `json:"byType"` // pathName -> kill count
	Total  int            `json:"total"`
}

// ConsumablesData tracks items consumed (eaten) by players, by item type.
type ConsumablesData struct {
	ByType map[string]int `json:"byType"` // pathName -> consume count
	Total  int            `json:"total"`
}

type PickupItem struct {
	ItemClass string     `json:"item_class"`
	ItemName  string     `json:"item_name"`
	NumItems  int        `json:"num_items"`
	Position  [3]float32 `json:"position"`
	Collected bool       `json:"collected"`
}

// CollectiblesFoundData tracks collectibles still in the save (not yet picked up).
type CollectiblesFoundData struct {
	PowerSlugsBlue    map[string]bool
	PowerSlugsYellow  map[string]bool
	PowerSlugsPurple  map[string]bool
	Somersloops       map[string]bool
	MercerSpheres     map[string]bool
	CrashSites        map[string]bool // all crash sites still in save
	CrashSiteOpened   map[string]bool // crash sites with mIsOpened=true
	CrashSiteUnopened map[string]bool // crash sites with mIsOpened=false or no mIsOpened
}

// CollectedCollectiblesData tracks collectibles the player has collected,
// from the collectables1 list in each level's TOC blob.
type CollectedCollectiblesData struct {
	PowerSlugsBlue   map[string]bool
	PowerSlugsYellow map[string]bool
	PowerSlugsPurple map[string]bool
	Somersloops      map[string]bool
	MercerSpheres    map[string]bool
	CrashSites       map[string]bool
}

type BeltTypeStats struct {
	Count       int
	TotalLength float64
}

type PipeTypeStats struct {
	Count       int
	TotalLength float64
}

type RailStats struct {
	TotalLength  float64
	Count        int
	BlockSignals int `json:",omitempty"`
	PathSignals  int `json:",omitempty"`
}

type TrainStats struct {
	Locomotives           int
	Wagons                int
	Stations              int
	FreightPlatforms      int
	FluidFreightPlatforms int
}

type DroneStats struct {
	Stations         int
	FreightPlatforms int
	Drones           int
}

type VehicleStats struct {
	Trucks             int
	FluidTrucks        int
	Explorers          int
	Tractors           int
	Golfcarts          int
	Cyberwagons        int
	TruckStations      int
	FluidTruckStations int
}

// LogisticsData holds detailed train, vehicle, and drone tracking data
// with resolved names, routes, and status warnings.
type LogisticsData struct {
	Trains          []TrainInfo          `json:"trains"`
	TrainStations   []TrainStationInfo   `json:"trainStations"`
	Vehicles        []VehicleInfo        `json:"vehicles"`
	VehicleStations []VehicleStationInfo `json:"vehicleStations"`
	Drones          []DroneInfo          `json:"drones"`
	DroneStations   []DroneStationInfo   `json:"droneStations"`
}

// TrackSegment is one piece of rail. We store its spline length and the
// pathNames of its two endpoints so we can wire up the graph later.
type TrackSegment struct {
	Length float64    // meters
	Conn0  string     // pathName of TrackConnection0
	Conn1  string     // pathName of TrackConnection1
	Pos    [3]float64 // actor origin in meters, fallback when the spline is unreadable

	// Where the two ends of the rail actually are, in world meters, taken from
	// the first and last spline point. Conn0 is the start of the spline and
	// Conn1 the end. HasEndpoints is false for the odd piece whose spline we
	// can't read, and those fall back to Pos for both ends.
	Conn0Pos     [3]float64
	Conn1Pos     [3]float64
	HasEndpoints bool
}

// TrackConn is a track connection component - basically a junction point
// that knows which other connection components it links to.
type TrackConn struct {
	ConnectedTo []string // pathNames of connected connection components
}

// SignalInfo holds the directional info from a railroad signal. Signals
// enforce one-way travel: trains can go from observed -> guarded but not back.
type SignalInfo struct {
	Type     string   // "block" or "path"
	Guarded  []string // connection pathNames the signal protects
	Observed []string // connection pathNames trains approach from
}

type TrainInfo struct {
	Name          string   `json:"name"`
	Locomotives   int      `json:"locomotives"`
	Wagons        int      `json:"wagons"`
	WagonFillPct  int      `json:"wagonFillPct"`
	CurrentStop   int      `json:"currentStop"`
	Route         []string `json:"route"`                   // station names
	TotalDistance float64  `json:"totalDistance,omitempty"` // total route distance in km
	Warnings      []string `json:"warnings,omitempty"`
}

type TrainStationInfo struct {
	Name            string     `json:"name"`
	DockedTrain     string     `json:"dockedTrain"`
	Platforms       int        `json:"platforms"`
	PlatformFillPct int        `json:"platformFillPct"`
	Location        [3]float64 `json:"location,omitempty"`
	Warnings        []string   `json:"warnings,omitempty"`
}

type VehicleInfo struct {
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	Fuel     string     `json:"fuel"`
	PathName string     `json:"pathName"`
	Location [3]float64 `json:"location,omitempty"`
	OnRoute  bool       `json:"onRoute"`
	Warnings []string   `json:"warnings,omitempty"`
}

type VehicleStationInfo struct {
	Name                string     `json:"name"`
	Type                string     `json:"type"`
	Status              string     `json:"status"`
	LastDockTime        float64    `json:"lastDockTime"`
	AvgItemsTransferred float64    `json:"avgItemsTransferred"`
	AvgTimeBetweenDocks float64    `json:"avgTimeBetweenDocks"`
	DockedVehicle       string     `json:"dockedVehicle"`
	Location            [3]float64 `json:"location,omitempty"`
	Warnings            []string   `json:"warnings,omitempty"`
}

type DroneInfo struct {
	Fuel          string   `json:"fuel"`
	DockingState  string   `json:"dockingState"`
	HomeStation   string   `json:"homeStation"`
	Destination   string   `json:"destination,omitempty"`
	TripItemCount int      `json:"tripItemCount,omitempty"`
	TripDuration  float64  `json:"tripDuration,omitempty"`
	Warnings      []string `json:"warnings,omitempty"`
}

type DroneStationInfo struct {
	Name          string     `json:"name"`
	PairedStation string     `json:"pairedStation"`
	IsProducing   bool       `json:"isProducing"`
	TransferStage string     `json:"transferStage"`
	Location      [3]float64 `json:"location,omitempty"`
	Warnings      []string   `json:"warnings,omitempty"`
}

type PowerGridData struct {
	Circuits       []CircuitInfo
	ComponentCount int
	LineCount      int
}

type CircuitInfo struct {
	ID             int32
	ComponentCount int
	// Every connection hanging off this circuit, e.g.
	// "...Build_TrainStation_C_123.PowerConnection" or "...ThirdRail4".
	// Kept out of the JSON, it's only here so we can work out which circuits
	// have a generator on them.
	Components []string `json:"-"`
}

type StructureData struct {
	Details              map[string]map[string]int `json:"details"`
	Totals               map[string]int            `json:"totals"`
	lightweightCounts    map[string]map[string]int // category -> typePath -> count (from lightweight subsystem)
	sublevelCounts       map[string]map[string]int // category -> typePath -> count (individual objects NOT from Persistent_Level)
	persistentLevelTypes map[string]bool           // typePaths that appear as individual Persistent_Level objects
}
