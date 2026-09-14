package casefile

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"rootforge/internal/trigger"
)

// Service establishes and retrieves authoritative Incident Cases.
type Service struct {
	store      Store
	now        func() time.Time
	generateID IDGenerator
}

// Option configures a Service.
type Option func(*Service)

// WithClock replaces the wall clock, primarily for deterministic tests.
func WithClock(now func() time.Time) Option {
	return func(service *Service) { service.now = now }
}

// WithIDGenerator replaces Case ID generation.
func WithIDGenerator(generate IDGenerator) Option {
	return func(service *Service) { service.generateID = generate }
}

// NewService creates a Case service backed by store.
func NewService(store Store, options ...Option) (*Service, error) {
	if store == nil {
		return nil, errors.New("create case service: store is required")
	}
	service := &Service{store: store, now: time.Now, generateID: randomCaseID}
	for _, option := range options {
		option(service)
	}
	if service.now == nil || service.generateID == nil {
		return nil, errors.New("create case service: clock and ID generator are required")
	}
	return service, nil
}

// RecordEvent creates a Case or appends the Event to its matching open Case.
func (s *Service) RecordEvent(ctx context.Context, event trigger.Event) (Case, bool, error) {
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return Case{}, false, err
	}
	now := s.now().UTC()
	if event.ObservedAt.IsZero() {
		event.ObservedAt = now
	}
	return s.store.ApplyEvent(ctx, event, now, s.generateID)
}

// Get returns a Case by ID.
func (s *Service) Get(ctx context.Context, caseID string) (Case, error) {
	if caseID == "" {
		return Case{}, ErrNotFound
	}
	return s.store.Get(ctx, caseID)
}

// ListPendingInvestigation returns Cases that still need an investigation Run.
func (s *Service) ListPendingInvestigation(ctx context.Context) ([]Case, error) {
	return s.store.ListPendingInvestigation(ctx)
}

func randomCaseID(now time.Time) (string, error) {
	var suffix [4]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate case ID: %w", err)
	}
	return "INC-" + now.UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(suffix[:]), nil
}
