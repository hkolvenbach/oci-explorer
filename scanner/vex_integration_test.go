package scanner

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/hkolvenbach/oci-explorer/registry"
)

// --- Test-only types ---

// ModifiedFinding mirrors Trivy's ExperimentalModifiedFindings JSON structure.
type ModifiedFinding struct {
	Type    string `json:"Type"`
	Status  string `json:"Status"`
	Finding struct {
		VulnerabilityID string `json:"VulnerabilityID"`
		PkgName         string `json:"PkgName"`
		Severity        string `json:"Severity"`
	} `json:"Finding"`
	Source string `json:"Source"`
}

// trivyResultWithSuppressed extends TargetResult with the experimental field.
type trivyResultWithSuppressed struct {
	TargetResult
	ExperimentalModifiedFindings []ModifiedFinding `json:"ExperimentalModifiedFindings"`
}

// trivyReportWithSuppressed is the top-level Trivy JSON with per-result suppressed findings.
type trivyReportWithSuppressed struct {
	SchemaVersion int                         `json:"SchemaVersion"`
	ArtifactName  string                      `json:"ArtifactName"`
	Results       []trivyResultWithSuppressed `json:"Results"`
}

// CrossRefResult pairs a scan finding with its VEX status.
type CrossRefResult struct {
	VulnerabilityID string
	PkgName         string
	Severity        string
	VEXStatus       string // "not_affected", "fixed", etc., or "" if no VEX match
}

// --- Test images ---

var testImages = []struct {
	name  string
	image string
}{
	{"oci-explorer:0.2.2", "ghcr.io/hkolvenbach/oci-explorer:0.2.2"},
	{"dmitriylewen/alpine:3.21.1", "dmitriylewen/alpine:3.21.1"},
	{"dmitriylewen/alpine:3.21.2", "dmitriylewen/alpine:3.21.2"},
}

// --- Helper functions ---

// runTrivyWithVEX runs trivy with --vex oci --show-suppressed and returns the
// parsed report along with all modified (suppressed) findings.
func runTrivyWithVEX(t *testing.T, image string) (trivyReportWithSuppressed, []ModifiedFinding) {
	t.Helper()

	trivyPath, err := exec.LookPath("trivy")
	if err != nil {
		t.Fatalf("trivy not found in PATH: %v", err)
	}

	cmd := exec.Command(trivyPath, "image", "--format", "json", "--quiet",
		"--vex", "oci", "--show-suppressed", image)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("trivy --vex oci failed for %s: %s\nstderr: %s", image, err, string(exitErr.Stderr))
		}
		t.Fatalf("trivy --vex oci failed for %s: %v", image, err)
	}

	var report trivyReportWithSuppressed
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("failed to parse trivy --vex oci output for %s: %v", image, err)
	}

	// Collect all modified findings across all results
	var allModified []ModifiedFinding
	for _, r := range report.Results {
		allModified = append(allModified, r.ExperimentalModifiedFindings...)
	}

	return report, allModified
}

// fetchVEXStatements uses the registry client to discover VEX referrers,
// fetch the VEX document, and build a CVE ID → status map.
func fetchVEXStatements(t *testing.T, image string) map[string]string {
	t.Helper()

	client := registry.NewClient()
	info, err := client.InspectImage(image)
	if err != nil {
		t.Fatalf("InspectImage(%s) failed: %v", image, err)
	}

	// Find VEX referrers
	var vexReferrers []registry.Referrer
	for _, ref := range info.Referrers {
		if ref.Type == "vex" {
			vexReferrers = append(vexReferrers, ref)
		}
	}

	vexMap := make(map[string]string) // CVE ID → status
	if len(vexReferrers) == 0 {
		t.Logf("  No VEX referrers found for %s", image)
		return vexMap
	}

	// Fetch each VEX document and build the map
	for _, vexRef := range vexReferrers {
		doc, err := client.FetchVEXContent(info.Repository, vexRef.Digest)
		if err != nil {
			t.Logf("  Warning: FetchVEXContent failed for digest %s: %v", vexRef.Digest[:20], err)
			continue
		}

		for _, stmt := range doc.Statements {
			if stmt.Vulnerability.Name != "" {
				vexMap[stmt.Vulnerability.Name] = stmt.Status
			}
			// Also index by aliases (e.g., GHSA IDs)
			for _, alias := range stmt.Vulnerability.Aliases {
				vexMap[alias] = stmt.Status
			}
		}
	}

	return vexMap
}

// crossReferenceVulns matches scan results against a VEX map.
func crossReferenceVulns(vulns []VulnSummary, vexMap map[string]string) []CrossRefResult {
	var results []CrossRefResult
	for _, v := range vulns {
		status := vexMap[v.VulnerabilityID]
		results = append(results, CrossRefResult{
			VulnerabilityID: v.VulnerabilityID,
			PkgName:         v.PkgName,
			Severity:        v.Severity,
			VEXStatus:       status,
		})
	}
	return results
}

