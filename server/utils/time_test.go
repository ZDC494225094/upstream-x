package utils

import (
	"testing"
	"time"
)

func TestRangeStart3d(t *testing.T) {
	before := time.Now().AddDate(0, 0, -3)
	got := RangeStart("3d")
	after := time.Now().AddDate(0, 0, -3)

	if got.Before(before.Add(-time.Second)) || got.After(after.Add(time.Second)) {
		t.Fatalf("RangeStart(3d) = %v, want about 3 days ago", got)
	}
}
