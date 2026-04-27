package main

import (
	"testing"
	"time"

	"certponto-claim-calc/internal/calc"
)

func TestFullPlanMarch2026NoHolidays(t *testing.T) {
	plan := calc.CalcModoA(2026, time.March, nil)

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
	planA := calc.CalcModoA(2026, time.March, nil)
	planB := calc.CalcModoB(2026, time.March, nil)
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