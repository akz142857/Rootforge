package trigger

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Event is a normalized, low-volume signal that may create or update an Incident.
type Event struct {
	ID          string
	Type        string
	Source      string
	OccurredAt  time.Time
	ObservedAt  time.Time
	Environment string
	Service     string
	Node        string
	Workload    string
	Container   string
	Severity    string
	DedupeKey   string
	Attributes  map[string]string
}

// Normalize trims boundary whitespace and copies mutable data owned by callers.
func (e Event) Normalize() Event {
	e.ID = strings.TrimSpace(e.ID)
	e.Type = strings.ToLower(strings.TrimSpace(e.Type))
	e.Source = strings.TrimSpace(e.Source)
	e.Environment = strings.TrimSpace(e.Environment)
	e.Service = strings.TrimSpace(e.Service)
	e.Node = strings.TrimSpace(e.Node)
	e.Workload = strings.TrimSpace(e.Workload)
	e.Container = strings.TrimSpace(e.Container)
	e.Severity = strings.ToLower(strings.TrimSpace(e.Severity))
	e.DedupeKey = strings.TrimSpace(e.DedupeKey)
	e.OccurredAt = e.OccurredAt.UTC()
	if !e.ObservedAt.IsZero() {
		e.ObservedAt = e.ObservedAt.UTC()
	}
	e.Attributes = cloneAttributes(e.Attributes)
	return e
}

// Validate checks the minimum contract needed to establish an Incident scope.
func (e Event) Validate() error {
	var problems []error
	if strings.TrimSpace(e.Type) == "" {
		problems = append(problems, errors.New("type is required"))
	}
	if strings.TrimSpace(e.Source) == "" {
		problems = append(problems, errors.New("source is required"))
	}
	if e.OccurredAt.IsZero() {
		problems = append(problems, errors.New("occurred_at is required"))
	}
	if strings.TrimSpace(e.Environment) == "" {
		problems = append(problems, errors.New("environment is required"))
	}
	if strings.TrimSpace(e.Service) == "" && strings.TrimSpace(e.Node) == "" &&
		strings.TrimSpace(e.Workload) == "" && strings.TrimSpace(e.Container) == "" {
		problems = append(problems, errors.New("at least one of service, node, workload, or container is required"))
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid incident event: %w", errors.Join(problems...))
	}
	return nil
}

// Fingerprint returns a stable, opaque key for open-Incident deduplication.
// An explicit source-scoped dedupe key takes precedence over the fallback scope.
func (e Event) Fingerprint() string {
	e = e.Normalize()
	parts := []string{"rootforge-trigger-v1"}
	if e.DedupeKey != "" {
		parts = append(parts, "explicit", e.Source, e.DedupeKey)
	} else {
		parts = append(parts, "scope", e.Type, e.Environment, e.Service, e.Node, e.Workload, e.Container)
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(digest[:])
}

// Clone returns an Event whose mutable fields do not alias the original.
func (e Event) Clone() Event {
	e.Attributes = cloneAttributes(e.Attributes)
	return e
}

func cloneAttributes(attributes map[string]string) map[string]string {
	if attributes == nil {
		return nil
	}
	cloned := make(map[string]string, len(attributes))
	for key, value := range attributes {
		cloned[key] = value
	}
	return cloned
}
