package editor

import (
	"encoding/binary"
	"strings"
	"testing"

	"satisfacts/parser"
)

func TestCheckSessionName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		hasIssues bool
		autoFix   string
		wantTypes []string
	}{
		{name: "clean", input: "My Factory", hasIssues: false},
		{name: "empty", input: "", hasIssues: false},
		{name: "inner spaces are fine", input: "A B C", hasIssues: false},
		{name: "inner period is fine", input: "v1.2 run", hasIssues: false},
		// Square brackets are legal in Windows folder names. They used to be
		// flagged and stripped because the character set was written as a
		// regex-shaped literal and the brackets became part of the set.
		{name: "brackets are fine", input: "My Factory [1.2]", hasIssues: false},
		{name: "bare brackets are fine", input: "[]", hasIssues: false},
		{
			name: "trailing space", input: "Save ", hasIssues: true, autoFix: "Save",
			wantTypes: []string{"trailing_whitespace"},
		},
		{
			name: "leading space", input: " Save", hasIssues: true, autoFix: "Save",
			wantTypes: []string{"leading_whitespace"},
		},
		{
			name: "both ends", input: " Save ", hasIssues: true, autoFix: "Save",
			wantTypes: []string{"trailing_whitespace", "leading_whitespace"},
		},
		{
			name: "trailing tab", input: "Save\t", hasIssues: true, autoFix: "Save",
			wantTypes: []string{"trailing_whitespace"},
		},
		{
			name: "trailing period", input: "Factory.", hasIssues: true, autoFix: "Factory",
			wantTypes: []string{"trailing_period"},
		},
		{
			name: "multiple trailing periods", input: "Factory..", hasIssues: true, autoFix: "Factory",
			wantTypes: []string{"trailing_period"},
		},
		{
			name: "colon", input: "Save:1", hasIssues: true, autoFix: "Save1",
			wantTypes: []string{"invalid_chars"},
		},
		{
			name: "several invalid chars", input: `a<b>c|d"e`, hasIssues: true, autoFix: "abcde",
			wantTypes: []string{"invalid_chars"},
		},
		{
			name: "forward slash", input: "a/b", hasIssues: true, autoFix: "ab",
			wantTypes: []string{"invalid_chars"},
		},
		{
			name: "backslash", input: `a\b`, hasIssues: true, autoFix: "ab",
			wantTypes: []string{"invalid_chars"},
		},
		{
			// Trailing space hides the period, so both have to be stripped.
			name: "period then space", input: "Factory. ", hasIssues: true, autoFix: "Factory",
			wantTypes: []string{"trailing_whitespace", "trailing_period"},
		},
		{
			name: "everything at once", input: ` My:Save. `, hasIssues: true, autoFix: "MySave",
			wantTypes: []string{"trailing_whitespace", "leading_whitespace", "trailing_period", "invalid_chars"},
		},
		{
			// A newline is whitespace too, and it must be reported rather than
			// only silently trimmed.
			name: "trailing newline", input: "Save\n", hasIssues: true, autoFix: "Save",
			wantTypes: []string{"trailing_whitespace"},
		},
		{
			// Stripping the invalid character exposes a trailing space, so the
			// suggestion has to be trimmed again afterwards.
			name: "invalid char hides trailing space", input: "Save :", hasIssues: true, autoFix: "Save",
			wantTypes: []string{"invalid_chars"},
		},
		{
			name: "repeated invalid char reported once", input: "a:b:c", hasIssues: true, autoFix: "abc",
			wantTypes: []string{"invalid_chars"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CheckSessionName(tc.input)
			if got.HasIssues != tc.hasIssues {
				t.Fatalf("HasIssues = %v, want %v (issues: %+v)", got.HasIssues, tc.hasIssues, got.Issues)
			}
			if !tc.hasIssues {
				return
			}
			if got.AutoFix != tc.autoFix {
				t.Errorf("AutoFix = %q, want %q", got.AutoFix, tc.autoFix)
			}
			// The auto-fix has to actually be clean, or the UI offers a
			// suggestion that fails validation the moment it's submitted.
			if again := CheckSessionName(got.AutoFix); again.HasIssues {
				t.Errorf("AutoFix %q still has issues: %+v", got.AutoFix, again.Issues)
			}
			for _, want := range tc.wantTypes {
				found := false
				for _, iss := range got.Issues {
					if iss.Type == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing issue type %q; got %+v", want, got.Issues)
				}
			}
			if len(got.Issues) != len(tc.wantTypes) {
				t.Errorf("got %d issues, want %d: %+v", len(got.Issues), len(tc.wantTypes), got.Issues)
			}
		})
	}
}

