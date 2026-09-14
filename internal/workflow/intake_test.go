package workflow_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"rootforge/internal/casefile"
	"rootforge/internal/investigation"
	localstorage "rootforge/internal/storage/local"
	"rootforge/internal/trigger"
	"rootforge/internal/workflow"
)

func TestIntakeRecordsCaseAndEnsuresInvestigationTask(t *testing.T) {
	now := time.Date(2026, 9, 13, 4, 0, 0, 0, time.UTC)
	cases := newMemoryCaseService(t, now)
	outbox, err := localstorage.OpenInvestigationOutbox(t.TempDir())
	if err != nil {
		t.Fatalf("OpenInvestigationOutbox() error = %v", err)
	}
	intake, err := workflow.NewIntakeService(cases, outbox, workflow.WithIntakeClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewIntakeService() error = %v", err)
	}
	event := trigger.Event{ID: "event-1", Type: "container.oom", Source: "docker", OccurredAt: now, Environment: "production", Container: "abc"}

	stored, created, err := intake.RecordEvent(context.Background(), event)
	if err != nil || !created {
		t.Fatalf("RecordEvent() = (%+v, %v, %v), want created", stored, created, err)
	}
	tasks, err := outbox.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tasks) != 1 || tasks[0].CaseID != stored.ID || tasks[0].CaseRevision != stored.Revision {
		t.Fatalf("tasks = %+v", tasks)
	}
}

func TestIntakeRetryRepairsOutboxGapWithoutDuplicatingEvent(t *testing.T) {
	now := time.Date(2026, 9, 13, 4, 0, 0, 0, time.UTC)
	cases := newMemoryCaseService(t, now)
	durable, err := localstorage.OpenInvestigationOutbox(t.TempDir())
	if err != nil {
		t.Fatalf("OpenInvestigationOutbox() error = %v", err)
	}
	outbox := &failOnceOutbox{Outbox: durable}
	intake, err := workflow.NewIntakeService(cases, outbox, workflow.WithIntakeClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewIntakeService() error = %v", err)
	}
	event := trigger.Event{ID: "event-1", Type: "container.oom", Source: "docker", OccurredAt: now, Environment: "production", Container: "abc"}

	first, created, err := intake.RecordEvent(context.Background(), event)
	if err == nil || !created || first.Revision != 1 {
		t.Fatalf("first RecordEvent() = (%+v, %v, %v), want persisted Case and Outbox error", first, created, err)
	}
	second, created, err := intake.RecordEvent(context.Background(), event)
	if err != nil || created || second.Revision != 1 || len(second.Incident.Events) != 1 {
		t.Fatalf("retry RecordEvent() = (%+v, %v, %v), want idempotent revision 1", second, created, err)
	}
	tasks, err := durable.List(context.Background())
	if err != nil || len(tasks) != 1 || tasks[0].CaseRevision != 1 {
		t.Fatalf("List() = (%+v, %v)", tasks, err)
	}
}

func TestIntakeReconcilesPendingCaseWithoutTask(t *testing.T) {
	now := time.Date(2026, 9, 13, 4, 0, 0, 0, time.UTC)
	cases := newMemoryCaseService(t, now)
	event := trigger.Event{ID: "event-1", Type: "container.oom", Source: "docker", OccurredAt: now, Environment: "production", Container: "abc"}
	stored, _, err := cases.RecordEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("RecordEvent() error = %v", err)
	}
	outbox, err := localstorage.OpenInvestigationOutbox(t.TempDir())
	if err != nil {
		t.Fatalf("OpenInvestigationOutbox() error = %v", err)
	}
	intake, err := workflow.NewIntakeService(cases, outbox, workflow.WithIntakeClock(func() time.Time { return now }))
	if err != nil {
		t.Fatalf("NewIntakeService() error = %v", err)
	}

	ensured, err := intake.ReconcilePending(context.Background())
	if err != nil || ensured != 1 {
		t.Fatalf("ReconcilePending() = (%d, %v), want 1", ensured, err)
	}
	tasks, err := outbox.List(context.Background())
	if err != nil || len(tasks) != 1 || tasks[0].CaseID != stored.ID {
		t.Fatalf("List() = (%+v, %v)", tasks, err)
	}
}

type failOnceOutbox struct {
	investigation.Outbox
	failed bool
}

func (o *failOnceOutbox) EnsurePending(ctx context.Context, caseID string, revision uint64, now time.Time) (investigation.Task, bool, error) {
	if !o.failed {
		o.failed = true
		return investigation.Task{}, false, errors.New("injected Outbox failure")
	}
	return o.Outbox.EnsurePending(ctx, caseID, revision, now)
}

func newMemoryCaseService(t *testing.T, now time.Time) *casefile.Service {
	t.Helper()
	service, err := casefile.NewService(
		casefile.NewMemoryStore(),
		casefile.WithClock(func() time.Time { return now }),
		casefile.WithIDGenerator(func(time.Time) (string, error) { return "INC-workflow", nil }),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}
