package editor

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"satisfacts/parser"
)

// indexAll returns the starting offset of every occurrence of needle in
// haystack (including overlapping ones, matching the byte-by-byte scans this
// replaces), using bytes.Index's faster search instead of a naive
// byte-by-byte comparison loop over the whole body on every call.
func indexAll(haystack, needle []byte) []int {
	var offsets []int
	start := 0
	for {
		idx := bytes.Index(haystack[start:], needle)
		if idx < 0 {
			break
		}
		offsets = append(offsets, start+idx)
		start += idx + 1
	}
	return offsets
}

// BeltType identifies which kind of conveyor we're dealing with.
type BeltType string

const (
	BeltTypeBelt       BeltType = "ConveyorBeltMk"
	BeltTypeLift       BeltType = "ConveyorLiftMk"
	BeltTypePipe       BeltType = "ConveyorPipelineMk"
	BeltTypeModdedBelt BeltType = "ModdedBelt"
	BeltTypeModdedLift BeltType = "ModdedLift"
	BeltTypeModdedPipe BeltType = "ModdedPipe"
)

// BeltInventoryEntry counts how many belts/lifts/pipes exist at each Mk tier.
// For vanilla items, Type is BeltTypeBelt/Lift/Pipe and Mk is the tier number.
// For modded items, Type is BeltTypeModded*, Mk is 0, and ClassName holds the
// full class name so the frontend can display it.
type BeltInventoryEntry struct {
	Type      BeltType `json:"type"`
	Mk        int      `json:"mk"`
	Count     int      `json:"count"`
	ClassName string   `json:"className,omitempty"`
}

// BeltUpgradeResult reports what was upgraded.
type BeltUpgradeResult struct {
	Upgraded int `json:"upgraded"`
	Patches  int `json:"patches"`
}

// HUB tier -> max unlocked Mk for each type.
// Belts: Mk1 at start, Mk2 at tier 2, Mk3 at tier 4, Mk4 at tier 5, Mk5 at tier 7, Mk6 at tier 9.
// Lifts: Mk1 at tier 1, Mk2 at tier 2, Mk3 at tier 4, Mk4 at tier 5, Mk5 at tier 7, Mk6 at tier 9.
// Pipes: Mk1 at tier 3, Mk2 at tier 6.
var tierToMaxBeltMk = map[int]int{
	0: 1, 1: 1, 2: 2, 3: 2, 4: 3, 5: 4, 6: 4, 7: 5, 8: 5, 9: 6,
}
var tierToMaxLiftMk = map[int]int{
	0: 0, 1: 1, 2: 2, 3: 2, 4: 3, 5: 4, 6: 4, 7: 5, 8: 5, 9: 6,
}
var tierToMaxPipeMk = map[int]int{
	0: 0, 1: 0, 2: 0, 3: 1, 4: 1, 5: 1, 6: 2, 7: 2, 8: 2, 9: 2,
}

// maxKnownTier is the highest HUB tier the tables above cover.
const maxKnownTier = 9

// MaxUnlockedMk returns the highest unlocked Mk for each belt type, given the HUB tier.
// Tiers past the end of the tables clamp to the last known one - a straight map
// lookup would return 0 there and report everything as still locked.
func MaxUnlockedMk(hubTier int) (beltMk, liftMk, pipeMk int) {
	if hubTier < 0 {
		hubTier = 0
	}
	if hubTier > maxKnownTier {
		hubTier = maxKnownTier
	}
	beltMk = tierToMaxBeltMk[hubTier]
	liftMk = tierToMaxLiftMk[hubTier]
	pipeMk = tierToMaxPipeMk[hubTier]
	return
}

// SchematicTier maps schematic names (e.g. "1-2") to HUB tiers.
// This mirrors the map in extraction/analytics_power_prod_manuf.go.
var schematicTier = map[string]int{
	"1-1": 1, "1-2": 1, "1-3": 1,
	"2-1": 2, "2-2": 2, "2-3": 2, "2-5": 2, "3-2": 2,
	"3-1": 3, "3-3": 3, "3-4": 3, "4-2": 3,
	"4-1": 4, "4-3": 4, "4-4": 4, "4-5": 4, "5-3": 4,
	"5-1": 5, "5-4": 5, "5-5": 5, "6-1": 5, "6-2": 5,
	"5-2": 6, "6-3": 6, "6-5": 6, "6-6": 6, "6-7": 6,
	"7-1": 7, "7-2": 7, "7-3": 7, "7-5": 7, "8-3": 7,
	"7-4": 8, "8-1": 8, "8-2": 8, "8-4": 8, "8-5": 8,
	"9-1": 9, "9-2": 9, "9-3": 9, "9-4": 9, "9-5": 9,
}