// totalBodyRestSize excludes the size field itself, and on UE5-era saves also the
// 4-byte zero that follows it. Writing -8 for every save left pre-UE5 bodies four
// bytes short, which truncates the tail of the last data blob.
func TestTotalBodyRestSizeFor(t *testing.T) {
	body := make([]byte, 1000)

	ue5 := &parser.VersionContext{SaveVersion: parser.SaveUnrealEngine5}
	if got, want := TotalBodyRestSizeFor(body, ue5), int32(992); got != want {
		t.Errorf("UE5 save: got %d, want %d", got, want)
	}

	pre := &parser.VersionContext{SaveVersion: parser.SaveUnrealEngine5 - 1}
	if got, want := TotalBodyRestSizeFor(body, pre), int32(996); got != want {
		t.Errorf("pre-UE5 save: got %d, want %d", got, want)
	}

	// WriteTotalBodyRestSize must agree with the calculation and must not panic
	// on a body too short to hold the field.
	for _, ctx := range []*parser.VersionContext{ue5, pre} {
		buf := make([]byte, len(body))
		WriteTotalBodyRestSize(buf, ctx)
		got := int32(binary.LittleEndian.Uint32(buf[0:4]))
		if want := TotalBodyRestSizeFor(buf, ctx); got != want {
			t.Errorf("written %d, want %d", got, want)
		}
	}
	WriteTotalBodyRestSize([]byte{1, 2}, ue5)
}

func TestMaxUnlockedMk(t *testing.T) {
	// Higher tiers must never unlock less than lower ones.
	prevBelt, prevLift, prevPipe := 0, 0, 0
	for tier := 0; tier <= 10; tier++ {
		belt, lift, pipe := MaxUnlockedMk(tier)
		if belt < prevBelt || lift < prevLift || pipe < prevPipe {
			t.Errorf("tier %d regressed: belt %d->%d, lift %d->%d, pipe %d->%d",
				tier, prevBelt, belt, prevLift, lift, prevPipe, pipe)
		}
		prevBelt, prevLift, prevPipe = belt, lift, pipe
	}

	// Pipes only ever have two tiers.
	for tier := 0; tier <= 10; tier++ {
		if _, _, pipe := MaxUnlockedMk(tier); pipe > 2 {
			t.Errorf("tier %d unlocks pipe Mk%d, but pipes only go to Mk2", tier, pipe)
		}
	}

	// Belts top out at Mk6.
	if belt, _, _ := MaxUnlockedMk(10); belt > 6 {
		t.Errorf("belt Mk%d exceeds the Mk6 maximum", belt)
	}
}

// The pipe upgrade must rewrite class names but leave path names alone -
// swapping a path name is what broke every pipe-to-building connection once.
func TestPipeStringNeedsUpgrade(t *testing.T) {
	const (
		mk1Class  = "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline.Build_Pipeline_C"
		mk1NoInd  = "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline_NoIndicator.Build_Pipeline_NoIndicator_C"
		mk2Class  = "/Game/FactoryGame/Buildable/Factory/PipelineMk2/Build_PipelineMK2.Build_PipelineMK2_C"
		pathName  = "Build_Pipeline_C_2144330431"
		support   = "/Game/FactoryGame/Buildable/Factory/PipelineSupport/Build_PipelineSupport.Build_PipelineSupport_C"
		flowInd   = "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_PipelineFlowIndicator.Build_PipelineFlowIndicator_C"
		junction  = "/Game/FactoryGame/Buildable/Factory/PipeJunction/Build_PipelineJunction_Cross.Build_PipelineJunction_Cross_C"
		unrelated = "/Game/FactoryGame/Buildable/Factory/ConveyorBelt/Build_ConveyorBeltMk1.Build_ConveyorBeltMk1_C"
		pumpMk1   = "/Game/FactoryGame/Buildable/Factory/PipelinePump/Build_PipelinePump.Build_PipelinePump_C"
	)

	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"mk1 class name", mk1Class, true},
		{"mk1 no-indicator class name", mk1NoInd, true},
		{"already mk2", mk2Class, false},
		{"path name must not be touched", pathName, false},
		{"pipeline support", support, false},
		{"flow indicator", flowInd, false},
		{"junction", junction, false},
		{"pump", pumpMk1, false},
		{"unrelated belt", unrelated, false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pipeStringNeedsUpgrade(tc.in); got != tc.want {
				t.Errorf("pipeStringNeedsUpgrade(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestUpgradePipeString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "mk1 class becomes mk2",
			in:   "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline.Build_Pipeline_C",
			want: "/Game/FactoryGame/Buildable/Factory/PipelineMk2/Build_PipelineMK2.Build_PipelineMK2_C",
		},
		{
			name: "no-indicator variant",
			in:   "/Game/FactoryGame/Buildable/Factory/Pipeline/Build_Pipeline_NoIndicator.Build_Pipeline_NoIndicator_C",
			want: "/Game/FactoryGame/Buildable/Factory/PipelineMk2/Build_PipelineMK2_NoIndicator.Build_PipelineMK2_NoIndicator_C",
		},
		{
			name: "path name unchanged",
			in:   "Build_Pipeline_C_2144330431",
			want: "Build_Pipeline_C_2144330431",
		},
		{
			name: "support unchanged",
			in:   "/Game/FactoryGame/Buildable/Factory/PipelineSupport/Build_PipelineSupport.Build_PipelineSupport_C",
			want: "/Game/FactoryGame/Buildable/Factory/PipelineSupport/Build_PipelineSupport.Build_PipelineSupport_C",
		},
		{
			name: "unrelated string unchanged",
			in:   "Build_ConveyorBeltMk1_C_123",
			want: "Build_ConveyorBeltMk1_C_123",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := upgradePipeString(tc.in)
			if got != tc.want {
				t.Errorf("upgradePipeString(%q)\n got %q\nwant %q", tc.in, got, tc.want)
			}
			// Upgrading twice must be a no-op, otherwise re-running the tool
			// would keep mangling the string.
			if again := upgradePipeString(got); again != got {
				t.Errorf("not idempotent: second pass turned %q into %q", got, again)
			}
		})
	}
}

