package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"certponto-claim-calc/internal/calc"
)

// responseWeek represents a week in the JSON response.
type responseWeek struct {
	Label     string         `json:"label"`
	AccExtra float64        `json:"acc_extra"`
	Days     []responseDay  `json:"days"`
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
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		d, err := strconv.Atoi(part)
		if err != nil || d < 1 || d > 31 {
			return nil, fmt.Errorf("dia de feriado invalido: %q", part)
		}
		result = append(result, d)
	}
	return result, nil
}

// parseModo returns (uniforme, error) for the modo query param.
func parseModo(s string) (bool, error) {
	if s == "" || s == "a" || s == "atual" {
		return false, nil
	}
	if s == "b" || s == "uniforme" {
		return true, nil
	}
	return false, fmt.Errorf("modo invalido %q, use: a, atual, b ou uniforme", s)
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
func buildResponse(plan calc.MonthPlan, feriados []int) successResponse {
	feriadoSet := make(map[int]bool)
	for _, d := range feriados {
		feriadoSet[d] = true
	}

	modo := "Jeito atual"
	if plan.ModoUniforme {
		modo = "Uniforme"
	}

	horasBase := plan.TotalHours - plan.ExtraHours

	resp := successResponse{
		Mes:         fmt.Sprintf("%s %d", calc.MonthPT[plan.Month], plan.Year),
		Modo:        modo,
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
				CertPonto:  entrada + "–" + saida,
				Extra:      extra,
			}
			rw.Days = append(rw.Days, rd)
		}

		rw.AccExtra = accExtra
		resp.Weeks = append(resp.Weeks, rw)
	}

	return resp
}

func Handler(w http.ResponseWriter, r *http.Request) {
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

	uniforme, err := parseModo(r.URL.Query().Get("modo"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var plan calc.MonthPlan
	if uniforme {
		plan = calc.CalcModoB(year, month, feriados)
	} else {
		plan = calc.CalcModoA(year, month, feriados)
	}

	resp := buildResponse(plan, feriados)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}