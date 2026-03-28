# Shields.io Supply Chain Score Badge — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add embeddable shields.io-compatible badge endpoints that expose the OCI Explorer supply chain score for any public container image.

**Architecture:** New `score` package ports the frontend scoring logic to Go. New `badge` package renders SVG and shields.io JSON. Two new HTTP handlers (`/badge/score.svg`, `/badge/score.json`) reuse the existing inspect + S3 cache flow. The `/api/inspect` response gains a `score` field, and the frontend drops its local score computation.

**Tech Stack:** Go 1.25, `text/template` for SVG, existing `registry` + `cache` packages, Svelte 5 + TypeScript frontend.

**Spec:** `docs/superpowers/specs/2026-03-28-shields-io-badge-design.md`

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `score/score.go` | Score computation: `Compute()` → `Result` |
| Create | `score/score_test.go` | Table-driven unit tests for scoring |
| Create | `badge/badge.go` | SVG + JSON badge rendering |
| Create | `badge/badge_test.go` | Rendering tests (SVG validity, JSON schema) |
| Modify | `main.go:1-28` (imports), `main.go:184-234` (routes) | Add badge route handlers + import score/badge |
| Modify | `main.go:319-370` (handleInspect) | Include score in inspect response |
| Modify | `web/src/lib/types.ts:87-100` | Add `ScoreResult` interface and `score` field to `ImageInfo` |

---

### Task 1: Score Package — Tests

**Files:**
- Create: `score/score_test.go`

- [ ] **Step 1: Create `score/score_test.go` with all table-driven tests**

```go
package score

import (
	"testing"

	"github.com/hkolvenbach/oci-explorer/registry"
)

func TestCompute(t *testing.T) {
	tests := []struct {
		name      string
		referrers []registry.Referrer
		manifest  *registry.Manifest
		config    *registry.ImageConfig
		wantScore float64
		wantGrade string
		wantColor string
	}{
		{
			name: "perfect score: all artifacts + minimal base",
			referrers: []registry.Referrer{
				{Type: "signature"},
				{Type: "attestation"},
				{Type: "sbom"},
				{Type: "vex"},
			},
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{
					{Size: 1024},
					{Size: 2048},
				},
			},
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "nonroot",
					Entrypoint: []string{"/app"},
				},
			},
			wantScore: 10,
			wantGrade: "A+",
			wantColor: "22c55e",
		},
		{
			name: "artifacts only, no base image traits",
			referrers: []registry.Referrer{
				{Type: "signature"},
				{Type: "attestation"},
				{Type: "sbom"},
				{Type: "vex"},
			},
			manifest:  &registry.Manifest{Layers: make([]registry.Descriptor, 10)},
			config:    &registry.ImageConfig{},
			wantScore: 8,
			wantGrade: "A",
			wantColor: "4ade80",
		},
		{
			name:      "minimal base only, no artifacts",
			referrers: nil,
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{
					{Size: 1024},
				},
			},
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "1000",
					Entrypoint: []string{"/app"},
				},
			},
			wantScore: 2,
			wantGrade: "D",
			wantColor: "f87171",
		},
		{
			name: "partial: sig + sbom + few layers + small",
			referrers: []registry.Referrer{
				{Type: "signature"},
				{Type: "sbom"},
			},
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{
					{Size: 1024},
					{Size: 2048},
				},
			},
			config:    &registry.ImageConfig{},
			wantScore: 5,
			wantGrade: "C",
			wantColor: "fb923c",
		},
		{
			name:      "empty image: no artifacts, no config",
			referrers: nil,
			manifest:  nil,
			config:    nil,
			wantScore: 0,
			wantGrade: "D",
			wantColor: "f87171",
		},
		{
			name: "single signature only",
			referrers: []registry.Referrer{
				{Type: "signature"},
			},
			manifest: &registry.Manifest{Layers: make([]registry.Descriptor, 10)},
			config:   &registry.ImageConfig{},
			wantScore: 2,
			wantGrade: "D",
			wantColor: "f87171",
		},
		{
			name: "boundary: exactly 6 = B",
			referrers: []registry.Referrer{
				{Type: "signature"},
				{Type: "attestation"},
				{Type: "sbom"},
			},
			manifest: &registry.Manifest{Layers: make([]registry.Descriptor, 10)},
			config:   &registry.ImageConfig{},
			wantScore: 6,
			wantGrade: "B",
			wantColor: "eab308",
		},
		{
			name: "boundary: exactly 8 = A",
			referrers: []registry.Referrer{
				{Type: "signature"},
				{Type: "attestation"},
				{Type: "sbom"},
				{Type: "vex"},
			},
			manifest: &registry.Manifest{Layers: make([]registry.Descriptor, 10)},
			config:   &registry.ImageConfig{},
			wantScore: 8,
			wantGrade: "A",
			wantColor: "4ade80",
		},
		{
			name:      "nil manifest with config",
			referrers: nil,
			manifest:  nil,
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "nonroot",
					Entrypoint: []string{"/app"},
				},
			},
			wantScore: 1,
			wantGrade: "D",
			wantColor: "f87171",
		},
		{
			name:      "shell entrypoint does not score",
			referrers: nil,
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{{Size: 1024}},
			},
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "nonroot",
					Entrypoint: []string{"/bin/sh", "-c", "exec /app"},
				},
			},
			wantScore: 1.5,
			wantGrade: "D",
			wantColor: "f87171",
		},
		{
			name:      "root user does not score",
			referrers: nil,
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{{Size: 1024}},
			},
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "root",
					Entrypoint: []string{"/app"},
				},
			},
			wantScore: 1.5,
			wantGrade: "D",
			wantColor: "f87171",
		},
		{
			name:      "user 0 does not score",
			referrers: nil,
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{{Size: 1024}},
			},
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "0",
					Entrypoint: []string{"/app"},
				},
			},
			wantScore: 1.5,
			wantGrade: "D",
			wantColor: "f87171",
		},
		{
			name:      "size exactly 50MB scores",
			referrers: nil,
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{{Size: 50 * 1024 * 1024}},
			},
			config:    &registry.ImageConfig{},
			wantScore: 1,
			wantGrade: "D",
			wantColor: "f87171",
		},
		{
			name:      "size over 50MB does not score",
			referrers: nil,
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{{Size: 50*1024*1024 + 1}},
			},
			config:    &registry.ImageConfig{},
			wantScore: 0.5,
			wantGrade: "D",
			wantColor: "f87171",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.referrers, tt.manifest, tt.config)
			if got.Score != tt.wantScore {
				t.Errorf("Score = %v, want %v", got.Score, tt.wantScore)
			}
			if got.Grade != tt.wantGrade {
				t.Errorf("Grade = %q, want %q", got.Grade, tt.wantGrade)
			}
			if got.Color != tt.wantColor {
				t.Errorf("Color = %q, want %q", got.Color, tt.wantColor)
			}
			if got.MaxScore != 10 {
				t.Errorf("MaxScore = %v, want 10", got.MaxScore)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io && go test ./score/ -v`