// DetectHubTier scans the decompressed body for purchased schematic strings
// and returns the highest HUB tier found. This is a lightweight scan that
// searches for "Schematic_X-Y" patterns in the raw bytes.
func DetectHubTier(decompressed []byte) int {
	maxTier := 0
	// Search for "Schematic_" followed by X-Y pattern
	needle := []byte("Schematic_")
	searchStart := 0
	for {
		idx := bytes.Index(decompressed[searchStart:], needle)
		if idx < 0 {
			break
		}
		i := searchStart + idx
		// Read the X-Y pattern after "Schematic_"
		// Pattern: digit, dash, digit
		j := i + len(needle)
		if j+2 < len(decompressed) &&
			decompressed[j] >= '0' && decompressed[j] <= '9' &&
			decompressed[j+1] == '-' &&
			decompressed[j+2] >= '0' && decompressed[j+2] <= '9' {
			schematic := fmt.Sprintf("%c-%c", decompressed[j], decompressed[j+2])
			if tier, ok := schematicTier[schematic]; ok && tier > maxTier {
				maxTier = tier
			}
		}
		searchStart = i + len(needle)
	}
	return maxTier
}

// ScanBeltInventory scans all levels for conveyor belts, lifts, and pipelines,
// returning counts per type and Mk tier. Modded belts/lifts/pipes that don't
// match vanilla class name patterns are detected and returned as separate
// entries with their full class name so the frontend can warn the user.
func ScanBeltInventory(decompressed []byte, ctx *parser.VersionContext) ([]BeltInventoryEntry, error) {
	_, levels, _, _, err := ParseBodyLevels(decompressed, ctx)
	if err != nil {
		return nil, fmt.Errorf("parsing body: %w", err)
	}

	counts := map[BeltType]map[int]int{
		BeltTypeBelt: {},
		BeltTypeLift: {},
		BeltTypePipe: {},
	}
	moddedCounts := map[BeltType]map[string]int{
		BeltTypeModdedBelt: {},
		BeltTypeModdedLift: {},
		BeltTypeModdedPipe: {},
	}

	for _, level := range levels {
		if level.ObjectCount == 0 {
			continue
		}

		tocData := decompressed[level.TocDataStart:level.TocDataEnd]
		tocReader := parser.NewBinaryReader(tocData[4:])
		headers, err := parser.ParseObjectHeaders(tocReader, ctx, int(level.ObjectCount))
		if err != nil {
			continue
		}

		for _, h := range headers {
			className := h.ClassName
			lowerName := strings.ToLower(className)

			// Skip non-buildable conveyor/pipeline items (supports, connections, etc.)
			if isBeltPipeSupportItem(lowerName) {
				continue
			}

			matched := false

			// Check belts and lifts (ConveyorBeltMk<N>, ConveyorLiftMk<N>)
			for _, bt := range []BeltType{BeltTypeBelt, BeltTypeLift} {
				if !strings.Contains(className, string(bt)) {
					continue
				}
				mkIdx := strings.Index(className, "Mk")
				if mkIdx < 0 || mkIdx+2 >= len(className) {
					continue
				}
				mkChar := className[mkIdx+2]
				if mkChar < '1' || mkChar > '6' {
					continue
				}
				mk := int(mkChar - '0')
				counts[bt][mk]++
				matched = true
			}

			// Check pipes: Build_Pipeline (Mk1) or Build_PipelineMK2 (Mk2)
			// Must NOT match Build_PipelineSupport, Build_PipelineFlowIndicator, etc.
			if strings.Contains(className, "Build_PipelineMK2") {
				counts[BeltTypePipe][2]++
				matched = true
			} else if strings.Contains(className, "Build_Pipeline.") ||
				strings.Contains(className, "Build_Pipeline_NoIndicator.") {
				counts[BeltTypePipe][1]++
				matched = true
			}

			// Detect modded belts/lifts/pipes that weren't matched above.
			// These are conveyor/pipeline buildables that don't follow vanilla
			// class name patterns (e.g. modded Mk3+ pipes, custom belts).
			if !matched {
				if isModdedConveyorBelt(lowerName) {
					moddedCounts[BeltTypeModdedBelt][className]++
				} else if isModdedConveyorLift(lowerName) {
					moddedCounts[BeltTypeModdedLift][className]++
				} else if isModdedPipeline(lowerName) {
					moddedCounts[BeltTypeModdedPipe][className]++
				}
			}
		}
	}

	var result []BeltInventoryEntry
	for _, bt := range []BeltType{BeltTypeBelt, BeltTypeLift, BeltTypePipe} {
		for mk := 1; mk <= 6; mk++ {
			if counts[bt][mk] > 0 {
				result = append(result, BeltInventoryEntry{
					Type:  bt,
					Mk:    mk,
					Count: counts[bt][mk],
				})
			}
		}
	}
	// Append modded entries (sorted by count descending)
	for _, bt := range []BeltType{BeltTypeModdedBelt, BeltTypeModdedLift, BeltTypeModdedPipe} {
		type moddedEntry struct {
			name  string
			count int
		}
		var entries []moddedEntry
		for name, count := range moddedCounts[bt] {
			entries = append(entries, moddedEntry{name, count})
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].count > entries[j].count })
		for _, e := range entries {
			result = append(result, BeltInventoryEntry{
				Type:      bt,
				Mk:        0,
				Count:     e.count,
				ClassName: e.name,
			})
		}
	}
	return result, nil
}

