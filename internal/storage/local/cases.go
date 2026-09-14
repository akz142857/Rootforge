package local

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"rootforge/internal/casefile"
	"rootforge/internal/incident"
	"rootforge/internal/trigger"
)

const caseStateVersion = 1

type caseDiskState struct {
	Version int             `json:"version"`
	Cases   []casefile.Case `json:"cases"`
}

// CaseStore persists Case snapshots as an atomically replaced local JSON file.
// It supports one rootforged process at a time and is not a distributed store.
type CaseStore struct {
	mu          sync.RWMutex
	path        string
	byID        map[string]casefile.Case
	openByPrint map[string]string
}

// OpenCaseStore opens or creates local Case storage below directory.
func OpenCaseStore(directory string) (*CaseStore, error) {
	directory, err := ensureDataDirectory(directory)
	if err != nil {
		return nil, err
	}
	store := &CaseStore{
		path:        filepath.Join(directory, "cases.v1.json"),
		byID:        make(map[string]casefile.Case),
		openByPrint: make(map[string]string),
	}
	var state caseDiskState
	found, err := readJSONFile(store.path, &state)
	if err != nil {
		return nil, err
	}
	if !found {
		return store, nil
	}
	if state.Version != caseStateVersion {
		return nil, fmt.Errorf("load Case state: unsupported version %d", state.Version)
	}
	for _, stored := range state.Cases {
		if err := validateStoredCase(stored); err != nil {
			return nil, fmt.Errorf("load Case state: %w", err)
		}
		if _, exists := store.byID[stored.ID]; exists {
			return nil, fmt.Errorf("load Case state: duplicate Case ID %q", stored.ID)
		}
		store.byID[stored.ID] = stored.Clone()
		if stored.Incident.Open() {
			fingerprint := stored.Incident.Fingerprint
			if existing, exists := store.openByPrint[fingerprint]; exists {
				return nil, fmt.Errorf("load Case state: open Cases %q and %q share a fingerprint", existing, stored.ID)
			}
			store.openByPrint[fingerprint] = stored.ID
		}
	}
	return store, nil
}

// ApplyEvent implements casefile.Store with an atomic durable snapshot update.
func (s *CaseStore) ApplyEvent(ctx context.Context, event trigger.Event, now time.Time, generateID casefile.IDGenerator) (casefile.Case, bool, error) {
	if err := ctx.Err(); err != nil {
		return casefile.Case{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	nextCases := cloneCases(s.byID)
	nextOpen := cloneStrings(s.openByPrint)
	fingerprint := event.Fingerprint()
	if caseID, ok := nextOpen[fingerprint]; ok {
		current := nextCases[caseID]
		if current.Incident.Open() {
			changed, err := current.Incident.Apply(event)
			if err != nil {
				return casefile.Case{}, false, err
			}
			if !changed {
				return current.Clone(), false, nil
			}
			current.Revision++
			updatedAt := now.UTC()
			if updatedAt.Before(current.UpdatedAt) {
				updatedAt = current.UpdatedAt
			}
			current.UpdatedAt = updatedAt
			nextCases[caseID] = current.Clone()
			if err := s.persist(nextCases); err != nil {
				return casefile.Case{}, false, err
			}
			s.byID = nextCases
			s.openByPrint = nextOpen
			return current.Clone(), false, nil
		}
		delete(nextOpen, fingerprint)
	}

	record, err := incident.New(event)
	if err != nil {
		return casefile.Case{}, false, err
	}
	caseID, err := generateID(now)
	if err != nil {
		return casefile.Case{}, false, err
	}
	if caseID == "" {
		return casefile.Case{}, false, errors.New("generated case ID is empty")
	}
	if _, exists := nextCases[caseID]; exists {
		return casefile.Case{}, false, casefile.ErrIDConflict
	}
	created := casefile.Case{
		ID:        caseID,
		Revision:  1,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
		Incident:  record,
	}
	nextCases[caseID] = created.Clone()
	nextOpen[fingerprint] = caseID
	if err := s.persist(nextCases); err != nil {
		return casefile.Case{}, false, err
	}
	s.byID = nextCases
	s.openByPrint = nextOpen
	return created.Clone(), true, nil
}

// Get implements casefile.Store.
func (s *CaseStore) Get(ctx context.Context, caseID string) (casefile.Case, error) {
	if err := ctx.Err(); err != nil {
		return casefile.Case{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.byID[caseID]
	if !ok {
		return casefile.Case{}, casefile.ErrNotFound
	}
	return stored.Clone(), nil
}

// ListPendingInvestigation implements casefile.Store.
func (s *CaseStore) ListPendingInvestigation(ctx context.Context) ([]casefile.Case, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]casefile.Case, 0)
	for _, stored := range s.byID {
		if stored.Incident.Status == incident.StatusPendingInvestigation {
			result = append(result, stored.Clone())
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result, nil
}

func (s *CaseStore) persist(cases map[string]casefile.Case) error {
	ordered := make([]casefile.Case, 0, len(cases))
	for _, stored := range cases {
		ordered = append(ordered, stored.Clone())
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].ID < ordered[right].ID })
	if err := writeJSONAtomic(s.path, caseDiskState{Version: caseStateVersion, Cases: ordered}); err != nil {
		return fmt.Errorf("persist Case state: %w", err)
	}
	return nil
}

func validateStoredCase(stored casefile.Case) error {
	if stored.ID == "" || stored.Revision == 0 {
		return errors.New("Case ID and revision are required")
	}
	if stored.CreatedAt.IsZero() || stored.UpdatedAt.IsZero() || stored.UpdatedAt.Before(stored.CreatedAt) {
		return fmt.Errorf("Case %q has invalid timestamps", stored.ID)
	}
	if stored.Incident.Fingerprint == "" || len(stored.Incident.Events) == 0 {
		return fmt.Errorf("Case %q has incomplete Incident state", stored.ID)
	}
	switch stored.Incident.Status {
	case incident.StatusPendingInvestigation, incident.StatusInvestigating, incident.StatusResolved, incident.StatusClosed:
	default:
		return fmt.Errorf("Case %q has unsupported Incident status %q", stored.ID, stored.Incident.Status)
	}
	first := stored.Incident.Events[0].OccurredAt
	last := first
	for _, event := range stored.Incident.Events {
		if err := event.Validate(); err != nil {
			return fmt.Errorf("Case %q contains invalid Event: %w", stored.ID, err)
		}
		if event.Fingerprint() != stored.Incident.Fingerprint {
			return fmt.Errorf("Case %q contains an Event with a different fingerprint", stored.ID)
		}
		if event.Type != stored.Incident.Type || event.Environment != stored.Incident.Environment ||
			event.Service != stored.Incident.Service || event.Node != stored.Incident.Node ||
			event.Workload != stored.Incident.Workload || event.Container != stored.Incident.Container {
			return fmt.Errorf("Case %q contains an Event with a different scope", stored.ID)
		}
		if event.OccurredAt.Before(first) {
			first = event.OccurredAt
		}
		if event.OccurredAt.After(last) {
			last = event.OccurredAt
		}
	}
	if !stored.Incident.FirstOccurredAt.Equal(first) || !stored.Incident.LastOccurredAt.Equal(last) {
		return fmt.Errorf("Case %q has an invalid occurrence window", stored.ID)
	}
	return nil
}

func cloneCases(source map[string]casefile.Case) map[string]casefile.Case {
	cloned := make(map[string]casefile.Case, len(source))
	for key, value := range source {
		cloned[key] = value.Clone()
	}
	return cloned
}

func cloneStrings(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