Expected: FAIL — `score` package does not exist yet.

- [ ] **Step 3: Commit**

```bash
git add score/score_test.go
git commit -m "test(score): add table-driven tests for supply chain score computation"
```

---

### Task 2: Score Package — Implementation

**Files:**
- Create: `score/score.go`

- [ ] **Step 1: Create `score/score.go`**

```go
package score

import (
	"regexp"
	"strings"

	"github.com/hkolvenbach/oci-explorer/registry"
)

// Result holds the computed supply chain security score.
type Result struct {
	Score    float64 `json:"score"`
	MaxScore float64 `json:"maxScore"`
	Grade    string  `json:"grade"`
	Color    string  `json:"color"`
}

var shellRe = regexp.MustCompile(`\b(sh|bash|ash|zsh)\b`)

// Compute calculates a supply chain security score (0-10) based on
// the presence of supply chain artifacts and minimal base image traits.
//
// Scoring:
//   - Signature, Attestation, SBOM, VEX: 2 points each
//   - Few layers (<=5), small size (<=50MB), non-root user, no shell entrypoint: 0.5 each
func Compute(referrers []registry.Referrer, manifest *registry.Manifest, config *registry.ImageConfig) Result {
	var score float64

	// Supply chain artifacts: 2 points each
	types := map[string]bool{}
	for _, r := range referrers {
		types[r.Type] = true
	}
	for _, key := range []string{"signature", "attestation", "sbom", "vex"} {
		if types[key] {
			score += 2
		}
	}

	// Minimal base image: 0.5 points each, max 2
	if manifest != nil {
		if len(manifest.Layers) <= 5 {
			score += 0.5
		}
		var totalSize int64
		for _, l := range manifest.Layers {
			totalSize += l.Size
		}
		if totalSize <= 50*1024*1024 {
			score += 0.5
		}
	}

	if config != nil && config.Config != nil {
		user := config.Config.User
		if user != "" && user != "0" && user != "root" {
			score += 0.5
		}
		ep := strings.Join(config.Config.Entrypoint, " ")
		if ep != "" && !shellRe.MatchString(ep) {
			score += 0.5
		}
	}

	var grade, color string
	switch {
	case score >= 10:
		grade, color = "A+", "22c55e"
	case score >= 8:
		grade, color = "A", "4ade80"
	case score >= 6:
		grade, color = "B", "eab308"
	case score >= 4:
		grade, color = "C", "fb923c"
	default:
		grade, color = "D", "f87171"
	}

	return Result{
		Score:    score,
		MaxScore: 10,
		Grade:    grade,
		Color:    color,
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io && go test ./score/ -v`
Expected: All tests PASS.

- [ ] **Step 3: Commit**

