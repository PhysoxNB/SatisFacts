package editor

import (
	"bytes"
	"encoding/binary"
	"math"
	"sort"
	"testing"

	"satisfacts/parser"
)

// --- Helpers for building synthetic property binary data ---

func writeFStringBytes(buf *bytes.Buffer, s string) {
	binary.Write(buf, binary.LittleEndian, int32(len(s)+1))
	buf.WriteString(s)
	buf.WriteByte(0)
}

// buildLegacyFloatProp builds a legacy-format FloatProperty with name and value.
func buildLegacyFloatProp(name string, value float32) []byte {
	var buf bytes.Buffer
	writeFStringBytes(&buf, name)
	writeFStringBytes(&buf, "FloatProperty")
	binary.Write(&buf, binary.LittleEndian, int32(4)) // propBinarySize
	binary.Write(&buf, binary.LittleEndian, int32(0)) // index
	buf.WriteByte(0)                                  // hasGuid
	binary.Write(&buf, binary.LittleEndian, math.Float32bits(value))
	return buf.Bytes()
}

// buildCompleteTagFloatProp builds a complete-tag-format FloatProperty.
func buildCompleteTagFloatProp(name string, value float32) []byte {
	var buf bytes.Buffer
	writeFStringBytes(&buf, name)
	writeFStringBytes(&buf, "FloatProperty")          // tag node name
	binary.Write(&buf, binary.LittleEndian, int32(0)) // child count
	binary.Write(&buf, binary.LittleEndian, int32(4)) // propBinarySize
	buf.WriteByte(0)                                  // propFlags
	binary.Write(&buf, binary.LittleEndian, math.Float32bits(value))
	return buf.Bytes()
}

// buildLegacyIntProp builds a legacy-format IntProperty.
func buildLegacyIntProp(name string, value int32) []byte {
	var buf bytes.Buffer
	writeFStringBytes(&buf, name)
	writeFStringBytes(&buf, "IntProperty")
	binary.Write(&buf, binary.LittleEndian, int32(4))
	binary.Write(&buf, binary.LittleEndian, int32(0)) // index
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, value)
	return buf.Bytes()
}

// buildCompleteTagIntProp builds a complete-tag-format IntProperty.
func buildCompleteTagIntProp(name string, value int32) []byte {
	var buf bytes.Buffer
	writeFStringBytes(&buf, name)
	writeFStringBytes(&buf, "IntProperty")
	binary.Write(&buf, binary.LittleEndian, int32(0))
	binary.Write(&buf, binary.LittleEndian, int32(4))
	buf.WriteByte(0)
	binary.Write(&buf, binary.LittleEndian, value)
	return buf.Bytes()
}

// buildNoneTerminator writes the "None" FString that terminates a property list.
func buildNoneTerminator() []byte {
	var buf bytes.Buffer
	writeFStringBytes(&buf, "None")
	return buf.Bytes()
}

// buildLegacyPropList assembles a legacy property list (non-actor) from props + None.
func buildLegacyPropList(props ...[]byte) []byte {
	var buf bytes.Buffer
	for _, p := range props {
		buf.Write(p)
	}
	buf.Write(buildNoneTerminator())
	return buf.Bytes()
}

// buildCompleteTagPropList assembles a complete-tag property list (non-actor) with
// the leading complete-tag byte, props, and None.
func buildCompleteTagPropList(props ...[]byte) []byte {
	var buf bytes.Buffer
	buf.WriteByte(1) // complete-tag indicator
	for _, p := range props {
		buf.Write(p)
	}
	buf.Write(buildNoneTerminator())
	return buf.Bytes()
}

// buildActorPreamble writes a minimal actor header: parent level, parent path, 0 components.
func buildActorPreamble() []byte {
	var buf bytes.Buffer
	writeFStringBytes(&buf, "Persistent_Level")
	writeFStringBytes(&buf, "/Game/Map/Test.Test")
	binary.Write(&buf, binary.LittleEndian, int32(0)) // component count
	return buf.Bytes()
}

