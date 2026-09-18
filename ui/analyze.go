package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"satisfacts/editor"
	"satisfacts/extraction"
	"satisfacts/parser"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ProgressUpdate is sent to the frontend via Wails events during analysis.
type ProgressUpdate struct {
	Step       int    `json:"step"`
	TotalSteps int    `json:"totalSteps"`
	Phase      string `json:"phase"`
	Current    int    `json:"current"`
	Total      int    `json:"total"`
}

// analyzePhase tracks the current step in the pipeline.
type analyzePhase struct {
	step       int
	totalSteps int
	emitFn     func(step, totalSteps int, phase string, current, total int)
}

func (p *analyzePhase) emit(phase string, current, total int) {
	p.emitFn(p.step, p.totalSteps, phase, current, total)
}

func (p *analyzePhase) next() {
	p.step++
}

// AnalyzeResult holds the result of an analysis run.
type AnalyzeResult struct {
	HtmlPath       string  `json:"htmlPath"`
	Elapsed        string  `json:"elapsed"`
	TotalObjects   int     `json:"totalObjects"`
	TotalBuildings int     `json:"totalBuildings"`
	HtmlSizeKB     float64 `json:"htmlSizeKB"`
	CanOptimize    bool    `json:"canOptimize"`
}

// AnalyzeSave runs the full extraction pipeline on a save file and generates an HTML report.
// Output files are written to outputDir. If outputDir is empty, defaults to the executable directory.
func (a *App) AnalyzeSave(savePath string, outputDir string, includeMap bool) (result *AnalyzeResult, err error) {
	startTime := time.Now()

	// Set up cancellation channel. The busy guard matters because a second
	// concurrent run would overwrite a.analyzeCancel and then the first run's
	// deferred cleanup would nil it out, silently breaking Cancel for the run
	// that is still going.
	a.mu.Lock()
	if a.analyzeRunning {
		a.mu.Unlock()
		return nil, errAnalyzeBusy
	}
	a.analyzeRunning = true
	a.analyzeCancel = make(chan struct{})
	analyzeCancel := a.analyzeCancel
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		a.analyzeCancel = nil
		a.analyzeRunning = false
		a.mu.Unlock()
	}()

	// checkCancel returns true if the analysis was cancelled
	checkCancel := func() bool {
		select {
		case <-analyzeCancel:
			return true
		default:
			return false
		}
	}

	// Output directory: use provided dir, or fall back to next to the app
	if outputDir == "" {
		outputDir = a.GetExeDir()
	}

	saveBaseName := filepath.Base(savePath)
	outputPrefix := strings.TrimSuffix(saveBaseName, filepath.Ext(saveBaseName))
	htmlFile := filepath.Join(outputDir, outputPrefix+".html")

	totalSteps := 10
	phase := &analyzePhase{
		step:       0,
		totalSteps: totalSteps,
		emitFn: func(step, totalSteps int, ph string, current, total int) {
			wailsruntime.EventsEmit(a.ctx, "analyze:progress", ProgressUpdate{
				Step:       step,
				TotalSteps: totalSteps,
				Phase:      ph,
				Current:    current,
				Total:      total,
			})
		},
	}

	// Corrupt or truncated saves panic deep in the parser. The return values are
	// named because otherwise this assignment goes nowhere and the frontend just
	// gets (null, null).
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("could not parse %q — the file may be corrupt, "+
				"truncated, or from an unsupported game version (details: %v)", savePath, r)
		}
	}()

	// Step 1: Read and parse header
	phase.next()
	phase.emit("reading", 0, 0)
	file, err := os.Open(savePath)
	if err != nil {
		return nil, fmt.Errorf("opening save file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat save file: %w", err)
	}
	fileSize := stat.Size()

	header, ctx, headerEnd, err := readSaveHeader(file, fileSize)
	if err != nil {
		return nil, fmt.Errorf("header parse: %w", err)
	}

	phase.next()
	phase.emit("header", 0, 0)

	// Phase 1: Stream-decompress body
	bodyReader := io.NewSectionReader(file, int64(headerEnd), fileSize-int64(headerEnd))

	decompressor := parser.NewDecompressor(bodyReader)
	chunkCh, err := decompressor.StreamChunks()
	if err != nil {
		return nil, fmt.Errorf("decompression setup: %w", err)
	}

	// Step 3: Parse body structure
	phase.next()
	phase.emit("body", 0, 0)
	sr := parser.NewStreamingReader(chunkCh)
	bodyParser := parser.NewBodyParser(sr, ctx)
	body, err := bodyParser.ParseBody()
	if err != nil {
		return nil, fmt.Errorf("body parse: %w", err)
	}
	if checkCancel() {
		return nil, fmt.Errorf("analysis cancelled")
	}

	// Step 4: Prepare extraction
	phase.next()
	phase.emit("preparing", 0, 0)

	// Load recipe and data files
	if err := extraction.LoadRecipes(""); err != nil {
		// non-fatal, just limited analytics
	}
	if err := extraction.LoadDataFiles(""); err != nil {
		// non-fatal
	}

	// Phase 4: Stream levels and extract
	extData := extraction.NewExtractedData(true)

	var totalObjects int
	var totalBuildings int
	var objectsWritten int
	var allSorted bool = true

	// Step 5: Stream objects
	phase.next()
	phase.emit("streaming", 0, 0)

	_, err = bodyParser.StreamLevels(body.SublevelCount, func(sr *parser.StreamingReader, dataBlobLength int64, level *parser.Level) error {
		extData.ProcessCollectables1(level.Collectables1)

		if level.ObjectHeaderCount == 0 || dataBlobLength == 0 {
			if dataBlobLength > 0 {
				return sr.Skip(int(dataBlobLength))
			}
			return nil
		}

		// If every level is already in optimized order, there's nothing to gain.
		if allSorted && !editor.IsOptimizedOrder(level.ObjectHeaders) {
			allSorted = false
		}

		dbParser := parser.NewDataBlobParser(ctx, level.ObjectHeaders, "DEEP")
		_, err := dbParser.StreamObjects(sr, dataBlobLength, int(level.ObjectHeaderCount), func(obj *parser.SaveObject, index int) {
			totalObjects++
			extData.ProcessObject(obj, true)

			if obj.Header != nil && obj.Header.Type == "Actor" && len(obj.Properties) > 0 {
				className := obj.Header.ClassName
				shortName := className
				if idx := strings.LastIndex(shortName, "/"); idx >= 0 {
					shortName = shortName[idx+1:]
				}
				if strings.HasPrefix(shortName, "Build_") {
					if extraction.IsBuilding(className) {
						totalBuildings++
					}
				}

				objectsWritten++

				if objectsWritten%50000 == 0 {
					phase.emit("streaming", objectsWritten, 0)
				}
			}
		})

		if err != nil {
			return fmt.Errorf("streaming objects: %w", err)
		}

		if checkCancel() {
			return fmt.Errorf("analysis cancelled")
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("streaming levels: %w", err)
	}

	// Build map data before cleanup (needs BuildingFacts, MapEntries, CollectibleLocations, rawTrackSegments)
	var mapData *extraction.MapData
	if includeMap {
		mapData = extraction.BuildMapData(extData)
	}

	// Free streaming maps
	extData.CleanupAfterStreaming()
	runtime.GC()

	if checkCancel() {
		return nil, fmt.Errorf("analysis cancelled")
	}

	// Resolve power circuits and logistics
	// Step 6: Analyze power systems
	phase.next()
	phase.emit("power", 0, 0)
	extData.ResolvePowerCircuits()

	if checkCancel() {
		return nil, fmt.Errorf("analysis cancelled")
	}

	// Step 7: Analyze logistics
	phase.next()
	phase.emit("logistics", 0, 0)
	extData.ResolveLogistics()

	if checkCancel() {
		return nil, fmt.Errorf("analysis cancelled")
	}

	// Step 8: Post-extraction
	phase.next()
	phase.emit("post", 0, 0)
	postData := extraction.RunPostExtraction(extData.BuildingFacts, extData.RetainedObjects, extData.RetainedObjectMap, extData.ResourceNodeFacts, extData.InventoryPotentialMap, extData.DefaultClockSpeeds, extData.BuildingConnections, extData.InventoryStacks)

	gameProgression := extraction.ExtractGameProgression(extData.RetainedObjects, extData.RetainedObjectMap)
	playerInfo := extraction.ExtractPlayerInfo(extData.RetainedObjects, extData.RetainedObjectMap)

	extData.CleanupAfterPostExtraction()
	runtime.GC()

	// Structures
	extData.FinalizePowerGrid()
	if extData.Structures != nil {
		extData.ApplyLightweightFixup()
	}

	// Production data
	productionOut := map[string]interface{}{
		"belts":                 extData.Belts,
		"beltHeads":             extData.BeltHeads,
		"pipes":                 extData.Pipes,
		"pipePumps":             extData.PipePumps,
		"lifts":                 extData.Lifts,
		"powerLines":            extData.PowerLines,
		"hypertubes":            extData.Hypertubes,
		"elevators":             extData.Elevators,
		"vehiclePaths":          extData.VehiclePaths,
		"rails":                 extData.Rails,
		"railComponents":        extData.RailComponents,
		"trains":                extData.Trains,
		"drones":                extData.Drones,
		"vehicles":              extData.Vehicles,
		"mergers":               extData.Mergers,
		"splitters":             extData.Splitters,
		"smartSplitters":        extData.SmartSplitters,
		"programmableSplitters": extData.ProgrammableSplitters,
		"powerPoles":            extData.PowerPoles,
		"powerPoleTypes":        extData.PowerPoleTypes,
		"hypertubeJunctions":    extData.HypertubeJunctions,
		"liftFloorHoles":        extData.LiftFloorHoles,
		"beltWallHoles":         extData.BeltWallHoles,
		"pipeFloorHoles":        extData.PipeFloorHoles,
		"itemsInTransit":        extData.ItemsInTransit,
		"others":                extData.Others,
		"elevatorComponents":    extData.ElevatorComponents,
	}

	// Analytics
	var analyticsResult interface{}
	{
		// Step 9: Analytics
		phase.next()
		phase.emit("analytics", 0, 0)

		resolvedInventories := extraction.BuildResolvedInventories(postData.BuildingInventories, extData.InventoryStacks)

		analyticsInput := extraction.AnalyticsInput{
			Buildings:               extData.Buildings,
			ClockSpeeds:             postData.ClockSpeeds,
			ProductionBoosts:        postData.ProductionBoosts,
			BuildingTypes:           postData.BuildingTypes,
			BuildingRecipes:         postData.BuildingRecipes,
			BuildingInventories:     postData.BuildingInventories,
			BuildingConnections:     postData.BuildingConnections,
			InventoryContents:       postData.InventoryContents,
			PowerConsumption:        postData.PowerConsumption,
			GeneratorPower:          postData.GeneratorPower,
			NuclearWaste:            postData.NuclearWaste,
			APABuildingCount:        postData.APABuildingCount,
			APAFueledCount:          postData.APAFueledCount,
			MinerPurities:           postData.MinerPurities,
			MinerResources:          postData.MinerResources,
			GeothermalPurities:      postData.GeothermalPurities,
			FrackingFluidTypes:      postData.FrackingFluidTypes,
			FrackingPurities:        postData.FrackingPurities,
			Collectibles:            postData.Collectibles,
			CollectiblesFound:       extData.CollectiblesFound,
			CollectedCollectibles:   extData.CollectedCollectibles,
			CollectibleLocations:    extData.CollectibleLocations,
			Pickups:                 extData.Pickups,
			GameProgression:         gameProgression,
			Players:                 playerInfo,
			SaveVersion:             int(header.SaveVersion),
			Belts:                   extData.Belts,
			BeltHeads:               extData.BeltHeads,
			Lifts:                   extData.Lifts,
			Pipes:                   extData.Pipes,
			PowerLines:              extData.PowerLines,
			Hypertubes:              extData.Hypertubes,
			Elevators:               extData.Elevators,
			VehiclePaths:            extData.VehiclePaths,
			Rails:                   extData.Rails,
			Trains:                  extData.Trains,
			Drones:                  extData.Drones,
			Vehicles:                extData.Vehicles,
			Structures:              extData.Structures,
			ResolvedInventories:     resolvedInventories,
			RetainedObjects:         extData.RetainedObjects,
			BuildingFacts:           extData.BuildingFacts,
			StorageContainerDetails: postData.StorageContainerDetails,
		}

		analyticsResult = extraction.CalculateAnalytics(analyticsInput)
		extData.InventoryStacks = nil
	}

	// Slim down postData for HTML report. The viewer only reads GeneratorPower
	// and PowerConsumption from extraction. All other fields (BuildingConnections,
	// BuildingTypes, BuildingRecipes, ClockSpeeds, BuildingInventories, etc.) are
	// only used as analytics inputs and are dead weight in the HTML embed.
	postData.ClockSpeeds = nil
	postData.ProductionBoosts = nil
	postData.BuildingTypes = nil
	postData.BuildingRecipes = nil
	postData.BuildingInventories = nil
	postData.InventoryContents = nil
	postData.BuildingConnections = nil
	postData.APABuildingCount = 0
	postData.APAFueledCount = 0
	postData.MinerPurities = nil
	postData.MinerResources = nil
	postData.MinerResourceRefs = nil
	postData.GeothermalPurities = nil
	postData.FrackingFluidTypes = nil
	postData.FrackingPurities = nil
	postData.Collectibles = extraction.CollectiblesData{}
	postData.StorageContainerDetails = nil
	postData.NuclearWaste = extraction.NuclearWasteData{}

	// Strip bottlenecks from manufacturing analytics - the viewer never reads
	// bottlenecks.waiting/blocked/paused (100+ MB on large saves). The same
	// info is available via the power tab's consumer table.
	if ad, ok := analyticsResult.(*extraction.AnalyticsData); ok && ad != nil && ad.Manufacturing != nil {
		delete(ad.Manufacturing, "bottlenecks")
	}

	runtime.GC()

	// Generate HTML report
	// Step 10: Generate HTML report
	phase.next()
	phase.emit("html", 0, 0)

	headerMap := map[string]interface{}{
		"SaveName":            header.SaveName,
		"SessionName":         header.SessionName,
		"SaveVersion":         header.SaveVersion,
		"BuildVersion":        header.BuildVersion,
		"MapName":             header.MapName,
		"MapOptions":          header.MapOptions,
		"SaveDateTime":        header.SaveDateTime,
		"SaveHeaderType":      header.SaveHeaderType,
		"IsModdedSave":        header.IsModdedSave,
		"CreativeModeEnabled": header.CreativeModeEnabled,
		"PlayDurationSeconds": header.PlayDurationSeconds,
	}

	report := extraction.ReportData{
		Header:              headerMap,
		Mode:                "DEEP",
		TotalObjects:        totalObjects,
		TotalBuildings:      totalBuildings,
		ObjectsWritten:      objectsWritten,
		Buildings:           extData.Buildings,
		Blueprints:          extData.Blueprints,
		Pets:                extData.Pets,
		CreatureKills:       extData.CreatureKills,
		ConsumablesConsumed: extData.ConsumablesConsumed,
		Logistics:           extData.Logistics,
		Extraction:          postData,
		Structures:          extData.Structures,
		Production:          productionOut,
		PowerGridData:       extData.PowerGrid,
		GameProgression:     gameProgression,
		Analytics:           nil,
		ItemDisplayNames:    extraction.GetItemDisplayNamesEn(),
		ItemDisplayNamesDe:  extraction.GetItemDisplayNamesDe(),
		UIStrings:           extraction.GetUIStrings(),
		UIStringsDe:         extraction.GetUIStringsDe(),
		MapData:             mapData,
	}
	report.Analytics = analyticsResult

	// Inject custom map colors from user settings (if any)
	if settings := LoadSettings(); settings != nil && len(settings.MapColors) > 0 {
		report.MapColors = settings.MapColors
	}

	if err := extraction.GenerateHTML(report, htmlFile); err != nil {
		return nil, fmt.Errorf("HTML generation: %w", err)
	}

	htmlSize, statErr := statSize(htmlFile)
	elapsed := time.Since(startTime)

	// No next() here - the HTML step above is already the last one. Advancing
	// again reported step 11 of 10 and pushed the frontend progress bar past
	// 100%.
	phase.emit("done", 0, 0)

	var htmlSizeKB float64
	if statErr == nil {
		htmlSizeKB = float64(htmlSize) / 1024
	}

	// Cleanup
	extData.RetainedObjects = nil
	extData.RetainedObjectMap = nil
	postData = nil
	analyticsResult = nil
	report = extraction.ReportData{}
	runtime.GC()

	return &AnalyzeResult{
		HtmlPath:       htmlFile,
		Elapsed:        elapsed.String(),
		TotalObjects:   totalObjects,
		TotalBuildings: totalBuildings,
		HtmlSizeKB:     htmlSizeKB,
		CanOptimize:    !allSorted,
	}, nil
}

// readSaveHeader parses the header without loading the whole save. It starts with
// a small buffer, since headers are normally tiny, and grows it when the header
// runs past the end. Mod metadata in the header makes its size hard to predict.
func readSaveHeader(r io.ReaderAt, fileSize int64) (*parser.SaveHeader, *parser.VersionContext, int, error) {
	for size := int64(64 * 1024); ; size *= 4 {
		if size > fileSize {
			size = fileSize
		}
		buf := make([]byte, size)
		if _, err := io.ReadFull(io.NewSectionReader(r, 0, size), buf); err != nil {
			return nil, nil, 0, fmt.Errorf("reading header: %w", err)
		}
		header, ctx, end, err, complete := tryParseHeader(buf)
		if complete {
			return header, ctx, end, err
		}
		if size == fileSize {
			return nil, nil, 0, fmt.Errorf("the header runs past the end of the file - the file may be corrupt")
		}
	}
}

// tryParseHeader reports complete=false when buf ended before the header did,
// which BinaryReader signals with a panic.
func tryParseHeader(buf []byte) (header *parser.SaveHeader, ctx *parser.VersionContext, end int, err error, complete bool) {
	defer func() {
		if r := recover(); r != nil {
			complete = false
		}
	}()
	reader := parser.NewBinaryReader(buf)
	header, ctx, err = parser.ParseHeader(reader)
	return header, ctx, reader.Position(), err, true
}
