package calc

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// MonthNames maps Portuguese month abbreviations to time.Month values.
var MonthNames = map[string]time.Month{
	"Jan": time.January,
	"Fev": time.February,
	"Mar": time.March,
	"Abr": time.April,
	"Mai": time.May,
	"Jun": time.June,
	"Jul": time.July,
	"Ago": time.August,
	"Set": time.September,
	"Out": time.October,
	"Nov": time.November,
	"Dez": time.December,
}

// Config holds the parameters for a month calculation.
type Config struct {
	Year         int
	Month        time.Month
	Feriados     []int
	ModoUniforme bool
}

// ParseMonthYear parses a "Mmm-YYYY" string (e.g. "Mar-2026") into year and month.
func ParseMonthYear(s string) (int, time.Month, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("formato invalido, use: Mmm-YYYY (ex: Mar-2026)")
	}
	m, ok := MonthNames[parts[0]]
	if !ok {
		return 0, 0, fmt.Errorf("mes invalido %q, use abreviacao em portugues (Jan, Fev, Mar...)", parts[0])
	}
	y, err := strconv.Atoi(parts[1])
	if err != nil || y < 2000 || y > 2100 {
		return 0, 0, fmt.Errorf("ano invalido %q, use formato YYYY (ex: 2026)", parts[1])
	}
	return y, m, nil
}

// Day represents a working day in the month.
type Day struct {
	Date time.Time
}

// Workdays returns the list of working days in the given month, excluding weekends and holidays.
func Workdays(year int, month time.Month, feriados []int) []Day {
	feriadoSet := make(map[int]bool)
	for _, d := range feriados {
		feriadoSet[d] = true
	}

	var days []Day
	d := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	for d.Month() == month {
		wd := d.Weekday()
		if wd != time.Saturday && wd != time.Sunday && !feriadoSet[d.Day()] {
			days = append(days, Day{Date: d})
		}
		d = d.AddDate(0, 0, 1)
	}
	return days
}

const (
	BaseHoursPerDay = 8.0
	ExtraMonthly    = 16.0
	// ExtraOito is the monthly extra cap of the 42h/week contract (2h per week).
	ExtraOito     = 8.0
	DefaultMonThu = 9.0
	DefaultFri    = 8.0

	// BlocoHoraCheia and BlocoMeiaHora are the extra-hour block sizes per day.
	BlocoHoraCheia = 1.0
	BlocoMeiaHora  = 0.5
)

// WorkDay represents a single working day with its allocated hours.
type WorkDay struct {
	Date  time.Time
	Hours float64
}

// Week represents a week within the month plan.
type Week struct {
	Number int
	Days   []WorkDay
}

// MonthPlan holds the complete calculation result for a month.
type MonthPlan struct {
	Year         int
	Month        time.Month
	TotalDays    int
	TotalHours   float64
	ExtraHours   float64
	HolidayCount int
	Weeks        []Week
	ModoUniforme bool
	Warning      string
}