```bash
git add score/score.go
git commit -m "feat(score): implement supply chain score computation in Go"
```

---

### Task 3: Badge Package — Tests

**Files:**
- Create: `badge/badge_test.go`

- [ ] **Step 1: Create `badge/badge_test.go`**

```go
package badge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hkolvenbach/oci-explorer/score"
)

func TestRenderSVG(t *testing.T) {
	tests := []struct {
		name          string
		result        score.Result
		wantGrade     string
		wantColor     string
		wantAriaLabel string
	}{
		{
			name:          "A+ badge",
			result:        score.Result{Score: 10, MaxScore: 10, Grade: "A+", Color: "22c55e"},
			wantGrade:     "A+",
			wantColor:     "#22c55e",
			wantAriaLabel: "supply chain score: A+",
		},
		{
			name:          "A badge",
			result:        score.Result{Score: 8, MaxScore: 10, Grade: "A", Color: "4ade80"},
			wantGrade:     "A",
			wantColor:     "#4ade80",
			wantAriaLabel: "supply chain score: A",
		},
		{
			name:          "B badge",
			result:        score.Result{Score: 6, MaxScore: 10, Grade: "B", Color: "eab308"},
			wantGrade:     "B",
			wantColor:     "#eab308",
			wantAriaLabel: "supply chain score: B",
		},
		{
			name:          "C badge",
			result:        score.Result{Score: 4, MaxScore: 10, Grade: "C", Color: "fb923c"},
			wantGrade:     "C",
			wantColor:     "#fb923c",
			wantAriaLabel: "supply chain score: C",
		},
		{
			name:          "D badge",
			result:        score.Result{Score: 2, MaxScore: 10, Grade: "D", Color: "f87171"},
			wantGrade:     "D",
			wantColor:     "#f87171",
			wantAriaLabel: "supply chain score: D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svg := RenderSVG(tt.result)
			s := string(svg)

			if !strings.HasPrefix(s, "<svg") {
				t.Error("SVG does not start with <svg")
			}
			if !strings.HasSuffix(strings.TrimSpace(s), "</svg>") {
				t.Error("SVG does not end with </svg>")
			}
			if !strings.Contains(s, tt.wantGrade) {
				t.Errorf("SVG does not contain grade %q", tt.wantGrade)
			}
			if !strings.Contains(s, tt.wantColor) {
				t.Errorf("SVG does not contain color %q", tt.wantColor)
			}
			if !strings.Contains(s, tt.wantAriaLabel) {
				t.Errorf("SVG does not contain aria-label %q", tt.wantAriaLabel)
			}
			// Verify OCI Explorer favicon is embedded
			if !strings.Contains(s, "fill=\"#f97316\"") {
				t.Error("SVG does not contain OCI Explorer favicon (orange #f97316)")
			}
		})
	}
}

func TestRenderSVGError(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{name: "error badge", message: "error"},
		{name: "not found badge", message: "not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svg := RenderErrorSVG(tt.message)
			s := string(svg)

			if !strings.HasPrefix(s, "<svg") {
				t.Error("SVG does not start with <svg")
			}
			if !strings.Contains(s, tt.message) {
				t.Errorf("SVG does not contain message %q", tt.message)
			}
			if !strings.Contains(s, "#9f9f9f") {
				t.Error("error SVG does not use gray color")
			}
		})
	}
}

func TestRenderJSON(t *testing.T) {
	tests := []struct {
		name   string
		result score.Result
	}{
		{name: "A+", result: score.Result{Score: 10, MaxScore: 10, Grade: "A+", Color: "22c55e"}},
		{name: "D", result: score.Result{Score: 2, MaxScore: 10, Grade: "D", Color: "f87171"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := RenderJSON(tt.result)

			var resp ShieldsResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if resp.SchemaVersion != 1 {
				t.Errorf("schemaVersion = %d, want 1", resp.SchemaVersion)
			}
			if resp.Label != "supply chain score" {
				t.Errorf("label = %q, want %q", resp.Label, "supply chain score")
			}
			if resp.Message != tt.result.Grade {
				t.Errorf("message = %q, want %q", resp.Message, tt.result.Grade)
			}
			if resp.Color != tt.result.Color {
				t.Errorf("color = %q, want %q", resp.Color, tt.result.Color)
			}
			if resp.LogoSVG == "" {
				t.Error("logoSvg is empty")
			}
			if !strings.Contains(resp.LogoSVG, "f97316") {
				t.Error("logoSvg does not contain favicon")
			}
		})
	}
}

func TestRenderErrorJSON(t *testing.T) {
	data := RenderErrorJSON("not found")

	var resp ShieldsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.SchemaVersion != 1 {
		t.Errorf("schemaVersion = %d, want 1", resp.SchemaVersion)
	}
	if resp.Message != "not found" {
		t.Errorf("message = %q, want %q", resp.Message, "not found")
	}
	if !resp.IsError {
		t.Error("isError should be true for error badges")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io && go test ./badge/ -v`