func TestFindUnusedID(t *testing.T) {
	t.Run("respects digit count", func(t *testing.T) {
		for _, digits := range []int{4, 7, 10} {
			got := findUnusedID(map[string]bool{}, digits)
			if got == "" {
				t.Errorf("no ID found for %d digits", digits)
				continue
			}
			if len(got) != digits {
				t.Errorf("findUnusedID(_, %d) = %q, which has %d digits", digits, got, len(got))
			}
			if strings.Trim(got, "0123456789") != "" {
				t.Errorf("findUnusedID returned non-numeric %q", got)
			}
		}
	})

	t.Run("avoids used IDs", func(t *testing.T) {
		used := map[string]bool{}
		// Claim several IDs in a row; each call must return something new.
		for i := 0; i < 5; i++ {
			got := findUnusedID(used, 10)
			if got == "" {
				t.Fatalf("ran out of IDs after %d allocations", i)
			}
			if used[got] {
				t.Fatalf("findUnusedID returned already-used %q", got)
			}
			used[got] = true
		}
	})

	t.Run("rejects nonsense digit counts", func(t *testing.T) {
		for _, d := range []int{0, -1} {
			if got := findUnusedID(map[string]bool{}, d); got != "" {
				t.Errorf("findUnusedID(_, %d) = %q, want empty", d, got)
			}
		}
	})
}

func TestValidatePermutation(t *testing.T) {
	tests := []struct {
		name    string
		order   []int
		n       int
		wantErr string
	}{
		{name: "identity", order: []int{0, 1, 2}, n: 3},
		{name: "reversed", order: []int{2, 1, 0}, n: 3},
		{name: "single", order: []int{0}, n: 1},
		{name: "too short", order: []int{0, 1}, n: 3, wantErr: "3 items"},
		{name: "too long", order: []int{0, 1, 2, 3}, n: 3, wantErr: "3 items"},
		{name: "duplicate", order: []int{0, 0, 1}, n: 3, wantErr: "duplicate"},
		{name: "out of range high", order: []int{0, 1, 5}, n: 3, wantErr: "out-of-range"},
		{name: "negative", order: []int{0, 1, -1}, n: 3, wantErr: "out-of-range"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validatePermutation(tc.order, tc.n)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestObjectDepths(t *testing.T) {
	hotbar := "Persistent_Level:PersistentLevel.BP_PlayerState_C_1.FGPlayerHotbar_2"
	headers := []parser.ObjectHeader{
		{ClassName: "/Script/FactoryGame.FGPlayerHotbar", Reference: parser.ObjectRef{PathName: hotbar},
			OuterPathName: "Persistent_Level:PersistentLevel.BP_PlayerState_C_1"},
		{ClassName: "/Script/FactoryGame.FGBlueprintShortcut", Reference: parser.ObjectRef{PathName: hotbar + ".FGBlueprintShortcut_3"},
			OuterPathName: hotbar},
		{ClassName: "/Game/FactoryGame/Character/Player/BP_PlayerState.BP_PlayerState_C",
			Reference: parser.ObjectRef{PathName: "Persistent_Level:PersistentLevel.BP_PlayerState_C_1"}},
	}
	depths, ok := objectDepths(headers)
	if !ok {
		t.Fatal("objectDepths rejected well-formed paths")
	}
	if !(depths[2] < depths[0] && depths[0] < depths[1]) {
		t.Errorf("depths %v: want player state < hotbar < shortcut", depths)
	}

	// An outer whose path isn't a prefix-shaped parent must disable reordering.
	headers[1].OuterPathName = hotbar + ".Weird.Nested"
	if _, ok := objectDepths(headers); ok {
		t.Error("objectDepths accepted an outer deeper than its child")
	}
}
