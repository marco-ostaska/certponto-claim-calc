package calc

import (
	"math"
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
		{2026, time.February, nil, 20},        // fev 2026: 28 dias, 4 sáb, 4 dom = 20 úteis
		{2026, time.March, []int{19}, 21},     // março com 1 feriado
		{2026, time.March, []int{19, 20}, 20}, // março com 2 feriados
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
	plan := CalcModoA(2026, time.March, nil, 16, BlocoHoraCheia)

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
		plan := CalcModoA(2026, m, nil, 16, BlocoHoraCheia)
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
	plan := CalcModoA(2026, time.March, []int{19}, 16, BlocoHoraCheia)

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
	plan := CalcModoB(2026, time.March, nil, 16)

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
	plan := CalcModoB(2026, time.March, nil, 16)
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
	plan := CalcModoB(2026, time.March, nil, 16)
	if !plan.ModoUniforme {
		t.Error("ModoUniforme deve ser true para CalcModoB")
	}
}

func TestCapacity_SomenteSextas(t *testing.T) {
	// Com todos os seg-qui como feriado restam 4 sextas, cada uma numa semana que
	// perdeu seg-qui => 1h de capacidade cada (D14), total 4h.
	allMonThu := []int{2, 3, 4, 5, 9, 10, 11, 12, 16, 17, 18, 19, 23, 24, 25, 26, 30, 31}
	if got := Capacity(2026, time.March, allMonThu); got != 4.0 {
		t.Errorf("Capacity com só sextas = %.1f, want 4.0", got)
	}
}

func TestCapacity_Weeks(t *testing.T) {
	tests := []struct {
		year     int
		month    time.Month
		feriados []int
		want     float64
	}{
		{2026, time.March, nil, 18},           // 4 semanas cheias (4h) + semana final de 2 dias (2h)
		{2026, time.February, nil, 16},        // 4 semanas cheias
		{2027, time.October, []int{12}, 16},   // sexta 01/10 sozinha não conta; as 4 semanas seguintes fecham 16h
		{2026, time.February, []int{17}, 16},  // semana com 1 feriado usa a sexta e mantém 4h
		{2026, time.March, []int{17, 18}, 17}, // semana com 2 feriados perde 1h
		{2026, time.February, []int{17, 18}, 15},
	}
	for _, tt := range tests {
		if got := Capacity(tt.year, tt.month, tt.feriados); got != tt.want {
			t.Errorf("Capacity(%d, %v, %v) = %.1f, want %.1f", tt.year, tt.month, tt.feriados, got, tt.want)
		}
	}
}

func TestModoA_FeriadoNaSemana(t *testing.T) {
	// Mar/2026 com feriado em 19 (quinta): a semana 16-20 precisa somar 44h,
	// então a sexta 20 também recebe 1h (redistribuição semanal, D14).
	plan := CalcModoA(2026, time.March, []int{19}, 16, BlocoHoraCheia)
	if plan.TotalHours != 184 {
		t.Errorf("TotalHours = %.1f, want 184", plan.TotalHours)
	}
	got := map[int]float64{}
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			got[d.Date.Day()] = d.Hours
		}
	}
	if got[19] != BaseHoursPerDay {
		t.Errorf("feriado 19 com %.1fh, want 8", got[19])
	}
	if got[20] != DefaultMonThu {
		t.Errorf("sexta 20 com %.1fh, want 9 (exceção da sexta)", got[20])
	}
}

func TestModoA_Teto8(t *testing.T) {
	plan := CalcModoA(2026, time.March, nil, 8, BlocoHoraCheia)
	if plan.TotalHours != 184 || plan.ExtraHours != 8 {
		t.Errorf("Total/Extra = %.1f/%.1f, want 184/8", plan.TotalHours, plan.ExtraHours)
	}
	nove, oito := 0, 0
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			switch d.Hours {
			case DefaultMonThu:
				nove++
			case BaseHoursPerDay:
				oito++
			}
		}
	}
	if nove != 8 || oito != 14 {
		t.Errorf("dias 9h/8h = %d/%d, want 8/14", nove, oito)
	}
}

func TestModoMeia_Teto8(t *testing.T) {
	plan := CalcModoA(2026, time.March, nil, 8, BlocoMeiaHora)
	if plan.TotalHours != 184 {
		t.Errorf("TotalHours = %.1f, want 184", plan.TotalHours)
	}
	n := 0
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			if d.Hours == BaseHoursPerDay+BlocoMeiaHora {
				n++
			}
		}
	}
	if n != 16 {
		t.Errorf("dias de 8,5h = %d, want 16", n)
	}
}

func TestModoMeia_Promocao(t *testing.T) {
	// Fev/2026 com 17 e 18: a alocação a 0,5h só alcança 7,5h, então 1 dia é
	// promovido para 9h (D12) e o mês fecha 152h.
	plan := CalcModoA(2026, time.February, []int{17, 18}, 8, BlocoMeiaHora)
	if plan.TotalHours != 152 || plan.ExtraHours != 8 {
		t.Errorf("Total/Extra = %.1f/%.1f, want 152/8", plan.TotalHours, plan.ExtraHours)
	}
	nove := 0
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			if d.Hours == DefaultMonThu {
				nove++
			}
		}
	}
	if nove != 1 {
		t.Errorf("dias de 9h = %d, want 1 (promoção)", nove)
	}
}

func TestModoA_MesCurto(t *testing.T) {
	plan := CalcModoA(2026, time.February, []int{17, 18}, 16, BlocoHoraCheia)
	if plan.ExtraHours != 15 || plan.TotalHours != 159 {
		t.Errorf("Extra/Total = %.1f/%.1f, want 15/159", plan.ExtraHours, plan.TotalHours)
	}
	if plan.Warning == "" {
		t.Error("esperava aviso de teto inatingível")
	}
}

