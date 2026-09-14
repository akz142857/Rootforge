package local

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"rootforge/internal/investigation"
)

const outboxStateVersion = 1

type outboxDiskState struct {
	Version int                  `json:"version"`
	Tasks   []investigation.Task `json:"tasks"`
}

// InvestigationOutbox is a durable single-process investigation task queue.
type InvestigationOutbox struct {
	mu    sync.RWMutex
	path  string
	tasks map[string]investigation.Task
}

// OpenInvestigationOutbox opens or creates a local durable Outbox.
func OpenInvestigationOutbox(directory string) (*InvestigationOutbox, error) {
	directory, err := ensureDataDirectory(directory)
	if err != nil {
		return nil, err
	}
	outbox := &InvestigationOutbox{
		path:  filepath.Join(directory, "investigation-outbox.v1.json"),
		tasks: make(map[string]investigation.Task),
	}
	var state outboxDiskState
	found, err := readJSONFile(outbox.path, &state)
	if err != nil {
		return nil, err
	}
	if !found {
		return outbox, nil
	}
	if state.Version != outboxStateVersion {
		return nil, fmt.Errorf("load investigation Outbox: unsupported version %d", state.Version)
	}
	for _, task := range state.Tasks {
		if err := validateTask(task); err != nil {
			return nil, fmt.Errorf("load investigation Outbox: %w", err)
		}
		if _, exists := outbox.tasks[task.ID]; exists {
			return nil, fmt.Errorf("load investigation Outbox: duplicate task ID %q", task.ID)
		}
		outbox.tasks[task.ID] = task
	}
	return outbox, nil
}

// EnsurePending implements investigation.Outbox.
func (o *InvestigationOutbox) EnsurePending(ctx context.Context, caseID string, revision uint64, now time.Time) (investigation.Task, bool, error) {
	if err := ctx.Err(); err != nil {
		return investigation.Task{}, false, err
	}
	if caseID == "" || revision == 0 {
		return investigation.Task{}, false, errors.New("ensure investigation task: Case ID and revision are required")
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	id := taskID(caseID)
	if current, exists := o.tasks[id]; exists {
		if current.CaseID != caseID {
			return investigation.Task{}, false, errors.New("ensure investigation task: task ID collision")
		}
		if revision <= current.CaseRevision {
			return current, false, nil
		}
		next := cloneTasks(o.tasks)
		current.CaseRevision = revision
		current.UpdatedAt = now.UTC()
		next[id] = current
		if err := o.persist(next); err != nil {
			return investigation.Task{}, false, err
		}
		o.tasks = next
		return current, true, nil
	}

	created := investigation.Task{
		ID:           id,
		CaseID:       caseID,
		CaseRevision: revision,
		Status:       investigation.TaskPending,
		AvailableAt:  now.UTC(),
		CreatedAt:    now.UTC(),
		UpdatedAt:    now.UTC(),
	}
	next := cloneTasks(o.tasks)
	next[id] = created
	if err := o.persist(next); err != nil {
		return investigation.Task{}, false, err
	}
	o.tasks = next
	return created, true, nil
}

// LeaseNext implements investigation.Outbox.
func (o *InvestigationOutbox) LeaseNext(ctx context.Context, owner string, now time.Time, duration time.Duration) (investigation.Task, error) {
	if err := ctx.Err(); err != nil {
		return investigation.Task{}, err
	}
	owner = strings.TrimSpace(owner)
	if owner == "" || duration <= 0 {
		return investigation.Task{}, errors.New("lease investigation task: owner and positive duration are required")
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	eligible := make([]investigation.Task, 0)
	for _, task := range o.tasks {
		pending := task.Status == investigation.TaskPending && !task.AvailableAt.After(now)
		expired := task.Status == investigation.TaskLeased && !task.LeaseUntil.After(now)
		if pending || expired {
			eligible = append(eligible, task)
		}
	}
	if len(eligible) == 0 {
		return investigation.Task{}, investigation.ErrNoTask
	}
	sort.Slice(eligible, func(left, right int) bool {
		if eligible[left].AvailableAt.Equal(eligible[right].AvailableAt) {
			return eligible[left].ID < eligible[right].ID
		}
		return eligible[left].AvailableAt.Before(eligible[right].AvailableAt)
	})
	leased := eligible[0]
	token, err := randomLeaseToken()
	if err != nil {
		return investigation.Task{}, err
	}
	leased.Status = investigation.TaskLeased
	leased.Attempts++
	leased.LeaseOwner = owner
	leased.LeaseToken = token
	leased.LeaseUntil = now.UTC().Add(duration)
	leased.LeasedRevision = leased.CaseRevision
	leased.UpdatedAt = now.UTC()

	next := cloneTasks(o.tasks)
	next[leased.ID] = leased
	if err := o.persist(next); err != nil {
		return investigation.Task{}, err
	}
	o.tasks = next
	return leased, nil
}

// Complete implements investigation.Outbox.
func (o *InvestigationOutbox) Complete(ctx context.Context, taskID, leaseToken string, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	current, err := o.ownedLease(taskID, leaseToken, now)
	if err != nil {
		return err
	}
	next := cloneTasks(o.tasks)
	if current.CaseRevision > current.LeasedRevision {
		current.Status = investigation.TaskPending
		current.AvailableAt = now.UTC()
		clearLease(&current)
		current.UpdatedAt = now.UTC()
		next[taskID] = current
	} else {
		delete(next, taskID)
	}
	if err := o.persist(next); err != nil {
		return err
	}
	o.tasks = next
	return nil
}

// Retry implements investigation.Outbox.
func (o *InvestigationOutbox) Retry(ctx context.Context, taskID, leaseToken string, now, availableAt time.Time, failure string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	current, err := o.ownedLease(taskID, leaseToken, now)
	if err != nil {
		return err
	}
	current.Status = investigation.TaskPending
	current.AvailableAt = availableAt.UTC()
	current.LastError = truncate(strings.TrimSpace(failure), 2048)
	clearLease(&current)
	current.UpdatedAt = now.UTC()
	next := cloneTasks(o.tasks)
	next[taskID] = current
	if err := o.persist(next); err != nil {
		return err
	}
	o.tasks = next
	return nil
}

// List implements investigation.Outbox.
func (o *InvestigationOutbox) List(ctx context.Context) ([]investigation.Task, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	result := make([]investigation.Task, 0, len(o.tasks))
	for _, task := range o.tasks {
		result = append(result, task)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].ID < result[right].ID })
	return result, nil
}

