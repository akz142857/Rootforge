package incident

import (
	"errors"
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
	if _, err := record.Apply(earlier); err != nil {
		t.Fatalf("Apply(earlier) error = %v", err)
	}
	if _, err := record.Apply(later); err != nil {
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

	if _, err := record.Apply(event); err == nil {
		t.Fatal("Apply() error = nil, want fingerprint mismatch")
	}
}

func TestIncidentApplyIsIdempotentForSourceScopedEventID(t *testing.T) {
	now := time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC)
	event := trigger.Event{ID: "event-1", Type: "container.oom", Source: "docker", OccurredAt: now, Environment: "production", Container: "abc"}
	record, err := New(event)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	replayed := event
	replayed.ObservedAt = now.Add(time.Minute)

	changed, err := record.Apply(replayed)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if changed || len(record.Events) != 1 {
		t.Fatalf("Apply() changed/events = %v/%d, want false/1", changed, len(record.Events))
	}
}

func TestIncidentApplyRejectsReusedEventIDWithDifferentContent(t *testing.T) {
	now := time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC)
	event := trigger.Event{ID: "event-1", Type: "container.oom", Source: "docker", OccurredAt: now, Environment: "production", Container: "abc", Severity: "critical"}
	record, err := New(event)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	event.Severity = "warning"

	if _, err := record.Apply(event); !errors.Is(err, ErrEventIDConflict) {
		t.Fatalf("Apply() error = %v, want ErrEventIDConflict", err)
	}
}

func TestIncidentApplyRejectsExplicitDedupeKeyAcrossDifferentScopes(t *testing.T) {
	now := time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC)
	event := trigger.Event{Type: "container.oom", Source: "docker", DedupeKey: "oom-1", OccurredAt: now, Environment: "production", Container: "abc"}
	record, err := New(event)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	event.Container = "different"

	if _, err := record.Apply(event); !errors.Is(err, ErrEventScopeConflict) {
		t.Fatalf("Apply() error = %v, want ErrEventScopeConflict", err)
	}
}