// --- Tests for WalkObjectProperties ---

func TestWalkObjectPropertiesLegacy(t *testing.T) {
	data := buildLegacyPropList(
		buildLegacyFloatProp("mEnergyCostMultiplier", 2.5),
		buildLegacyIntProp("mPartsCostMultiplier", 1),
		buildLegacyFloatProp("mSpacePartsCostMultiplier", 3.0),
	)

	var visited []string
	WalkObjectProperties(data, false, false, func(it PropIteration) bool {
		visited = append(visited, it.Name)
		return true
	})

	want := []string{"mEnergyCostMultiplier", "mPartsCostMultiplier", "mSpacePartsCostMultiplier"}
	if len(visited) != len(want) {
		t.Fatalf("visited %d properties, want %d: %v", len(visited), len(want), visited)
	}
	for i, name := range visited {
		if name != want[i] {
			t.Errorf("prop %d: got %q, want %q", i, name, want[i])
		}
	}
}

func TestWalkObjectPropertiesCompleteTag(t *testing.T) {
	data := buildCompleteTagPropList(
		buildCompleteTagFloatProp("mEnergyCostMultiplier", 2.5),
		buildCompleteTagIntProp("mPartsCostMultiplier", 1),
	)

	var visited []string
	WalkObjectProperties(data, false, true, func(it PropIteration) bool {
		visited = append(visited, it.Name)
		return true
	})

	want := []string{"mEnergyCostMultiplier", "mPartsCostMultiplier"}
	if len(visited) != len(want) {
		t.Fatalf("visited %d properties, want %d: %v", len(visited), len(want), visited)
	}
	for i, name := range visited {
		if name != want[i] {
			t.Errorf("prop %d: got %q, want %q", i, name, want[i])
		}
	}
}

func TestWalkObjectPropertiesStopsOnFalse(t *testing.T) {
	data := buildLegacyPropList(
		buildLegacyFloatProp("first", 1.0),
		buildLegacyFloatProp("second", 2.0),
		buildLegacyFloatProp("third", 3.0),
	)

	count := 0
	WalkObjectProperties(data, false, false, func(it PropIteration) bool {
		count++
		if it.Name == "second" {
			return false // stop
		}
		return true
	})

	if count != 2 {
		t.Errorf("visited %d properties before stopping, want 2", count)
	}
}

func TestWalkObjectPropertiesActorPreamble(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(buildActorPreamble())
	buf.Write(buildLegacyPropList(
		buildLegacyFloatProp("mTest", 42.0),
	))

	var visited []string
	WalkObjectProperties(buf.Bytes(), true, false, func(it PropIteration) bool {
		visited = append(visited, it.Name)
		return true
	})

	if len(visited) != 1 || visited[0] != "mTest" {
		t.Errorf("actor preamble: visited %v, want [mTest]", visited)
	}
}

func TestWalkObjectPropertiesEmptyList(t *testing.T) {
	// Just a None terminator, no properties.
	data := buildLegacyPropList()

	count := 0
	WalkObjectProperties(data, false, false, func(it PropIteration) bool {
		count++
		return true
	})

	if count != 0 {
		t.Errorf("empty property list visited %d properties, want 0", count)
	}
}

