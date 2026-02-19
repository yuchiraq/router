package notify

import (
	"testing"
	"time"
)

func TestInQuietHours(t *testing.T) {
	now := time.Date(2026, 1, 1, 21, 0, 0, 0, time.UTC)
	if !inQuietHours(now, true, 20, 8) {
		t.Fatalf("expected quiet hours at 21:00 for range 20-8")
	}
	if inQuietHours(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), true, 20, 8) {
		t.Fatalf("did not expect quiet hours at 12:00 for range 20-8")
	}
	if !inQuietHours(time.Date(2026, 1, 1, 7, 0, 0, 0, time.UTC), true, 20, 8) {
		t.Fatalf("expected quiet hours at 07:00 for range 20-8")
	}
	if inQuietHours(now, false, 20, 8) {
		t.Fatalf("quiet hours should be disabled")
	}
}
