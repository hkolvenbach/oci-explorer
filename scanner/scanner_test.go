package scanner

import (
	"testing"
)

func TestProcessReport(t *testing.T) {
	report := TrivyReport{
		ArtifactName: "nginx:latest",
		Results: []TargetResult{
			{
				Target: "nginx:latest (debian 12.5)",
				Class:  "os-pkgs",
				Type:   "debian",
				Vulnerabilities: []Vulnerability{
					{
						VulnerabilityID:  "CVE-2024-0001",
						PkgName:          "openssl",
						InstalledVersion: "3.0.13-1~deb12u1",
						FixedVersion:     "3.0.13-1~deb12u2",
						Severity:         "CRITICAL",
						Title:            "OpenSSL: Buffer overflow",
						Description:      "A buffer overflow in OpenSSL allows remote attackers...",
						PrimaryURL:       "https://avd.aquasec.com/nvd/cve-2024-0001",
					},
					{
						VulnerabilityID:  "CVE-2024-0002",
						PkgName:          "curl",
						InstalledVersion: "7.88.1-10+deb12u5",
						Severity:         "HIGH",
						Title:            "curl: Use after free",
						Description:      "Use after free in curl...",
						PrimaryURL:       "https://avd.aquasec.com/nvd/cve-2024-0002",
					},
				},
			},
			{
				Target: "usr/local/bin/app",
				Class:  "lang-pkgs",
				Type:   "gobinary",
				Vulnerabilities: []Vulnerability{
					{
						VulnerabilityID:  "CVE-2024-0003",
						PkgName:          "golang.org/x/net",
						InstalledVersion: "v0.17.0",
						FixedVersion:     "v0.23.0",
						Severity:         "MEDIUM",
						Title:            "golang.org/x/net: HTTP/2 rapid reset",
						Description:      "HTTP/2 rapid reset vulnerability...",
						PrimaryURL:       "https://avd.aquasec.com/nvd/cve-2024-0003",
						References:       []string{"https://go.dev/cl/547335"},
					},
					{
						VulnerabilityID:  "CVE-2024-0004",
						PkgName:          "stdlib",
						InstalledVersion: "1.21.0",
						FixedVersion:     "1.21.8",
						Severity:         "CRITICAL",
						Title:            "Go: path traversal",
						Description:      "Path traversal in Go stdlib...",
						PrimaryURL:       "https://avd.aquasec.com/nvd/cve-2024-0004",
					},
				},
			},
		},
	}

	result := processReport(report)

	// Check artifact name
	if result.ArtifactName != "nginx:latest" {
		t.Errorf("ArtifactName = %q, want %q", result.ArtifactName, "nginx:latest")
	}

	// Check total count
	if result.TotalCount != 4 {
		t.Errorf("TotalCount = %d, want 4", result.TotalCount)
	}

	// Check severity counts
	if result.SeverityCounts["CRITICAL"] != 2 {
		t.Errorf("CRITICAL count = %d, want 2", result.SeverityCounts["CRITICAL"])
	}
	if result.SeverityCounts["HIGH"] != 1 {
		t.Errorf("HIGH count = %d, want 1", result.SeverityCounts["HIGH"])
	}
	if result.SeverityCounts["MEDIUM"] != 1 {
		t.Errorf("MEDIUM count = %d, want 1", result.SeverityCounts["MEDIUM"])
	}

	// Check BySeverity grouping
	criticals := result.BySeverity["CRITICAL"]
	if len(criticals) != 2 {
		t.Fatalf("CRITICAL group has %d vulns, want 2", len(criticals))
	}
	// Should be sorted by CVE ID
	if criticals[0].VulnerabilityID != "CVE-2024-0001" {
		t.Errorf("First critical CVE = %q, want CVE-2024-0001", criticals[0].VulnerabilityID)
	}
	if criticals[1].VulnerabilityID != "CVE-2024-0004" {
		t.Errorf("Second critical CVE = %q, want CVE-2024-0004", criticals[1].VulnerabilityID)
	}

	// Check target preservation
	if criticals[0].Target != "nginx:latest (debian 12.5)" {
		t.Errorf("First critical target = %q, want %q", criticals[0].Target, "nginx:latest (debian 12.5)")
	}
	if criticals[1].Target != "usr/local/bin/app" {
		t.Errorf("Second critical target = %q, want %q", criticals[1].Target, "usr/local/bin/app")
	}

	// Check targets list
	if len(result.Targets) != 2 {
		t.Fatalf("Targets has %d entries, want 2", len(result.Targets))
	}
	if result.Targets[0].Count != 2 {
		t.Errorf("First target count = %d, want 2", result.Targets[0].Count)
	}
	if result.Targets[1].Type != "gobinary" {
		t.Errorf("Second target type = %q, want %q", result.Targets[1].Type, "gobinary")
	}

	// Check scan time is set
	if result.ScanTime == "" {
		t.Error("ScanTime should not be empty")
	}
}

func TestProcessReportEmpty(t *testing.T) {
	report := TrivyReport{
		ArtifactName: "alpine:latest",
		Results:      []TargetResult{},
	}

	result := processReport(report)

	if result.TotalCount != 0 {
		t.Errorf("TotalCount = %d, want 0", result.TotalCount)
	}
	if len(result.SeverityCounts) != 0 {
		t.Errorf("SeverityCounts should be empty, got %v", result.SeverityCounts)
	}
	if len(result.BySeverity) != 0 {
		t.Errorf("BySeverity should be empty, got %v", result.BySeverity)
	}
	if len(result.Targets) != 0 {
		t.Errorf("Targets should be empty, got %d", len(result.Targets))
	}
	if result.ArtifactName != "alpine:latest" {
		t.Errorf("ArtifactName = %q, want %q", result.ArtifactName, "alpine:latest")
	}
}

func TestProcessReportEmptySeverity(t *testing.T) {
	report := TrivyReport{
		ArtifactName: "test:latest",
		Results: []TargetResult{
			{
				Target: "test (debian)",
				Class:  "os-pkgs",
				Type:   "debian",
				Vulnerabilities: []Vulnerability{
					{
						VulnerabilityID:  "CVE-2024-9999",
						PkgName:          "libc6",
						InstalledVersion: "2.36-9",
						Severity:         "", // empty severity
						Title:            "Unknown severity vuln",
					},
				},
			},
		},
	}

	result := processReport(report)

	if result.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1", result.TotalCount)
	}
	if result.SeverityCounts["UNKNOWN"] != 1 {
		t.Errorf("UNKNOWN count = %d, want 1", result.SeverityCounts["UNKNOWN"])
	}
	unknowns := result.BySeverity["UNKNOWN"]
	if len(unknowns) != 1 {
		t.Fatalf("UNKNOWN group has %d vulns, want 1", len(unknowns))
	}
	if unknowns[0].Severity != "UNKNOWN" {
		t.Errorf("Severity = %q, want UNKNOWN", unknowns[0].Severity)
	}
}