func TestWalkObjectPropertiesValueOffsets(t *testing.T) {
	prop1 := buildLegacyFloatProp("alpha", 1.5)
	prop2 := buildLegacyIntProp("beta", 7)
	data := buildLegacyPropList(prop1, prop2)

	type record struct {
		name       string
		valuePos   int
		binarySize int32
	}
	var records []record
	WalkObjectProperties(data, false, false, func(it PropIteration) bool {
		records = append(records, record{
			name:       it.Name,
			valuePos:   it.ValuePos,
			binarySize: it.Header.PropBinarySize,
		})
		return true
	})

	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	// Verify we can read the float value at the reported offset
	r := parser.NewBinaryReader(data)
	r.SetPosition(records[0].valuePos)
	gotFloat := r.ReadFloat32()
	if gotFloat != 1.5 {
		t.Errorf("value at offset %d = %v, want 1.5", records[0].valuePos, gotFloat)
	}
	if records[0].binarySize != 4 {
		t.Errorf("FloatProperty binarySize = %d, want 4", records[0].binarySize)
	}
	// Verify we can read the int value at the reported offset
	r.SetPosition(records[1].valuePos)
	gotInt := r.ReadInt32()
	if gotInt != 7 {
		t.Errorf("value at offset %d = %d, want 7", records[1].valuePos, gotInt)
	}
	if records[1].binarySize != 4 {
		t.Errorf("IntProperty binarySize = %d, want 4", records[1].binarySize)
	}
}

// --- Tests for bytesReplaceAll ---

func TestBytesReplaceAll(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		old        []byte
		new        []byte
		wantCount  int
		wantResult []byte
	}{
		{
			name:       "basic replacement",
			data:       []byte("hello world hello"),
			old:        []byte("hello"),
			new:        []byte("HELLO"),
			wantCount:  2,
			wantResult: []byte("HELLO world HELLO"),
		},
		{
			name:       "no match",
			data:       []byte("nothing here"),
			old:        []byte("xyz"),
			new:        []byte("ABC"),
			wantCount:  0,
			wantResult: []byte("nothing here"),
		},
		{
			name:       "empty data",
			data:       []byte{},
			old:        []byte("x"),
			new:        []byte("y"),
			wantCount:  0,
			wantResult: []byte{},
		},
		{
			name:       "empty old pattern",
			data:       []byte("abc"),
			old:        []byte{},
			new:        []byte("x"),
			wantCount:  0,
			wantResult: []byte("abc"),
		},
		{
			name:       "length mismatch",
			data:       []byte("abc"),
			old:        []byte("b"),
			new:        []byte("xy"),
			wantCount:  0,
			wantResult: []byte("abc"),
		},
		{
			name:       "adjacent matches",
			data:       []byte("aaa"),
			old:        []byte("a"),
			new:        []byte("b"),
			wantCount:  3,
			wantResult: []byte("bbb"),
		},
		{
			name:       "overlapping pattern not double-counted",
			data:       []byte("aaaa"),
			old:        []byte("aa"),
			new:        []byte("bb"),
			wantCount:  2,
			wantResult: []byte("bbbb"),
		},
		{
			name:       "pattern longer than data",
			data:       []byte("ab"),
			old:        []byte("abc"),
			new:        []byte("xyz"),
			wantCount:  0,
			wantResult: []byte("ab"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]byte, len(tc.data))
			copy(data, tc.data)
			gotCount := bytesReplaceAll(data, tc.old, tc.new)
			if gotCount != tc.wantCount {
				t.Errorf("count = %d, want %d", gotCount, tc.wantCount)
			}
			if !bytes.Equal(data, tc.wantResult) {
				t.Errorf("result = %q, want %q", data, tc.wantResult)
			}
		})
	}
}

// --- Tests for FindFloatPropOffset ---

func TestFindFloatPropOffsetLegacy(t *testing.T) {
	data := buildLegacyPropList(
		buildLegacyIntProp("someInt", 5),
		buildLegacyFloatProp("mEnergyCostMultiplier", 2.5),
		buildLegacyFloatProp("mPartsCostMultiplier", 1.0),
	)

	offset, value := FindFloatPropOffset(data, "mEnergyCostMultiplier", false, false)
	if offset < 0 {
		t.Fatal("FloatProperty not found")
	}
	if value != 2.5 {
		t.Errorf("value = %v, want 2.5", value)
	}

	// Verify by reading at the offset
	r := parser.NewBinaryReader(data)
	r.SetPosition(offset)
	got := r.ReadFloat32()
	if got != 2.5 {
		t.Errorf("value at offset %d = %v, want 2.5", offset, got)
	}
}

