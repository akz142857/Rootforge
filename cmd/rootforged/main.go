package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"rootforge/internal/casefile"
	localstorage "rootforge/internal/storage/local"
	"rootforge/internal/transport/httpapi"
	"rootforge/internal/workflow"
)

func main() {
	listenAddress := flag.String("listen", "127.0.0.1:8080", "HTTP listen address")
	dataDirectory := flag.String("data-dir", ".rootforge/data", "local durable state directory")
	reconcileInterval := flag.Duration("reconcile-interval", 30*time.Second, "pending Case reconciliation interval")
	shutdownTimeout := flag.Duration("shutdown-timeout", 10*time.Second, "graceful shutdown timeout")
	flag.Parse()
	if *reconcileInterval <= 0 {
		log.Fatal("reconcile interval must be positive")
	}

	caseStore, err := localstorage.OpenCaseStore(*dataDirectory)
	if err != nil {
		log.Fatalf("open Case storage: %v", err)
	}
	outbox, err := localstorage.OpenInvestigationOutbox(*dataDirectory)
	if err != nil {
		log.Fatalf("open investigation Outbox: %v", err)
	}
	caseService, err := casefile.NewService(caseStore)
	if err != nil {
		log.Fatalf("configure case service: %v", err)
	}
	intake, err := workflow.NewIntakeService(caseService, outbox)
	if err != nil {
		log.Fatalf("configure intake workflow: %v", err)
	}
	reconciled, err := intake.ReconcilePending(context.Background())
	if err != nil {
		log.Fatalf("reconcile pending investigations: %v", err)
	}
	handler, err := httpapi.New(intake)
	if err != nil {
		log.Fatalf("configure HTTP API: %v", err)
	}

	server := &http.Server{
		Addr:              *listenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go reconcilePending(ctx, intake, *reconcileInterval)
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			log.Printf("HTTP shutdown: %v", err)
		}
	}()

	log.Printf("rootforged listening on %s with durable state in %s (%d investigation tasks reconciled)", *listenAddress, *dataDirectory, reconciled)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("serve HTTP API: %v", err)
	}
}

type pendingReconciler interface {
	ReconcilePending(context.Context) (int, error)
}

func reconcilePending(ctx context.Context, reconciler pendingReconciler, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := reconciler.ReconcilePending(ctx)
			if err != nil {
				log.Printf("reconcile pending investigations: %v", err)
				continue
			}
			if count > 0 {
				log.Printf("reconciled %d pending investigation task(s)", count)
			}
		}
	}
}
