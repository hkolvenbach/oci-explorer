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

// CVSSScores holds CVSS v2/v3 scores from a single source.
type CVSSScores struct {
	V3Score  float64 `json:"V3Score,omitempty"`
	V2Score  float64 `json:"V2Score,omitempty"`
	V3Vector string  `json:"V3Vector,omitempty"`
}

// Vulnerability is a raw CVE entry from Trivy.
type Vulnerability struct {
	VulnerabilityID  string                 `json:"VulnerabilityID"`
	PkgName          string                 `json:"PkgName"`
	InstalledVersion string                 `json:"InstalledVersion"`
	FixedVersion     string                 `json:"FixedVersion"`
	Severity         string                 `json:"Severity"`
	Title            string                 `json:"Title"`
	Description      string                 `json:"Description"`
	PrimaryURL       string                 `json:"PrimaryURL"`
	References       []string               `json:"References"`
	CVSS             map[string]CVSSScores  `json:"CVSS"`
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
// When the same CVE+package appears across multiple targets (e.g., the same
// Go stdlib vuln in 19 binaries), they are deduplicated into a single entry
// with multiple targets.
// CvssSource is a single CVSS score from a specific provider (NVD, Red Hat, etc.).
type CvssSource struct {
	Source   string  `json:"source"`
	V3Score  float64 `json:"v3Score,omitempty"`
	V3Vector string  `json:"v3Vector,omitempty"`
	V2Score  float64 `json:"v2Score,omitempty"`
}

type VulnSummary struct {
	VulnerabilityID  string       `json:"vulnerabilityID"`
	PkgName          string       `json:"pkgName"`
	InstalledVersion string       `json:"installedVersion"`
	FixedVersion     string       `json:"fixedVersion"`
	Severity         string       `json:"severity"`
	CvssScore        float64      `json:"cvssScore,omitempty"`
	CvssSources      []CvssSource `json:"cvssSources,omitempty"`
	Title            string       `json:"title"`
	Description      string       `json:"description"`
	PrimaryURL       string       `json:"primaryURL"`
	References       []string     `json:"references"`
	Target           string       `json:"target"`
	Targets          []string     `json:"targets,omitempty"`
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

// buildCvssSources converts Trivy's CVSS map into a sorted slice for the frontend.
func buildCvssSources(cvss map[string]CVSSScores) []CvssSource {
	if len(cvss) == 0 {
		return nil
	}
	sources := make([]CvssSource, 0, len(cvss))
	for name, s := range cvss {
		if s.V3Score > 0 || s.V2Score > 0 {
			sources = append(sources, CvssSource{
				Source:   name,
				V3Score:  s.V3Score,
				V3Vector: s.V3Vector,
				V2Score:  s.V2Score,
			})
		}
	}
	// Stable sort: NVD first, then alphabetical
	sort.Slice(sources, func(i, j int) bool {
		if sources[i].Source == "nvd" {
			return true
		}
		if sources[j].Source == "nvd" {
			return false
		}
		return sources[i].Source < sources[j].Source
	})
	return sources
}

// bestCVSSScore extracts the most relevant CVSS score from Trivy's multi-source
// CVSS map. Prefers NVD V3, then any V3, then any V2. Returns 0 if unavailable.
func bestCVSSScore(cvss map[string]CVSSScores) float64 {
	if len(cvss) == 0 {
		return 0
	}
	// Prefer NVD V3
	if nvd, ok := cvss["nvd"]; ok && nvd.V3Score > 0 {
		return nvd.V3Score
	}
	// Any source V3
	for _, s := range cvss {
		if s.V3Score > 0 {
			return s.V3Score
		}
	}
	// Fallback to V2
	for _, s := range cvss {
		if s.V2Score > 0 {
			return s.V2Score
		}
	}
	return 0
}

// processReport transforms raw Trivy output into a frontend-friendly structure.
// Vulnerabilities with the same CVE ID and package name are deduplicated,
// with their targets aggregated.
func processReport(report TrivyReport) *ScanResult {
	result := &ScanResult{
		ArtifactName:   report.ArtifactName,
		ScanTime:       time.Now().UTC().Format(time.RFC3339),
		SeverityCounts: make(map[string]int),
		BySeverity:     make(map[string][]VulnSummary),
		Targets:        []TargetSummary{},
	}

	// Deduplicate by CVE ID + package name
	type dedupKey struct{ cve, pkg string }
	seen := make(map[dedupKey]*VulnSummary)

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

			key := dedupKey{cve: v.VulnerabilityID, pkg: v.PkgName}
			if existing, ok := seen[key]; ok {
				// Same CVE+package in another target — just add the target
				existing.Targets = append(existing.Targets, tr.Target)
				continue
			}

			summary := VulnSummary{
				VulnerabilityID:  v.VulnerabilityID,
				PkgName:          v.PkgName,
				InstalledVersion: v.InstalledVersion,
				FixedVersion:     v.FixedVersion,
				Severity:         severity,
				CvssScore:        bestCVSSScore(v.CVSS),
				CvssSources:      buildCvssSources(v.CVSS),
				Title:            v.Title,
				Description:      v.Description,
				PrimaryURL:       v.PrimaryURL,
				References:       v.References,
				Target:           tr.Target,
				Targets:          []string{tr.Target},
			}

			result.SeverityCounts[severity]++
			result.TotalCount++
			result.BySeverity[severity] = append(result.BySeverity[severity], summary)
			seen[key] = &result.BySeverity[severity][len(result.BySeverity[severity])-1]
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