func TestFindFloatPropOffsetCompleteTag(t *testing.T) {
	data := buildCompleteTagPropList(
		buildCompleteTagFloatProp("mEnergyCostMultiplier", 10.0),
		buildCompleteTagIntProp("mPartsCostMultiplier", 3),
	)

	offset, value := FindFloatPropOffset(data, "mEnergyCostMultiplier", false, true)
	if offset < 0 {
		t.Fatal("FloatProperty not found")
	}
	if value != 10.0 {
		t.Errorf("value = %v, want 10.0", value)
	}
}

func TestFindFloatPropOffsetNotFound(t *testing.T) {
	data := buildLegacyPropList(
		buildLegacyFloatProp("existing", 1.0),
	)

	offset, value := FindFloatPropOffset(data, "nonexistent", false, false)
	if offset != -1 {
		t.Errorf("offset = %d for missing property, want -1", offset)
	}
	if value != 0 {
		t.Errorf("value = %v for missing property, want 0", value)
	}
}

func TestFindFloatPropOffsetActor(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(buildActorPreamble())
	buf.Write(buildLegacyPropList(
		buildLegacyFloatProp("mTest", 99.0),
	))

	offset, value := FindFloatPropOffset(buf.Bytes(), "mTest", true, false)
	if offset < 0 {
		t.Fatal("FloatProperty not found in actor data")
	}
	if value != 99.0 {
		t.Errorf("value = %v, want 99.0", value)
	}
}

// --- Tests for findNoneTerminatorOffset ---

func TestFindNoneTerminatorOffset(t *testing.T) {
	t.Run("legacy with properties", func(t *testing.T) {
		prop1 := buildLegacyFloatProp("alpha", 1.0)
		prop2 := buildLegacyIntProp("beta", 2)
		noneBytes := buildNoneTerminator()
		data := append(append(prop1, prop2...), noneBytes...)

		offset, err := findNoneTerminatorOffset(data, false, false)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		// The offset should point at the "None" FString
		r := parser.NewBinaryReader(data)
		r.SetPosition(offset)
		got := r.ReadString()
		if got != "None" {
			t.Errorf("string at offset %d = %q, want \"None\"", offset, got)
		}
	})

	t.Run("complete-tag with properties", func(t *testing.T) {
		data := buildCompleteTagPropList(
			buildCompleteTagFloatProp("alpha", 1.0),
		)

		offset, err := findNoneTerminatorOffset(data, false, true)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		r := parser.NewBinaryReader(data)
		r.SetPosition(offset)
		got := r.ReadString()
		if got != "None" {
			t.Errorf("string at offset %d = %q, want \"None\"", offset, got)
		}
	})

	t.Run("empty property list", func(t *testing.T) {
		data := buildLegacyPropList()
		offset, err := findNoneTerminatorOffset(data, false, false)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		r := parser.NewBinaryReader(data)
		r.SetPosition(offset)
		got := r.ReadString()
		if got != "None" {
			t.Errorf("string at offset %d = %q, want \"None\"", offset, got)
		}
	})

	t.Run("actor with properties", func(t *testing.T) {
		var buf bytes.Buffer
		buf.Write(buildActorPreamble())
		buf.Write(buildLegacyPropList(
			buildLegacyFloatProp("x", 1.0),
		))
		offset, err := findNoneTerminatorOffset(buf.Bytes(), true, false)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		r := parser.NewBinaryReader(buf.Bytes())
		r.SetPosition(offset)
		got := r.ReadString()
		if got != "None" {
			t.Errorf("string at offset %d = %q, want \"None\"", offset, got)
		}
	})
}

// --- Tests for buildFloatProperty round-trip ---

