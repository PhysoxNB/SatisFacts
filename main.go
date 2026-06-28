package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"satisfacts/extraction"
	"satisfacts/parser"
)

// release version, shown by --version
const version = "1.0.0"

// Shared buffered reader for all console input. Using one reader avoids
// bufio swallowing bytes that a separate os.Stdin.Read never sees (Git Bash,
// MSYS2, mintty, or typed-ahead Enter).
var stdin = bufio.NewReader(os.Stdin)

// Reused in the write loop to avoid allocating per object on big saves.
type objectJSON struct {
	Index      int                        `json:"index"`
	ClassName  string                     `json:"className"`
	Type       string                     `json:"type"`
	Reference  parser.ObjectRef           `json:"reference"`
	Properties map[string]parser.Property `json:"properties"`
	Transform  *parser.Transform3f        `json:"transform,omitempty"`
}

func main() {
	args := os.Args[1:]

	// Check for flags before we treat arg 0 as a save path.
	if len(args) >= 1 {
		switch strings.ToLower(args[0]) {
		case "-h", "--help", "help":
			printUsage()
			return
		case "-v", "--version", "version":
			fmt.Printf("SatisFacts v%s by Physox\n", version)
			return
		}
	}

	// No args, so fall back to the interactive picker.
	if len(args) == 0 {
		runInteractive()
		return
	}

	// Direct CLI invocation: <save.sav> [MODE]
	saveFile := args[0]
	mode := "DEEP"
	if len(args) >= 2 {
		mode = strings.ToUpper(args[1])
	}
	if !isValidMode(mode) {
		printErrorf("Unknown mode %q. Valid modes: QUICK, DEEP.", mode)
		pauseExit(1)
	}

	if err := processSave(saveFile, mode); err != nil {
		printErrorf("%v", err)
		pauseExit(1)
	}
	pauseExit(0)
}

// runInteractive lists the .sav files in the current folder and asks the user
// to pick a file and a mode. It re-prompts on bad input and pauses before
// exiting so the window stays open when someone double-clicks the exe.
func runInteractive() {
	reader := stdin

	// If there's no real terminal (e.g. launched from a file manager with no
	// console attached) we can't show the menu, so bail with a hint instead of
	// spinning forever on closed stdin.
	if !stdinIsTerminal() {
		printStep("SatisFacts needs an interactive terminal for the menu.")
		printStep("Open a terminal and run:  ./satisfacts")
		printStep("or pass arguments:        ./satisfacts <save.sav> [QUICK|DEEP]")
		return
	}

	printBanner()

	// Grab the .sav files in the current folder.
	entries, err := os.ReadDir(".")
	if err != nil {
		printErrorf("reading directory: %v", err)
		pauseExit(1)
	}

	var savFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".sav") {
			savFiles = append(savFiles, e.Name())
		}
	}

	if len(savFiles) == 0 {
		printStep("No .sav files found in current directory.")
		printStep("Place this executable in your save folder and run again.")
		pauseExit(1)
	}

	// Show what we found.
	printSectionHeader("Save files found")
	for i, f := range savFiles {
		if info, err := os.Stat(f); err == nil {
			name := f
			if len(name) > 31 {
				name = name[:28] + "..."
			}
			fmt.Printf("│  %s. %-31s %s%s\n",
				wrap(cBold, fmt.Sprintf("%d", i+1)),
				name,
				wrap(cDim, fmt.Sprintf("%8s", fmt.Sprintf("%.1f MB", float64(info.Size())/1024/1024))),
				wrap(cCyan, "│"))
		} else {
			fmt.Printf("│  %s. %-31s %8s%s\n",
				wrap(cBold, fmt.Sprintf("%d", i+1)),
				f,
				"",
				wrap(cCyan, "│"))
		}
	}
	printSectionFooter()

	// Ask which file. Re-prompt on bad input, 'q' quits.
	var saveFile string
	for {
		fmt.Print("\n")
		fmt.Printf("%s ", wrap(cYellow, "Select file number (or 'q' to quit):"))
		fileInput, err := reader.ReadString('\n')
		line := strings.TrimSpace(fileInput)
		if line == "" && err != nil {
			// stdin closed (EOF), nothing left to read so just leave.
			return
		}
		if strings.EqualFold(line, "q") {
			return
		}
		fileIdx, convErr := strconv.Atoi(line)
		if convErr != nil || fileIdx < 1 || fileIdx > len(savFiles) {
			printWarnf("Invalid selection. Enter a number between 1 and %d.", len(savFiles))
			continue
		}
		saveFile = savFiles[fileIdx-1]
		break
	}

	// Ask for the mode, same re-prompt loop.
	printSectionHeader("Extraction modes")
	printMenuItem(1, "QUICK", "Basic counts (fastest)")
	printRecommended(2, "DEEP", "Full analysis")
	printSectionFooter()

	var mode string
	for {
		fmt.Print("\n")
		fmt.Printf("%s ", wrap(cYellow, "Select mode [1-2] (default 2):"))
		modeInput, err := reader.ReadString('\n')
		line := strings.TrimSpace(modeInput)
		if line == "" && err != nil {
			// stdin closed (EOF), just use the default mode.
			mode = "DEEP"
			break
		}
		switch strings.ToUpper(line) {
		case "1", "QUICK":
			mode = "QUICK"
		case "2", "", "DEEP":
			mode = "DEEP"
		default:
			printWarn("Invalid mode. Enter 1 or 2.")
			continue
		}
		break
	}

	fmt.Println()
	if err := processSave(saveFile, mode); err != nil {
		printErrorf("%v", err)
		pauseExit(1)
	}
	pauseExit(0)
}

