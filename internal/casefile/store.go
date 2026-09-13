package casefile

import (
	"context"
	"errors"
	"sync"
	"time"

	"rootforge/internal/incident"
	"rootforge/internal/trigger"
)

// ErrNotFound indicates that a Case ID is unknown to a Store.
var ErrNotFound = errors.New("case not found")

// ErrIDConflict indicates that an ID generator returned an existing Case ID.
var ErrIDConflict = errors.New("case ID already exists")

// IDGenerator creates a new opaque Case ID.
type IDGenerator func(time.Time) (string, error)

// Store atomically creates or updates an open Case for an Event fingerprint.
type Store interface {
	ApplyEvent(context.Context, trigger.Event, time.Time, IDGenerator) (Case, bool, error)
	Get(context.Context, string) (Case, error)
}

// MemoryStore is a concurrency-safe, process-local Store for development and tests.
// It intentionally provides no durability guarantee.
type MemoryStore struct {
	mu          sync.RWMutex
	byID        map[string]Case
	openByPrint map[string]string
}

// NewMemoryStore creates an empty process-local Case store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		byID:        make(map[string]Case),
		openByPrint: make(map[string]string),
	}
}

// ApplyEvent implements Store.
func (s *MemoryStore) ApplyEvent(ctx context.Context, event trigger.Event, now time.Time, generateID IDGenerator) (Case, bool, error) {
	if err := ctx.Err(); err != nil {
		return Case{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	fingerprint := event.Fingerprint()
	if caseID, ok := s.openByPrint[fingerprint]; ok {
		current := s.byID[caseID]
		if current.Incident.Open() {
			if err := current.Incident.Apply(event); err != nil {
				return Case{}, false, err
			}
			current.Revision++
			current.UpdatedAt = now.UTC()
			s.byID[caseID] = current.Clone()
			return current.Clone(), false, nil
		}
		delete(s.openByPrint, fingerprint)
	}

	record, err := incident.New(event)
	if err != nil {
		return Case{}, false, err
	}
	caseID, err := generateID(now)
	if err != nil {
		return Case{}, false, err
	}
	if caseID == "" {
		return Case{}, false, errors.New("generated case ID is empty")
	}
	if _, exists := s.byID[caseID]; exists {
		return Case{}, false, ErrIDConflict
	}
	created := Case{
		ID:        caseID,
		Revision:  1,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
		Incident:  record,
	}
	s.byID[caseID] = created.Clone()
	s.openByPrint[fingerprint] = caseID
	return created.Clone(), true, nil
}

// Get implements Store.
func (s *MemoryStore) Get(ctx context.Context, caseID string) (Case, error) {
	if err := ctx.Err(); err != nil {
		return Case{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.byID[caseID]
	if !ok {
		return Case{}, ErrNotFound
	}
	return stored.Clone(), nil
}
