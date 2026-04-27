package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_MissingMonthParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc", nil)
	w := httptest.NewRecorder()
	Handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Fatalf("expected 'error' key in response, got %v", body)
	}
}

func TestHandler_ValidMonth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc?month=Mar-2026", nil)
	w := httptest.NewRecorder()
	Handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := body["mes"]; !ok {
		t.Fatalf("expected 'mes' key in response, got %v", body)
	}
	if _, ok := body["weeks"]; !ok {
		t.Fatalf("expected 'weeks' key in response, got %v", body)
	}
}

func TestHandler_WithFeriados(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc?month=Mar-2026&feriados=19,25", nil)
	w := httptest.NewRecorder()
	Handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	numFeriados, ok := body["num_feriados"]
	if !ok {
		t.Fatalf("expected 'num_feriados' key in response, got %v", body)
	}
	// March 19 is Thursday (weekday), March 25 is Wednesday (weekday)
	if numFeriados != float64(2) {
		t.Fatalf("expected num_feriados=2, got %v", numFeriados)
	}
}

func TestHandler_ModoUniforme(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc?month=Mar-2026&modo=uniforme", nil)
	w := httptest.NewRecorder()
	Handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	modo, ok := body["modo"]
	if !ok {
		t.Fatalf("expected 'modo' key in response, got %v", body)
	}
	if modo != "Uniforme" {
		t.Fatalf("expected modo='Uniforme', got %v", modo)
	}
}

func TestHandler_InvalidMonth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc?month=invalid", nil)
	w := httptest.NewRecorder()
	Handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Fatalf("expected 'error' key in response, got %v", body)
	}
}

func TestHandler_InvalidModo(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc?month=Mar-2026&modo=invalid", nil)
	w := httptest.NewRecorder()
	Handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d; body: %s", w.Code, w.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Fatalf("expected 'error' key in response, got %v", body)
	}
}