func TestBuildFloatPropertyRoundTrip(t *testing.T) {
	tests := []struct {
		name           string
		propName       string
		value          float32
		useCompleteTag bool
	}{
		{"legacy format", "mEnergyCostMultiplier", 5.0, false},
		{"complete-tag format", "mEnergyCostMultiplier", 5.0, true},
		{"legacy zero", "mTest", 0.0, false},
		{"complete-tag negative", "mTest", -3.14, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			propBytes := buildFloatProperty(tc.propName, tc.value, tc.useCompleteTag)

			// Build a property list with just this property + None
			var data []byte
			if tc.useCompleteTag {
				data = append([]byte{1}, propBytes...) // complete-tag byte
			} else {
				data = propBytes
			}
			data = append(data, buildNoneTerminator()...)

			// Use FindFloatPropOffset to read it back
			offset, got := FindFloatPropOffset(data, tc.propName, false, tc.useCompleteTag)
			if offset < 0 {
				t.Fatalf("property %q not found in round-trip data", tc.propName)
			}
			if got != tc.value {
				t.Errorf("round-trip value = %v, want %v", got, tc.value)
			}
		})
	}
}

// --- Tests for replacePipeStrings ---

func TestReplacePipeStringsUpgradesMk1Class(t *testing.T) {
	mk1Class := "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline.Build_Pipeline_C"

	// Build an FString containing the Mk1 class name
	var buf bytes.Buffer
	writeFStringBytes(&buf, mk1Class)
	data := buf.Bytes()

	result := replacePipeStrings(data, nil, nil)
	if len(result) == len(data) {
		t.Error("replacePipeStrings did not change the data length (Mk2 is longer)")
	}

	// Parse the result as an FString
	r := parser.NewBinaryReader(result)
	got := r.ReadString()
	want := "/Game/FactoryGame/Buildable/Factory/PipelineMk2/Build_PipelineMK2.Build_PipelineMK2_C"
	if got != want {
		t.Errorf("class name = %q, want %q", got, want)
	}
}

func TestReplacePipeStringsPreservesMk2(t *testing.T) {
	mk2Class := "/Game/FactoryGame/Buildable/Factory/PipelineMk2/Build_PipelineMK2.Build_PipelineMK2_C"

	var buf bytes.Buffer
	writeFStringBytes(&buf, mk2Class)
	data := buf.Bytes()

	result := replacePipeStrings(data, nil, nil)
	// Should be unchanged
	r := parser.NewBinaryReader(result)
	got := r.ReadString()
	if got != mk2Class {
		t.Errorf("Mk2 class was modified: got %q, want %q", got, mk2Class)
	}
}

func TestReplacePipeStringsPreservesPathNames(t *testing.T) {
	pathName := "Build_Pipeline_C_2144330431"

	var buf bytes.Buffer
	writeFStringBytes(&buf, pathName)
	data := buf.Bytes()

	result := replacePipeStrings(data, nil, nil)
	r := parser.NewBinaryReader(result)
	got := r.ReadString()
	if got != pathName {
		t.Errorf("path name was modified: got %q, want %q", got, pathName)
	}
}

func TestReplacePipeStringsRejectsNonGamePath(t *testing.T) {
	// A string that contains "Build_Pipeline" but doesn't start with /Game/
	// — the tightened validation should reject it.
	fake := "some_prefix/Build_Pipeline.Build_Pipeline_C"

	var buf bytes.Buffer
	writeFStringBytes(&buf, fake)
	data := buf.Bytes()

	result := replacePipeStrings(data, nil, nil)
	r := parser.NewBinaryReader(result)
	got := r.ReadString()
	if got != fake {
		t.Errorf("non-/Game/ string was modified: got %q, want %q", got, fake)
	}
}

func TestReplacePipeStringsMultipleReplacements(t *testing.T) {
	mk1Class := "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline.Build_Pipeline_C"
	mk1NoInd := "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline_NoIndicator.Build_Pipeline_NoIndicator_C"

	var buf bytes.Buffer
	writeFStringBytes(&buf, mk1Class)
	writeFStringBytes(&buf, mk1NoInd)
	data := buf.Bytes()

	result := replacePipeStrings(data, nil, nil)

	r := parser.NewBinaryReader(result)
	got1 := r.ReadString()
	got2 := r.ReadString()

	if !contains(got1, "PipelineMK2") {
		t.Errorf("first string not upgraded: %q", got1)
	}
	if !contains(got2, "PipelineMK2_NoIndicator") {
		t.Errorf("second string not upgraded: %q", got2)
	}
}

