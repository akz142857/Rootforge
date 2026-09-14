package main

import (
	"context"
	"testing"
	"time"
)

func TestReconcilePendingRunsUntilContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	called := make(chan struct{}, 1)
	reconciler := signalingReconciler{called: called}
	go reconcilePending(ctx, reconciler, time.Millisecond)

	select {
	case <-called:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("reconcilePending() did not invoke reconciler")
	}
}

type signalingReconciler struct {
	called chan<- struct{}
}

func (r signalingReconciler) ReconcilePending(context.Context) (int, error) {
	select {
	case r.called <- struct{}{}:
	default:
	}
	return 0, nil
}
