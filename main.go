package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Year         int
	Month        time.Month
	Feriados     []int
	ModoUniforme bool
}

func parseArgs(args []string) (Config, error) {
	if len(args) < 1 {
		return Config{}, fmt.Errorf("uso: calc Mmm-YYYY [--feriados 1,2,3] [--modo uniforme]")
	}

	y, m, err := parseMonthYear(args[0])
	if err != nil {
		return Config{}, err
	}

	cfg := Config{Year: y, Month: m}

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--feriados":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--feriados requer uma lista de dias (ex: --feriados 19,25)")
			}
			i++
			for _, d := range strings.Split(args[i], ",") {
				day, err := strconv.Atoi(strings.TrimSpace(d))
				if err != nil || day < 1 || day > 31 {
					return Config{}, fmt.Errorf("dia inválido em --feriados: %q", d)
				}
				cfg.Feriados = append(cfg.Feriados, day)
			}
		case "--modo":
			if i+1 >= len(args) {
				return Config{}, fmt.Errorf("--modo requer um valor: atual ou uniforme")
			}
			i++
			if args[i] != "atual" && args[i] != "uniforme" {
				return Config{}, fmt.Errorf("--modo inválido %q, use: atual ou uniforme", args[i])
			}
			cfg.ModoUniforme = args[i] == "uniforme"
		default:
			return Config{}, fmt.Errorf("argumento desconhecido: %q", args[i])
		}
	}

	return cfg, nil
}

func main() {
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro: %v\n", err)
		fmt.Fprintf(os.Stderr, "Uso: calc Mmm-YYYY [--feriados 1,2,3] [--modo uniforme]\n")
		os.Exit(1)
	}

	var plan MonthPlan
	if cfg.ModoUniforme {
		plan = calcModoB(cfg.Year, cfg.Month, cfg.Feriados)
	} else {
		plan = calcModoA(cfg.Year, cfg.Month, cfg.Feriados)
	}

	renderPlan(plan)
}
