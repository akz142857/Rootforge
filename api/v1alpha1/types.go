package v1alpha1

import "time"

// IncidentEvent is the public event accepted by rootforged.
type IncidentEvent struct {
	EventID     string            `json:"event_id,omitempty"`
	Type        string            `json:"type"`
	Source      string            `json:"source"`
	OccurredAt  time.Time         `json:"occurred_at"`
	ObservedAt  time.Time         `json:"observed_at,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Service     string            `json:"service,omitempty"`
	Node        string            `json:"node,omitempty"`
	Workload    string            `json:"workload,omitempty"`
	Container   string            `json:"container,omitempty"`
	Severity    string            `json:"severity,omitempty"`
	DedupeKey   string            `json:"dedupe_key,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// IncidentSummary is the normalized incident state exposed by the API.
type IncidentSummary struct {
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	Environment     string    `json:"environment,omitempty"`
	Service         string    `json:"service,omitempty"`
	Node            string    `json:"node,omitempty"`
	Workload        string    `json:"workload,omitempty"`
	Container       string    `json:"container,omitempty"`
	Severity        string    `json:"severity,omitempty"`
	FirstOccurredAt time.Time `json:"first_occurred_at"`
	LastOccurredAt  time.Time `json:"last_occurred_at"`
	EventCount      int       `json:"event_count"`
}

// Case is the initial external representation of an Incident Case.
type Case struct {
	ID        string          `json:"id"`
	Revision  uint64          `json:"revision"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Incident  IncidentSummary `json:"incident"`
}

// EventAccepted reports the Case created or updated by an event.
type EventAccepted struct {
	Created bool `json:"created"`
	Case    Case `json:"case"`
}

// Error is the stable JSON error envelope for v1alpha1 endpoints.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
