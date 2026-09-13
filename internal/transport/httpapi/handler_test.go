package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1alpha1 "rootforge/api/v1alpha1"
	"rootforge/internal/casefile"
)

func TestEventIntakeCreatesThenUpdatesCase(t *testing.T) {
	now := time.Date(2026, 9, 12, 4, 0, 0, 0, time.UTC)
	service, err := casefile.NewService(
		casefile.NewMemoryStore(),
		casefile.WithClock(func() time.Time { return now }),
		casefile.WithIDGenerator(func(time.Time) (string, error) { return "INC-test", nil }),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	handler, err := New(service)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	payload := v1alpha1.IncidentEvent{Type: "container.oom", Source: "docker", OccurredAt: now.Add(-time.Minute), Environment: "production", Service: "payments", Container: "abc", Severity: "critical"}

	first := postEvent(t, handler, payload)
	if !first.Created || first.Case.ID != "INC-test" || first.Case.Incident.EventCount != 1 {
		t.Fatalf("first response = %+v", first)
	}
	payload.OccurredAt = now
	second := postEvent(t, handler, payload)
	if second.Created || second.Case.ID != first.Case.ID || second.Case.Revision != 2 || second.Case.Incident.EventCount != 2 {
		t.Fatalf("second response = %+v", second)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1alpha1/cases/INC-test", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET case status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestEventIntakeRejectsUnknownFields(t *testing.T) {
	service, err := casefile.NewService(casefile.NewMemoryStore())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	handler, err := New(service)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1alpha1/events", bytes.NewBufferString(`{"type":"container.oom","unknown":true}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func postEvent(t *testing.T, handler http.Handler, event v1alpha1.IncidentEvent) v1alpha1.EventAccepted {
	t.Helper()
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1alpha1/events", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("POST event status = %d, body = %s", response.Code, response.Body.String())
	}
	var accepted v1alpha1.EventAccepted
	if err := json.NewDecoder(response.Body).Decode(&accepted); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return accepted
}