func TestTodosDiasUteisAbaixoDe9h(t *testing.T) {
	// `uniforme` fica de fora: D23 diz que ele não tem teto diário.
	meses := []time.Month{time.January, time.February, time.March, time.April, time.May, time.June,
		time.July, time.August, time.September, time.October, time.November, time.December}
	for _, m := range meses {
		for _, target := range []float64{ExtraMonthly, ExtraOito} {
			for _, bloco := range []float64{BlocoHoraCheia, BlocoMeiaHora} {
				plan := CalcModoA(2026, m, nil, target, bloco)
				for _, w := range plan.Weeks {
					for _, d := range w.Days {
						if d.Hours > DefaultMonThu+1e-9 {
							t.Errorf("mês %v target %.0f bloco %.1f: dia %d com %.2fh",
								m, target, bloco, d.Date.Day(), d.Hours)
						}
					}
				}
			}
		}
	}
}

func TestFeriadoNoPlano(t *testing.T) {
	plan := CalcModoA(2026, time.March, []int{19}, 16, BlocoHoraCheia)
	if plan.TotalDays != 21 {
		t.Errorf("TotalDays = %d, want 21 (feriado não é dia útil)", plan.TotalDays)
	}
	visto := false
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			if d.Date.Day() == 19 {
				visto = true
				if d.Hours != BaseHoursPerDay {
					t.Errorf("feriado 19 com %.1fh, want 8", d.Hours)
				}
			}
		}
	}
	if !visto {
		t.Error("feriado 19 não apareceu no plano (D22)")
	}
}

func TestModoB_MesSemDiaUtil(t *testing.T) {
	// Todos os dias seg-sex como feriado: totais de trabalho zeram, mas as linhas
	// de feriado continuam no plano (D17/D22).
	todos := []int{2, 3, 4, 5, 6, 9, 10, 11, 12, 13, 16, 17, 18, 19, 20, 23, 24, 25, 26, 27, 30, 31}
	plan := CalcModoB(2026, time.March, todos, 16)
	if plan.TotalDays != 0 || plan.TotalHours != 0 || plan.ExtraHours != 0 {
		t.Errorf("totais = %d/%.1f/%.1f, want 0/0/0", plan.TotalDays, plan.TotalHours, plan.ExtraHours)
	}
	if plan.Warning == "" {
		t.Error("esperava aviso de mês sem dia útil")
	}
	linhas := 0
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			linhas++
			if d.Hours != BaseHoursPerDay {
				t.Errorf("feriado %d com %.1fh, want 8", d.Date.Day(), d.Hours)
			}
		}
	}
	if linhas != len(todos) {
		t.Errorf("linhas de feriado = %d, want %d", linhas, len(todos))
	}
}

func TestFeriadosDuplicados(t *testing.T) {
	if got := CountHolidays(2026, time.March, []int{19, 19}); got != 1 {
		t.Errorf("CountHolidays com duplicata = %d, want 1", got)
	}
}

func TestInvarianteSoma(t *testing.T) {
	casos := []struct {
		month    time.Month
		feriados []int
		target   float64
		bloco    float64
		uniforme bool
	}{
		{time.March, nil, ExtraMonthly, BlocoHoraCheia, false},
		{time.March, []int{19}, ExtraMonthly, BlocoHoraCheia, false},
		{time.March, nil, ExtraOito, BlocoHoraCheia, false},
		{time.March, nil, ExtraOito, BlocoMeiaHora, false},
		{time.February, []int{17, 18}, ExtraMonthly, BlocoHoraCheia, false},
		{time.February, []int{17, 18}, ExtraOito, BlocoMeiaHora, false},
		{time.March, nil, ExtraMonthly, BlocoHoraCheia, true},
		{time.March, []int{19}, ExtraOito, BlocoHoraCheia, true},
	}
	for _, c := range casos {
		var plan MonthPlan
		if c.uniforme {
			plan = CalcModoB(2026, c.month, c.feriados, c.target)
		} else {
			plan = CalcModoA(2026, c.month, c.feriados, c.target, c.bloco)
		}
		fer := map[int]bool{}
		for _, f := range c.feriados {
			fer[f] = true
		}
		var uteis, feriados float64
		for _, w := range plan.Weeks {
			for _, d := range w.Days {
				if fer[d.Date.Day()] {
					feriados += d.Hours
				} else {
					uteis += d.Hours
				}
			}
		}
		wantUteis := float64(plan.TotalDays)*BaseHoursPerDay + plan.ExtraHours
		wantFeriados := float64(plan.HolidayCount) * BaseHoursPerDay
		if math.Abs(uteis-wantUteis) > 1e-9 {
			t.Errorf("%v %v: soma úteis = %.2f, want %.2f", c.month, c.feriados, uteis, wantUteis)
		}
		if math.Abs(feriados-wantFeriados) > 1e-9 {
			t.Errorf("%v %v: soma feriados = %.2f, want %.2f", c.month, c.feriados, feriados, wantFeriados)
		}
		if (plan.ExtraHours < c.target) != (plan.Warning != "") {
			t.Errorf("%v %v target %.0f: extra=%.1f warning=%q", c.month, c.feriados, c.target, plan.ExtraHours, plan.Warning)
		}
		if math.Abs(plan.TotalHours-wantUteis) > 1e-9 {
			t.Errorf("%v: TotalHours=%.1f inconsistente com base+extra", c.month, plan.TotalHours)
		}
	}
}
