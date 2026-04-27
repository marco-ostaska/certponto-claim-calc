package calc

import (
	"testing"
	"time"
)

func TestWorkdays(t *testing.T) {
	tests := []struct {
		year     int
		month    time.Month
		feriados []int
		wantDays int
	}{
		{2026, time.March, nil, 22},           // março 2026: 31 dias, 4 sáb, 5 dom = 22 úteis
		{2026, time.February, nil, 20},         // fev 2026: 28 dias, 4 sáb, 4 dom = 20 úteis
		{2026, time.March, []int{19}, 21},      // março com 1 feriado
		{2026, time.March, []int{19, 20}, 20},  // março com 2 feriados
	}
	for _, tt := range tests {
		days := Workdays(tt.year, tt.month, tt.feriados)
		if len(days) != tt.wantDays {
			t.Errorf("Workdays(%d, %v, %v) = %d dias, want %d", tt.year, tt.month, tt.feriados, len(days), tt.wantDays)
		}
	}
}

func TestParseMonthYear(t *testing.T) {
	tests := []struct {
		input   string
		wantY   int
		wantM   time.Month
		wantErr bool
	}{
		{"Mar-2026", 2026, time.March, false},
		{"Jan-2024", 2024, time.January, false},
		{"Dez-2025", 2025, time.December, false},
		{"Mar-26", 0, 0, true},
		{"março-2026", 0, 0, true},
		{"", 0, 0, true},
	}
	for _, tt := range tests {
		y, m, err := ParseMonthYear(tt.input)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseMonthYear(%q): esperava erro, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseMonthYear(%q): erro inesperado: %v", tt.input, err)
		}
		if y != tt.wantY || m != tt.wantM {
			t.Errorf("ParseMonthYear(%q) = (%d, %v), want (%d, %v)", tt.input, y, m, tt.wantY, tt.wantM)
		}
	}
}

func TestModoA_MarchNoHolidays(t *testing.T) {
	plan := CalcModoA(2026, time.March, nil)

	if plan.TotalDays != 22 {
		t.Errorf("TotalDays = %d, want 22", plan.TotalDays)
	}
	// total = 22*8 + 16 = 192
	if plan.TotalHours != 192 {
		t.Errorf("TotalHours = %.1f, want 192", plan.TotalHours)
	}

	// Extra accumulated must not exceed 16h
	var extra float64
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			extra += d.Hours - BaseHoursPerDay
		}
	}
	if extra > ExtraMonthly+0.01 {
		t.Errorf("extra acumulada = %.2f, nao deve exceder %.1f", extra, ExtraMonthly)
	}
}

func TestModoA_ExtraDoesNotExceed16(t *testing.T) {
	months := []time.Month{
		time.January, time.March, time.May,
		time.July, time.August, time.October, time.December,
	}
	for _, m := range months {
		plan := CalcModoA(2026, m, nil)
		var extra float64
		for _, w := range plan.Weeks {
			for _, d := range w.Days {
				extra += d.Hours - BaseHoursPerDay
			}
		}
		if extra > ExtraMonthly+0.01 {
			t.Errorf("mês %v: extra = %.2f excede %.1f", m, extra, ExtraMonthly)
		}
	}
}

func TestModoA_WithHoliday(t *testing.T) {
	// March 2026 with holiday on day 19 (Thursday)
	// workdays = 21 (one less), total = 21*8+16 = 184
	plan := CalcModoA(2026, time.March, []int{19})

	if plan.TotalDays != 21 {
		t.Errorf("TotalDays = %d, want 21", plan.TotalDays)
	}
	if plan.TotalHours != 184 {
		t.Errorf("TotalHours = %.1f, want 184", plan.TotalHours)
	}
	if plan.HolidayCount != 1 {
		t.Errorf("HolidayCount = %d, want 1", plan.HolidayCount)
	}

	var extra float64
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			extra += d.Hours - BaseHoursPerDay
		}
	}
	if extra > ExtraMonthly+0.01 {
		t.Errorf("extra = %.2f excede %.1f", extra, ExtraMonthly)
	}
}

func TestModoB_UniformDistribution(t *testing.T) {
	plan := CalcModoB(2026, time.March, nil)

	if plan.TotalDays != 22 {
		t.Errorf("TotalDays = %d, want 22", plan.TotalDays)
	}

	// Days should differ by at most 0.25h (remainder distribution)
	var minH, maxH float64
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			if minH == 0 || d.Hours < minH {
				minH = d.Hours
			}
			if d.Hours > maxH {
				maxH = d.Hours
			}
		}
	}
	if maxH-minH > 0.25+1e-9 {
		t.Errorf("variacao entre dias = %.2f, deve ser <= 0.25h", maxH-minH)
	}
}

func TestModoB_TotalHoursCorrect(t *testing.T) {
	plan := CalcModoB(2026, time.March, nil)
	var total float64
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			total += d.Hours
		}
	}
	expected := float64(plan.TotalDays)*BaseHoursPerDay + ExtraMonthly // 192
	// Allow tolerance of 0.25h (one 0.25h quantum due to remainder distribution)
	diff := total - expected
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.25+1e-9 {
		t.Errorf("total = %.2f, want %.2f (diff %.2f)", total, expected, diff)
	}
}

func TestModoB_ModoUniformeFlag(t *testing.T) {
	plan := CalcModoB(2026, time.March, nil)
	if !plan.ModoUniforme {
		t.Error("ModoUniforme deve ser true para CalcModoB")
	}
}

func TestMaxPossibleExtra(t *testing.T) {
	// March 2026 no holidays: many Mon-Thu available, easily > 16h
	days := Workdays(2026, time.March, nil)
	max := MaxPossibleExtra(days)
	if max < 16.0 {
		t.Errorf("março sem feriados: MaxPossibleExtra = %.1f, should be >= 16", max)
	}

	// March 2026 with all Mon-Thu as holidays (days 2,3,4,5,9,10,11,12,16,17,18,19,23,24,25,26,30,31)
	// Only Fridays remain: 6,13,20,27 — zero extra possible
	allMonThu := []int{2, 3, 4, 5, 9, 10, 11, 12, 16, 17, 18, 19, 23, 24, 25, 26, 30, 31}
	days2 := Workdays(2026, time.March, allMonThu)
	max2 := MaxPossibleExtra(days2)
	if max2 >= 16.0 {
		t.Errorf("março com todos seg-qui como feriado: MaxPossibleExtra = %.1f, should be < 16", max2)
	}
	if max2 != 0.0 {
		t.Errorf("com só sextas: MaxPossibleExtra = %.1f, should be 0.0", max2)
	}
}