Expected: FAIL — `badge` package does not exist yet.

- [ ] **Step 3: Commit**

```bash
git add badge/badge_test.go
git commit -m "test(badge): add rendering tests for SVG and JSON badge output"
```

---

### Task 4: Badge Package — Implementation

**Files:**
- Create: `badge/badge.go`

- [ ] **Step 1: Create `badge/badge.go`**

```go
package badge

import (
	"bytes"
	"encoding/json"
	"text/template"

	"github.com/hkolvenbach/oci-explorer/score"
)

const faviconSVG = `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'><rect width='32' height='32' rx='6' fill='#f97316'/><path d='M16 6l-9 4.5v11L16 26l9-4.5v-11L16 6z' fill='none' stroke='white' stroke-width='1.5' stroke-linejoin='round'/><path d='M7 10.5L16 15l9-4.5M16 15v11' fill='none' stroke='white' stroke-width='1.5' stroke-linejoin='round'/></svg>`

// ShieldsResponse is the shields.io endpoint badge JSON schema.
type ShieldsResponse struct {
	SchemaVersion int    `json:"schemaVersion"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	Color         string `json:"color"`
	LogoSVG       string `json:"logoSvg,omitempty"`
	IsError       bool   `json:"isError,omitempty"`
}

// badgeData is passed to the SVG template.
type badgeData struct {
	Label      string
	Message    string
	Color      string // hex with #
	LabelWidth int
	MsgWidth   int
	TotalWidth int
	LabelX     int // text center x * 10 (for scale .1)
	MsgX       int // text center x * 10
}

var svgTmpl = template.Must(template.New("badge").Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="{{.TotalWidth}}" height="20" role="img" aria-label="{{.Label}}: {{.Message}}"><linearGradient id="s" x2="0" y2="100%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient><clipPath id="r"><rect width="{{.TotalWidth}}" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#r)"><rect width="{{.LabelWidth}}" height="20" fill="#555"/><rect x="{{.LabelWidth}}" width="{{.MsgWidth}}" height="20" fill="{{.Color}}"/><rect width="{{.TotalWidth}}" height="20" fill="url(#s)"/></g><g transform="translate(5,3) scale(0.4375)"><rect width="32" height="32" rx="6" fill="#f97316"/><path d="M16 6l-9 4.5v11L16 26l9-4.5v-11L16 6z" fill="none" stroke="white" stroke-width="1.5" stroke-linejoin="round"/><path d="M7 10.5L16 15l9-4.5M16 15v11" fill="none" stroke="white" stroke-width="1.5" stroke-linejoin="round"/></g><g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110"><text aria-hidden="true" x="{{.LabelX}}" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)">{{.Label}}</text><text x="{{.LabelX}}" y="140" transform="scale(.1)">{{.Label}}</text><text aria-hidden="true" x="{{.MsgX}}" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)">{{.Message}}</text><text x="{{.MsgX}}" y="140" transform="scale(.1)">{{.Message}}</text></g></svg>`))

// estimateTextWidth approximates the pixel width of text rendered in Verdana 11px.
func estimateTextWidth(s string) int {
	// Verdana 11px average character widths (approximate)
	w := 0
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z':
			w += 75 // uppercase ~7.5px
		case c >= 'a' && c <= 'z':
			w += 65 // lowercase ~6.5px
		case c >= '0' && c <= '9':
			w += 70 // digits ~7px
		case c == ' ':
			w += 35
		case c == '+':
			w += 70
		default:
			w += 65
		}
	}
	return (w + 5) / 10 // round up to nearest pixel
}

func makeBadgeData(label, message, hexColor string) badgeData {
	iconWidth := 19 // 14px icon + 5px padding
	labelTextWidth := estimateTextWidth(label)
	labelWidth := iconWidth + labelTextWidth + 10 // 5px padding on each side of text, icon on left
	msgWidth := estimateTextWidth(message) + 12   // 6px padding each side

	totalWidth := labelWidth + msgWidth
	// Label text center: icon takes first iconWidth pixels, text is centered in remainder
	labelX := (iconWidth + labelWidth) * 10 / 2
	// Message text center
	msgX := (labelWidth + labelWidth + msgWidth) * 10 / 2

	return badgeData{
		Label:      label,
		Message:    message,
		Color:      "#" + hexColor,
		LabelWidth: labelWidth,
		MsgWidth:   msgWidth,
		TotalWidth: totalWidth,
		LabelX:     labelX,
		MsgX:       msgX,
	}
}

// RenderSVG renders a shields.io flat-style SVG badge for the given score result.
func RenderSVG(result score.Result) []byte {
	d := makeBadgeData("supply chain score", result.Grade, result.Color)
	var buf bytes.Buffer
	svgTmpl.Execute(&buf, d)
	return buf.Bytes()
}