// isBeltPipeSupportItem returns true for conveyor/pipeline items that are
// supports, connections, passthroughs, or other non-belt/lift/pipe buildables.
func isBeltPipeSupportItem(lower string) bool {
	return strings.Contains(lower, "support") ||
		strings.Contains(lower, "pole") ||
		strings.Contains(lower, "ceiling") ||
		strings.Contains(lower, "wall") ||
		strings.Contains(lower, "passthrough") ||
		strings.Contains(lower, "stackable") ||
		strings.Contains(lower, "valve") ||
		strings.Contains(lower, "junction") ||
		strings.Contains(lower, "connection") ||
		strings.Contains(lower, "flowindicator") ||
		strings.Contains(lower, "pump") ||
		strings.Contains(lower, "buffer") ||
		strings.Contains(lower, "storagetank") ||
		strings.Contains(lower, "industrialtank") ||
		strings.Contains(lower, "hypertube") ||
		strings.Contains(lower, "pipehyper") ||
		strings.Contains(lower, "chain") ||
		strings.Contains(lower, "splitter") ||
		strings.Contains(lower, "merger")
}

// isModdedConveyorBelt detects conveyor belt buildables that aren't vanilla
// ConveyorBeltMk1-6 (e.g. modded belts with different naming or higher Mk tiers).
func isModdedConveyorBelt(lower string) bool {
	if !strings.Contains(lower, "conveyorbelt") {
		return false
	}
	if strings.Contains(lower, "lift") {
		return false
	}
	// Vanilla: Build_ConveyorBeltMk<N>_C
	if strings.Contains(lower, "conveyorbeltmk") {
		// Could be Mk7+ (modded) — check if it's outside vanilla range
		mkIdx := strings.Index(lower, "conveyorbeltmk")
		rest := lower[mkIdx+len("conveyorbeltmk"):]
		if len(rest) > 0 && rest[0] >= '1' && rest[0] <= '6' {
			return false // vanilla, already counted
		}
	}
	return true
}

// isModdedConveyorLift detects conveyor lift buildables that aren't vanilla
// ConveyorLiftMk1-6.
func isModdedConveyorLift(lower string) bool {
	if !strings.Contains(lower, "conveyorlift") {
		return false
	}
	if strings.Contains(lower, "conveyorliftmk") {
		mkIdx := strings.Index(lower, "conveyorliftmk")
		rest := lower[mkIdx+len("conveyorliftmk"):]
		if len(rest) > 0 && rest[0] >= '1' && rest[0] <= '6' {
			return false // vanilla, already counted
		}
	}
	return true
}

