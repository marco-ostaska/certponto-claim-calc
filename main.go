package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"certponto-claim-calc/internal/calc"
)

//go:embed index.html
var staticFS embed.FS

// ── CLI types ──

// config is the CLI-only argument structure (unexported).
type config struct {
	year         int
	month        int
	feriados     []int
	modoUniforme bool
	modoBloco    float64
	extra        float64
}

func parseArgs(args []string) (config, error) {
	if len(args) < 1 {
		return config{}, fmt.Errorf("uso: calc Mmm-YYYY [--feriados 1,2,3] [--modo atual|uniforme|meia] [--extra 16|8]")
	}

	y, m, err := calc.ParseMonthYear(args[0])
	if err != nil {
		return config{}, err
	}

	cfg := config{year: y, month: int(m), modoBloco: calc.BlocoHoraCheia, extra: calc.ExtraMonthly}

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
				return config{}, fmt.Errorf("--modo requer um valor: atual, uniforme ou meia")
			}
			i++
			uniforme, bloco, err := parseModo(args[i])
			if err != nil {
				return config{}, err
			}
			cfg.modoUniforme, cfg.modoBloco = uniforme, bloco
		case "--extra":
			if i+1 >= len(args) {
				return config{}, fmt.Errorf("--extra requer um valor: 16 ou 8")
			}
			i++
			extra, err := parseExtra(args[i])
			if err != nil {
				return config{}, err
			}
			cfg.extra = extra
		default:
			return config{}, fmt.Errorf("argumento desconhecido: %q", args[i])
		}
	}

	// ADR-02-2: a recusa é a mesma função e a mesma mensagem nos dois caminhos;
	// o CLI devolve o erro já montado, sem reescrever o texto.
	if err := validateModoExtra(cfg.modoUniforme, cfg.modoBloco, cfg.extra); err != nil {
		return config{}, err
	}

	return cfg, nil
}

func renderPlan(plan calc.MonthPlan, feriados []int, modoLabel string) {
	feriadoSet := make(map[int]bool, len(feriados))
	for _, d := range feriados {
		feriadoSet[d] = true
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
	fmt.Printf("Modo: %s\n\n", modoLabel)

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
			if feriadoSet[day] {
				fmt.Printf("  %s %02d  Feriado\n", wd, day)
				continue
			}
			claim := calc.FormatClaim(d.Hours)
			entrada, saida := calc.FormatCertPonto(d.Hours)
			fmt.Printf("  %s %02d  Claim: %-5s  CertPonto: %s–%s\n",
				wd, day, claim, entrada, saida)
			weekExtra += d.Hours - calc.BaseHoursPerDay
		}
		totalExtra += weekExtra
		fmt.Printf("  Extra acumulada: +%.2fh\n\n", totalExtra)
	}

	fmt.Printf("Total: %.1fh  |  Extra: %.1fh\n\n", plan.TotalHours, plan.ExtraHours)
}

// ── API response types ──

// responseWeek represents a week in the JSON response.
type responseWeek struct {
	Label    string        `json:"label"`
	AccExtra float64       `json:"acc_extra"`
	Days     []responseDay `json:"days"`
}

// responseDay represents a single day in the JSON response.
type responseDay struct {
	Date       string  `json:"date"`
	D          int     `json:"d"`
	DiaAbrev   string  `json:"dia_abrev"`
	DiaNome    string  `json:"dia_nome"`
	IsWeekend  bool    `json:"is_weekend"`
	IsFeriado  bool    `json:"is_feriado"`
	IsWorking  bool    `json:"is_working"`
	IsShortDay bool    `json:"is_short_day"`
	Claim      string  `json:"claim"`
	CertPonto  string  `json:"certponto"`
	Extra      float64 `json:"extra"`
}