// CalcModoA builds the plan filling the monthly extra target chronologically,
// `bloco` hours per eligible day (BlocoHoraCheia = 1h, BlocoMeiaHora = 0.5h),
// capped by Capacity(). A Friday only takes extra when its week lost a Mon-Thu
// day to a holiday. Holidays are plan days with base hours and no extra.
func CalcModoA(year int, month time.Month, feriados []int, target, bloco float64) MonthPlan {
	days := Workdays(year, month, feriados)
	extra := math.Min(target, Capacity(year, month, feriados, target))

	plan := MonthPlan{
		Year:         year,
		Month:        month,
		TotalDays:    len(days),
		TotalHours:   float64(len(days))*BaseHoursPerDay + extra,
		ExtraHours:   extra,
		HolidayCount: CountHolidays(year, month, feriados),
		Warning:      warningFor(target, extra, len(days)),
	}

	placed := 0.0
	cota := math.Min(target/4, 4)
	for i, wk := range monthWeeks(year, month) {
		week := Week{Number: i + 1}
		friOK := weekLostMonThu(wk, feriados)

		// Orçamento da semana: a cota do contrato limitada pelos dias elegíveis
		// e pelo que ainda falta para fechar o teto mensal.
		budget := 0.0
		for _, d := range wk {
			if !isFeriado(d, feriados) && eligibleDay(d, friOK) {
				budget += bloco
			}
		}
		budget = math.Min(budget, cota)
		budget = math.Min(budget, extra-placed)

		weekPlaced := 0.0
		for _, d := range wk {
			if isFeriado(d, feriados) {
				week.Days = append(week.Days, WorkDay{Date: d, Hours: BaseHoursPerDay})
				continue
			}
			add := 0.0
			if weekPlaced < budget && eligibleDay(d, friOK) {
				add = math.Min(bloco, budget-weekPlaced)
				weekPlaced += add
				placed += add
			}
			week.Days = append(week.Days, WorkDay{Date: d, Hours: BaseHoursPerDay + add})
		}
		plan.Weeks = append(plan.Weeks, week)
	}

	// D12: quando o bloco de 0,5h não fecha o teto, promove dias para 1h (máx 9h/dia
	// e sem passar da cota semanal do contrato).
	for wi := range plan.Weeks {
		weeksExtra := 0.0
		for _, d := range plan.Weeks[wi].Days {
			weeksExtra += d.Hours - BaseHoursPerDay
		}
		for di := range plan.Weeks[wi].Days {
			if placed >= extra || weeksExtra >= cota-1e-9 {
				break
			}
			d := &plan.Weeks[wi].Days[di]
			if d.Hours > BaseHoursPerDay && d.Hours < DefaultMonThu {
				inc := math.Min(DefaultMonThu-d.Hours, extra-placed)
				inc = math.Min(inc, cota-weeksExtra)
				d.Hours += inc
				placed += inc
				weeksExtra += inc
			}
		}
	}

	return plan
}

// CalcModoB calculates the monthly plan using uniform (evenly distributed) mode.
// Hours are distributed equally across all working days, which always reaches
// the target when there is at least one working day.
func CalcModoB(year int, month time.Month, feriados []int, target float64) MonthPlan {
	days := Workdays(year, month, feriados)
	plan := MonthPlan{
		Year:         year,
		Month:        month,
		TotalDays:    len(days),
		TotalHours:   float64(len(days))*BaseHoursPerDay + target,
		ExtraHours:   target,
		HolidayCount: CountHolidays(year, month, feriados),
		ModoUniforme: true,
	}

	// hoursPerDay só existe com pelo menos um dia útil; sem ele o mês é
	// composto apenas por feriados e os totais de trabalho vão a zero.
	var hoursPerDay float64
	var extraDays int
	if len(days) > 0 {
		total := float64(len(days))*BaseHoursPerDay + target
		hoursPerDay = math.Floor(total/float64(len(days))*4) / 4
		extraDays = int(math.Round((total - hoursPerDay*float64(len(days))) / 0.25))
	} else {
		plan.TotalHours = 0
		plan.ExtraHours = 0
		plan.Warning = "Erro: nenhum dia util encontrado neste mes (todos sao feriados?)"
	}

	// Feriados entram como dias de 8h; o contador do resto só anda em dias úteis.
	dayIndex := 0
	for i, wk := range monthWeeks(year, month) {
		week := Week{Number: i + 1}
		for _, d := range wk {
			if isFeriado(d, feriados) {
				week.Days = append(week.Days, WorkDay{Date: d, Hours: BaseHoursPerDay})
				continue
			}
			h := hoursPerDay
			if dayIndex >= len(days)-extraDays {
				h += 0.25
			}
			week.Days = append(week.Days, WorkDay{Date: d, Hours: h})
			dayIndex++
		}
		plan.Weeks = append(plan.Weeks, week)
	}

	return plan
}