// isModdedPipeline detects pipeline buildables that aren't vanilla Mk1/Mk2.
// This catches modded Mk3+ pipes and other custom pipeline types.
func isModdedPipeline(lower string) bool {
	if !strings.Contains(lower, "pipeline") && !strings.Contains(lower, "build_pipe") {
		return false
	}
	// Vanilla Mk1: Build_Pipeline. or Build_Pipeline_NoIndicator.
	// Vanilla Mk2: Build_PipelineMK2 or Build_PipelineMK2_NoIndicator
	if strings.Contains(lower, "build_pipeline.") ||
		strings.Contains(lower, "build_pipeline_noindicator.") ||
		strings.Contains(lower, "build_pipelinemk2") {
		return false
	}
	return true
}

// UpgradeBeltsOptions controls which types to upgrade and the target Mk.
type UpgradeBeltsOptions struct {
	UpgradeBelts bool
	UpgradeLifts bool
	UpgradePipes bool
	TargetMk     int // target Mk for belts and lifts
	TargetPipeMk int // target Mk for pipes (max 2 in Satisfactory)
}

// UpgradeBelts patches all conveyor belts, lifts, and/or pipelines in the
// decompressed body to the target Mk tier. It works by scanning for
// "ConveyorBeltMk<N>", "ConveyorLiftMk<N>", "ConveyorPipelineMk<N>" patterns
// and replacing the Mk digit in-place. Since all Mk strings are the same
// length, no offsets or sizes change.
//
// Processing order: highest source Mk first (e.g. Mk5->target before Mk4->target)
// to avoid double-patching.
//
// Before patching, path name collisions are detected and resolved: if two
// belts of different Mk tiers share the same object ID, upgrading both to the
// same target Mk would create duplicate path names. One belt's ID is
// re-assigned to an unused value (same digit count) to prevent this.
func UpgradeBelts(sd *SaveData, opts UpgradeBeltsOptions) (*BeltUpgradeResult, error) {
	// UpgradeBeltsSave is a public Wails-bound method, reachable directly via
	// devtools/IPC bypassing the UI's own dropdown constraints. TargetMk ends
	// up patched byte-for-byte into every matching class name string (see
	// targetMk below), so an out-of-range value would silently corrupt those
	// strings instead of failing cleanly.
	if (opts.UpgradeBelts || opts.UpgradeLifts) && (opts.TargetMk < 1 || opts.TargetMk > 6) {
		return nil, fmt.Errorf("invalid target belt/lift Mk: %d (must be 1-6)", opts.TargetMk)
	}

	result := &BeltUpgradeResult{}

	// Step 1: Detect and resolve path name collisions before Mk patching.
	resolvePathCollisions(sd, opts, result)

	// Step 2: Collect which types and source Mks to process
	type patchJob struct {
		baseName string
		oldMk    byte
		newMk    byte
	}

	var jobs []patchJob
	targetMk := byte('0' + opts.TargetMk)

	if opts.UpgradeBelts {
		for mk := 5; mk >= 1; mk-- {
			if mk >= opts.TargetMk {
				continue
			}
			jobs = append(jobs, patchJob{"ConveyorBeltMk", byte('0' + mk), targetMk})
		}
	}
	if opts.UpgradeLifts {
		for mk := 5; mk >= 1; mk-- {
			if mk >= opts.TargetMk {
				continue
			}
			jobs = append(jobs, patchJob{"ConveyorLiftMk", byte('0' + mk), targetMk})
		}
	}
	// Step 3: Upgrade pipes (Mk1->Mk2) if requested.
	// Pipes use different class name patterns (Build_Pipeline vs Build_PipelineMK2)
	// and require string length changes, so they can't use in-place digit patching.
	if opts.UpgradePipes {
		pipeCount, err := UpgradePipes(sd)
		if err != nil {
			return result, err
		}
		result.Patches += pipeCount
		result.Upgraded++
	}

	for _, job := range jobs {
		searchStr := job.baseName + string(job.oldMk)
		searchBytes := []byte(searchStr)

		for _, j := range indexAll(sd.Decompressed, searchBytes) {
			sd.Decompressed[j+len(searchBytes)-1] = job.newMk
			result.Patches++
		}
		result.Upgraded++
	}

	return result, nil
}