// TestReplacePipeStringsWithStructuralOffsets is a regression test for the
// headerOffsets/headerSizes clamping added to guard against a heuristic
// backward scan matching an out-of-bounds coincidentally-plausible string.
// It's the shape UpgradePipes actually calls with: each string's own exact
// [start, start+size) region is passed, so the scan must still find and
// upgrade both, exactly as it does with the unclamped (nil, nil) path.
func TestReplacePipeStringsWithStructuralOffsets(t *testing.T) {
	mk1Class := "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline.Build_Pipeline_C"
	mk1NoInd := "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline_NoIndicator.Build_Pipeline_NoIndicator_C"

	var buf bytes.Buffer
	firstStart := buf.Len()
	writeFStringBytes(&buf, mk1Class)
	firstSize := buf.Len() - firstStart
	secondStart := buf.Len()
	writeFStringBytes(&buf, mk1NoInd)
	secondSize := buf.Len() - secondStart
	data := buf.Bytes()

	result := replacePipeStrings(data, []int{firstStart, secondStart}, []int{firstSize, secondSize})

	r := parser.NewBinaryReader(result)
	got1 := r.ReadString()
	got2 := r.ReadString()

	if !contains(got1, "PipelineMK2") {
		t.Errorf("first string not upgraded with structural offsets: %q", got1)
	}
	if !contains(got2, "PipelineMK2_NoIndicator") {
		t.Errorf("second string not upgraded with structural offsets: %q", got2)
	}
}

// --- Tests for sortReplacements ---

func TestSortReplacements(t *testing.T) {
	reps := []replacement{
		{lengthPos: 100, oldLen: 5, newData: []byte("c")},
		{lengthPos: 50, oldLen: 5, newData: []byte("a")},
		{lengthPos: 200, oldLen: 5, newData: []byte("d")},
		{lengthPos: 75, oldLen: 5, newData: []byte("b")},
	}

	sortReplacements(reps)

	if !sort.SliceIsSorted(reps, func(i, j int) bool {
		return reps[i].lengthPos < reps[j].lengthPos
	}) {
		t.Errorf("replacements not sorted by lengthPos: %v", reps)
	}
}

func TestSortReplacementsEmpty(t *testing.T) {
	reps := []replacement{}
	sortReplacements(reps) // must not panic
	if len(reps) != 0 {
		t.Errorf("empty sort changed length to %d", len(reps))
	}
}

func TestSortReplacementsSingle(t *testing.T) {
	reps := []replacement{{lengthPos: 42, oldLen: 3, newData: []byte("x")}}
	sortReplacements(reps)
	if reps[0].lengthPos != 42 {
		t.Errorf("single element moved: got lengthPos %d", reps[0].lengthPos)
	}
}

// --- Tests for SerializeCategoryRecords -> parseCategoryRecord round-trip ---

