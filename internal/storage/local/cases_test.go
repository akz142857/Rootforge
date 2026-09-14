package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rootforge/internal/casefile"
	"rootforge/internal/trigger"
)

func TestCaseStorePersistsAndRebuildsOpenIndex(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 9, 13, 2, 0, 0, 0, time.UTC)
	store, err := OpenCaseStore(directory)
	if err != nil {
		t.Fatalf("OpenCaseStore() error = %v", err)
	}
	service := newCaseService(t, store, now)
	event := trigger.Event{ID: "docker-1", Type: "container.oom", Source: "docker", OccurredAt: now, Environment: "production", Container: "abc", Attributes: map[string]string{"image": "sha256:123"}}
	created, wasCreated, err := service.RecordEvent(context.Background(), event)
	if err != nil || !wasCreated {
		t.Fatalf("RecordEvent() = (%+v, %v, %v), want created", created, wasCreated, err)
	}

	reopened, err := OpenCaseStore(directory)
	if err != nil {
		t.Fatalf("reopen Case Store: %v", err)
	}
	reopenedService := newCaseService(t, reopened, now.Add(time.Minute))
	stored, err := reopenedService.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() after reopen error = %v", err)
	}
	if got := stored.Incident.Events[0].Attributes["image"]; got != "sha256:123" {
		t.Fatalf("persisted image = %q", got)
	}

	event.ID = "docker-2"
	event.OccurredAt = now.Add(time.Minute)
	updated, wasCreated, err := reopenedService.RecordEvent(context.Background(), event)
	if err != nil || wasCreated {
		t.Fatalf("RecordEvent() after reopen = (%+v, %v, %v), want update", updated, wasCreated, err)
	}
	if updated.ID != created.ID || updated.Revision != 2 || len(updated.Incident.Events) != 2 {
		t.Fatalf("updated Case = %+v", updated)
	}

	info, err := os.Stat(filepath.Join(directory, "cases.v1.json"))
	if err != nil {
		t.Fatalf("stat Case state: %v", err)
	}
	if permissions := info.Mode().Perm(); permissions != 0o600 {
		t.Fatalf("Case state permissions = %o, want 600", permissions)
	}
}

func TestCaseStoreRejectsCorruptState(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "cases.v1.json"), []byte(`{"version":1,"cases":[`), 0o600); err != nil {
		t.Fatalf("write corrupt state: %v", err)
	}
	if _, err := OpenCaseStore(directory); err == nil {
		t.Fatal("OpenCaseStore() error = nil, want corrupt state error")
	}
}

func newCaseService(t *testing.T, store casefile.Store, now time.Time) *casefile.Service {
	t.Helper()
	service, err := casefile.NewService(
		store,
		casefile.WithClock(func() time.Time { return now }),
		casefile.WithIDGenerator(func(time.Time) (string, error) { return "INC-persisted", nil }),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}
