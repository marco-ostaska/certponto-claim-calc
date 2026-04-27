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
	DefaultMonThu   = 9.0
	DefaultFri      = 8.0
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

// CalcModoA calculates the monthly plan using the current (non-uniform) mode.
// Extra hours are concentrated on Mon-Thu until the monthly target is reached.
func CalcModoA(year int, month time.Month, feriados []int) MonthPlan {
	days := Workdays(year, month, feriados)
	var warning string
	if maxExtra := MaxPossibleExtra(days); maxExtra < ExtraMonthly {
		warning = fmt.Sprintf("Aviso: nao e possivel atingir %.0fh de extra neste mes com os feriados informados (maximo: %.1fh)",
			ExtraMonthly, maxExtra)
	}
	holidayCount := CountHolidays(year, month, feriados)
	totalHours := float64(len(days))*BaseHoursPerDay + ExtraMonthly

	plan := MonthPlan{
		Year:         year,
		Month:        month,
		TotalDays:    len(days),
		TotalHours:   totalHours,
		ExtraHours:   ExtraMonthly,
		HolidayCount: holidayCount,
		Warning:      warning,
	}

	// Group days by ISO week
	weekMap := make(map[int][]Day)
	weekOrder := []int{}
	for _, d := range days {
		_, w := d.Date.ISOWeek()
		if _, exists := weekMap[w]; !exists {
			weekOrder = append(weekOrder, w)
		}
		weekMap[w] = append(weekMap[w], d)
	}

	var extraAccum float64

	for i, wNum := range weekOrder {
		wDays := weekMap[wNum]
		week := Week{Number: i + 1}

		for _, d := range wDays {
			var h float64
			if extraAccum >= ExtraMonthly {
				h = BaseHoursPerDay
			} else if d.Date.Weekday() == time.Friday {
				h = DefaultFri // Friday: always 8h (no extra)
			} else {
				// Mon-Thu: 9h if still accumulating
				h = DefaultMonThu
				extraAccum += h - BaseHoursPerDay
				if extraAccum > ExtraMonthly {
					overflow := extraAccum - ExtraMonthly
					h -= overflow
					extraAccum = ExtraMonthly
				}
			}
			week.Days = append(week.Days, WorkDay{Date: d.Date, Hours: h})
		}

		plan.Weeks = append(plan.Weeks, week)
	}

	return plan
}

// CalcModoB calculates the monthly plan using uniform (evenly distributed) mode.
// Hours are distributed equally across all working days.
func CalcModoB(year int, month time.Month, feriados []int) MonthPlan {
	days := Workdays(year, month, feriados)
	var warning string
	if maxExtra := MaxPossibleExtra(days); maxExtra < ExtraMonthly {
		warning = fmt.Sprintf("Aviso: nao e possivel atingir %.0fh de extra neste mes com os feriados informados (maximo: %.1fh)",
			ExtraMonthly, maxExtra)
	}
	holidayCount := CountHolidays(year, month, feriados)
	totalHours := float64(len(days))*BaseHoursPerDay + ExtraMonthly

	plan := MonthPlan{
		Year:         year,
		Month:        month,
		TotalDays:    len(days),
		TotalHours:   totalHours,
		ExtraHours:   ExtraMonthly,
		HolidayCount: holidayCount,
		ModoUniforme: true,
		Warning:      warning,
	}

	if len(days) == 0 {
		plan.Warning = "Erro: nenhum dia util encontrado neste mes (todos sao feriados?)"
		return plan
	}

	// Floor to nearest 0.25h to avoid exceeding totalHours.
	// Distribute remainder (in 0.25h increments) to the last days.
	hoursPerDay := math.Floor(totalHours/float64(len(days))*4) / 4
	remainder := totalHours - hoursPerDay*float64(len(days))
	// How many days get an extra 0.25h
	extraDays := int(math.Round(remainder / 0.25))

	// Group by ISO week
	weekMap := make(map[int][]Day)
	weekOrder := []int{}
	for _, d := range days {
		_, w := d.Date.ISOWeek()
		if _, exists := weekMap[w]; !exists {
			weekOrder = append(weekOrder, w)
		}
		weekMap[w] = append(weekMap[w], d)
	}

	// Assign hours day by day; last extraDays days get +0.25h
	dayIndex := 0
	for i, wNum := range weekOrder {
		week := Week{Number: i + 1}
		for _, d := range weekMap[wNum] {
			h := hoursPerDay
			if dayIndex >= len(days)-extraDays {
				h += 0.25
			}
			week.Days = append(week.Days, WorkDay{Date: d.Date, Hours: h})
			dayIndex++
		}
		plan.Weeks = append(plan.Weeks, week)
	}

	return plan
}

// CountHolidays counts how many of the given days are weekdays (not Sat/Sun).
func CountHolidays(year int, month time.Month, feriados []int) int {
	count := 0
	for _, day := range feriados {
		d := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		if d.Month() == month && d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			count++
		}
	}
	return count
}

// MaxPossibleExtra calculates the maximum extra hours achievable in a month.
// Only Mon-Thu can generate extra (1h each). Fri generates 0 extra.
// Workdays() already excludes holidays, so we just count non-Friday days.
func MaxPossibleExtra(days []Day) float64 {
	var max float64
	for _, d := range days {
		if d.Date.Weekday() != time.Friday {
			max += 1.0
		}
	}
	return max
}