// successResponse is the JSON structure returned on success.
type successResponse struct {
	Mes         string         `json:"mes"`
	Modo        string         `json:"modo"`
	DiasUteis   int            `json:"dias_uteis"`
	NumFeriados int            `json:"num_feriados"`
	HorasBase   float64        `json:"horas_base"`
	HorasExtra  float64        `json:"horas_extra"`
	HorasTotal  float64        `json:"horas_total"`
	Weeks       []responseWeek `json:"weeks"`
	Warning     string         `json:"warning,omitempty"`
}

// errorResponse is the JSON structure returned on error.
type errorResponse struct {
	Error string `json:"error"`
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorResponse{Error: msg})
}

// parseFeriados parses a comma-separated list of day numbers.
func parseFeriados(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	var result []int
	seen := make(map[int]bool)
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		d, err := strconv.Atoi(part)
		if err != nil || d < 1 || d > 31 {
			return nil, fmt.Errorf("dia de feriado invalido: %q", part)
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		result = append(result, d)
	}
	return result, nil
}

// parseModo returns the distribution axis: uniforme=true for the evenly
// distributed mode, bloco = extra hours per day (1h or 0.5h).
func parseModo(s string) (uniforme bool, bloco float64, err error) {
	switch s {
	case "", "a", "atual":
		return false, calc.BlocoHoraCheia, nil
	case "b", "uniforme":
		return true, calc.BlocoHoraCheia, nil
	case "meia":
		return false, calc.BlocoMeiaHora, nil
	}
	return false, 0, fmt.Errorf("modo invalido %q, use: a, atual, b, uniforme ou meia", s)
}

// parseExtra returns the monthly extra-hour target (16 by default, or 8).
func parseExtra(s string) (float64, error) {
	switch s {
	case "", "16":
		return calc.ExtraMonthly, nil
	case "8":
		return calc.ExtraOito, nil
	}
	return 0, fmt.Errorf("extra invalido %q, use: 16 ou 8", s)
}

// modoLabel is the single source of the mode label (JSON response and CLI).
func modoLabel(uniforme bool, bloco float64) string {
	if uniforme {
		return "Uniforme"
	}
	if bloco == calc.BlocoMeiaHora {
		return "Meia hora"
	}
	return "Jeito atual"
}

// validateModoExtra rejects the infeasible combination: "meia" only exists for
// the 8h target (the 0.5h blocks cannot reach 16h in a month). The message is
// shared by the HTTP handler and the CLI, so both refuse with the same text.
func validateModoExtra(uniforme bool, bloco, extra float64) error {
	if uniforme || bloco == calc.BlocoHoraCheia || extra == calc.ExtraOito {
		return nil
	}
	return fmt.Errorf("modo meia requer extra=8 (use --extra 8 ou &extra=8)")
}

// fullWeekdayName maps abbreviations to full Portuguese weekday names.
var fullWeekdayName = map[string]string{
	"Seg": "Segunda",
	"Ter": "Terca",
	"Qua": "Quarta",
	"Qui": "Quinta",
	"Sex": "Sexta",
	"Sab": "Sabado",
	"Dom": "Domingo",
}

