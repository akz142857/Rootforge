package trigger

import (
	"strings"
	"testing"
	"time"
)

func TestEventValidateRequiresIdentityAndScope(t *testing.T) {
	event := Event{Type: "container.oom", Source: "docker", OccurredAt: time.Now(), Environment: "production"}

	err := event.Validate()
	if err == nil || !strings.Contains(err.Error(), "at least one of") {
		t.Fatalf("Validate() error = %v, want missing scope error", err)
	}
}

func TestEventFingerprintIsStableAcrossNonIdentityChanges(t *testing.T) {
	base := Event{
		Type:        " Container.OOM ",
		Source:      "docker",
		OccurredAt:  time.Date(2026, 9, 12, 1, 0, 0, 0, time.UTC),
		Environment: "production",
		Service:     "payments",
		Container:   "abc",
		Severity:    "critical",
	}
	changed := base
	changed.OccurredAt = base.OccurredAt.Add(time.Minute)
	changed.Severity = "warning"

	if base.Fingerprint() != changed.Fingerprint() {
		t.Fatal("Fingerprint() changed when only occurrence metadata changed")
	}
}

func TestEventFingerprintNamespacesExplicitKeysBySource(t *testing.T) {
	base := Event{DedupeKey: "alert-42", Source: "alertmanager"}
	other := base
	other.Source = "docker"

	if base.Fingerprint() == other.Fingerprint() {
		t.Fatal("Fingerprint() must namespace explicit keys by source")
	}
}
