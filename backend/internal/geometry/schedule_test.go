package geometry

import "testing"

func TestStableScheduleHashIgnoresSelectionOrder(t *testing.T) {
	first := StableScheduleHash("resurvey-schedule-v1.0.0", []uint{3, 1, 2})
	second := StableScheduleHash("resurvey-schedule-v1.0.0", []uint{2, 3, 1})
	if first != second {
		t.Fatal("schedule hash should be stable across selection order")
	}
	if changed := StableScheduleHash("resurvey-schedule-v2.0.0", []uint{1, 2, 3}); changed == first {
		t.Fatal("algorithm version must affect schedule hash")
	}
	if changed := StableScheduleHash("resurvey-schedule-v1.0.0", []uint{1, 2, 4}); changed == first {
		t.Fatal("snapshot set must affect schedule hash")
	}
	if len(first) != 64 {
		t.Fatalf("hash length = %d, want 64 hex characters", len(first))
	}
}