// flattenVulns extracts all VulnSummary entries from a trivyReportWithSuppressed.
func flattenVulns(report trivyReportWithSuppressed) []VulnSummary {
	var vulns []VulnSummary
	for _, r := range report.Results {
		for _, v := range r.Vulnerabilities {
			vulns = append(vulns, VulnSummary{
				VulnerabilityID: v.VulnerabilityID,
				PkgName:         v.PkgName,
				Severity:        strings.ToUpper(v.Severity),
				Target:          r.Target,
			})
		}
	}
	return vulns
}

// --- Tests ---

// TestTrivyNativeVEX runs trivy with --vex oci --show-suppressed against each
// test image and logs what was suppressed.
func TestTrivyNativeVEX(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires trivy + network)")
	}

	for _, img := range testImages {
		t.Run(img.name, func(t *testing.T) {
			report, modified := runTrivyWithVEX(t, img.image)

			// Count total vulns across all results
			totalVulns := 0
			for _, r := range report.Results {
				totalVulns += len(r.Vulnerabilities)
			}

			t.Logf("Image: %s", img.image)
			t.Logf("  Total remaining vulns: %d", totalVulns)
			t.Logf("  Suppressed (modified) findings: %d", len(modified))

			if len(modified) > 0 {
				t.Logf("  Suppressed CVEs:")
				for _, m := range modified {
					t.Logf("    - %s (pkg=%s, severity=%s, status=%s, source=%s)",
						m.Finding.VulnerabilityID, m.Finding.PkgName,
						m.Finding.Severity, m.Status, m.Source)
				}
			} else {
				t.Logf("  (no findings were suppressed by VEX)")
			}
		})
	}
}

// TestAppSideVEXCrossReference runs trivy normally, fetches VEX via the registry
// client, cross-references CVE IDs, and sanity-checks against trivy-native VEX.
func TestAppSideVEXCrossReference(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires trivy + network)")
	}

	for _, img := range testImages {
		t.Run(img.name, func(t *testing.T) {
			// Run normal trivy scan
			trivyPath, err := exec.LookPath("trivy")
			if err != nil {
				t.Fatalf("trivy not found in PATH: %v", err)
			}

			cmd := exec.Command(trivyPath, "image", "--format", "json", "--quiet", img.image)
			output, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Fatalf("trivy scan failed for %s: %s\nstderr: %s", img.image, err, string(exitErr.Stderr))
				}
				t.Fatalf("trivy scan failed for %s: %v", img.image, err)
			}

			var report TrivyReport
			if err := json.Unmarshal(output, &report); err != nil {
				t.Fatalf("failed to parse trivy output: %v", err)
			}

			result := processReport(report)

			// Fetch VEX statements via registry client
			vexMap := fetchVEXStatements(t, img.image)

			t.Logf("Image: %s", img.image)
			t.Logf("  Total scan vulns: %d", result.TotalCount)
			t.Logf("  VEX statements found: %d", len(vexMap))

			if len(vexMap) > 0 {
				t.Logf("  VEX entries:")
				for cve, status := range vexMap {
					t.Logf("    - %s → %s", cve, status)
				}
			}

			// Cross-reference: collect all vulns and check against VEX map
			var allVulns []VulnSummary
			for _, sevGroup := range result.BySeverity {
				allVulns = append(allVulns, sevGroup...)
			}

			crossRefs := crossReferenceVulns(allVulns, vexMap)
			appSideMatchedIDs := make(map[string]bool)
			for _, cr := range crossRefs {
				if cr.VEXStatus != "" {
					appSideMatchedIDs[cr.VulnerabilityID] = true
				}
			}

			t.Logf("  Cross-reference matches: %d / %d vulns", len(appSideMatchedIDs), len(crossRefs))

			if len(appSideMatchedIDs) > 0 {
				t.Logf("  Matched CVEs:")
				for _, cr := range crossRefs {
					if cr.VEXStatus != "" {
						t.Logf("    - %s (pkg=%s, severity=%s) → VEX: %s",
							cr.VulnerabilityID, cr.PkgName, cr.Severity, cr.VEXStatus)
					}
				}
			}

			// For images that have VEX, verify at least some overlap
			if len(vexMap) > 0 && len(appSideMatchedIDs) == 0 {
				t.Logf("  WARNING: VEX statements exist but no CVEs matched scan results")
			}

			// --- Sanity check: trivy-native VEX should be a subset of app-side ---
			_, modified := runTrivyWithVEX(t, img.image)

			trivySuppressedIDs := make(map[string]bool)
			for _, m := range modified {
				trivySuppressedIDs[m.Finding.VulnerabilityID] = true
			}

			t.Logf("  Trivy-native suppressed: %d", len(trivySuppressedIDs))

			// Every CVE suppressed by trivy-native should also be in our app-side map
			var missing []string
			for cve := range trivySuppressedIDs {
				if !appSideMatchedIDs[cve] {
					missing = append(missing, cve)
				}
			}

			if len(missing) > 0 {
				t.Errorf("  App-side VEX map is missing %d CVEs that trivy-native suppressed: %v",
					len(missing), missing)
			} else if len(trivySuppressedIDs) > 0 {
				t.Logf("  Sanity check PASSED: all %d trivy-native suppressed CVEs found in app-side map",
					len(trivySuppressedIDs))
			}
		})
	}
}

