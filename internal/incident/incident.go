package incident

import (
	"errors"
	"time"

	"rootforge/internal/trigger"
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
	Fingerprint     string
	Type            string
	Status          Status
	Environment     string
	Service         string
	Node            string
	Workload        string
	Container       string
	Severity        string
	FirstOccurredAt time.Time
	LastOccurredAt  time.Time
	Events          []trigger.Event
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

// Apply adds another occurrence to the same open Incident.
func (i *Incident) Apply(event trigger.Event) error {
	if i == nil {
		return errors.New("apply event: incident is nil")
	}
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return err
	}
	if event.Fingerprint() != i.Fingerprint {
		return errors.New("apply event: fingerprint does not match incident")
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
	return nil
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