// printUsage prints command-line help.
func printUsage() {
	fmt.Printf(`SatisFacts v%s — Satisfactory Save File Analyzer (by Physox)

Usage:
  satisfacts                      Interactive mode (scan current folder for .sav files)
  satisfacts <save.sav>           Analyze a save (default mode: DEEP)
  satisfacts <save.sav> <MODE>    Analyze with a specific mode

Modes:
  QUICK   Basic counts only: buildings, structures, overview (fastest)
  DEEP    Full analysis: power, production, storage, collectibles, transport (default)

Flags:
  -h, --help      Show this help
  -v, --version   Show version

Output:
  <save>_<mode>.json and <save>_<mode>.html next to the save file.
`, version)
}

// isValidMode checks the mode string. MAP is accepted but left undocumented
// since the map tool is still a work in progress.
func isValidMode(mode string) bool {
	switch mode {
	case "QUICK", "DEEP", "MAP":
		return true
	}
	return false
}

// stdinIsTerminal reports whether stdin is a real interactive terminal.
func stdinIsTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// pauseExit waits for an Enter press before exiting. Keeps the console open for
// people who double-click the executable.
func pauseExit(code int) {
	fmt.Println()
	fmt.Printf("%s\n", wrap(cDim, "Press Enter to exit..."))
	waitForEnter()
	os.Exit(code)
}

// waitForEnter reads from the shared reader until newline or EOF.
func waitForEnter() {
	_, _ = stdin.ReadString('\n')
}