// RenderErrorSVG renders a gray error badge SVG.
func RenderErrorSVG(message string) []byte {
	d := makeBadgeData("supply chain score", message, "9f9f9f")
	var buf bytes.Buffer
	svgTmpl.Execute(&buf, d)
	return buf.Bytes()
}

// RenderJSON renders a shields.io endpoint JSON response.
func RenderJSON(result score.Result) []byte {
	resp := ShieldsResponse{
		SchemaVersion: 1,
		Label:         "supply chain score",
		Message:       result.Grade,
		Color:         result.Color,
		LogoSVG:       faviconSVG,
	}
	data, _ := json.Marshal(resp)
	return data
}

// RenderErrorJSON renders a shields.io error JSON response.
func RenderErrorJSON(message string) []byte {
	resp := ShieldsResponse{
		SchemaVersion: 1,
		Label:         "supply chain score",
		Message:       message,
		Color:         "gray",
		IsError:       true,
	}
	data, _ := json.Marshal(resp)
	return data
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io && go test ./badge/ -v`
Expected: All tests PASS.

- [ ] **Step 3: Commit**

```bash
git add badge/badge.go
git commit -m "feat(badge): implement SVG and JSON badge rendering"
```

---

### Task 5: Add Badge HTTP Handlers to `main.go`

**Files:**
- Modify: `main.go` (imports, route registration, two new handler functions)

- [ ] **Step 1: Add imports for score and badge packages**

In `main.go`, add to the import block:

```go
"github.com/hkolvenbach/oci-explorer/badge"
"github.com/hkolvenbach/oci-explorer/score"
```

- [ ] **Step 2: Register badge routes**

After the existing `api` route registrations (after line ~194, the `api.HandleFunc("/openapi.yaml", ...)` line), add:

```go
// Badge routes (outside /api prefix — separate namespace for embeddable badges)
r.HandleFunc("/badge/score.svg", handleBadgeSVG).Methods("GET")
r.HandleFunc("/badge/score.json", handleBadgeJSON).Methods("GET")
```

These must be registered on `r` (the main router), NOT `api`, because badge URLs are `/badge/...` not `/api/badge/...`. They must be registered **before** the catch-all `r.PathPrefix("/")` web file server handler.

- [ ] **Step 3: Add `handleBadgeSVG` handler function**

Add after the `handleScanImage` function:

```go
func handleBadgeSVG(w http.ResponseWriter, r *http.Request) {
	imageRef := r.URL.Query().Get("image")
	if imageRef == "" {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "no-cache")
		writeBytes(w, badge.RenderErrorSVG("error"))
		return
	}

	countRequest("badge")

	result, err := computeScoreForImage(r, imageRef)
	if err != nil {
		slog.Warn("badge score failed", "image", imageRef, "error", err)
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "no-cache")
		writeBytes(w, badge.RenderErrorSVG("not found"))
		return
	}

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	writeBytes(w, badge.RenderSVG(*result))
}
```

- [ ] **Step 4: Add `handleBadgeJSON` handler function**

```go
func handleBadgeJSON(w http.ResponseWriter, r *http.Request) {
	imageRef := r.URL.Query().Get("image")
	if imageRef == "" {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		writeBytes(w, badge.RenderErrorJSON("error"))
		return
	}

	countRequest("badge")

	result, err := computeScoreForImage(r, imageRef)
	if err != nil {
		slog.Warn("badge score failed", "image", imageRef, "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		writeBytes(w, badge.RenderErrorJSON("not found"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	writeBytes(w, badge.RenderJSON(*result))
}
```

- [ ] **Step 5: Add shared `computeScoreForImage` helper**

This extracts the inspect-then-score logic shared by both badge handlers. Add it before the two handlers:

```go
// computeScoreForImage inspects the image (using cache if available) and computes the supply chain score.
func computeScoreForImage(r *http.Request, imageRef string) (*score.Result, error) {
	var info *registry.ImageInfo

	if cacheStore != nil {
		digest, err := registry.ResolveDigest(imageRef)
		if err == nil {
			result, err := cacheStore.GetOrCompute(r.Context(), "inspect/"+digest, inspectCacheTTL, func() ([]byte, error) {
				client := registry.NewClient()
				imgInfo, err := client.InspectImage(imageRef)
				if err != nil {
					return nil, err
				}
				return json.Marshal(imgInfo)
			})
			if err == nil {
				var parsed registry.ImageInfo
				if err := json.Unmarshal(result.Data, &parsed); err == nil {
					info = &parsed
				}
			}
		}
	}

	if info == nil {
		client := registry.NewClient()
		imgInfo, err := client.InspectImage(imageRef)
		if err != nil {
			return nil, err
		}
		info = imgInfo
	}

	sr := score.Compute(info.Referrers, info.Manifest, info.Config)
	return &sr, nil
}
```

**Note on cache key:** The badge handler caches the raw `ImageInfo` JSON (not wrapped in `APIResponse`). This is a different format from what `handleInspect` caches (which wraps in `APIResponse{Success: true, Data: ...}`). To share the same cache key, the badge handler must use the same wrapping. However, this adds coupling. Instead, badge uses its own cache key prefix `badge-inspect/` OR the badge handler reuses the inspect cache and handles the unwrapping.

**Correction:** To reuse the existing inspect cache entries, the badge handler should cache data in the same format as `handleInspect`. Update `computeScoreForImage` to marshal/unmarshal `APIResponse`:

```go
func computeScoreForImage(r *http.Request, imageRef string) (*score.Result, error) {
	var info *registry.ImageInfo

	if cacheStore != nil {
		digest, err := registry.ResolveDigest(imageRef)
		if err == nil {
			result, err := cacheStore.GetOrCompute(r.Context(), "inspect/"+digest, inspectCacheTTL, func() ([]byte, error) {
				client := registry.NewClient()
				imgInfo, err := client.InspectImage(imageRef)
				if err != nil {
					return nil, err
				}
				return json.Marshal(APIResponse{Success: true, Data: imgInfo})
			})
			if err == nil {
				// Parse the cached APIResponse to extract ImageInfo
				var apiResp struct {
					Data registry.ImageInfo `json:"data"`
				}
				if err := json.Unmarshal(result.Data, &apiResp); err == nil {
					info = &apiResp.Data
				}
			}
		}
	}

	if info == nil {
		client := registry.NewClient()
		imgInfo, err := client.InspectImage(imageRef)
		if err != nil {
			return nil, err
		}
		info = imgInfo
	}

	sr := score.Compute(info.Referrers, info.Manifest, info.Config)
	return &sr, nil
}
```

- [ ] **Step 6: Add verbose log lines for badge routes**

After the existing `logVerbose` calls for routes (around line ~228), add:

```go
logVerbose("  - GET /badge/score.svg")
logVerbose("  - GET /badge/score.json")
```

- [ ] **Step 7: Run existing tests to verify no breakage**

Run: `cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io && go build ./...`
Expected: Builds without errors.

- [ ] **Step 8: Commit**

```bash
git add main.go
git commit -m "feat: add badge HTTP handlers for /badge/score.svg and /badge/score.json"
```

---

### Task 6: Add Score to `/api/inspect` Response

**Files:**
- Modify: `main.go` (handleInspect function)

The goal: compute the score server-side and include it in the inspect API response so the frontend can consume it.

- [ ] **Step 1: Define an enriched response struct**

Add near the `APIResponse` struct definition:

```go
// ImageInfoWithScore wraps ImageInfo with the computed supply chain score.
type ImageInfoWithScore struct {
	*registry.ImageInfo
	Score score.Result `json:"score"`
}
```

- [ ] **Step 2: Update `handleInspect` — uncached path**

In the uncached path of `handleInspect` (around lines 358-369), change from:

```go
w.Header().Set("Content-Type", "application/json")
writeJSON(w, APIResponse{Success: true, Data: imageInfo})
```

to:

```go
sr := score.Compute(imageInfo.Referrers, imageInfo.Manifest, imageInfo.Config)
w.Header().Set("Content-Type", "application/json")
writeJSON(w, APIResponse{Success: true, Data: ImageInfoWithScore{ImageInfo: imageInfo, Score: sr}})
```

- [ ] **Step 3: Update `handleInspect` — cached path**

The cached path stores `APIResponse{Data: imageInfo}` in S3. Since the cache stores the entire JSON response, and existing cache entries don't include the score, we need to inject the score after deserializing.

Update the cached path's compute function (around line 334-341) to include the score in the cached response:

```go
result, err := cacheStore.GetOrCompute(r.Context(), "inspect/"+digest, inspectCacheTTL, func() ([]byte, error) {
	client := registry.NewClient()
	info, err := client.InspectImage(imageRef)
	if err != nil {
		return nil, err
	}
	sr := score.Compute(info.Referrers, info.Manifest, info.Config)
	return json.Marshal(APIResponse{Success: true, Data: ImageInfoWithScore{ImageInfo: info, Score: sr}})
})
```

**Note on existing cache entries:** Existing cached inspect responses won't have the `score` field. This is fine — the `score` key will simply be absent from those responses, and the frontend can handle its absence gracefully. The 30-day TTL means old entries will naturally expire.

- [ ] **Step 4: Run the app to verify inspect still works**

Run: `cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io && go build -o oci-explorer . && ./oci-explorer &`
Then: `curl -s 'http://localhost:8080/api/inspect?image=alpine:latest' | jq '.data.score'`
Expected: `{"score": <number>, "maxScore": 10, "grade": "<letter>", "color": "<hex>"}`
Clean up: `kill %1`

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: include supply chain score in /api/inspect response"
```

---

### Task 7: Frontend — Consume Score from API

**Files:**
- Modify: `web/src/lib/types.ts`
- Modify: `web/src/lib/utils.ts`
- Modify: `web/src/components/ImageSummary.svelte`

- [ ] **Step 1: Add `score` to `ImageInfo` type**

In `web/src/lib/types.ts`, add a `ScoreResult` interface and update `ImageInfo`:

After the `SecurityScoreResult` interface (line 233), add:

```typescript
export interface ScoreResult {
  score: number;
  maxScore: number;
  grade: string;
  color: string;
}
```

In the `ImageInfo` interface (around line 87), add after `platformDigest`:

```typescript
  score?: ScoreResult;
```

- [ ] **Step 2: Update `ImageSummary.svelte` to use API score**

In `web/src/components/ImageSummary.svelte`:

Change the import (line 3) from:

```typescript
import { formatBytes, truncateDigest, computeSecurityScore } from '../lib/utils';
```

to:

```typescript
import { formatBytes, truncateDigest } from '../lib/utils';
```

Change line 9 from:

```typescript
let scoreResult = $derived(computeSecurityScore(data));
```

to:

```typescript
let scoreResult = $derived(data.score ? {
  score: data.score.score,
  maxScore: data.score.maxScore,
  grade: data.score.grade,
  color: data.score.color,
  colorClass: data.score.score >= 8 ? 'text-green-400' : data.score.score >= 6 ? 'text-yellow-400' : data.score.score >= 4 ? 'text-orange-400' : 'text-red-400',
  criteria: [
    { key: 'signature', label: 'Signature', desc: 'Image is signed (Cosign/Notary)', present: (data.referrers || []).some(r => r.type === 'signature') },
    { key: 'attestation', label: 'Attestation', desc: 'Build provenance attestation (SLSA)', present: (data.referrers || []).some(r => r.type === 'attestation') },
    { key: 'sbom', label: 'SBOM', desc: 'Software Bill of Materials attached', present: (data.referrers || []).some(r => r.type === 'sbom') },
    { key: 'vex', label: 'VEX', desc: 'Vulnerability Exploitability eXchange document', present: (data.referrers || []).some(r => r.type === 'vex') },
    { key: 'minimalBase', label: 'Minimal Base', desc: 'Few layers, small size, non-root, no shell entrypoint', present: data.score.score - ((data.referrers || []).filter(r => ['signature','attestation','sbom','vex'].includes(r.type)).length * 2) >= 1 },
  ],
  minimalBaseDetails: {
    fewLayers: (data.manifest?.layers?.length || 0) <= 5,
    smallSize: (data.manifest?.layers?.reduce((s, l) => s + l.size, 0) || 0) <= 50 * 1024 * 1024,
    nonRoot: !!(data.config?.config?.User && data.config.config.User !== '0' && data.config.config.User !== 'root'),
    noShellEntrypoint: !!((data.config?.config?.Entrypoint || []).join(' ')) && !/\b(sh|bash|ash|zsh)\b/.test((data.config?.config?.Entrypoint || []).join(' ')),
  },
} as SecurityScoreResult : computeSecurityScore(data));
```

**Wait — this is getting complicated.** The criteria and minimalBaseDetails are only used for the expandable breakdown display. The score (grade, color, numeric score) comes from the API. The criteria breakdown still needs the referrers data.

**Simpler approach:** Keep `computeSecurityScore` in the frontend for the criteria/details breakdown, but override the grade/score/color with the API value when present. This avoids duplicating breakdown logic while ensuring the score displayed matches the backend.

Replace line 9 with:

```typescript
let scoreResult = $derived(() => {
  const computed = computeSecurityScore(data);
  if (data.score) {
    computed.score = data.score.score;
    computed.maxScore = data.score.maxScore;
    computed.grade = data.score.grade;
    computed.color = data.score.color;
  }
  return computed;
})();
```

**Actually, even simpler**: keep `computeSecurityScore` as-is for now. The backend and frontend implement identical logic — they'll produce the same result. The criteria breakdown relies on it. We'll remove the frontend computation in a future pass once the backend also returns criteria details. For now, add `score` to the `ImageInfo` type (so TypeScript doesn't complain) but don't change the rendering flow.

**Revised approach for this task:**

- [ ] **Step 2 (revised): Add `score` to `ImageInfo` interface only**

In `web/src/lib/types.ts`, inside the `ImageInfo` interface, add:

```typescript
  score?: ScoreResult;
```

No changes to `ImageSummary.svelte` or `utils.ts` in this task. The frontend continues to compute the score locally (identical logic). The API now also returns it, which unlocks future simplification and is already consumed by the badge endpoints.

- [ ] **Step 3: Build frontend to verify no type errors**

Run: `cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io/web && npm run build`
Expected: Build succeeds with no errors.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/types.ts
git commit -m "feat(web): add score field to ImageInfo type for backend-computed score"
```

---

### Task 8: Integration Test — Badge HTTP Handlers

**Files:**
- Create: `badge_handler_test.go` (in root package, alongside `main.go`)

- [ ] **Step 1: Create `badge_handler_test.go`**

This tests the full HTTP handler using `httptest`, with a mock that bypasses the real registry. Since the badge handlers use `computeScoreForImage` which calls `registry.NewClient()`, we need to test at the HTTP level.

Create a test that starts the handler with cache disabled (simpler) and a mock-able registry client. The simplest approach: test the error paths (no image param, invalid image) which don't hit the registry.

```go
package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBadgeSVGMissingImage(t *testing.T) {
	req := httptest.NewRequest("GET", "/badge/score.svg", nil)
	w := httptest.NewRecorder()

	handleBadgeSVG(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Type") != "image/svg+xml" {
		t.Errorf("Content-Type = %q, want image/svg+xml", resp.Header.Get("Content-Type"))
	}
	body := w.Body.String()
	if !strings.Contains(body, "<svg") {
		t.Error("response is not SVG")
	}
	if !strings.Contains(body, "error") {
		t.Error("error badge should contain 'error'")
	}
}

func TestBadgeJSONMissingImage(t *testing.T) {
	req := httptest.NewRequest("GET", "/badge/score.json", nil)
	w := httptest.NewRecorder()

	handleBadgeJSON(w, req)

	resp := w.Result()
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", resp.Header.Get("Content-Type"))
	}
	body := w.Body.String()
	if !strings.Contains(body, `"isError"`) {
		t.Error("error JSON should contain isError field")
	}
	if !strings.Contains(body, `"schemaVersion"`) {
		t.Error("JSON should contain schemaVersion")
	}
}

func TestBadgeSVGCacheControl(t *testing.T) {
	// Test that missing image returns no-cache (not public cache)
	req := httptest.NewRequest("GET", "/badge/score.svg", nil)
	w := httptest.NewRecorder()

	handleBadgeSVG(w, req)

	cc := w.Result().Header.Get("Cache-Control")
	if cc != "no-cache" {
		t.Errorf("error badge Cache-Control = %q, want no-cache", cc)
	}
}
```

- [ ] **Step 2: Run tests**

Run: `cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io && go test -run TestBadge -v`
Expected: All tests PASS.

- [ ] **Step 3: Commit**

```bash
git add badge_handler_test.go
git commit -m "test: add integration tests for badge HTTP handlers"
```

---

### Task 9: Manual Verification with Docker Compose

**Files:** None (testing only)

- [ ] **Step 1: Build and run locally**

```bash
cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io
docker compose -f docker-compose.dev.yml up --build -d
```

If `docker-compose.dev.yml` doesn't work, fall back to:

```bash
go build -o oci-explorer . && ./oci-explorer &
```

- [ ] **Step 2: Test SVG badge endpoint**

```bash
curl -s 'http://localhost:8080/badge/score.svg?image=alpine:latest' | head -1
```

Expected: starts with `<svg xmlns=`

```bash
curl -sI 'http://localhost:8080/badge/score.svg?image=alpine:latest' | grep -E 'Content-Type|Cache-Control'
```

Expected:
```
Content-Type: image/svg+xml
Cache-Control: public, max-age=86400
```

- [ ] **Step 3: Test JSON badge endpoint**

```bash
curl -s 'http://localhost:8080/badge/score.json?image=alpine:latest' | jq .
```

Expected:
```json
{
  "schemaVersion": 1,
  "label": "supply chain score",
  "message": "<grade>",
  "color": "<hex>",
  "logoSvg": "<svg ...>"
}
```

- [ ] **Step 4: Test error handling**

```bash
curl -s 'http://localhost:8080/badge/score.svg' | grep -o 'error'
curl -s 'http://localhost:8080/badge/score.svg?image=nonexistent/image:fake' | grep -o 'not found'
```

- [ ] **Step 5: Test inspect response includes score**

```bash
curl -s 'http://localhost:8080/api/inspect?image=alpine:latest' | jq '.data.score'
```

Expected: `{"score": <number>, "maxScore": 10, "grade": "<letter>", "color": "<hex>"}`

- [ ] **Step 6: Open SVG in browser**

Open `http://localhost:8080/badge/score.svg?image=alpine:latest` in a browser. Verify the badge renders with the OCI Explorer icon, "supply chain score" label, and letter grade with correct color.

- [ ] **Step 7: Clean up**

```bash
docker compose -f docker-compose.dev.yml down
# or: kill %1
```

---

### Task 10: Run Full Test Suite

- [ ] **Step 1: Run all Go tests**

```bash
cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io && go test ./... -v
```

Expected: All tests pass (score, badge, existing tests).

- [ ] **Step 2: Run frontend build**

```bash
cd /Users/hk/Documents/Code/oci-explorer-public/.claude/worktrees/shields-io/web && npm run build
```

Expected: Build succeeds.

- [ ] **Step 3: Final commit if any cleanup needed**

Only if there were fixes needed from the test run.
