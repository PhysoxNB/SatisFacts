package parser

import (
	"encoding/binary"
	"math"
	"testing"
)

// TestParseRailroadVehicleSpecialRejectsGarbagePhysicsCount is a regression
// test for a bug where a corrupted/implausible physicsCount would panic
// inside the read loop, and a recover() in this function (now removed)
// silently swallowed it and returned (nil, nil) - identical to "no special
// data", with zero error signal. It must now return a clean error instead.
func TestParseRailroadVehicleSpecialRejectsGarbagePhysicsCount(t *testing.T) {
	d := NewDataBlobParser(&VersionContext{}, nil, "REPORT")

	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, uint32(int32(999999999)))
	r := NewBinaryReader(buf)

	_, err := d.parseRailroadVehicleSpecial(r, 4, 0)
	if err == nil {
		t.Fatal("expected an error for an implausible physicsCount, got nil")
	}
}

// bsBuilder builds a synthetic FGLightweightBuildableSubsystem special-props
// byte stream for TestParseBuildableSubsystemSpecialAccumulatesRepeatedType.
type bsBuilder struct{ buf []byte }

func (b *bsBuilder) i32(v int32) *bsBuilder {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], uint32(v))
	b.buf = append(b.buf, tmp[:]...)
	return b
}

func (b *bsBuilder) f64(v float64) *bsBuilder {
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], math.Float64bits(v))
	b.buf = append(b.buf, tmp[:]...)
	return b
}

func (b *bsBuilder) zeros(n int) *bsBuilder {
	b.buf = append(b.buf, make([]byte, n)...)
	return b
}

// str writes an empty FString (the test doesn't care about swatch/material/
// pattern/skin/paintFinish/recipe/blueprintProxy content, only about
// typeRefPn and the per-instance transform).
func (b *bsBuilder) emptyStr() *bsBuilder { return b.i32(0) }

// instance writes one lightweight-buildable instance record with lwVersion
// 0 (so the FGDynamicStruct/PlayerInfoHandle tails, gated on lwVersion>=2/3,
// are absent) and ObjectVersion < 48 (so no lwVersion field precedes
// entriesCount at the top level either).
func (b *bsBuilder) instance(qx, qy, qz, qw, tx, ty, tz float64) *bsBuilder {
	b.f64(qx).f64(qy).f64(qz).f64(qw).f64(tx).f64(ty).f64(tz)
	b.zeros(24) // scale x,y,z
	for i := 0; i < 8; i++ {
		b.emptyStr() // swatch/material/pattern/skin (2 strings each)
	}
	b.zeros(32) // colors
	b.emptyStr().emptyStr()
	b.zeros(1) // patternRotation
	b.emptyStr().emptyStr()
	b.emptyStr().emptyStr() // blueprintProxy level+path
	return b
}

func (b *bsBuilder) entry(typeRefPn string, instances [][7]float64) *bsBuilder {
	b.emptyStr() // typeRef levelName
	// typeRefPn
	nameBytes := []byte(typeRefPn)
	b.i32(int32(len(nameBytes) + 1))
	b.buf = append(b.buf, nameBytes...)
	b.buf = append(b.buf, 0)
	b.i32(int32(len(instances)))
	for _, inst := range instances {
		b.instance(inst[0], inst[1], inst[2], inst[3], inst[4], inst[5], inst[6])
	}
	return b
}

// TestParseBuildableSubsystemSpecialAccumulatesRepeatedType is a regression
// test for a bug where a class name (typeRefPn) appearing in more than one
// top-level entry caused typeTransforms to be overwritten by the last entry
// while typeBeamLengths/typeCounts kept accumulating, desyncing the
// transform<->beamLength index alignment that
// extraction/collector_structures.go relies on.
func TestParseBuildableSubsystemSpecialAccumulatesRepeatedType(t *testing.T) {
	d := NewDataBlobParser(&VersionContext{ObjectVersion: 40}, nil, "REPORT")

	b := &bsBuilder{}
	b.i32(2) // entriesCount
	b.entry("Build_Foo_C", [][7]float64{{1, 0, 0, 0, 10, 20, 30}})
	b.entry("Build_Foo_C", [][7]float64{{0, 1, 0, 0, 40, 50, 60}, {0, 0, 1, 0, 70, 80, 90}})

	r := NewBinaryReader(b.buf)
	resultAny, err := d.parseBuildableSubsystemSpecial(r)
	if err != nil {
		t.Fatalf("parseBuildableSubsystemSpecial: %v", err)
	}
	result := resultAny.(map[string]interface{})

	typeCounts := result["typeCounts"].(map[string]int)
	if typeCounts["Build_Foo_C"] != 3 {
		t.Errorf("typeCounts[Build_Foo_C] = %d, want 3 (1 + 2 across both entries)", typeCounts["Build_Foo_C"])
	}

	typeTransforms := result["typeTransforms"].(map[string][]float64)
	transforms := typeTransforms["Build_Foo_C"]
	if len(transforms) != 3*7 {
		t.Fatalf("len(typeTransforms[Build_Foo_C]) = %d, want %d (3 instances x 7 floats)", len(transforms), 3*7)
	}
	// The 3rd instance (index 2, from the second entry) must be present -
	// with the old overwrite bug, typeTransforms only ever held the last
	// entry's own transforms need not line up with typeCounts/typeBeamLengths.
	if transforms[2*7+4] != 70 || transforms[2*7+5] != 80 || transforms[2*7+6] != 90 {
		t.Errorf("3rd instance translation = (%v,%v,%v), want (70,80,90)", transforms[2*7+4], transforms[2*7+5], transforms[2*7+6])
	}
}
