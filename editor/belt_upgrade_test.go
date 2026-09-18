package editor

import "testing"

// TestUpgradeBeltsRejectsOutOfRangeTargetMk is a regression test for a bug
// where TargetMk was only constrained by the frontend's dropdown - not a
// security boundary, since UpgradeBeltsSave is a public Wails-bound method
// reachable directly via devtools/IPC. An out-of-range value got patched
// byte-for-byte into class name strings instead of being rejected.
func TestUpgradeBeltsRejectsOutOfRangeTargetMk(t *testing.T) {
	for _, mk := range []int{-1, 0, 7, 10, 99} {
		sd := &SaveData{}
		_, err := UpgradeBelts(sd, UpgradeBeltsOptions{UpgradeBelts: true, TargetMk: mk})
		if err == nil {
			t.Errorf("TargetMk=%d: expected an error, got nil", mk)
		}
	}

	// In-range values (belts/lifts go up to Mk6) must not be rejected by
	// this check specifically - use a zero-value SaveData so any other
	// error we hit here is unrelated to the range validation.
	for _, mk := range []int{1, 6} {
		sd := &SaveData{}
		_, err := UpgradeBelts(sd, UpgradeBeltsOptions{UpgradeBelts: true, TargetMk: mk})
		if err != nil {
			t.Errorf("TargetMk=%d: unexpected error: %v", mk, err)
		}
	}

	// TargetMk is irrelevant when neither belts nor lifts are being
	// upgraded (e.g. pipes-only), so an out-of-range value must not be
	// rejected in that case.
	sd := &SaveData{}
	if _, err := UpgradeBelts(sd, UpgradeBeltsOptions{TargetMk: 999}); err != nil {
		t.Errorf("TargetMk should be ignored when UpgradeBelts/UpgradeLifts are both false, got: %v", err)
	}
}
