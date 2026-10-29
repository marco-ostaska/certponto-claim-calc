package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"certponto-claim-calc/internal/calc"
)

// ── CLI tests ──

func TestFullPlanMarch2026NoHolidays(t *testing.T) {
	plan := calc.CalcModoA(2026, time.March, nil, 16, calc.BlocoHoraCheia)

	// March 2026 starts on Monday, has 31 days → 5 weeks
	if len(plan.Weeks) != 5 {
		t.Errorf("esperava 5 semanas, got %d", len(plan.Weeks))
	}

	// Total working days = 22
	var totalDays int
	for _, w := range plan.Weeks {
		totalDays += len(w.Days)
	}
	if totalDays != 22 {
		t.Errorf("total de dias uteis = %d, want 22", totalDays)
	}

	// Extra must not exceed 16h
	var extra float64
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			extra += d.Hours - calc.BaseHoursPerDay
		}
	}
	if extra > 16.01 {
		t.Errorf("extra = %.2f, nao deve exceder 16h", extra)
	}
}

func TestCertPontoEntryTime(t *testing.T) {
	entrada, saida := calc.FormatCertPonto(9.0)
	if entrada != "07:00" {
		t.Errorf("entrada para 9h = %q, want 07:00", entrada)
	}
	if saida != "17:00" {
		t.Errorf("saida = %q, want 17:00", saida)
	}
}

func TestModoAAndModoBProduceSameTotalDays(t *testing.T) {
	planA := calc.CalcModoA(2026, time.March, nil, 16, calc.BlocoHoraCheia)
	planB := calc.CalcModoB(2026, time.March, nil, 16)
	if planA.TotalDays != planB.TotalDays {
		t.Errorf("ModoA.TotalDays=%d != ModoB.TotalDays=%d", planA.TotalDays, planB.TotalDays)
	}
	if planA.TotalHours != planB.TotalHours {
		t.Errorf("ModoA.TotalHours=%.1f != ModoB.TotalHours=%.1f", planA.TotalHours, planB.TotalHours)
	}
}

func TestParseArgsValid(t *testing.T) {
	cfg, err := parseArgs([]string{"Mar-2026"})
	if err != nil {
		t.Fatalf("parseArgs: unexpected error: %v", err)
	}
	if cfg.year != 2026 || cfg.month != int(time.March) {
		t.Errorf("parseArgs: got year=%d month=%d, want 2026 March", cfg.year, cfg.month)
	}
	if cfg.modoUniforme {
		t.Error("modoUniforme should default to false")
	}
}

func TestParseArgsWithFeriados(t *testing.T) {
	cfg, err := parseArgs([]string{"Mar-2026", "--feriados", "19,25"})
	if err != nil {
		t.Fatalf("parseArgs: unexpected error: %v", err)
	}
	if len(cfg.feriados) != 2 || cfg.feriados[0] != 19 || cfg.feriados[1] != 25 {
		t.Errorf("feriados = %v, want [19 25]", cfg.feriados)
	}
}

func TestParseArgsInvalid(t *testing.T) {
	invalidArgs := [][]string{
		{"invalido"},
		{"Mar-26"},
		{},
		{"Mar-2026", "--modo", "errado"},
		{"Mar-2026", "--feriados"},
	}
	for _, args := range invalidArgs {
		_, err := parseArgs(args)
		if err == nil {
			t.Errorf("parseArgs(%v): esperava erro, got nil", args)
		}
	}
}

// ── HTTP handler tests ──

func TestHandleAPI_MissingMonthParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc", nil)
	w := httptest.NewRecorder()
	handleAPI(w, req)

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

func TestHandleAPI_ValidMonth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc?month=Mar-2026", nil)
	w := httptest.NewRecorder()
	handleAPI(w, req)

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

func TestHandleAPI_InvalidModo(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/calc?month=Mar-2026&modo=invalid", nil)
	w := httptest.NewRecorder()
	handleAPI(w, req)

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

func TestServeIndex(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	serveIndex(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Fatalf("expected Content-Type text/html; charset=utf-8, got %s", ct)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Fatal("expected non-empty body for index.html")
	}
}

func TestMuxRouting(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/api/calc", handleAPI)

	// Test / serves index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /, got %d", w.Code)
	}

	// Test /api/calc without month returns 400
	req2 := httptest.NewRequest(http.MethodGet, "/api/calc", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for /api/calc without month, got %d", w2.Code)
	}

	// Test /api/calc with month returns 200
	req3 := httptest.NewRequest(http.MethodGet, "/api/calc?month=Mar-2026", nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	if w3.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/calc?month=Mar-2026, got %d; body: %s", w3.Code, w3.Body.String())
	}
}

// ── Dois eixos: extra (16/8) e distribuição (atual/meia/uniforme) ──

type calcResp struct {
	Modo        string  `json:"modo"`
	DiasUteis   int     `json:"dias_uteis"`
	NumFeriados int     `json:"num_feriados"`
	HorasBase   float64 `json:"horas_base"`
	HorasExtra  float64 `json:"horas_extra"`
	HorasTotal  float64 `json:"horas_total"`
	Warning     string  `json:"warning"`
	Weeks       []struct {
		Days []struct {
			D         int     `json:"d"`
			Claim     string  `json:"claim"`
			CertPonto string  `json:"certponto"`
			IsFeriado bool    `json:"is_feriado"`
			IsWorking bool    `json:"is_working"`
			Extra     float64 `json:"extra"`
		} `json:"days"`
	} `json:"weeks"`
}