func (o *InvestigationOutbox) ownedLease(id, token string, now time.Time) (investigation.Task, error) {
	current, exists := o.tasks[id]
	if !exists {
		return investigation.Task{}, investigation.ErrTaskNotFound
	}
	if current.Status != investigation.TaskLeased || token == "" || current.LeaseToken != token || !current.LeaseUntil.After(now) {
		return investigation.Task{}, investigation.ErrLeaseConflict
	}
	return current, nil
}

func (o *InvestigationOutbox) persist(tasks map[string]investigation.Task) error {
	ordered := make([]investigation.Task, 0, len(tasks))
	for _, task := range tasks {
		ordered = append(ordered, task)
	}
	sort.Slice(ordered, func(left, right int) bool { return ordered[left].ID < ordered[right].ID })
	if err := writeJSONAtomic(o.path, outboxDiskState{Version: outboxStateVersion, Tasks: ordered}); err != nil {
		return fmt.Errorf("persist investigation Outbox: %w", err)
	}
	return nil
}

func validateTask(task investigation.Task) error {
	if task.ID == "" || task.CaseID == "" || task.CaseRevision == 0 {
		return errors.New("task ID, Case ID, and revision are required")
	}
	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() || task.AvailableAt.IsZero() {
		return fmt.Errorf("task %q has incomplete timestamps", task.ID)
	}
	if task.ID != taskID(task.CaseID) {
		return fmt.Errorf("task %q does not match its Case ID", task.ID)
	}
	if task.Status != investigation.TaskPending && task.Status != investigation.TaskLeased {
		return fmt.Errorf("task %q has unsupported status %q", task.ID, task.Status)
	}
	if task.Status == investigation.TaskLeased {
		if task.LeaseToken == "" || task.LeaseOwner == "" || task.LeaseUntil.IsZero() || task.LeasedRevision == 0 || task.LeasedRevision > task.CaseRevision {
			return fmt.Errorf("task %q has incomplete lease state", task.ID)
		}
	} else if task.LeaseToken != "" || task.LeaseOwner != "" || !task.LeaseUntil.IsZero() || task.LeasedRevision != 0 {
		return fmt.Errorf("task %q has lease state while pending", task.ID)
	}
	return nil
}

func taskID(caseID string) string {
	digest := sha256.Sum256([]byte("rootforge-investigation-v1\x00" + caseID))
	return "INV-" + hex.EncodeToString(digest[:16])
}

func randomLeaseToken() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", fmt.Errorf("generate investigation lease token: %w", err)
	}
	return hex.EncodeToString(token[:]), nil
}

func clearLease(task *investigation.Task) {
	task.LeaseOwner = ""
	task.LeaseToken = ""
	task.LeaseUntil = time.Time{}
	task.LeasedRevision = 0
}

func cloneTasks(source map[string]investigation.Task) map[string]investigation.Task {
	cloned := make(map[string]investigation.Task, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func truncate(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	return value[:maximum]
}
