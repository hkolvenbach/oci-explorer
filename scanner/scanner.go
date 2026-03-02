package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// verbose controls whether verbose logging is enabled
var verbose bool

// SetVerbose enables or disables verbose logging
func SetVerbose(v bool) {
	verbose = v
}

// logVerbose prints a message only if verbose mode is enabled
func logVerbose(format string, args ...any) {
	if verbose {
		log.Printf("[VERBOSE] [scanner] "+format, args...)
	}
}

// TrivyReport is the top-level Trivy JSON output structure.
type TrivyReport struct {
	SchemaVersion int            `json:"SchemaVersion"`
	ArtifactName  string         `json:"ArtifactName"`
	Results       []TargetResult `json:"Results"`
}

// TargetResult is a per-target group in the Trivy output (e.g., an OS package set or a language-specific file).
type TargetResult struct {
	Target          string          `json:"Target"`
	Class           string          `json:"Class"`
	Type            string          `json:"Type"`
	Vulnerabilities []Vulnerability `json:"Vulnerabilities"`
}

// Vulnerability is a raw CVE entry from Trivy.
type Vulnerability struct {
	VulnerabilityID  string   `json:"VulnerabilityID"`
	PkgName          string   `json:"PkgName"`
	InstalledVersion string   `json:"InstalledVersion"`
	FixedVersion     string   `json:"FixedVersion"`
	Severity         string   `json:"Severity"`
	Title            string   `json:"Title"`
	Description      string   `json:"Description"`
	PrimaryURL       string   `json:"PrimaryURL"`
	References       []string `json:"References"`
}

// ScanResult is the processed result sent to the frontend.
type ScanResult struct {
	ArtifactName   string                      `json:"artifactName"`
	ScanTime       string                      `json:"scanTime"`
	SeverityCounts map[string]int              `json:"severityCounts"`
	TotalCount     int                         `json:"totalCount"`
	BySeverity     map[string][]VulnSummary    `json:"bySeverity"`
	Targets        []TargetSummary             `json:"targets"`
}

// VulnSummary is a flattened CVE for frontend display.
type VulnSummary struct {
	VulnerabilityID  string   `json:"vulnerabilityID"`
	PkgName          string   `json:"pkgName"`
	InstalledVersion string   `json:"installedVersion"`
	FixedVersion     string   `json:"fixedVersion"`
	Severity         string   `json:"severity"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	PrimaryURL       string   `json:"primaryURL"`
	References       []string `json:"references"`
	Target           string   `json:"target"`
}

// TargetSummary provides per-target metadata.
type TargetSummary struct {
	Target string `json:"target"`
	Class  string `json:"class"`
	Type   string `json:"type"`
	Count  int    `json:"count"`
}

// ScanImage runs trivy against the given image reference and returns processed results.
func ScanImage(ctx context.Context, imageRef string) (*ScanResult, error) {
	// Check that trivy is installed
	trivyPath, err := exec.LookPath("trivy")
	if err != nil {
		return nil, fmt.Errorf("trivy is not installed or not in PATH: %w", err)
	}
	logVerbose("Found trivy at: %s", trivyPath)

	// Run trivy with a 5-minute timeout
	scanCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	logVerbose("Scanning image: %s", imageRef)
	cmd := exec.CommandContext(scanCtx, trivyPath, "image", "--format", "json", "--quiet", imageRef)
	output, err := cmd.Output()
	if err != nil {
		if scanCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("trivy scan timed out after 5 minutes")
		}
		// Include stderr in the error message if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("trivy scan failed: %s", string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("trivy scan failed: %w", err)
	}
	logVerbose("Trivy output: %d bytes", len(output))

	var report TrivyReport
	if err := json.Unmarshal(output, &report); err != nil {
		return nil, fmt.Errorf("failed to parse trivy output: %w", err)
	}

	result := processReport(report)
	logVerbose("Scan complete: %d total vulnerabilities across %d targets", result.TotalCount, len(result.Targets))
	return result, nil
}

// processReport transforms raw Trivy output into a frontend-friendly structure.
func processReport(report TrivyReport) *ScanResult {
	result := &ScanResult{
		ArtifactName:   report.ArtifactName,
		ScanTime:       time.Now().UTC().Format(time.RFC3339),
		SeverityCounts: make(map[string]int),
		BySeverity:     make(map[string][]VulnSummary),
		Targets:        []TargetSummary{},
	}

	for _, tr := range report.Results {
		target := TargetSummary{
			Target: tr.Target,
			Class:  tr.Class,
			Type:   tr.Type,
			Count:  len(tr.Vulnerabilities),
		}
		result.Targets = append(result.Targets, target)

		for _, v := range tr.Vulnerabilities {
			severity := strings.ToUpper(v.Severity)
			if severity == "" {
				severity = "UNKNOWN"
			}

			summary := VulnSummary{
				VulnerabilityID:  v.VulnerabilityID,
				PkgName:          v.PkgName,
				InstalledVersion: v.InstalledVersion,
				FixedVersion:     v.FixedVersion,
				Severity:         severity,
				Title:            v.Title,
				Description:      v.Description,
				PrimaryURL:       v.PrimaryURL,
				References:       v.References,
				Target:           tr.Target,
			}

			result.SeverityCounts[severity]++
			result.TotalCount++
			result.BySeverity[severity] = append(result.BySeverity[severity], summary)
		}
	}

	// Sort vulns within each severity by CVE ID for stable ordering
	for sev := range result.BySeverity {
		sort.Slice(result.BySeverity[sev], func(i, j int) bool {
			return result.BySeverity[sev][i].VulnerabilityID < result.BySeverity[sev][j].VulnerabilityID
		})
	}

	return result
}
