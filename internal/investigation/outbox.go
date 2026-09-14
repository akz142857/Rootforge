package investigation

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrNoTask indicates that no investigation task is currently available.
	ErrNoTask = errors.New("no investigation task available")
	// ErrTaskNotFound indicates that an investigation task does not exist.
	ErrTaskNotFound = errors.New("investigation task not found")
	// ErrLeaseConflict indicates that a worker no longer owns the task lease.
	ErrLeaseConflict = errors.New("investigation task lease conflict")
)

// TaskStatus is the delivery state of an investigation task.
type TaskStatus string

const (
	// TaskPending is available once AvailableAt has passed.
	TaskPending TaskStatus = "pending"
	// TaskLeased is temporarily owned by one dispatcher.
	TaskLeased TaskStatus = "leased"
)

// Task represents durable delivery work for the latest known Case revision.
type Task struct {
	ID             string     `json:"id"`
	CaseID         string     `json:"case_id"`
	CaseRevision   uint64     `json:"case_revision"`
	Status         TaskStatus `json:"status"`
	Attempts       uint32     `json:"attempts"`
	AvailableAt    time.Time  `json:"available_at"`
	LeaseOwner     string     `json:"lease_owner,omitempty"`
	LeaseToken     string     `json:"lease_token,omitempty"`
	LeaseUntil     time.Time  `json:"lease_until,omitempty"`
	LeasedRevision uint64     `json:"leased_revision,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// Outbox durably schedules at-least-once investigation delivery.
type Outbox interface {
	EnsurePending(context.Context, string, uint64, time.Time) (Task, bool, error)
	LeaseNext(context.Context, string, time.Time, time.Duration) (Task, error)
	Complete(context.Context, string, string, time.Time) error
	Retry(context.Context, string, string, time.Time, time.Time, string) error
	List(context.Context) ([]Task, error)
}
