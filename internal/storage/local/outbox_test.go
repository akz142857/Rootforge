package local

import (
	"context"
	"errors"
	"testing"
	"time"

	"rootforge/internal/investigation"
)

func TestInvestigationOutboxPersistsLeaseAndReschedulesNewRevision(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 9, 13, 3, 0, 0, 0, time.UTC)
	outbox, err := OpenInvestigationOutbox(directory)
	if err != nil {
		t.Fatalf("OpenInvestigationOutbox() error = %v", err)
	}
	created, changed, err := outbox.EnsurePending(context.Background(), "INC-1", 1, now)
	if err != nil || !changed || created.Status != investigation.TaskPending {
		t.Fatalf("EnsurePending() = (%+v, %v, %v)", created, changed, err)
	}
	leased, err := outbox.LeaseNext(context.Background(), "dispatcher-1", now, time.Minute)
	if err != nil {
		t.Fatalf("LeaseNext() error = %v", err)
	}
	if leased.CaseRevision != 1 || leased.LeasedRevision != 1 || leased.LeaseToken == "" {
		t.Fatalf("leased task = %+v", leased)
	}
	if _, changed, err := outbox.EnsurePending(context.Background(), "INC-1", 2, now.Add(time.Second)); err != nil || !changed {
		t.Fatalf("EnsurePending(revision 2) changed/error = %v/%v", changed, err)
	}

	reopened, err := OpenInvestigationOutbox(directory)
	if err != nil {
		t.Fatalf("reopen Outbox: %v", err)
	}
	if err := reopened.Complete(context.Background(), leased.ID, leased.LeaseToken, now.Add(2*time.Second)); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	rescheduled, err := reopened.LeaseNext(context.Background(), "dispatcher-2", now.Add(3*time.Second), time.Minute)
	if err != nil {
		t.Fatalf("LeaseNext(rescheduled) error = %v", err)
	}
	if rescheduled.CaseRevision != 2 || rescheduled.LeasedRevision != 2 || rescheduled.Attempts != 2 {
		t.Fatalf("rescheduled task = %+v", rescheduled)
	}
	if err := reopened.Complete(context.Background(), rescheduled.ID, rescheduled.LeaseToken, now.Add(4*time.Second)); err != nil {
		t.Fatalf("Complete(revision 2) error = %v", err)
	}
	if _, err := reopened.LeaseNext(context.Background(), "dispatcher-3", now.Add(5*time.Second), time.Minute); !errors.Is(err, investigation.ErrNoTask) {
		t.Fatalf("LeaseNext() error = %v, want ErrNoTask", err)
	}
}

func TestInvestigationOutboxReclaimsExpiredLeaseAndRejectsStaleWorker(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 9, 13, 3, 0, 0, 0, time.UTC)
	outbox, err := OpenInvestigationOutbox(directory)
	if err != nil {
		t.Fatalf("OpenInvestigationOutbox() error = %v", err)
	}
	if _, _, err := outbox.EnsurePending(context.Background(), "INC-1", 1, now); err != nil {
		t.Fatalf("EnsurePending() error = %v", err)
	}
	first, err := outbox.LeaseNext(context.Background(), "dispatcher-1", now, time.Second)
	if err != nil {
		t.Fatalf("LeaseNext() error = %v", err)
	}
	second, err := outbox.LeaseNext(context.Background(), "dispatcher-2", now.Add(time.Second), time.Minute)
	if err != nil {
		t.Fatalf("LeaseNext(expired) error = %v", err)
	}
	if second.LeaseToken == first.LeaseToken || second.Attempts != 2 {
		t.Fatalf("reclaimed task = %+v", second)
	}
	if err := outbox.Complete(context.Background(), first.ID, first.LeaseToken, now.Add(2*time.Second)); !errors.Is(err, investigation.ErrLeaseConflict) {
		t.Fatalf("stale Complete() error = %v, want ErrLeaseConflict", err)
	}
	retryAt := now.Add(10 * time.Second)
	if err := outbox.Retry(context.Background(), second.ID, second.LeaseToken, now.Add(2*time.Second), retryAt, "temporary failure"); err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if _, err := outbox.LeaseNext(context.Background(), "dispatcher-3", now.Add(9*time.Second), time.Minute); !errors.Is(err, investigation.ErrNoTask) {
		t.Fatalf("LeaseNext(before retry) error = %v, want ErrNoTask", err)
	}
	third, err := outbox.LeaseNext(context.Background(), "dispatcher-3", retryAt, time.Minute)
	if err != nil || third.LastError != "temporary failure" {
		t.Fatalf("LeaseNext(at retry) = (%+v, %v)", third, err)
	}
}
