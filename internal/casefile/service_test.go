package casefile

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"rootforge/internal/trigger"
)

func TestServiceDeduplicatesMatchingOpenIncident(t *testing.T) {
	now := time.Date(2026, 9, 12, 3, 0, 0, 0, time.UTC)
	nextID := 0
	service, err := NewService(
		NewMemoryStore(),
		WithClock(func() time.Time { return now }),
		WithIDGenerator(func(time.Time) (string, error) {
			nextID++
			return fmt.Sprintf("INC-%d", nextID), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	event := trigger.Event{Type: "container.oom", Source: "docker", OccurredAt: now.Add(-time.Minute), Environment: "production", Container: "abc"}

	first, created, err := service.RecordEvent(context.Background(), event)
	if err != nil || !created {
		t.Fatalf("first RecordEvent() = (%+v, %v, %v), want created", first, created, err)
	}
	event.OccurredAt = now
	second, created, err := service.RecordEvent(context.Background(), event)
	if err != nil || created {
		t.Fatalf("second RecordEvent() = (%+v, %v, %v), want update", second, created, err)
	}

	if second.ID != first.ID || second.Revision != 2 || len(second.Incident.Events) != 2 {
		t.Fatalf("updated case = %+v, want same ID, revision 2, two events", second)
	}
}

func TestServiceDeduplicatesConcurrentEventsAtomically(t *testing.T) {
	now := time.Date(2026, 9, 12, 3, 0, 0, 0, time.UTC)
	nextID := 0
	service, err := NewService(
		NewMemoryStore(),
		WithClock(func() time.Time { return now }),
		WithIDGenerator(func(time.Time) (string, error) {
			nextID++
			return fmt.Sprintf("INC-%d", nextID), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	event := trigger.Event{Type: "container.oom", Source: "docker", OccurredAt: now, Environment: "production", Container: "abc"}

	const eventCount = 32
	var wait sync.WaitGroup
	results := make(chan Case, eventCount)
	errors := make(chan error, eventCount)
	for range eventCount {
		wait.Add(1)
		go func() {
			defer wait.Done()
			stored, _, recordErr := service.RecordEvent(context.Background(), event)
			if recordErr != nil {
				errors <- recordErr
				return
			}
			results <- stored
		}()
	}
	wait.Wait()
	close(results)
	close(errors)
	for recordErr := range errors {
		t.Errorf("RecordEvent() error = %v", recordErr)
	}
	if t.Failed() {
		return
	}

	var caseID string
	for stored := range results {
		if caseID == "" {
			caseID = stored.ID
		}
		if stored.ID != caseID {
			t.Fatalf("case ID = %q, want every event in %q", stored.ID, caseID)
		}
	}
	stored, err := service.Get(context.Background(), caseID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Revision != eventCount || len(stored.Incident.Events) != eventCount {
		t.Fatalf("final Case revision/events = %d/%d, want %d/%d", stored.Revision, len(stored.Incident.Events), eventCount, eventCount)
	}
}

func TestMemoryStoreReturnsClones(t *testing.T) {
	now := time.Date(2026, 9, 12, 3, 0, 0, 0, time.UTC)
	service, err := NewService(NewMemoryStore(), WithClock(func() time.Time { return now }), WithIDGenerator(func(time.Time) (string, error) { return "INC-1", nil }))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	event := trigger.Event{Type: "container.oom", Source: "docker", OccurredAt: now, Environment: "production", Container: "abc", Attributes: map[string]string{"image": "v1"}}
	created, _, err := service.RecordEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("RecordEvent() error = %v", err)
	}
	created.Incident.Events[0].Attributes["image"] = "mutated"

	stored, err := service.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got := stored.Incident.Events[0].Attributes["image"]; got != "v1" {
		t.Fatalf("stored attribute = %q, want v1", got)
	}
}