func TestSerializeExtractCategoryRecordsRoundTrip(t *testing.T) {
	categories := []BlueprintCategoryRecord{
		{
			CategoryName: "Production",
			IconID:       5,
			MenuPriority: 1.5,
			IsUndefined:  false,
			SubCategories: []BlueprintSubCategory{
				{
					SubCategoryName: "Belts",
					MenuPriority:    2.0,
					IsUndefined:     false,
					BlueprintNames:  []string{"BP_Belt_1", "BP_Belt_2"},
				},
				{
					SubCategoryName: "Pipes",
					MenuPriority:    3.0,
					IsUndefined:     true,
					BlueprintNames:  []string{"BP_Pipe_1"},
				},
			},
		},
		{
			CategoryName:  "Power",
			IconID:        10,
			MenuPriority:  0.5,
			IsUndefined:   true,
			SubCategories: []BlueprintSubCategory{},
		},
	}

	for _, useCompleteTag := range []bool{false, true} {
		label := "legacy"
		if useCompleteTag {
			label = "complete-tag"
		}
		t.Run(label, func(t *testing.T) {
			serialized := SerializeCategoryRecords(categories, useCompleteTag)

			// Parse it back using the same reader logic as ExtractCategoryRecords
			r := parser.NewBinaryReader(serialized)
			structCount := r.ReadInt32()

			if int(structCount) != len(categories) {
				t.Fatalf("structCount = %d, want %d", structCount, len(categories))
			}

			// Skip old-format array header
			if !useCompleteTag {
				r.ReadString() // hName
				r.ReadString() // hType
				r.ReadInt32()  // hBinSize
				r.ReadInt32()  // hIdx
				r.ReadString() // hSubtype
				r.Skip(16)
				hasGuid := r.ReadUInt8()
				if hasGuid != 0 {
					r.Skip(16)
				}
			}

			for i := int32(0); i < structCount; i++ {
				got, err := parseCategoryRecord(r, useCompleteTag)
				if err != nil {
					t.Fatalf("category %d parse error: %v", i, err)
				}
				want := categories[i]
				if got.CategoryName != want.CategoryName {
					t.Errorf("category %d: name = %q, want %q", i, got.CategoryName, want.CategoryName)
				}
				if got.IconID != want.IconID {
					t.Errorf("category %d: iconID = %d, want %d", i, got.IconID, want.IconID)
				}
				if got.MenuPriority != want.MenuPriority {
					t.Errorf("category %d: menuPriority = %v, want %v", i, got.MenuPriority, want.MenuPriority)
				}
				if got.IsUndefined != want.IsUndefined {
					t.Errorf("category %d: isUndefined = %v, want %v", i, got.IsUndefined, want.IsUndefined)
				}
				if len(got.SubCategories) != len(want.SubCategories) {
					t.Errorf("category %d: %d sub-categories, want %d", i, len(got.SubCategories), len(want.SubCategories))
					continue
				}
				for j, sub := range got.SubCategories {
					wantSub := want.SubCategories[j]
					if sub.SubCategoryName != wantSub.SubCategoryName {
						t.Errorf("cat %d sub %d: name = %q, want %q", i, j, sub.SubCategoryName, wantSub.SubCategoryName)
					}
					if sub.MenuPriority != wantSub.MenuPriority {
						t.Errorf("cat %d sub %d: priority = %v, want %v", i, j, sub.MenuPriority, wantSub.MenuPriority)
					}
					if sub.IsUndefined != wantSub.IsUndefined {
						t.Errorf("cat %d sub %d: undefined = %v, want %v", i, j, sub.IsUndefined, wantSub.IsUndefined)
					}
					if len(sub.BlueprintNames) != len(wantSub.BlueprintNames) {
						t.Errorf("cat %d sub %d: %d blueprint names, want %d", i, j, len(sub.BlueprintNames), len(wantSub.BlueprintNames))
						continue
					}
					for k, bpName := range sub.BlueprintNames {
						if bpName != wantSub.BlueprintNames[k] {
							t.Errorf("cat %d sub %d bp %d: name = %q, want %q", i, j, k, bpName, wantSub.BlueprintNames[k])
						}
					}
				}
			}
		})
	}
}

func TestSerializeCategoryRecordsEmpty(t *testing.T) {
	for _, useCompleteTag := range []bool{false, true} {
		label := "legacy"
		if useCompleteTag {
			label = "complete-tag"
		}
		t.Run(label, func(t *testing.T) {
			serialized := SerializeCategoryRecords(nil, useCompleteTag)
			r := parser.NewBinaryReader(serialized)
			count := r.ReadInt32()
			if count != 0 {
				t.Errorf("empty serialization: count = %d, want 0", count)
			}
		})
	}
}

// --- Utility ---

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