// CountHolidays counts how many distinct days of the given list are weekdays
// (not Sat/Sun) in the given month.
func CountHolidays(year int, month time.Month, feriados []int) int {
	seen := make(map[int]bool, len(feriados))
	count := 0
	for _, day := range feriados {
		if seen[day] {
			continue
		}
		seen[day] = true
		d := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		if d.Month() == month && d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			count++
		}
	}
	return count
}

// Capacity returns the maximum extra hours the month can carry following the
// weekly redistribution rule: a Friday only takes extra when its week lost a
// Mon-Thu day. The weekly allowance comes from the contract — 4h per week for
// the 16h target (44h/week) and 2h per week for the 8h target (42h/week) — so
// no week can exceed its contract limit.
func Capacity(year int, month time.Month, feriados []int, target float64) float64 {
	cota := math.Min(target/4, 4)
	var total float64
	for _, wk := range monthWeeks(year, month) {
		monThu, fri := 0, false
		for _, d := range wk {
			if isFeriado(d, feriados) {
				continue
			}
			if d.Weekday() == time.Friday {
				fri = true
			} else {
				monThu++
			}
		}
		friOK := 0
		if fri && weekLostMonThu(wk, feriados) {
			friOK = 1
		}
		total += math.Min(cota, float64(monThu+friOK))
	}
	return total
}

// monthWeeks groups every Mon-Fri day of the month by ISO week. Holidays are
// included (they are plan days); weekends never are. Weeks come out in
// first-seen (chronological) order and each week's days are in date order —
// grouping by first occurrence avoids the ISO-year wrap at Dec/Jan, which is
// what makes Week.Number and the chronological fill deterministic.
func monthWeeks(year int, month time.Month) [][]time.Time {
	var order []string
	buckets := map[string][]time.Time{}
	for d := time.Date(year, month, 1, 0, 0, 0, 0, time.Local); d.Month() == month; d = d.AddDate(0, 0, 1) {
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			continue
		}
		y, w := d.ISOWeek()
		key := fmt.Sprintf("%04d-%02d", y, w)
		if _, ok := buckets[key]; !ok {
			order = append(order, key)
		}
		buckets[key] = append(buckets[key], d)
	}
	weeks := make([][]time.Time, 0, len(order))
	for _, key := range order {
		weeks = append(weeks, buckets[key])
	}
	return weeks
}

// weekLostMonThu reports whether the week lost at least one Mon-Thu day to a
// holiday — the only case where Friday may take extra. A holiday that falls on
// Friday does NOT make the Friday eligible.
func weekLostMonThu(wk []time.Time, feriados []int) bool {
	for _, d := range wk {
		if d.Weekday() < time.Friday && isFeriado(d, feriados) {
			return true
		}
	}
	return false
}

// isFeriado reports whether d is in the feriados list (day of month).
func isFeriado(d time.Time, feriados []int) bool {
	for _, f := range feriados {
		if f == d.Day() {
			return true
		}
	}
	return false
}

// eligibleDay reports whether d may take an extra block: every Mon-Thu day,
// and a Friday only when its week lost a Mon-Thu day (friOK). Weekends never
// reach this function — monthWeeks only returns Mon-Fri.
func eligibleDay(d time.Time, friOK bool) bool {
	if d.Weekday() == time.Friday {
		return friOK
	}
	return d.Weekday() < time.Friday
}

// warningFor builds the warning of an unreachable target. Precedência: mês sem
// dia útil vence; depois E < target; senão string vazia.
func warningFor(target, extra float64, workdays int) string {
	if workdays == 0 {
		return "Erro: nenhum dia util encontrado neste mes (todos sao feriados?)"
	}
	if extra < target {
		return fmt.Sprintf("Aviso: nao e possivel atingir %.0fh de extra neste mes com os feriados informados (maximo: %.1fh)",
			target, extra)
	}
	return ""
}
