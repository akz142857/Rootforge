// Package httpapi exposes Rootforge's versioned control-plane HTTP API.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	v1alpha1 "rootforge/api/v1alpha1"
	"rootforge/internal/casefile"
	"rootforge/internal/incident"
	"rootforge/internal/trigger"
)

const maxRequestBytes = 1 << 20

// Handler serves the first Rootforge incident intake and Case inspection API.
type Handler struct {
	cases caseService
	mux   *http.ServeMux
}

type caseService interface {
	RecordEvent(context.Context, trigger.Event) (casefile.Case, bool, error)
	Get(context.Context, string) (casefile.Case, error)
}

// New creates an HTTP handler backed by the authoritative Case service.
func New(cases caseService) (*Handler, error) {
	if cases == nil {
		return nil, errors.New("create HTTP API: case service is required")
	}
	handler := &Handler{cases: cases, mux: http.NewServeMux()}
	handler.mux.HandleFunc("GET /healthz", handler.health)
	handler.mux.HandleFunc("POST /api/v1alpha1/events", handler.acceptEvent)
	handler.mux.HandleFunc("GET /api/v1alpha1/cases/{caseID}", handler.getCase)
	return handler, nil
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	h.mux.ServeHTTP(response, request)
}

func (h *Handler) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) acceptEvent(response http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(response, request.Body, maxRequestBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input v1alpha1.IncidentEvent
	if err := decoder.Decode(&input); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_json", friendlyDecodeError(err))
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	event := trigger.Event{
		ID:          input.EventID,
		Type:        input.Type,
		Source:      input.Source,
		OccurredAt:  input.OccurredAt,
		ObservedAt:  input.ObservedAt,
		Environment: input.Environment,
		Service:     input.Service,
		Node:        input.Node,
		Workload:    input.Workload,
		Container:   input.Container,
		Severity:    input.Severity,
		DedupeKey:   input.DedupeKey,
		Attributes:  input.Attributes,
	}
	if err := event.Normalize().Validate(); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_event", err.Error())
		return
	}
	stored, created, err := h.cases.RecordEvent(request.Context(), event)
	if errors.Is(err, incident.ErrEventIDConflict) || errors.Is(err, incident.ErrEventScopeConflict) {
		writeAPIError(response, http.StatusConflict, "event_conflict", err.Error())
		return
	}
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, "internal_error", "unable to record incident event")
		return
	}
	writeJSON(response, http.StatusAccepted, v1alpha1.EventAccepted{Created: created, Case: toAPICase(stored)})
}

func (h *Handler) getCase(response http.ResponseWriter, request *http.Request) {
	caseID := strings.TrimSpace(request.PathValue("caseID"))
	stored, err := h.cases.Get(request.Context(), caseID)
	if errors.Is(err, casefile.ErrNotFound) {
		writeAPIError(response, http.StatusNotFound, "case_not_found", "incident case not found")
		return
	}
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, "internal_error", "unable to read incident case")
		return
	}
	writeJSON(response, http.StatusOK, toAPICase(stored))
}

func toAPICase(stored casefile.Case) v1alpha1.Case {
	incident := stored.Incident
	return v1alpha1.Case{
		ID:        stored.ID,
		Revision:  stored.Revision,
		CreatedAt: stored.CreatedAt,
		UpdatedAt: stored.UpdatedAt,
		Incident: v1alpha1.IncidentSummary{
			Type:            incident.Type,
			Status:          string(incident.Status),
			Environment:     incident.Environment,
			Service:         incident.Service,
			Node:            incident.Node,
			Workload:        incident.Workload,
			Container:       incident.Container,
			Severity:        incident.Severity,
			FirstOccurredAt: incident.FirstOccurredAt,
			LastOccurredAt:  incident.LastOccurredAt,
			EventCount:      len(incident.Events),
		},
	}
}

func ensureEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("invalid trailing JSON: %w", err)
	}
	return errors.New("request body must contain one JSON object")
}

func friendlyDecodeError(err error) string {
	var maximum *http.MaxBytesError
	if errors.As(err, &maximum) {
		return fmt.Sprintf("request body exceeds %d bytes", maximum.Limit)
	}
	return err.Error()
}

func writeAPIError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, v1alpha1.Error{Code: code, Message: message})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
