package main

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// formatClaim converts decimal hours to Claim system string.
// Examples: 8.0 -> "8.0", 8.5 -> "8.5", 8.75 -> "8.75"
func formatClaim(hours float64) string {
	s := fmt.Sprintf("%.2f", hours)
	s = strings.TrimRight(s, "0")
	if strings.HasSuffix(s, ".") {
		s += "0"
	}
	return s
}

// formatCertPonto calculates entry time given hours to work.
// Exit is always 17:00. Lunch (1h) is not counted as work time.
// entry = 17:00 - hours - 1h(lunch)
func formatCertPonto(hours float64) (entrada, saida string) {
	totalMinutes := int(math.Round((hours + 1.0) * 60))
	endMinutes := 17 * 60
	startMinutes := endMinutes - totalMinutes
	h := startMinutes / 60
	m := startMinutes % 60
	return fmt.Sprintf("%02d:%02d", h, m), "17:00"
}

var weekdayPT = map[time.Weekday]string{
	time.Monday:    "Seg",
	time.Tuesday:   "Ter",
	time.Wednesday: "Qua",
	time.Thursday:  "Qui",
	time.Friday:    "Sex",
	time.Saturday:  "Sáb",
	time.Sunday:    "Dom",
}

var monthPT = map[time.Month]string{
	time.January:   "Janeiro",
	time.February:  "Fevereiro",
	time.March:     "Março",
	time.April:     "Abril",
	time.May:       "Maio",
	time.June:      "Junho",
	time.July:      "Julho",
	time.August:    "Agosto",
	time.September: "Setembro",
	time.October:   "Outubro",
	time.November:  "Novembro",
	time.December:  "Dezembro",
}

// renderPlan prints the monthly plan to stdout.
func renderPlan(plan MonthPlan) {
	modo := "jeito atual"
	if plan.ModoUniforme {
		modo = "uniforme"
	}

	holidayNote := ""
	if plan.HolidayCount > 0 {
		holidayNote = fmt.Sprintf(" | %d feriado(s)", plan.HolidayCount)
	}

	fmt.Printf("\n%s %d — %d dias úteis%s | Total: %.0fh (%.0fh base + %.0fh extra)\n",
		monthPT[plan.Month], plan.Year,
		plan.TotalDays,
		holidayNote,
		plan.TotalHours,
		plan.TotalHours-plan.ExtraHours,
		plan.ExtraHours,
	)
	fmt.Printf("Modo: %s\n\n", modo)

	var totalExtra float64

	for _, week := range plan.Weeks {
		if len(week.Days) == 0 {
			continue
		}
		first := week.Days[0].Date
		last := week.Days[len(week.Days)-1].Date
		fmt.Printf("Semana %d: %s %02d – %s %02d\n",
			week.Number,
			weekdayPT[first.Weekday()], first.Day(),
			weekdayPT[last.Weekday()], last.Day(),
		)

		var weekExtra float64
		for _, d := range week.Days {
			wd := weekdayPT[d.Date.Weekday()]
			day := d.Date.Day()
			claim := formatClaim(d.Hours)
			entrada, saida := formatCertPonto(d.Hours)
			fmt.Printf("  %s %02d  Claim: %-5s  CertPonto: %s–%s\n",
				wd, day, claim, entrada, saida)
			weekExtra += d.Hours - baseHoursPerDay
		}
		totalExtra += weekExtra
		fmt.Printf("  Extra acumulada: +%.2fh\n\n", totalExtra)
	}

	var total float64
	for _, w := range plan.Weeks {
		for _, d := range w.Days {
			total += d.Hours
		}
	}
	fmt.Printf("Total: %.1fh  |  Extra: %.1fh\n\n", total, totalExtra)
}
