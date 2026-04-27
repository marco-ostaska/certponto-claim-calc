package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

var monthNames = map[string]time.Month{
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

func parseMonthYear(s string) (int, time.Month, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("formato inválido, use: Mmm-YYYY (ex: Mar-2026)")
	}
	m, ok := monthNames[parts[0]]
	if !ok {
		return 0, 0, fmt.Errorf("mês inválido %q, use abreviação em português (Jan, Fev, Mar...)", parts[0])
	}
	y, err := strconv.Atoi(parts[1])
	if err != nil || y < 2000 || y > 2100 {
		return 0, 0, fmt.Errorf("ano inválido %q, use formato YYYY (ex: 2026)", parts[1])
	}
	return y, m, nil
}

// Day representa um dia útil do mês
type Day struct {
	Date time.Time
}

func workdays(year int, month time.Month, feriados []int) []Day {
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
	baseHoursPerDay = 8.0
	extraMonthly    = 16.0
	defaultMonThu   = 9.0
	defaultFri      = 8.0
)

type WorkDay struct {
	Date  time.Time
	Hours float64
}

type Week struct {
	Number int
	Days   []WorkDay
}

type MonthPlan struct {
	Year         int
	Month        time.Month
	TotalDays    int
	TotalHours   float64
	ExtraHours   float64
	HolidayCount int
	Weeks        []Week
	ModoUniforme bool
}

func calcModoA(year int, month time.Month, feriados []int) MonthPlan {
	days := workdays(year, month, feriados)
	if maxExtra := maxPossibleExtra(days); maxExtra < extraMonthly {
		fmt.Fprintf(os.Stderr, "Aviso: nao e possivel atingir %.0fh de extra neste mes com os feriados informados (maximo: %.1fh)\n",
			extraMonthly, maxExtra)
	}
	holidayCount := countHolidays(year, month, feriados)
	totalHours := float64(len(days))*baseHoursPerDay + extraMonthly

	plan := MonthPlan{
		Year:         year,
		Month:        month,
		TotalDays:    len(days),
		TotalHours:   totalHours,
		ExtraHours:   extraMonthly,
		HolidayCount: holidayCount,
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
			if extraAccum >= extraMonthly {
				h = baseHoursPerDay
			} else if d.Date.Weekday() == time.Friday {
				h = defaultFri // Friday: always 8h (no extra)
			} else {
				// Mon-Thu: 9h if still accumulating
				h = defaultMonThu
				extraAccum += h - baseHoursPerDay
				if extraAccum > extraMonthly {
					overflow := extraAccum - extraMonthly
					h -= overflow
					extraAccum = extraMonthly
				}
			}
			week.Days = append(week.Days, WorkDay{Date: d.Date, Hours: h})
		}

		plan.Weeks = append(plan.Weeks, week)
	}

	return plan
}

func calcModoB(year int, month time.Month, feriados []int) MonthPlan {
	days := workdays(year, month, feriados)
	if maxExtra := maxPossibleExtra(days); maxExtra < extraMonthly {
		fmt.Fprintf(os.Stderr, "Aviso: nao e possivel atingir %.0fh de extra neste mes com os feriados informados (maximo: %.1fh)\n",
			extraMonthly, maxExtra)
	}
	holidayCount := countHolidays(year, month, feriados)
	totalHours := float64(len(days))*baseHoursPerDay + extraMonthly

	plan := MonthPlan{
		Year:         year,
		Month:        month,
		TotalDays:    len(days),
		TotalHours:   totalHours,
		ExtraHours:   extraMonthly,
		HolidayCount: holidayCount,
		ModoUniforme: true,
	}

	if len(days) == 0 {
		fmt.Fprintf(os.Stderr, "Erro: nenhum dia útil encontrado neste mês (todos são feriados?)\n")
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

// countHolidays counts how many of the given days are weekdays (not Sat/Sun)
func countHolidays(year int, month time.Month, feriados []int) int {
	count := 0
	for _, day := range feriados {
		d := time.Date(year, month, day, 0, 0, 0, 0, time.Local)
		if d.Month() == month && d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			count++
		}
	}
	return count
}

// maxPossibleExtra calculates the maximum extra hours achievable in a month.
// Only Mon-Thu can generate extra (1h each). Fri generates 0 extra.
// workdays() already excludes holidays, so we just count non-Friday days.
func maxPossibleExtra(days []Day) float64 {
	var max float64
	for _, d := range days {
		if d.Date.Weekday() != time.Friday {
			max += 1.0
		}
	}
	return max
}
