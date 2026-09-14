package incident

import (
	"errors"
	"maps"
	"time"

	"rootforge/internal/trigger"
)

var (
	// ErrEventIDConflict indicates that one source reused an Event ID with different content.
	ErrEventIDConflict = errors.New("event ID already exists with different content")
	// ErrEventScopeConflict indicates that an explicit dedupe key grouped incompatible scopes.
	ErrEventScopeConflict = errors.New("event scope conflicts with incident")
)

// Status is the lifecycle state of an Incident.
type Status string

const (
	// StatusPendingInvestigation means a Case is ready to be delegated to ClayHarness.
	StatusPendingInvestigation Status = "pending_investigation"
	// StatusInvestigating means an investigation Run is active.
	StatusInvestigating Status = "investigating"
	// StatusResolved means verification established that the incident is resolved.
	StatusResolved Status = "resolved"
	// StatusClosed means no further automated work is scheduled.
	StatusClosed Status = "closed"
)

// Incident is the deduplicated lifecycle state derived from one or more Events.
type Incident struct {
	Fingerprint     string          `json:"fingerprint"`
	Type            string          `json:"type"`
	Status          Status          `json:"status"`
	Environment     string          `json:"environment"`
	Service         string          `json:"service,omitempty"`
	Node            string          `json:"node,omitempty"`
	Workload        string          `json:"workload,omitempty"`
	Container       string          `json:"container,omitempty"`
	Severity        string          `json:"severity,omitempty"`
	FirstOccurredAt time.Time       `json:"first_occurred_at"`
	LastOccurredAt  time.Time       `json:"last_occurred_at"`
	Events          []trigger.Event `json:"events"`
}

// New creates a pending Incident from a validated Event.
func New(event trigger.Event) (Incident, error) {
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return Incident{}, err
	}
	return Incident{
		Fingerprint:     event.Fingerprint(),
		Type:            event.Type,
		Status:          StatusPendingInvestigation,
		Environment:     event.Environment,
		Service:         event.Service,
		Node:            event.Node,
		Workload:        event.Workload,
		Container:       event.Container,
		Severity:        event.Severity,
		FirstOccurredAt: event.OccurredAt,
		LastOccurredAt:  event.OccurredAt,
		Events:          []trigger.Event{event.Clone()},
	}, nil
}

// Apply adds another occurrence to the same open Incident. Replaying the same
// source-scoped Event ID is idempotent and returns false.
func (i *Incident) Apply(event trigger.Event) (bool, error) {
	if i == nil {
		return false, errors.New("apply event: incident is nil")
	}
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return false, err
	}
	if event.Fingerprint() != i.Fingerprint {
		return false, errors.New("apply event: fingerprint does not match incident")
	}
	if event.Type != i.Type || event.Environment != i.Environment ||
		event.Service != i.Service || event.Node != i.Node ||
		event.Workload != i.Workload || event.Container != i.Container {
		return false, ErrEventScopeConflict
	}
	if event.ID != "" {
		for _, existing := range i.Events {
			if existing.Source != event.Source || existing.ID != event.ID {
				continue
			}
			if sameEvent(existing, event) {
				return false, nil
			}
			return false, ErrEventIDConflict
		}
	}
	if event.OccurredAt.Before(i.FirstOccurredAt) {
		i.FirstOccurredAt = event.OccurredAt
	}
	if event.OccurredAt.After(i.LastOccurredAt) {
		i.LastOccurredAt = event.OccurredAt
	}
	if event.Severity != "" {
		i.Severity = event.Severity
	}
	i.Events = append(i.Events, event.Clone())
	return true, nil
}

// Open reports whether future matching Events may still update this Incident.
func (i Incident) Open() bool {
	return i.Status != StatusResolved && i.Status != StatusClosed
}

// Clone returns an Incident whose mutable fields do not alias the original.
func (i Incident) Clone() Incident {
	events := i.Events
	i.Events = make([]trigger.Event, len(events))
	for index, event := range events {
		i.Events[index] = event.Clone()
	}
	return i
}

func sameEvent(left, right trigger.Event) bool {
	left = left.Normalize()
	right = right.Normalize()
	return left.ID == right.ID &&
		left.Type == right.Type &&
		left.Source == right.Source &&
		left.OccurredAt.Equal(right.OccurredAt) &&
		left.Environment == right.Environment &&
		left.Service == right.Service &&
		left.Node == right.Node &&
		left.Workload == right.Workload &&
		left.Container == right.Container &&
		left.Severity == right.Severity &&
		left.DedupeKey == right.DedupeKey &&
		maps.Equal(left.Attributes, right.Attributes)
}