func processSave(savePath, mode string) (err error) {
	startTime := time.Now()

	// Recover panics from corrupt/truncated saves so the user sees a message
	// instead of a stack trace.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("could not parse %q — the file may be corrupt, "+
				"truncated, or from an unsupported game version (details: %v)", savePath, r)
		}
	}()

	// Read the save file off disk.
	fmt.Println()
	printSectionHeader("Processing")
	printStepf("Reading %s ...", savePath)
	file, err := os.Open(savePath)
	if err != nil {
		return fmt.Errorf("opening save file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat save file: %w", err)
	}
	fileSize := stat.Size()
	printDonef("%s (%.1f MB)", savePath, float64(fileSize)/1024/1024)

	// Phase 0: header is uncompressed at the start of the file.
	printStep("Parsing header...")
	headerBufSize := 64 * 1024
	if int64(headerBufSize) > fileSize {
		headerBufSize = int(fileSize)
	}
	headerBuf := make([]byte, headerBufSize)
	if _, err := io.ReadFull(file, headerBuf); err != nil {
		return fmt.Errorf("reading header: %w", err)
	}
	headerReader := parser.NewBinaryReader(headerBuf)
	header, ctx, err := parser.ParseHeader(headerReader)
	if err != nil {
		return fmt.Errorf("header parse: %w", err)
	}
	headerEnd := headerReader.Position()

	// If the header somehow exceeds our buffer, fall back to full read.
	var fallbackFileData []byte
	if headerEnd > headerBufSize {
		file.Close()
		fallbackFileData, err = os.ReadFile(savePath)
		if err != nil {
			return fmt.Errorf("reading save file (fallback): %w", err)
		}
		headerReader = parser.NewBinaryReader(fallbackFileData)
		header, ctx, err = parser.ParseHeader(headerReader)
		if err != nil {
			return fmt.Errorf("header parse (fallback): %w", err)
		}
		headerEnd = headerReader.Position()
	}

	printDonef("Header parsed: %s (v%d)", header.SessionName, header.SaveVersion)
	printDetailf("Save: %s (v%d, header v%d)", header.SessionName, header.SaveVersion, header.SaveHeaderType)
	printDetailf("Map: %s", header.MapName)
	printDetailf("Build: %d", header.BuildVersion)
	if header.SaveName != "" {
		printDetailf("SaveName: %s", header.SaveName)
	}

	// Phase 1: stream-decompress everything after the header.
	var bodyReader io.ReadSeeker
	if fallbackFileData != nil {
		bodyReader = &sliceReader{data: fallbackFileData[headerEnd:], pos: 0}
	} else {
		bodyReader = io.NewSectionReader(file, int64(headerEnd), fileSize-int64(headerEnd))
	}

	decompressor := parser.NewDecompressor(bodyReader)
	chunkCh, err := decompressor.StreamChunks()
	if err != nil {
		return fmt.Errorf("decompression setup: %w", err)
	}

	// Header buffer no longer needed
	headerBuf = nil
	fallbackFileData = nil

	// Phase 2: parse the body structure and stream the levels.
	sr := parser.NewStreamingReader(chunkCh)
	bodyParser := parser.NewBodyParser(sr, ctx)

	printStep("Parsing body structure...")
	body, err := bodyParser.ParseBody()
	if err != nil {
		return fmt.Errorf("body parse: %w", err)
	}
	printDonef("Body parsed: %d levels", body.LevelCount)
	printDetailf("Sublevels: %d (total levels: %d)", body.SublevelCount, body.LevelCount)

	// Phase 3: open the JSON output and get ready to stream objects into it.
	outputPrefix := strings.TrimSuffix(savePath, filepath.Ext(savePath))
	modeLower := strings.ToLower(mode)
	jsonFile := outputPrefix + "_" + modeLower + ".json"
	printStepf("Streaming to %s ...", jsonFile)

	f, err := os.Create(jsonFile)
	if err != nil {
		return fmt.Errorf("creating JSON file: %w", err)
	}
	defer f.Close()
	bw := bufio.NewWriterSize(f, 256*1024)

	// Write the header and open the objects array.
	bw.WriteString(`{"mode":"` + mode + `","header":`)
	if data, err := json.Marshal(header); err != nil {
		return err
	} else {
		bw.Write(data)
	}
	bw.WriteString(`,"objects":[`)

	// Recipe data is embedded via go:embed (see extraction/data_loader.go).
	if err := extraction.LoadRecipes(""); err != nil {
		printWarnf("could not load recipes.json: %v (power status detection will be limited)", err)
	}

	// Load the rest of the data files. Has to happen before streaming because
	// collectibles tracking runs inside ProcessObject during the stream loop.
	if err := extraction.LoadDataFiles(""); err != nil {
		printWarnf("could not load data files: %v (analytics will be limited)", err)
	}

	// Phase 4: stream the levels, extract as we go, write objects to JSON.
	extractStructures := mode != ""
	extData := extraction.NewExtractedDataWithMode(extractStructures, mode == "MAP")

	var totalObjects int
	var totalBuildings int
	var totalActors int
	var totalObjectsWithProps int
	var objectsWritten int
	buildingTypes := make(map[string]int)
	firstObject := true
	var objJSON objectJSON // reused in the write loop to avoid allocating per object

	levels, err := bodyParser.StreamLevels(body.SublevelCount, func(sr *parser.StreamingReader, dataBlobLength int64, level *parser.Level) error {
		if level.ObjectHeaderCount == 0 || dataBlobLength == 0 {
			if dataBlobLength > 0 {
				return sr.Skip(int(dataBlobLength))
			}
			return nil
		}

		for _, h := range level.ObjectHeaders {
			if h.Type == "Actor" {
				totalActors++
			}
		}

		dbParser := parser.NewDataBlobParser(ctx, level.ObjectHeaders, mode)
		_, err := dbParser.StreamObjects(sr, dataBlobLength, int(level.ObjectHeaderCount), func(obj *parser.SaveObject, index int) {
			totalObjects++

			// Every object goes through extraction.
			extData.ProcessObject(obj, extractStructures)

			// Only objects that carry properties get written to JSON.
			if obj.Header != nil && obj.Header.Type == "Actor" && len(obj.Properties) > 0 {
				totalObjectsWithProps++
				className := obj.Header.ClassName
				shortName := className
				if idx := strings.LastIndex(shortName, "/"); idx >= 0 {
					shortName = shortName[idx+1:]
				}
				if strings.HasPrefix(shortName, "Build_") {
					if extraction.IsBuilding(className) {
						totalBuildings++
					}
					buildingTypes[shortName]++
				}

				// Write it out right away, nothing retained.
				if !firstObject {
					bw.WriteByte(',')
				}
				firstObject = false

				objJSON.Index = obj.Index
				objJSON.ClassName = obj.Header.ClassName
				objJSON.Type = obj.Header.Type
				objJSON.Reference = obj.Header.Reference
				objJSON.Properties = obj.Properties
				if obj.Header.NeedTransform {
					objJSON.Transform = &obj.Header.Transform
				} else {
					objJSON.Transform = nil
				}
				if data, err := json.Marshal(&objJSON); err == nil {
					bw.Write(data)
				}
				objectsWritten++

				if objectsWritten%50000 == 0 {
					printDetailf("Written %d objects...", objectsWritten)
				}
			}
		})

		if err != nil {
			return fmt.Errorf("streaming objects: %w", err)
		}

		// Process collectables1 from this level's TOC blob — these are
		// the collectibles the player has collected/dismantled.
		extData.ProcessCollectables1(level.Collectables1)

		return nil
	})
	if err != nil {
		return fmt.Errorf("streaming levels: %w", err)
	}

	// Close the objects array and tack on the summary metadata.
	bw.WriteString(`],"totalObjects":`)
	bw.WriteString(strconv.Itoa(totalObjects))
	bw.WriteString(`,"totalBuildings":`)
	bw.WriteString(strconv.Itoa(totalBuildings))
	bw.WriteString(`,"objectsWritten":`)
	bw.WriteString(strconv.Itoa(objectsWritten))

	// Building types
	bw.WriteString(`,"buildingTypes":`)
	if data, err := json.Marshal(buildingTypes); err != nil {
		return err
	} else {
		bw.Write(data)
	}

	// Free maps only needed during ProcessObject
	extData.CleanupAfterStreaming()
	runtime.GC()

	// Second pass over the retained objects: clock speeds, recipes, power,
	// inventories, nuclear, connections.
	printStepf("Retained %d objects + %d BuildingFacts (of %d processed) for post-extraction...",
		len(extData.RetainedObjects), len(extData.BuildingFacts), extData.TotalProcessed)

	printStep("Running post-extraction on BuildingFacts...")

	postData := extraction.RunPostExtraction(extData.BuildingFacts, extData.RetainedObjects, extData.RetainedObjectMap, extData.ResourceNodeFacts, extData.InventoryPotentialMap, extData.DefaultClockSpeeds, extData.BuildingConnections, extData.InventoryStacks)

	// Extract game progression
	gameProgression := extraction.ExtractGameProgression(extData.RetainedObjects, extData.RetainedObjectMap)

	// Free maps only needed by RunPostExtraction
	extData.CleanupAfterPostExtraction()
	runtime.GC()
	printDonef("Post-extraction: %d clocks, %d recipes, %d generators, %d connections",
		len(postData.ClockSpeeds), len(postData.BuildingRecipes),
		postData.GeneratorPower.TotalGenerators, len(postData.BuildingConnections))

	bw.WriteString(`,"extraction":`)
	if data, err := json.Marshal(postData); err != nil {
		return fmt.Errorf("marshalling extraction data: %w", err)
	} else {
		bw.Write(data)
	}

	// Structures (DEEP/MAP/QUICK modes)
	if extData.Structures != nil {
		extData.ApplyLightweightFixup()
		bw.WriteString(`,"structures":`)
		if data, err := json.Marshal(extData.Structures); err != nil {
			return fmt.Errorf("marshalling structures: %w", err)
		} else {
			bw.Write(data)
		}
	}

	// Production data
	productionOut := map[string]interface{}{
		"belts":               extData.Belts,
		"beltHeads":           extData.BeltHeads,
		"pipes":               extData.Pipes,
		"pipePumps":           extData.PipePumps,
		"lifts":               extData.Lifts,
		"powerLines":          extData.PowerLines,
		"hypertubes":          extData.Hypertubes,
		"elevators":           extData.Elevators,
		"vehiclePaths":        extData.VehiclePaths,
		"rails":               extData.Rails,
		"railComponents":      extData.RailComponents,
		"trains":              extData.Trains,
		"drones":              extData.Drones,
		"vehicles":            extData.Vehicles,
		"mergers":             extData.Mergers,
		"splitters":           extData.Splitters,
		"smartSplitters":      extData.SmartSplitters,
		"programmableSplitters": extData.ProgrammableSplitters,
		"powerPoles":          extData.PowerPoles,
		"hypertubeJunctions":  extData.HypertubeJunctions,
		"liftFloorHoles":      extData.LiftFloorHoles,
		"beltWallHoles":       extData.BeltWallHoles,
		"pipeFloorHoles":      extData.PipeFloorHoles,
		"itemsInTransit":      extData.ItemsInTransit,
		"others":              extData.Others,
		"elevatorComponents":  extData.ElevatorComponents,
	}
	bw.WriteString(`,"production":`)
	if data, err := json.Marshal(productionOut); err != nil {
		return fmt.Errorf("marshalling production data: %w", err)
	} else {
		bw.Write(data)
	}

	// Power grid data
	bw.WriteString(`,"powerGridData":`)
	if data, err := json.Marshal(extData.PowerGrid); err != nil {
		return fmt.Errorf("marshalling power grid data: %w", err)
	} else {
		bw.Write(data)
	}

	// Buildings summary (type -> count)
	bw.WriteString(`,"buildings":`)
	if data, err := json.Marshal(extData.Buildings); err != nil {
		return fmt.Errorf("marshalling buildings: %w", err)
	} else {
		bw.Write(data)
	}

	// Blueprints summary (type -> count)
	bw.WriteString(`,"blueprints":`)
	if data, err := json.Marshal(extData.Blueprints); err != nil {
		return fmt.Errorf("marshalling blueprints: %w", err)
	} else {
		bw.Write(data)
	}

	// Pets summary
	bw.WriteString(`,"pets":`)
	if data, err := json.Marshal(extData.Pets); err != nil {
		return fmt.Errorf("marshalling pets: %w", err)
	} else {
		bw.Write(data)
	}

	// Game progression
	bw.WriteString(`,"gameProgression":`)
	if data, err := json.Marshal(gameProgression); err != nil {
		return fmt.Errorf("marshalling game progression: %w", err)
	} else {
		bw.Write(data)
	}

	// Analytics (DEEP mode)
	var analyticsResult interface{}
	if mode == "DEEP" {
		printStep("Calculating analytics...")

		// Build resolved inventories for analytics
		resolvedInventories := extraction.BuildResolvedInventories(postData.BuildingInventories, extData.InventoryStacks)

		analyticsInput := extraction.AnalyticsInput{
			Buildings:           extData.Buildings,
			ClockSpeeds:         postData.ClockSpeeds,
			ProductionBoosts:    postData.ProductionBoosts,
			BuildingTypes:       postData.BuildingTypes,
			BuildingRecipes:     postData.BuildingRecipes,
			BuildingInventories: postData.BuildingInventories,
			BuildingConnections: postData.BuildingConnections,
			InventoryContents:   postData.InventoryContents,
			PowerConsumption:    postData.PowerConsumption,
			GeneratorPower:      postData.GeneratorPower,
			NuclearWaste:        postData.NuclearWaste,
			APABuildingCount:    postData.APABuildingCount,
			APAFueledCount:      postData.APAFueledCount,
			MinerPurities:       postData.MinerPurities,
			MinerResources:      postData.MinerResources,
			GeothermalPurities:  postData.GeothermalPurities,
			FrackingFluidTypes:  postData.FrackingFluidTypes,
			FrackingPurities:    postData.FrackingPurities,
			Collectibles:        postData.Collectibles,
			CollectiblesFound:   extData.CollectiblesFound,
			CollectedCollectibles: extData.CollectedCollectibles,
			Pickups:             extData.Pickups,
			GameProgression:     gameProgression,
			SaveVersion:         int(header.SaveVersion),
			Belts:               extData.Belts,
			BeltHeads:           extData.BeltHeads,
			Lifts:               extData.Lifts,
			Pipes:               extData.Pipes,
			PowerLines:          extData.PowerLines,
			Hypertubes:          extData.Hypertubes,
			Elevators:           extData.Elevators,
			VehiclePaths:        extData.VehiclePaths,
			Rails:               extData.Rails,
			Trains:              extData.Trains,
			Drones:              extData.Drones,
			Vehicles:            extData.Vehicles,
			Structures:          extData.Structures,
			ResolvedInventories: resolvedInventories,
			RetainedObjects:     extData.RetainedObjects,
			BuildingFacts:       extData.BuildingFacts,
		}

		analyticsResult = extraction.CalculateAnalytics(analyticsInput)
		extData.InventoryStacks = nil
		bw.WriteString(`,"analytics":`)
		if data, err := json.Marshal(analyticsResult); err != nil {
			return fmt.Errorf("marshalling analytics: %w", err)
		} else {
			bw.Write(data)
		}
	}

	// Level summaries
	bw.WriteString(`,"levels":[`)
	for i, lvl := range levels {
		if i > 0 {
			bw.WriteByte(',')
		}
		lvlSummary := map[string]interface{}{
			"name":              lvl.Name,
			"isPersistent":      lvl.IsPersistent,
			"objectHeaderCount": lvl.ObjectHeaderCount,
			"dataBlobLength":    lvl.DataBlobLength,
		}
		if data, err := json.Marshal(lvlSummary); err != nil {
			return err
		} else {
			bw.Write(data)
		}
	}
	bw.WriteString("]}")

	if err := bw.Flush(); err != nil {
		return fmt.Errorf("flushing JSON: %w", err)
	}

	// Generate interactive HTML report
	printStep("Generating HTML report...")
	htmlFile := outputPrefix + "_" + modeLower + ".html"

	// Build level summaries for report
	levelSummaries := make([]map[string]interface{}, 0, len(levels))
	for _, lvl := range levels {
		levelSummaries = append(levelSummaries, map[string]interface{}{
			"name":              lvl.Name,
			"isPersistent":      lvl.IsPersistent,
			"objectHeaderCount": lvl.ObjectHeaderCount,
			"dataBlobLength":    lvl.DataBlobLength,
		})
	}

	// Build header map for report
	headerMap := map[string]interface{}{
		"SaveName":           header.SaveName,
		"SessionName":        header.SessionName,
		"SaveVersion":        header.SaveVersion,
		"BuildVersion":       header.BuildVersion,
		"MapName":            header.MapName,
		"MapOptions":         header.MapOptions,
		"SaveDateTime":       header.SaveDateTime,
		"SaveHeaderType":     header.SaveHeaderType,
		"IsModdedSave":       header.IsModdedSave,
		"PlayDurationSeconds": header.PlayDurationSeconds,
	}

	report := extraction.ReportData{
		Header:          headerMap,
		Mode:            mode,
		TotalObjects:    totalObjects,
		TotalBuildings:  totalBuildings,
		ObjectsWritten:  objectsWritten,
		Buildings:       extData.Buildings,
		Blueprints:      extData.Blueprints,
		Pets:            extData.Pets,
		Extraction:      postData,
		Structures:      extData.Structures,
		Production:      productionOut,
		PowerGridData:   extData.PowerGrid,
		GameProgression: gameProgression,
		Analytics:       nil,
		Levels:          levelSummaries,
		SignDisplayNames: extraction.GetSignDisplayNames(),
	}

	// Include analytics if available
	report.Analytics = analyticsResult

	if err := extraction.GenerateHTML(report, htmlFile); err != nil {
		printWarnf("HTML generation failed: %v", err)
	} else {
		htmlStat, _ := os.Stat(htmlFile)
		printDonef("HTML report: %s (%.1f KB)", htmlFile, float64(htmlStat.Size())/1024)
	}
	printSectionFooter()

	// Print summary
	elapsed := time.Since(startTime)
	fmt.Println()
	printSectionHeader("Summary")
	printSummaryLine("Levels:", fmt.Sprintf("%d", len(levels)))
	printSummaryLine("Total objects:", fmt.Sprintf("%d", totalObjects))
	printSummaryLine("Actors:", fmt.Sprintf("%d", totalActors))
	printSummaryLine("With properties:", fmt.Sprintf("%d", totalObjectsWithProps))
	printSummaryLine("Buildings:", fmt.Sprintf("%d", totalBuildings))
	printSummaryLine("Written to JSON:", fmt.Sprintf("%d", objectsWritten))
	printSummaryLine("Retained objects:", fmt.Sprintf("%d", len(extData.RetainedObjects)))
	printSummaryLine("Clock speeds:", fmt.Sprintf("%d", len(postData.ClockSpeeds)))
	printSummaryLine("Recipes:", fmt.Sprintf("%d", len(postData.BuildingRecipes)))
	printSummaryLinef("Generators:", "%d (%d active)", postData.GeneratorPower.TotalGenerators, postData.GeneratorPower.ActiveGenerators)
	printSummaryLine("Nuclear plants:", fmt.Sprintf("%d", len(postData.NuclearWaste.Plants)))

	// Blueprints summary
	bpTotal := 0
	for _, count := range extData.Blueprints {
		bpTotal += count
	}
	if bpTotal > 0 {
		printSummaryLinef("Blueprints:", "%d (%d types)", bpTotal, len(extData.Blueprints))
	}

	// Pets summary
	if extData.Pets.TamedDoggos > 0 {
		printSummaryLinef("Pets:", "%d tamed Lizard Doggos", extData.Pets.TamedDoggos)
	}

	// Collectibles summary
	if extraction.IsCollectiblesWorldLoaded() {
		cc := extData.CollectedCollectibles
		cf := extData.CollectiblesFound
		blueTotal, yellowTotal, purpleTotal, sloopTotal, sphereTotal, crashTotal := extraction.GetCollectiblesWorldTotals()
		blueCol, yellowCol, purpleCol := len(cc.PowerSlugsBlue), len(cc.PowerSlugsYellow), len(cc.PowerSlugsPurple)
		slugTotal := blueTotal + yellowTotal + purpleTotal
		slugCol := blueCol + yellowCol + purpleCol
		sloopCol, sphereCol := len(cc.Somersloops), len(cc.MercerSpheres)
		crashDismantled := len(cc.CrashSites)
		crashOpened, crashUnopened := len(cf.CrashSiteOpened), len(cf.CrashSiteUnopened)
		crashCollected := crashOpened + crashDismantled

		printSubHeader("Collectibles")
		printSummaryLinef("Power Slugs:", "%d of %d collected", slugCol, slugTotal)
		printDetailf("Blue: %d of %d", blueCol, blueTotal)
		printDetailf("Yellow: %d of %d", yellowCol, yellowTotal)
		printDetailf("Purple: %d of %d", purpleCol, purpleTotal)
		printSummaryLinef("Somersloops:", "%d of %d", sloopCol, sloopTotal)
		printSummaryLinef("Mercer Spheres:", "%d of %d", sphereCol, sphereTotal)
		printSummaryLinef("Crash Sites:", "%d of %d", crashCollected, crashTotal)
		printDetailf("Crash: %d opened, %d unopened, %d dismantled", crashOpened, crashUnopened, crashDismantled)
	}

	printSummaryLine("Time:", wrap(cGreen, elapsed.String()))
	printSectionFooter()

	// Force a GC first so the memory numbers reflect what's actually live.
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	fmt.Println()
	fmt.Printf("  %s\n", wrap(cDim, fmt.Sprintf("Memory: %.1f MB alloc · %.1f MB sys · %d GCs",
		float64(m.Alloc)/1024/1024, float64(m.Sys)/1024/1024, m.NumGC)))

	fmt.Printf("  %s %s\n", wrap(cGreen, "Total time:"), wrap(cBold, time.Since(startTime).String()))

	// Drop the big structures before we sit waiting on user input.
	extData.RetainedObjects = nil
	extData.RetainedObjectMap = nil
	postData = nil
	analyticsResult = nil
	report = extraction.ReportData{}
	runtime.GC()
	runtime.ReadMemStats(&m)
	fmt.Printf("  %s\n", wrap(cDim, fmt.Sprintf("After cleanup: %.1f MB alloc · %.1f MB sys",
		float64(m.Alloc)/1024/1024, float64(m.Sys)/1024/1024)))

	return nil
}

// sliceReader lets us treat a []byte as an io.ReadSeeker.
type sliceReader struct {
	data []byte
	pos  int
}

func (s *sliceReader) Read(p []byte) (int, error) {
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	n := copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}

func (s *sliceReader) Seek(offset int64, whence int) (int64, error) {
	var newPos int
	switch whence {
	case io.SeekStart:
		newPos = int(offset)
	case io.SeekCurrent:
		newPos = s.pos + int(offset)
	case io.SeekEnd:
		newPos = len(s.data) + int(offset)
	}
	if newPos < 0 || newPos > len(s.data) {
		return 0, fmt.Errorf("seek out of bounds: %d", newPos)
	}
	s.pos = newPos
	return int64(newPos), nil
}
