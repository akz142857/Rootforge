package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"rootforge/internal/casefile"
	"rootforge/internal/investigation"
	"rootforge/internal/trigger"
)

// CaseService is the Case behavior required by incident intake.
type CaseService interface {
	RecordEvent(context.Context, trigger.Event) (casefile.Case, bool, error)
	Get(context.Context, string) (casefile.Case, error)
	ListPendingInvestigation(context.Context) ([]casefile.Case, error)
}

// IntakeService records an Event before ensuring its investigation task exists.
// A retry or startup reconciliation closes the deliberate two-file crash gap.
type IntakeService struct {
	cases  CaseService
	outbox investigation.Outbox
	now    func() time.Time
}

// IntakeOption configures an IntakeService.
type IntakeOption func(*IntakeService)

// WithIntakeClock replaces the wall clock, primarily for deterministic tests.
func WithIntakeClock(now func() time.Time) IntakeOption {
	return func(service *IntakeService) { service.now = now }
}

// NewIntakeService creates the event-to-Case-to-Outbox workflow.
func NewIntakeService(cases CaseService, outbox investigation.Outbox, options ...IntakeOption) (*IntakeService, error) {
	if cases == nil || outbox == nil {
		return nil, errors.New("create intake service: Case service and Outbox are required")
	}
	service := &IntakeService{cases: cases, outbox: outbox, now: time.Now}
	for _, option := range options {
		option(service)
	}
	if service.now == nil {
		return nil, errors.New("create intake service: clock is required")
	}
	return service, nil
}

// RecordEvent durably records an Event, then idempotently ensures investigation work.
func (s *IntakeService) RecordEvent(ctx context.Context, event trigger.Event) (casefile.Case, bool, error) {
	stored, created, err := s.cases.RecordEvent(ctx, event)
	if err != nil {
		return casefile.Case{}, false, err
	}
	if _, _, err := s.outbox.EnsurePending(ctx, stored.ID, stored.Revision, s.now().UTC()); err != nil {
		return stored, created, fmt.Errorf("schedule Case %s investigation: %w", stored.ID, err)
	}
	return stored, created, nil
}

// Get returns a Case by ID.
func (s *IntakeService) Get(ctx context.Context, caseID string) (casefile.Case, error) {
	return s.cases.Get(ctx, caseID)
}

// ReconcilePending ensures every pending Case has durable investigation work.
func (s *IntakeService) ReconcilePending(ctx context.Context) (int, error) {
	cases, err := s.cases.ListPendingInvestigation(ctx)
	if err != nil {
		return 0, fmt.Errorf("list pending investigation Cases: %w", err)
	}
	ensured := 0
	for _, stored := range cases {
		_, changed, ensureErr := s.outbox.EnsurePending(ctx, stored.ID, stored.Revision, s.now().UTC())
		if ensureErr != nil {
			return ensured, fmt.Errorf("reconcile Case %s investigation: %w", stored.ID, ensureErr)
		}
		if changed {
			ensured++
		}
	}
	return ensured, nil
}