// TestCompareApproaches runs both trivy-native VEX and app-side cross-referencing
// on each test image and logs a side-by-side comparison.
func TestCompareApproaches(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (requires trivy + network)")
	}

	for _, img := range testImages {
		t.Run(img.name, func(t *testing.T) {
			t.Logf("=== Comparing approaches for %s ===", img.image)

			// Approach 1: Trivy-native VEX
			report, modified := runTrivyWithVEX(t, img.image)
			nativeVulns := flattenVulns(report)

			nativeSuppressedIDs := make(map[string]string) // CVE → status
			for _, m := range modified {
				nativeSuppressedIDs[m.Finding.VulnerabilityID] = m.Status
			}

			// Approach 2: App-side cross-reference
			vexMap := fetchVEXStatements(t, img.image)

			// Run plain trivy to get the full (unfiltered) vuln list
			trivyPath, err := exec.LookPath("trivy")
			if err != nil {
				t.Fatalf("trivy not found: %v", err)
			}
			cmd := exec.Command(trivyPath, "image", "--format", "json", "--quiet", img.image)
			output, err := cmd.Output()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					t.Fatalf("trivy scan failed: %s\nstderr: %s", err, string(exitErr.Stderr))
				}
				t.Fatalf("trivy scan failed: %v", err)
			}

			var plainReport TrivyReport
			if err := json.Unmarshal(output, &plainReport); err != nil {
				t.Fatalf("failed to parse trivy output: %v", err)
			}
			plainResult := processReport(plainReport)

			var allPlainVulns []VulnSummary
			for _, sevGroup := range plainResult.BySeverity {
				allPlainVulns = append(allPlainVulns, sevGroup...)
			}

			appSideCrossRefs := crossReferenceVulns(allPlainVulns, vexMap)
			appSideMatchedIDs := make(map[string]string) // CVE → VEX status
			for _, cr := range appSideCrossRefs {
				if cr.VEXStatus != "" {
					appSideMatchedIDs[cr.VulnerabilityID] = cr.VEXStatus
				}
			}

			// Build union of all CVE IDs touched by either approach
			allCVEs := make(map[string]bool)
			for id := range nativeSuppressedIDs {
				allCVEs[id] = true
			}
			for id := range appSideMatchedIDs {
				allCVEs[id] = true
			}

			// Log summary
			t.Logf("")
			t.Logf("  Plain scan total vulns:        %d", plainResult.TotalCount)
			t.Logf("  Trivy-native remaining vulns:  %d", len(nativeVulns))
			t.Logf("  Trivy-native suppressed:       %d", len(nativeSuppressedIDs))
			t.Logf("  App-side VEX statements:       %d", len(vexMap))
			t.Logf("  App-side matched CVEs:         %d", len(appSideMatchedIDs))
			t.Logf("")

			if len(allCVEs) == 0 {
				t.Logf("  (No CVEs affected by VEX in either approach)")
				return
			}

			// Side-by-side comparison table
			t.Logf("  %-20s | %-25s | %-25s", "CVE ID", "Trivy-native", "App-side")
			t.Logf("  %s-+-%s-+-%s", strings.Repeat("-", 20), strings.Repeat("-", 25), strings.Repeat("-", 25))

			for cve := range allCVEs {
				nativeStatus := nativeSuppressedIDs[cve]
				if nativeStatus == "" {
					nativeStatus = "(not suppressed)"
				} else {
					nativeStatus = fmt.Sprintf("suppressed: %s", nativeStatus)
				}

				appStatus := appSideMatchedIDs[cve]
				if appStatus == "" {
					appStatus = "(no match)"
				} else {
					appStatus = fmt.Sprintf("vex: %s", appStatus)
				}

				t.Logf("  %-20s | %-25s | %-25s", cve, nativeStatus, appStatus)
			}

			// Highlight differences
			onlyNative := 0
			onlyAppSide := 0
			both := 0
			for cve := range allCVEs {
				_, inNative := nativeSuppressedIDs[cve]
				_, inApp := appSideMatchedIDs[cve]
				switch {
				case inNative && inApp:
					both++
				case inNative:
					onlyNative++
				case inApp:
					onlyAppSide++
				}
			}

			t.Logf("")
			t.Logf("  Agreement:     %d CVEs found by both approaches", both)
			t.Logf("  Only native:   %d CVEs suppressed by trivy but not matched app-side", onlyNative)
			t.Logf("  Only app-side: %d CVEs matched app-side but not suppressed by trivy", onlyAppSide)
		})
	}
}