// buildResponse converts a calc.MonthPlan into the API response structure.
// feriados is passed separately so individual days can be marked as is_feriado.
func buildResponse(plan calc.MonthPlan, feriados []int, modoLabel string) successResponse {
	feriadoSet := make(map[int]bool)
	for _, d := range feriados {
		feriadoSet[d] = true
	}

	horasBase := plan.TotalHours - plan.ExtraHours

	resp := successResponse{
		Mes:         fmt.Sprintf("%s %d", calc.MonthPT[plan.Month], plan.Year),
		Modo:        modoLabel,
		DiasUteis:   plan.TotalDays,
		NumFeriados: plan.HolidayCount,
		HorasBase:   horasBase,
		HorasExtra:  plan.ExtraHours,
		HorasTotal:  plan.TotalHours,
		Warning:     plan.Warning,
	}

	var accExtra float64

	for _, week := range plan.Weeks {
		if len(week.Days) == 0 {
			continue
		}
		first := week.Days[0].Date
		last := week.Days[len(week.Days)-1].Date
		label := fmt.Sprintf("%s %02d – %s %02d",
			calc.WeekdayPT[first.Weekday()], first.Day(),
			calc.WeekdayPT[last.Weekday()], last.Day(),
		)

		rw := responseWeek{
			Label: label,
		}

		for _, d := range week.Days {
			abrev := calc.WeekdayPT[d.Date.Weekday()]
			nomeCompleto := fullWeekdayName[abrev]
			wd := d.Date.Weekday()
			isWeekend := wd == time.Saturday || wd == time.Sunday
			isFeriado := feriadoSet[d.Date.Day()]
			isWorking := !isWeekend && !isFeriado
			isShortDay := isWorking && d.Hours == calc.BaseHoursPerDay
			extra := d.Hours - calc.BaseHoursPerDay
			if extra < 0 {
				extra = 0
			}
			accExtra += extra

			entrada, saida := calc.FormatCertPonto(d.Hours)

			certponto := entrada + "–" + saida
			if isFeriado {
				certponto = "FERIADO"
			}

			rd := responseDay{
				Date:       d.Date.Format("2006-01-02"),
				D:          d.Date.Day(),
				DiaAbrev:   abrev,
				DiaNome:    nomeCompleto,
				IsWeekend:  isWeekend,
				IsFeriado:  isFeriado,
				IsWorking:  isWorking,
				IsShortDay: isShortDay,
				Claim:      calc.FormatClaim(d.Hours),
				CertPonto:  certponto,
				Extra:      extra,
			}
			rw.Days = append(rw.Days, rd)
		}

		rw.AccExtra = accExtra
		resp.Weeks = append(resp.Weeks, rw)
	}

	return resp
}

// handleAPI is the HTTP handler for /api/calc.
func handleAPI(w http.ResponseWriter, r *http.Request) {
	monthParam := r.URL.Query().Get("month")
	if monthParam == "" {
		writeError(w, http.StatusBadRequest, "parametro 'month' e obrigatorio (ex: Mar-2026)")
		return
	}

	year, month, err := calc.ParseMonthYear(monthParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	feriados, err := parseFeriados(r.URL.Query().Get("feriados"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	uniforme, bloco, err := parseModo(r.URL.Query().Get("modo"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	extra, err := parseExtra(r.URL.Query().Get("extra"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := validateModoExtra(uniforme, bloco, extra); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var plan calc.MonthPlan
	if uniforme {
		plan = calc.CalcModoB(year, month, feriados, extra)
	} else {
		plan = calc.CalcModoA(year, month, feriados, extra, bloco)
	}

	resp := buildResponse(plan, feriados, modoLabel(uniforme, bloco))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ── Server mode ──

func serveIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("index.html")
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func runServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", serveIndex)
	mux.HandleFunc("/api/calc", handleAPI)

	fmt.Fprintf(os.Stderr, "Listening on :%s\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

// ── Entry point ──

func main() {
	// Server mode: PORT is set (Vercel) or no CLI args provided
	port := os.Getenv("PORT")
	if port != "" || len(os.Args) < 2 {
		runServer()
		return
	}

	// CLI mode
	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro: %v\n", err)
		fmt.Fprintf(os.Stderr, "Uso: calc Mmm-YYYY [--feriados 1,2,3] [--modo atual|uniforme|meia] [--extra 16|8]\n")
		os.Exit(1)
	}

	var plan calc.MonthPlan
	if cfg.modoUniforme {
		plan = calc.CalcModoB(cfg.year, time.Month(cfg.month), cfg.feriados, cfg.extra)
	} else {
		plan = calc.CalcModoA(cfg.year, time.Month(cfg.month), cfg.feriados, cfg.extra, cfg.modoBloco)
	}

	if plan.Warning != "" {
		fmt.Fprintf(os.Stderr, "%s\n", plan.Warning)
	}

	renderPlan(plan, cfg.feriados, modoLabel(cfg.modoUniforme, cfg.modoBloco))
}
