package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"certponto-claim-calc/internal/calc"
)

// config is the CLI-only argument structure (unexported).
type config struct {
	year          int
	month         int
	feriados      []int
	modoUniforme bool
}

func parseArgs(args []string) (config, error) {
	if len(args) < 1 {
		return config{}, fmt.Errorf("uso: calc Mmm-YYYY [--feriados 1,2,3] [--modo uniforme]")
	}

	y, m, err := calc.ParseMonthYear(args[0])
	if err != nil {
		return config{}, err
	}

	cfg := config{year: y, month: int(m)}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--feriados":
			if i+1 >= len(args) {
				return config{}, fmt.Errorf("--feriados requer uma lista de dias (ex: --feriados 19,25)")
			}
			i++
			for _, d := range strings.Split(args[i], ",") {
				day, err := strconv.Atoi(strings.TrimSpace(d))
				if err != nil || day < 1 || day > 31 {
					return config{}, fmt.Errorf("dia invalido em --feriados: %q", d)
				}
				cfg.feriados = append(cfg.feriados, day)
			}
		case "--modo":
			if i+1 >= len(args) {
				return config{}, fmt.Errorf("--modo requer um valor: atual ou uniforme")
			}
			i++
			if args[i] != "atual" && args[i] != "uniforme" {
				return config{}, fmt.Errorf("--modo invalido %q, use: atual ou uniforme", args[i])
			}
			cfg.modoUniforme = args[i] == "uniforme"
		default:
			return config{}, fmt.Errorf("argumento desconhecido: %q", args[i])
		}
	}

	return cfg, nil
}

func renderPlan(plan calc.MonthPlan) {
	modo := "jeito atual"
	if plan.ModoUniforme {
		modo = "uniforme"
	}

	holidayNote := ""
	if plan.HolidayCount > 0 {
		holidayNote = fmt.Sprintf(" | %d feriado(s)", plan.HolidayCount)
	}

	fmt.Printf("\n%s %d — %d dias úteis%s | Total: %.0fh (%.0fh base + %.0fh extra)\n",
		calc.MonthPT[plan.Month], plan.Year,
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
			calc.WeekdayPT[first.Weekday()], first.Day(),
			calc.WeekdayPT[last.Weekday()], last.Day(),
		)

		var weekExtra float64
		for _, d := range week.Days {
			wd := calc.WeekdayPT[d.Date.Weekday()]
			day := d.Date.Day()
			claim := calc.FormatClaim(d.Hours)
			entrada, saida := calc.FormatCertPonto(d.Hours)
			fmt.Printf("  %s %02d  Claim: %-5s  CertPonto: %s–%s\n",
				wd, day, claim, entrada, saida)
			weekExtra += d.Hours - calc.BaseHoursPerDay
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

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro: %v\n", err)
		fmt.Fprintf(os.Stderr, "Uso: calc Mmm-YYYY [--feriados 1,2,3] [--modo uniforme]\n")
		os.Exit(1)
	}

	var plan calc.MonthPlan
	if cfg.modoUniforme {
		plan = calc.CalcModoB(cfg.year, time.Month(cfg.month), cfg.feriados)
	} else {
		plan = calc.CalcModoA(cfg.year, time.Month(cfg.month), cfg.feriados)
	}

	if plan.Warning != "" {
		fmt.Fprintf(os.Stderr, "%s\n", plan.Warning)
	}

	renderPlan(plan)
}