package calc

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// FormatClaim converts decimal hours to Claim system string.
// Examples: 8.0 -> "8.0", 8.5 -> "8.5", 8.75 -> "8.75"
func FormatClaim(hours float64) string {
	s := fmt.Sprintf("%.2f", hours)
	s = strings.TrimRight(s, "0")
	if strings.HasSuffix(s, ".") {
		s += "0"
	}
	return s
}

// FormatCertPonto calculates entry time given hours to work.
// Exit is always 17:00. Lunch (1h) is not counted as work time.
// entry = 17:00 - hours - 1h(lunch)
func FormatCertPonto(hours float64) (entrada, saida string) {
	totalMinutes := int(math.Round((hours + 1.0) * 60))
	endMinutes := 17 * 60
	startMinutes := endMinutes - totalMinutes
	h := startMinutes / 60
	m := startMinutes % 60
	return fmt.Sprintf("%02d:%02d", h, m), "17:00"
}

// WeekdayPT maps time.Weekday values to Portuguese abbreviations.
var WeekdayPT = map[time.Weekday]string{
	time.Monday:    "Seg",
	time.Tuesday:   "Ter",
	time.Wednesday: "Qua",
	time.Thursday:  "Qui",
	time.Friday:    "Sex",
	time.Saturday:  "Sáb",
	time.Sunday:    "Dom",
}

// MonthPT maps time.Month values to Portuguese full names.
var MonthPT = map[time.Month]string{
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