// resolvePathCollisions detects and fixes path name collisions that would
// occur when upgrading belts/lifts/pipes to the same target Mk. If two objects
// of different Mk tiers share the same numeric object ID (e.g.,
// Build_ConveyorBeltMk3_C_2144330431 and Build_ConveyorBeltMk4_C_2144330431),
// upgrading both to Mk6 would create duplicate path names. This function
// re-assigns one of each colliding pair to an unused ID (same digit count).
func resolvePathCollisions(sd *SaveData, opts UpgradeBeltsOptions, result *BeltUpgradeResult) {
	body := sd.Decompressed

	// Determine which base names and source Mks will be patched.
	type baseMk struct {
		base string
		mk   int
	}
	var toPatch []baseMk
	if opts.UpgradeBelts {
		for mk := 1; mk <= 5; mk++ {
			if mk < opts.TargetMk {
				toPatch = append(toPatch, baseMk{"ConveyorBeltMk", mk})
			}
		}
	}
	if opts.UpgradeLifts {
		for mk := 1; mk <= 5; mk++ {
			if mk < opts.TargetMk {
				toPatch = append(toPatch, baseMk{"ConveyorLiftMk", mk})
			}
		}
	}
	// Note: Pipe collisions are handled by UpgradePipes, not here.
	// Pipes use different class name patterns and can't collide with belts/lifts.
	if len(toPatch) == 0 {
		return
	}

	// Collect all path name strings that contain our target base names.
	// We need to find occurrences of e.g. "Build_ConveyorBeltMk3_C_" in the body
	// and extract the numeric ID suffix.
	type beltRef struct {
		offset   int    // offset of the base name in the body
		base     string // e.g. "ConveyorBeltMk"
		mk       int    // source Mk
		id       string // numeric ID string (e.g. "2144330431")
		idStart  int    // offset where the ID digits begin
		idLen    int    // number of ID digits
		atTarget bool   // already at the target Mk, so it won't be patched
	}

	// Also collect all existing IDs for each base (including target Mk and
	// non-upgraded Mks) so we can find unused replacement IDs.
	usedIDs := map[string]map[string]bool{} // base -> set of IDs
	for _, base := range []string{"ConveyorBeltMk", "ConveyorLiftMk"} {
		usedIDs[base] = map[string]bool{}
	}

	// Scan for all "Build_<base><mk>_C_<id>" patterns to collect IDs.
	// We look for the pattern "Build_ConveyorBeltMk" etc.
	for _, base := range []string{"ConveyorBeltMk", "ConveyorLiftMk"} {
		searchStr := "Build_" + base
		searchBytes := []byte(searchStr)
		for _, i := range indexAll(body, searchBytes) {
			// After "Build_ConveyorBeltMk" we expect a digit (Mk), then "_C_"
			mkPos := i + len(searchBytes)
			if mkPos >= len(body) || body[mkPos] < '1' || body[mkPos] > '6' {
				continue
			}
			// Check for "_C_" after the Mk digit
			afterMk := mkPos + 1
			if afterMk+2 >= len(body) || body[afterMk] != '_' || body[afterMk+1] != 'C' || body[afterMk+2] != '_' {
				continue
			}
			// Extract the numeric ID after "_C_"
			idStart := afterMk + 3
			idEnd := idStart
			for idEnd < len(body) && body[idEnd] >= '0' && body[idEnd] <= '9' {
				idEnd++
			}
			if idEnd == idStart {
				continue
			}
			id := string(body[idStart:idEnd])
			usedIDs[base][id] = true
		}
	}

	// Collect refs for belts that WILL be patched (source Mk < target), plus the
	// ones ALREADY at the target Mk. The latter can't collide with each other, but
	// an upgraded belt landing on an existing target-Mk belt's ID is just as fatal,
	// and leaving them out of the scan hid that case entirely.
	scanList := make([]baseMk, 0, len(toPatch)+2)
	scanList = append(scanList, toPatch...)
	seenBases := map[string]bool{}
	for _, pm := range toPatch {
		if seenBases[pm.base] {
			continue
		}
		seenBases[pm.base] = true
		scanList = append(scanList, baseMk{pm.base, opts.TargetMk})
	}

	var refs []beltRef
	for _, pm := range scanList {
		searchStr := "Build_" + pm.base + strconv.Itoa(pm.mk)
		searchBytes := []byte(searchStr)
		for _, i := range indexAll(body, searchBytes) {
			mkPos := i + len(searchBytes)
			if mkPos >= len(body) {
				continue
			}
			// Verify "_C_" follows
			afterMk := mkPos
			if afterMk+2 >= len(body) || body[afterMk] != '_' || body[afterMk+1] != 'C' || body[afterMk+2] != '_' {
				continue
			}
			idStart := afterMk + 3
			idEnd := idStart
			for idEnd < len(body) && body[idEnd] >= '0' && body[idEnd] <= '9' {
				idEnd++
			}
			if idEnd == idStart {
				continue
			}
			id := string(body[idStart:idEnd])
			refs = append(refs, beltRef{
				offset:   i,
				base:     pm.base,
				mk:       pm.mk,
				id:       id,
				idStart:  idStart,
				idLen:    idEnd - idStart,
				atTarget: pm.mk == opts.TargetMk,
			})
		}
	}

	// Group refs by (base, id) to find collisions.
	// A collision is when two refs have the same base+id but DIFFERENT source Mk.
	// Multiple refs with the same base+id+mk are just cross-references to the
	// same belt, not a collision.
	type key struct {
		base string
		id   string
	}
	groups := map[key][]beltRef{}
	for _, r := range refs {
		k := key{r.base, r.id}
		groups[k] = append(groups[k], r)
	}

	// For each collision, we need to re-assign one belt's ID to an unused value.
	// The new ID must have the same number of digits to avoid changing string lengths.
	for _, group := range groups {
		// Check if this group has refs with different Mk values (actual collision)
		seenMks := map[int]bool{}
		hasTargetMk := false
		for _, r := range group {
			seenMks[r.mk] = true
			if r.atTarget {
				hasTargetMk = true
			}
		}
		if len(seenMks) < 2 {
			continue // not a collision, just multiple references to same belt
		}

		// One belt gets to keep the ID; the rest are re-assigned. A belt already at
		// the target Mk isn't being patched at all, so it has to be the keeper -
		// otherwise pick the lowest source Mk, arbitrarily but consistently.
		keeper := 999
		for mk := range seenMks {
			if mk < keeper {
				keeper = mk
			}
		}
		if hasTargetMk {
			keeper = opts.TargetMk
		}

		// Re-assign every other belt in the group to a fresh ID
		for mkToReassign := range seenMks {
			if mkToReassign == keeper {
				continue
			}

			// Find one representative ref for this Mk to get id and idLen
			var rep *beltRef
			for i := range group {
				if group[i].mk == mkToReassign {
					rep = &group[i]
					break
				}
			}
			if rep == nil {
				continue
			}

			newID := findUnusedID(usedIDs[rep.base], rep.idLen)
			if newID == "" {
				continue
			}
			// Patch the ID digits in the body at all occurrences of this belt's
			// path name. We must include the Mk digit in the search pattern to
			// avoid patching the OTHER belt in the collision pair (which shares
			// the same ID but has a different Mk digit).
			// Pattern: "Mk<digit>_C_<oldID>" -> "Mk<digit>_C_<newID>"
			mkChar := strconv.Itoa(rep.mk)
			oldPattern := "Mk" + mkChar + "_C_" + rep.id
			newPattern := "Mk" + mkChar + "_C_" + newID
			oldBytes := []byte(oldPattern)
			newBytes := []byte(newPattern)
			if len(oldBytes) != len(newBytes) {
				continue
			}

			// Patch all occurrences in the body
			for _, i := range indexAll(body, oldBytes) {
				copy(body[i:i+len(newBytes)], newBytes)
				result.Patches++
			}
			// Update used IDs map
			usedIDs[rep.base][newID] = true
		}
	}
}

// findUnusedID finds a numeric ID string with the given digit count that is
// not in the used set. Game object IDs are typically in the 2xxx-xxx range,
// so very high values (9xxx-xxx) are almost certainly free.
func findUnusedID(used map[string]bool, digitCount int) string {
	if digitCount <= 0 {
		return ""
	}
	// Try a small set of high-value candidates with the right digit count.
	// Game IDs are ~2 billion range; 9 billion range should be free.
	maxVal := int64(1)
	for i := 0; i < digitCount; i++ {
		maxVal *= 10
	}
	maxVal-- // e.g., 9999999999 for 10 digits

	// Try max, max-1, max-2, ... (up to 1000 candidates)
	for i := int64(0); i < 1000; i++ {
		v := maxVal - i
		id := fmt.Sprintf("%0*d", digitCount, v)
		if !used[id] {
			return id
		}
	}
	return ""
}
