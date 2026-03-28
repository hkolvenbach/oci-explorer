package badge_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hkolvenbach/oci-explorer/badge"
	"github.com/hkolvenbach/oci-explorer/score"
)

func gradeResult(grade string, s float64) score.Result {
	return score.Result{
		Score:    s,
		MaxScore: 10,
		Grade:    grade,
	}
}

func TestRenderSVG(t *testing.T) {
	tests := []struct {
		grade    string
		score    float64
		colorHex string // with # prefix, derived from grade
	}{
		{"A+", 10, "#22c55e"},
		{"A", 8, "#4ade80"},
		{"B", 6, "#eab308"},
		{"C", 4, "#fb923c"},
		{"D", 0, "#f87171"},
	}

	for _, tt := range tests {
		t.Run("grade_"+tt.grade, func(t *testing.T) {
			result := gradeResult(tt.grade, tt.score)
			out := badge.RenderSVG(result)
			s := string(out)

			if !strings.HasPrefix(s, "<svg") {
				t.Errorf("output should start with <svg, got: %q", s[:min(len(s), 20)])
			}
			if !strings.HasSuffix(strings.TrimSpace(s), "</svg>") {
				t.Errorf("output should end with </svg>")
			}
			if !strings.Contains(s, tt.grade) {
				t.Errorf("output should contain grade %q", tt.grade)
			}
			if !strings.Contains(s, tt.colorHex) {
				t.Errorf("output should contain color %q", tt.colorHex)
			}
			if !strings.Contains(s, `fill="#f97316"`) {
				t.Errorf("output should contain OCI Explorer favicon fill=\"#f97316\"")
			}
			wantAria := `aria-label="supply chain score: ` + tt.grade + `"`
			if !strings.Contains(s, wantAria) {
				t.Errorf("output should contain %q", wantAria)
			}
		})
	}
}

func TestRenderSVGError(t *testing.T) {
	tests := []string{"error", "not found"}

	for _, msg := range tests {
		t.Run(msg, func(t *testing.T) {
			out := badge.RenderErrorSVG(msg)
			s := string(out)

			if !strings.HasPrefix(s, "<svg") {
				t.Errorf("output should start with <svg")
			}
			if !strings.Contains(s, msg) {
				t.Errorf("output should contain message %q", msg)
			}
			if !strings.Contains(s, "#9f9f9f") {
				t.Errorf("output should use gray color #9f9f9f")
			}
		})
	}
}

func TestRenderJSON(t *testing.T) {
	tests := []struct {
		grade    string
		score    float64
		wantColor string
	}{
		{"A+", 10, "22c55e"},
		{"D", 0, "f87171"},
	}

	for _, tt := range tests {
		t.Run("grade_"+tt.grade, func(t *testing.T) {
			result := gradeResult(tt.grade, tt.score)
			out := badge.RenderJSON(result)

			var resp badge.ShieldsResponse
			if err := json.Unmarshal(out, &resp); err != nil {
				t.Fatalf("failed to unmarshal JSON: %v", err)
			}
			if resp.SchemaVersion != 1 {
				t.Errorf("schemaVersion = %d, want 1", resp.SchemaVersion)
			}
			if resp.Label != "supply chain score" {
				t.Errorf("label = %q, want \"supply chain score\"", resp.Label)
			}
			if resp.Message != tt.grade {
				t.Errorf("message = %q, want %q", resp.Message, tt.grade)
			}
			if resp.Color != tt.wantColor {
				t.Errorf("color = %q, want %q (no # prefix)", resp.Color, tt.wantColor)
			}
			if resp.LogoSVG == "" {
				t.Errorf("logoSvg should be non-empty")
			}
			if !strings.Contains(resp.LogoSVG, "f97316") {
				t.Errorf("logoSvg should contain \"f97316\"")
			}
		})
	}
}

func TestRenderErrorJSON(t *testing.T) {
	out := badge.RenderErrorJSON("not found")

	var resp badge.ShieldsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if resp.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", resp.SchemaVersion)
	}
	if resp.Message != "not found" {
		t.Errorf("message = %q, want \"not found\"", resp.Message)
	}
	if !resp.IsError {
		t.Errorf("isError should be true")
	}
}
