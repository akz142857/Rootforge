package incident

import (
	"testing"
	"time"

	"rootforge/internal/trigger"
)

func TestIncidentApplyTracksOccurrenceWindow(t *testing.T) {
	middle := time.Date(2026, 9, 12, 2, 0, 0, 0, time.UTC)
	base := trigger.Event{Type: "container.oom", Source: "docker", OccurredAt: middle, Environment: "production", Container: "abc"}
	record, err := New(base)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	earlier := base
	earlier.OccurredAt = middle.Add(-time.Minute)
	later := base
	later.OccurredAt = middle.Add(time.Minute)
	if err := record.Apply(earlier); err != nil {
		t.Fatalf("Apply(earlier) error = %v", err)
	}
	if err := record.Apply(later); err != nil {
		t.Fatalf("Apply(later) error = %v", err)
	}

	if !record.FirstOccurredAt.Equal(earlier.OccurredAt) || !record.LastOccurredAt.Equal(later.OccurredAt) {
		t.Fatalf("occurrence window = %s..%s", record.FirstOccurredAt, record.LastOccurredAt)
	}
	if len(record.Events) != 3 {
		t.Fatalf("event count = %d, want 3", len(record.Events))
	}
}

func TestIncidentApplyRejectsDifferentFingerprint(t *testing.T) {
	event := trigger.Event{Type: "container.oom", Source: "docker", OccurredAt: time.Now(), Environment: "production", Container: "abc"}
	record, err := New(event)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	event.Container = "different"

	if err := record.Apply(event); err == nil {
		t.Fatal("Apply() error = nil, want fingerprint mismatch")
	}
}
