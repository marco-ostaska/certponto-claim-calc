package calc

import (
	"strings"
	"testing"
)

func TestFormatClaim(t *testing.T) {
	tests := []struct {
		hours float64
		want  string
	}{
		{8.0, "8.0"},
		{8.5, "8.5"},
		{8.75, "8.75"},
		{9.0, "9.0"},
		{8.25, "8.25"},
	}
	for _, tt := range tests {
		got := FormatClaim(tt.hours)
		if got != tt.want {
			t.Errorf("FormatClaim(%.2f) = %q, want %q", tt.hours, got, tt.want)
		}
	}
}

func TestFormatCertPonto(t *testing.T) {
	tests := []struct {
		hours   float64
		wantIn  string
		wantOut string
	}{
		{9.0, "07:00", "17:00"},  // 9h + 1h almoço = 10h before 17:00
		{8.0, "08:00", "17:00"},  // 8h + 1h = 9h before 17:00
		{8.5, "07:30", "17:00"},  // 8.5h + 1h = 9.5h before 17:00
		{8.25, "07:45", "17:00"}, // 8.25h + 1h = 9.25h before 17:00
	}
	for _, tt := range tests {
		gotIn, gotOut := FormatCertPonto(tt.hours)
		if gotIn != tt.wantIn || gotOut != tt.wantOut {
			t.Errorf("FormatCertPonto(%.2f) = (%q, %q), want (%q, %q)",
				tt.hours, gotIn, gotOut, tt.wantIn, tt.wantOut)
		}
	}
}

func TestFormatClaimNoScientificNotation(t *testing.T) {
	result := FormatClaim(8.0)
	if strings.Contains(result, "e") || strings.Contains(result, "E") {
		t.Errorf("FormatClaim nao deve usar notacao cientifica: %q", result)
	}
}