func doCalc(t *testing.T, query string) (int, calcResp, map[string]string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/calc?"+query, nil)
	w := httptest.NewRecorder()
	handleAPI(w, req)
	var body calcResp
	var errBody map[string]string
	if w.Code == http.StatusOK {
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("resposta nao e JSON valido: %v", err)
		}
		return w.Code, body, nil
	}
	json.Unmarshal(w.Body.Bytes(), &errBody)
	return w.Code, body, errBody
}

func TestHandleAPI_DefaultExtra16(t *testing.T) {
	code, body, _ := doCalc(t, "month=Mar-2026")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body.Modo != "Jeito atual" || body.HorasExtra != 16 || body.HorasTotal != 192 {
		t.Errorf("default = %q/%.1f/%.1f, want Jeito atual/16/192", body.Modo, body.HorasExtra, body.HorasTotal)
	}
}

func TestHandleAPI_ModoMeiaOK(t *testing.T) {
	code, body, _ := doCalc(t, "month=Mar-2026&modo=meia&extra=8")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body.Modo != "Meia hora" || body.HorasExtra != 8 || body.HorasTotal != 184 {
		t.Errorf("meia = %q/%.1f/%.1f, want Meia hora/8/184", body.Modo, body.HorasExtra, body.HorasTotal)
	}
}

func TestHandleAPI_MeiaRequiresExtra8(t *testing.T) {
	for _, q := range []string{"month=Mar-2026&modo=meia", "month=Mar-2026&modo=meia&extra=16"} {
		code, _, errBody := doCalc(t, q)
		if code != http.StatusBadRequest {
			t.Errorf("%q: expected 400, got %d", q, code)
			continue
		}
		if !strings.Contains(errBody["error"], "modo meia requer extra=8") {
			t.Errorf("%q: error = %q, want mensagem de recusa compartilhada", q, errBody["error"])
		}
	}
}

func TestHandleAPI_ExtraInvalid(t *testing.T) {
	code, _, errBody := doCalc(t, "month=Mar-2026&extra=abc")
	if code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", code)
	}
	if _, ok := errBody["error"]; !ok {
		t.Errorf("expected 'error' key, got %v", errBody)
	}
}

func TestHandleAPI_ExtraValid(t *testing.T) {
	code, body, _ := doCalc(t, "month=Mar-2026&extra=8&modo=uniforme")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body.HorasExtra != 8 || body.HorasTotal != 184 {
		t.Errorf("extra/total = %.1f/%.1f, want 8/184", body.HorasExtra, body.HorasTotal)
	}
}

func TestHandleAPI_FeriadoNoJSON(t *testing.T) {
	code, body, _ := doCalc(t, "month=Mar-2026&feriados=19")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body.NumFeriados != 1 {
		t.Errorf("num_feriados = %d, want 1", body.NumFeriados)
	}
	achou := false
	for _, w := range body.Weeks {
		for _, d := range w.Days {
			if d.D == 19 {
				achou = true
				if !d.IsFeriado || d.IsWorking {
					t.Errorf("dia 19: is_feriado=%v is_working=%v, want true/false", d.IsFeriado, d.IsWorking)
				}
				if d.Claim != "8.0" || d.CertPonto != "FERIADO" || d.Extra != 0 {
					t.Errorf("dia 19: claim=%q certponto=%q extra=%.1f, want 8.0/FERIADO/0", d.Claim, d.CertPonto, d.Extra)
				}
			}
		}
	}
	if !achou {
		t.Error("feriado 19 nao apareceu no JSON (D22)")
	}
}

func TestHandleAPI_FeriadoDuplicado(t *testing.T) {
	code, body, _ := doCalc(t, "month=Mar-2026&feriados=19,19")
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if body.NumFeriados != 1 {
		t.Errorf("num_feriados com duplicata = %d, want 1", body.NumFeriados)
	}
	linhas := 0
	for _, w := range body.Weeks {
		for _, d := range w.Days {
			if d.IsFeriado {
				linhas++
			}
		}
	}
	if linhas != 1 {
		t.Errorf("linhas de feriado com duplicata = %d, want 1", linhas)
	}
}

func TestParseArgsExtra(t *testing.T) {
	cfg, err := parseArgs([]string{"Mar-2026", "--extra", "8"})
	if err != nil {
		t.Fatalf("parseArgs: unexpected error: %v", err)
	}
	if cfg.extra != 8 {
		t.Errorf("extra = %.1f, want 8", cfg.extra)
	}
	if _, err := parseArgs([]string{"Mar-2026", "--extra", "7"}); err == nil {
		t.Error("--extra 7 deveria falhar")
	}
}

func TestParseArgsMeiaCom16(t *testing.T) {
	_, err := parseArgs([]string{"Mar-2026", "--modo", "meia"})
	if err == nil {
		t.Fatal("--modo meia sem --extra 8 deveria falhar")
	}
	if !strings.Contains(err.Error(), "modo meia requer extra=8") {
		t.Errorf("mensagem = %q, want a mesma do 400 da API", err.Error())
	}
	cfg, err := parseArgs([]string{"Mar-2026", "--modo", "meia", "--extra", "8"})
	if err != nil {
		t.Fatalf("--modo meia --extra 8: unexpected error: %v", err)
	}
	if cfg.modoBloco != calc.BlocoMeiaHora || cfg.extra != 8 {
		t.Errorf("bloco/extra = %.1f/%.1f, want 0.5/8", cfg.modoBloco, cfg.extra)
	}
}
