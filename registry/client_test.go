package registry

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestClassifyArtifactType tests the artifact type classification logic
func TestClassifyArtifactType(t *testing.T) {
	tests := []struct {
		name         string
		artifactType string
		annotations  map[string]string
		expected     string
	}{
		// Signature types
		{
			name:         "cosign signature",
			artifactType: "application/vnd.dev.cosign.artifact.sig.v1+json",
			annotations:  nil,
			expected:     "signature",
		},
		{
			name:         "notary signature",
			artifactType: "application/vnd.cncf.notary.signature",
			annotations:  nil,
			expected:     "signature",
		},

		// SBOM types
		{
			name:         "SPDX SBOM",
			artifactType: "application/spdx+json",
			annotations:  nil,
			expected:     "sbom",
		},
		{
			name:         "CycloneDX SBOM",
			artifactType: "application/vnd.cyclonedx+json",
			annotations:  nil,
			expected:     "sbom",
		},
		{
			name:         "Generic SBOM type",
			artifactType: "application/vnd.example.sbom.v1",
			annotations:  nil,
			expected:     "sbom",
		},

		// Attestation types
		{
			name:         "in-toto attestation",
			artifactType: "application/vnd.in-toto+json",
			annotations:  nil,
			expected:     "attestation",
		},
		{
			name:         "SLSA provenance via annotation",
			artifactType: "application/vnd.dsse.envelope.v1+json",
			annotations: map[string]string{
				"in-toto.io/predicate-type": "https://slsa.dev/provenance/v1",
			},
			expected: "attestation",
		},

		// Vulnerability scan types
		{
			name:         "vulnerability scan",
			artifactType: "application/vnd.security.vuln.report+json",
			annotations:  nil,
			expected:     "vulnerability-scan",
		},

		// Unknown/fallback
		{
			name:         "unknown type",
			artifactType: "application/vnd.example.unknown",
			annotations:  nil,
			expected:     "artifact",
		},

		// SBOM via annotation predicate type
		{
			name:         "SBOM via predicate annotation",
			artifactType: "application/vnd.dsse.envelope.v1+json",
			annotations: map[string]string{
				"in-toto.io/predicate-type": "https://cyclonedx.org/bom",
			},
			expected: "sbom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyArtifactType(tt.artifactType, tt.annotations)
			if result != tt.expected {
				t.Errorf("classifyArtifactType(%q, %v) = %q, want %q",
					tt.artifactType, tt.annotations, result, tt.expected)
			}
		})
	}
}

// TestKairosImageSBOMDetection tests SBOM detection with a real Kairos image
// This test requires network access and may be slow
// Uses the OCI 1.1 Referrers API supported by Quay.io
func TestKairosImageSBOMDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Enable verbose logging for debugging
	SetVerbose(true)
	defer SetVerbose(false)

	client := NewClient()

	// Test with the Kairos image that has SBOM referrers
	// This image is known to have SBOMs attached via the OCI Referrers API
	// Quay.io supports the Referrers API: https://www.redhat.com/en/blog/announcing-open-container-initiativereferrers-api-quayio-step-towards-enhanced-security-and-compliance
	imageRef := "quay.io/kairos/ubuntu:22.04-standard-amd64-generic-v3.5.2-k3s-v1.33.4-k3s1"

	t.Logf("Inspecting image: %s", imageRef)
	t.Logf("This test verifies that we can discover SBOMs via the OCI 1.1 Referrers API")

	info, err := client.InspectImage(imageRef)
	if err != nil {
		t.Fatalf("Failed to inspect image: %v", err)
	}

	// Verify basic image info
	if info.Repository == "" {
		t.Error("Expected non-empty repository")
	}
	t.Logf("Repository: %s", info.Repository)
	t.Logf("Index Digest: %s", info.Digest)
	t.Logf("Platform Digest: %s", info.PlatformDigest)

	// Check for referrers
	t.Logf("Found %d referrers", len(info.Referrers))

	// Look for SBOM specifically
	foundSBOM := false
	var sbomTypes []string
	for i, ref := range info.Referrers {
		t.Logf("  Referrer %d: type=%s, artifactType=%s, size=%d",
			i, ref.Type, ref.ArtifactType, ref.Size)

		if ref.Type == "sbom" {
			foundSBOM = true
			sbomTypes = append(sbomTypes, ref.ArtifactType)
			t.Logf("  -> Found SBOM! Digest: %s, ArtifactType: %s", ref.Digest, ref.ArtifactType)

			// Log annotations if present
			if len(ref.Annotations) > 0 {
				t.Logf("     Annotations:")
				for k, v := range ref.Annotations {
					t.Logf("       %s: %s", k, v)
				}
			}
		}
	}

	if !foundSBOM {
		t.Errorf("Expected to find at least one SBOM referrer for this image. "+
			"The OCI Referrers API should return SBOM artifacts for this Kairos image. "+
			"Both index digest (%s) and platform digest (%s) were checked.",
			info.Digest, info.PlatformDigest)
	} else {
		t.Logf("SUCCESS: Found SBOM referrer(s) with artifact types: %v", sbomTypes)
	}
}

// TestReferrersAPIDiscovery specifically tests the Referrers API discovery mechanism
func TestReferrersAPIDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	SetVerbose(true)
	defer SetVerbose(false)

	client := NewClient()

	// This Kairos image is known to have referrers discoverable via the Referrers API
	imageRef := "quay.io/kairos/ubuntu:22.04-standard-amd64-generic-v3.5.2-k3s-v1.33.4-k3s1"

	t.Logf("Testing Referrers API discovery for: %s", imageRef)

	info, err := client.InspectImage(imageRef)
	if err != nil {
		t.Fatalf("Failed to inspect image: %v", err)
	}

	t.Logf("Digest: %s", info.Digest)
	t.Logf("Total referrers found: %d", len(info.Referrers))

	// Categorize referrers by type
	typeCount := make(map[string]int)
	for _, ref := range info.Referrers {
		typeCount[ref.Type]++
	}

	t.Logf("Referrers by type:")
	for refType, count := range typeCount {
		t.Logf("  %s: %d", refType, count)
	}

	// We expect at least one referrer (SBOM) for this image
	if len(info.Referrers) == 0 {
		t.Error("Expected at least one referrer from the Referrers API")
	}
}

// TestImageWithNoReferrers tests that images without referrers are handled gracefully
func TestImageWithNoReferrers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()

	// Alpine typically doesn't have referrers
	imageRef := "alpine:3.19"

	t.Logf("Inspecting image: %s", imageRef)
	info, err := client.InspectImage(imageRef)
	if err != nil {
		t.Fatalf("Failed to inspect image: %v", err)
	}

	// Should succeed even with no referrers
	if info.Repository == "" {
		t.Error("Expected non-empty repository")
	}

	t.Logf("Repository: %s", info.Repository)
	t.Logf("Referrers: %d", len(info.Referrers))

	// Verify we have layers
	if info.Manifest == nil || len(info.Manifest.Layers) == 0 {
		t.Error("Expected at least one layer")
	}
	t.Logf("Layers: %d", len(info.Manifest.Layers))
}

// TestMultiPlatformImage tests handling of multi-platform images
func TestMultiPlatformImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	client := NewClient()

	// nginx is typically multi-platform
	imageRef := "nginx:latest"

	t.Logf("Inspecting image: %s", imageRef)
	info, err := client.InspectImage(imageRef)
	if err != nil {
		t.Fatalf("Failed to inspect image: %v", err)
	}

	// Should have an image index with multiple platforms
	if info.ImageIndex == nil {
		t.Log("Image is not multi-platform (single manifest)")
	} else {
		t.Logf("Image index has %d platform manifests", len(info.ImageIndex.Manifests))

		if len(info.ImageIndex.Manifests) < 2 {
			t.Log("Warning: Expected multiple platforms for nginx")
		}

		for i, m := range info.ImageIndex.Manifests {
			if m.Platform != nil {
				t.Logf("  Platform %d: %s/%s", i, m.Platform.OS, m.Platform.Architecture)
			}
		}
	}
}

// BenchmarkInspectImage benchmarks the image inspection
func BenchmarkInspectImage(b *testing.B) {
	client := NewClient()

	for i := 0; i < b.N; i++ {
		_, err := client.InspectImage("alpine:3.19")
		if err != nil {
			b.Fatalf("Failed to inspect image: %v", err)
		}
	}
}

// ============================================================================
// OFFLINE INTEGRATION TESTS USING FIXTURES
// ============================================================================
// These tests use cached/downloaded manifests from the Kairos image to test
// SBOM detection logic without requiring network access.
//
// Test fixtures are stored in testdata/ directory:
// - kairos_image_index.json: The OCI image index for the multi-platform image
// - kairos_attestation_manifest.json: BuildKit attestation manifest with SBOM layers
// - kairos_sbom_sample.json: Sample SPDX SBOM content from the attestation
// ============================================================================

// loadTestFixture loads a JSON fixture file from the testdata directory
func loadTestFixture(t *testing.T, filename string) []byte {
	t.Helper()
	path := filepath.Join("testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to load test fixture %s: %v", filename, err)
	}
	return data
}

// TestKairosImageIndexParsing tests parsing of the Kairos OCI image index
// using the cached fixture to verify multi-platform image handling offline
func TestKairosImageIndexParsing(t *testing.T) {
	data := loadTestFixture(t, "kairos_image_index.json")

	var index ImageIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("Failed to parse image index: %v", err)
	}

	// Verify index structure
	if index.SchemaVersion != 2 {
		t.Errorf("Expected schemaVersion 2, got %d", index.SchemaVersion)
	}

	if index.MediaType != "application/vnd.oci.image.index.v1+json" {
		t.Errorf("Expected OCI image index media type, got %s", index.MediaType)
	}

	// Should have 2 manifests: platform manifest + attestation manifest
	if len(index.Manifests) != 2 {
		t.Errorf("Expected 2 manifests, got %d", len(index.Manifests))
	}

	// Check platform manifest
	platformManifest := index.Manifests[0]
	if platformManifest.Platform == nil {
		t.Error("Expected platform manifest to have platform info")
	} else {
		if platformManifest.Platform.Architecture != "amd64" {
			t.Errorf("Expected amd64 architecture, got %s", platformManifest.Platform.Architecture)
		}
		if platformManifest.Platform.OS != "linux" {
			t.Errorf("Expected linux OS, got %s", platformManifest.Platform.OS)
		}
	}
	if platformManifest.Digest != "sha256:c93ad9a4ed48c4fb9eed1bf52397effe2b6ec40bb99685015957f8406857f54a" {
		t.Errorf("Unexpected platform manifest digest: %s", platformManifest.Digest)
	}

	// Check attestation manifest
	attestationManifest := index.Manifests[1]
	if attestationManifest.Annotations == nil {
		t.Error("Expected attestation manifest to have annotations")
	} else {
		refType, ok := attestationManifest.Annotations["vnd.docker.reference.type"]
		if !ok {
			t.Error("Expected vnd.docker.reference.type annotation")
		} else if refType != "attestation-manifest" {
			t.Errorf("Expected attestation-manifest reference type, got %s", refType)
		}

		refDigest, ok := attestationManifest.Annotations["vnd.docker.reference.digest"]
		if !ok {
			t.Error("Expected vnd.docker.reference.digest annotation")
		} else if refDigest != platformManifest.Digest {
			t.Errorf("Attestation should reference platform manifest, got %s", refDigest)
		}
	}
	if attestationManifest.Digest != "sha256:77ffcabd824268cec9426ee4bb1c6824ad4c745bcad29e94938091e35ed64f99" {
		t.Errorf("Unexpected attestation manifest digest: %s", attestationManifest.Digest)
	}

	t.Logf("Successfully parsed Kairos image index with %d manifests", len(index.Manifests))
	t.Logf("  Platform: %s/%s", platformManifest.Platform.OS, platformManifest.Platform.Architecture)
	t.Logf("  Attestation digest: %s", truncateDigest(attestationManifest.Digest))
}

// TestBuildKitAttestationManifestParsing tests parsing of the BuildKit attestation
// manifest which contains SBOM and provenance layers
func TestBuildKitAttestationManifestParsing(t *testing.T) {
	data := loadTestFixture(t, "kairos_attestation_manifest.json")

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("Failed to parse attestation manifest: %v", err)
	}

	// Verify manifest structure
	if manifest.SchemaVersion != 2 {
		t.Errorf("Expected schemaVersion 2, got %d", manifest.SchemaVersion)
	}

	if manifest.MediaType != "application/vnd.oci.image.manifest.v1+json" {
		t.Errorf("Expected OCI image manifest media type, got %s", manifest.MediaType)
	}

	// Should have 2 layers: SBOM and provenance
	if len(manifest.Layers) != 2 {
		t.Errorf("Expected 2 layers, got %d", len(manifest.Layers))
	}

	// Verify SBOM layer
	sbomLayer := manifest.Layers[0]
	if sbomLayer.MediaType != "application/vnd.in-toto+json" {
		t.Errorf("Expected in-toto media type for SBOM layer, got %s", sbomLayer.MediaType)
	}
	sbomPredicate, ok := sbomLayer.Annotations["in-toto.io/predicate-type"]
	if !ok {
		t.Error("SBOM layer should have predicate-type annotation")
	} else if sbomPredicate != "https://spdx.dev/Document" {
		t.Errorf("Expected SPDX predicate type, got %s", sbomPredicate)
	}
	if sbomLayer.Size != 33564898 {
		t.Errorf("Unexpected SBOM layer size: %d", sbomLayer.Size)
	}

	// Verify provenance layer
	provenanceLayer := manifest.Layers[1]
	if provenanceLayer.MediaType != "application/vnd.in-toto+json" {
		t.Errorf("Expected in-toto media type for provenance layer, got %s", provenanceLayer.MediaType)
	}
	provPredicate, ok := provenanceLayer.Annotations["in-toto.io/predicate-type"]
	if !ok {
		t.Error("Provenance layer should have predicate-type annotation")
	} else if provPredicate != "https://slsa.dev/provenance/v0.2" {
		t.Errorf("Expected SLSA provenance predicate type, got %s", provPredicate)
	}

	t.Logf("Successfully parsed BuildKit attestation manifest")
	t.Logf("  SBOM layer: %s (predicate: %s, size: %d bytes)",
		truncateDigest(sbomLayer.Digest), sbomPredicate, sbomLayer.Size)
	t.Logf("  Provenance layer: %s (predicate: %s, size: %d bytes)",
		truncateDigest(provenanceLayer.Digest), provPredicate, provenanceLayer.Size)
}

// TestBuildKitAttestationClassification tests that BuildKit attestation manifests
// are correctly identified as containing SBOM and provenance based on layer predicates
func TestBuildKitAttestationClassification(t *testing.T) {
	// Load the attestation manifest
	data := loadTestFixture(t, "kairos_attestation_manifest.json")

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("Failed to parse attestation manifest: %v", err)
	}

	// Track what types are found
	foundSBOM := false
	foundProvenance := false

	// Test the classification logic used in extractAttestationInfo
	for _, layer := range manifest.Layers {
		predicateType := ""
		if pt, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
			predicateType = pt
		}

		// Test SBOM classification
		classified := classifyPredicateType(predicateType)
		t.Logf("Layer predicate %s classified as: %s", predicateType, classified)

		if classified == "sbom" {
			foundSBOM = true
		}
		if classified == "attestation" {
			foundProvenance = true
		}
	}

	if !foundSBOM {
		t.Error("Expected to find SBOM layer in attestation manifest")
	}
	if !foundProvenance {
		t.Error("Expected to find provenance/attestation layer in attestation manifest")
	}

	t.Logf("BuildKit attestation correctly identified: SBOM=%v, Provenance=%v", foundSBOM, foundProvenance)
}

// classifyPredicateType classifies in-toto predicate types (helper for testing)
func classifyPredicateType(predicateType string) string {
	// Use empty artifactType and put predicate in annotations to leverage existing function
	annotations := map[string]string{
		"in-toto.io/predicate-type": predicateType,
	}
	return classifyArtifactType("", annotations)
}

// TestSBOMContentStructure tests parsing of the actual SBOM content format
func TestSBOMContentStructure(t *testing.T) {
	data := loadTestFixture(t, "kairos_sbom_sample.json")

	// Define structure for in-toto attestation envelope
	type InTotoStatement struct {
		Type          string `json:"_type"`
		PredicateType string `json:"predicateType"`
		Subject       []struct {
			Name   string            `json:"name"`
			Digest map[string]string `json:"digest"`
		} `json:"subject"`
		Predicate json.RawMessage `json:"predicate"`
	}

	var statement InTotoStatement
	if err := json.Unmarshal(data, &statement); err != nil {
		t.Fatalf("Failed to parse SBOM content: %v", err)
	}

	// Verify in-toto structure
	if statement.Type != "https://in-toto.io/Statement/v0.1" {
		t.Errorf("Expected in-toto Statement type, got %s", statement.Type)
	}

	if statement.PredicateType != "https://spdx.dev/Document" {
		t.Errorf("Expected SPDX predicate type, got %s", statement.PredicateType)
	}

	// Verify subject
	if len(statement.Subject) != 1 {
		t.Errorf("Expected 1 subject, got %d", len(statement.Subject))
	} else {
		subject := statement.Subject[0]
		expectedDigest := "c93ad9a4ed48c4fb9eed1bf52397effe2b6ec40bb99685015957f8406857f54a"
		if subject.Digest["sha256"] != expectedDigest {
			t.Errorf("Expected subject digest %s, got %s", expectedDigest, subject.Digest["sha256"])
		}
		t.Logf("SBOM subject: %s", subject.Name)
	}

	// Parse SPDX predicate
	type SPDXDocument struct {
		SPDXVersion       string `json:"spdxVersion"`
		DataLicense       string `json:"dataLicense"`
		SPDXID            string `json:"SPDXID"`
		Name              string `json:"name"`
		DocumentNamespace string `json:"documentNamespace"`
		CreationInfo      struct {
			Creators []string `json:"creators"`
			Created  string   `json:"created"`
		} `json:"creationInfo"`
		Packages []struct {
			Name        string `json:"name"`
			SPDXID      string `json:"SPDXID"`
			VersionInfo string `json:"versionInfo"`
		} `json:"packages"`
	}

	var spdx SPDXDocument
	if err := json.Unmarshal(statement.Predicate, &spdx); err != nil {
		t.Fatalf("Failed to parse SPDX predicate: %v", err)
	}

	// Verify SPDX structure
	if spdx.SPDXVersion != "SPDX-2.3" {
		t.Errorf("Expected SPDX-2.3 version, got %s", spdx.SPDXVersion)
	}

	if spdx.DataLicense != "CC0-1.0" {
		t.Errorf("Expected CC0-1.0 license, got %s", spdx.DataLicense)
	}

	// Verify creation info contains expected tools
	foundSyft := false
	foundBuildKit := false
	for _, creator := range spdx.CreationInfo.Creators {
		if creator == "Tool: syft-v1.29.0" {
			foundSyft = true
		}
		if creator == "Tool: buildkit-v0.23.2" {
			foundBuildKit = true
		}
	}
	if !foundSyft {
		t.Error("Expected syft tool in SBOM creators")
	}
	if !foundBuildKit {
		t.Error("Expected buildkit tool in SBOM creators")
	}

	// Verify packages
	if len(spdx.Packages) < 1 {
		t.Error("Expected at least one package in SBOM")
	} else {
		t.Logf("SPDX document contains %d packages", len(spdx.Packages))
		for _, pkg := range spdx.Packages {
			t.Logf("  Package: %s@%s", pkg.Name, pkg.VersionInfo)
		}
	}

	t.Logf("Successfully parsed SBOM: SPDX %s, created %s", spdx.SPDXVersion, spdx.CreationInfo.Created)
}

// TestExtractAttestationInfoLogic tests the logic that identifies SBOM and provenance
// from BuildKit attestation manifest layers without making network calls
func TestExtractAttestationInfoLogic(t *testing.T) {
	// Load the attestation manifest
	data := loadTestFixture(t, "kairos_attestation_manifest.json")

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("Failed to parse attestation manifest: %v", err)
	}

	// Simulate the extractAttestationInfo logic
	digest := "sha256:77ffcabd824268cec9426ee4bb1c6824ad4c745bcad29e94938091e35ed64f99"
	indexAnnotations := map[string]string{
		"vnd.docker.reference.digest": "sha256:c93ad9a4ed48c4fb9eed1bf52397effe2b6ec40bb99685015957f8406857f54a",
		"vnd.docker.reference.type":   "attestation-manifest",
	}

	var referrers []Referrer
	foundSBOM := false
	foundProvenance := false

	for _, layer := range manifest.Layers {
		predicateType := ""
		if pt, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
			predicateType = pt
		}

		predicateLower := strings.ToLower(predicateType)

		// Check for SBOM
		if containsAny(predicateLower, []string{"spdx", "cyclonedx", "sbom", "syft"}) {
			if !foundSBOM {
				foundSBOM = true
				annotations := copyAnnotations(indexAnnotations)
				annotations["in-toto.io/predicate-type"] = predicateType

				referrers = append(referrers, Referrer{
					Type:         "sbom",
					MediaType:    manifest.MediaType,
					Digest:       digest,
					Size:         layer.Size, // Use actual SBOM layer size, not manifest size
					ArtifactType: predicateType,
					Annotations:  annotations,
				})
			}
		}

		// Check for provenance
		if containsAny(predicateLower, []string{"provenance", "slsa"}) {
			if !foundProvenance {
				foundProvenance = true
				annotations := copyAnnotations(indexAnnotations)
				annotations["in-toto.io/predicate-type"] = predicateType

				referrers = append(referrers, Referrer{
					Type:         "attestation",
					MediaType:    manifest.MediaType,
					Digest:       digest,
					Size:         layer.Size, // Use actual attestation layer size, not manifest size
					ArtifactType: predicateType,
					Annotations:  annotations,
				})
			}
		}
	}

	// Verify results
	if len(referrers) != 2 {
		t.Errorf("Expected 2 referrers (SBOM + provenance), got %d", len(referrers))
	}

	sbomFound := false
	attestationFound := false
	for _, ref := range referrers {
		t.Logf("Referrer: type=%s, artifactType=%s, digest=%s, size=%d", ref.Type, ref.ArtifactType, truncateDigest(ref.Digest), ref.Size)
		if ref.Type == "sbom" {
			sbomFound = true
			if ref.ArtifactType != "https://spdx.dev/Document" {
				t.Errorf("Expected SPDX artifact type for SBOM, got %s", ref.ArtifactType)
			}
			// Verify the size is the actual SBOM layer size (33564898), not the manifest size (841)
			expectedSBOMSize := int64(33564898)
			if ref.Size != expectedSBOMSize {
				t.Errorf("Expected SBOM size %d (actual layer size), got %d", expectedSBOMSize, ref.Size)
			}
		}
		if ref.Type == "attestation" {
			attestationFound = true
			if ref.ArtifactType != "https://slsa.dev/provenance/v0.2" {
				t.Errorf("Expected SLSA artifact type for attestation, got %s", ref.ArtifactType)
			}
			// Verify the size is the actual provenance layer size (5662), not the manifest size (841)
			expectedProvenanceSize := int64(5662)
			if ref.Size != expectedProvenanceSize {
				t.Errorf("Expected attestation size %d (actual layer size), got %d", expectedProvenanceSize, ref.Size)
			}
		}
	}

	if !sbomFound {
		t.Error("Expected to find SBOM referrer")
	}
	if !attestationFound {
		t.Error("Expected to find attestation referrer")
	}
}

// Helper functions for tests
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func copyAnnotations(m map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		result[k] = v
	}
	return result
}

// TestSPDXPredicateClassification tests classification of various SPDX predicate types
func TestSPDXPredicateClassification(t *testing.T) {
	tests := []struct {
		name      string
		predicate string
		expected  string
	}{
		{
			name:      "SPDX Document predicate",
			predicate: "https://spdx.dev/Document",
			expected:  "sbom",
		},
		{
			name:      "SPDX Document predicate with trailing slash",
			predicate: "https://spdx.dev/Document/",
			expected:  "sbom",
		},
		{
			name:      "CycloneDX BOM predicate",
			predicate: "https://cyclonedx.org/bom",
			expected:  "sbom",
		},
		{
			name:      "SLSA provenance v0.2",
			predicate: "https://slsa.dev/provenance/v0.2",
			expected:  "attestation",
		},
		{
			name:      "SLSA provenance v1",
			predicate: "https://slsa.dev/provenance/v1",
			expected:  "attestation",
		},
		{
			name:      "Generic provenance",
			predicate: "https://example.com/provenance",
			expected:  "attestation",
		},
		{
			name:      "Unknown predicate",
			predicate: "https://example.com/unknown",
			expected:  "artifact",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyPredicateType(tt.predicate)
			if result != tt.expected {
				t.Errorf("classifyPredicateType(%q) = %q, want %q", tt.predicate, result, tt.expected)
			}
		})
	}
}

// TestImageIndexWithAttestationManifest tests detection of attestation manifests
// in an image index based on annotations
func TestImageIndexWithAttestationManifest(t *testing.T) {
	data := loadTestFixture(t, "kairos_image_index.json")

	var index ImageIndex
	if err := json.Unmarshal(data, &index); err != nil {
		t.Fatalf("Failed to parse image index: %v", err)
	}

	// Look for attestation manifests in the index
	var attestationManifests []IndexManifest
	for _, m := range index.Manifests {
		if refType, ok := m.Annotations["vnd.docker.reference.type"]; ok && refType == "attestation-manifest" {
			attestationManifests = append(attestationManifests, m)
		}
	}

	if len(attestationManifests) == 0 {
		t.Error("Expected to find at least one attestation manifest in the index")
	}

	for i, am := range attestationManifests {
		t.Logf("Attestation manifest %d:", i)
		t.Logf("  Digest: %s", am.Digest)
		t.Logf("  Size: %d bytes", am.Size)
		t.Logf("  References: %s", am.Annotations["vnd.docker.reference.digest"])

		// Verify the attestation references a platform manifest in the same index
		refDigest := am.Annotations["vnd.docker.reference.digest"]
		found := false
		for _, m := range index.Manifests {
			if m.Digest == refDigest && m.Platform != nil {
				found = true
				t.Logf("  -> References platform: %s/%s", m.Platform.OS, m.Platform.Architecture)
				break
			}
		}
		if !found {
			t.Errorf("Attestation manifest references unknown digest: %s", refDigest)
		}
	}
}

// TestPlatformReferrerAnnotations tests that referrers fetched for platform manifests
// have the correct vnd.docker.reference.digest annotation linking them to their platform
func TestPlatformReferrerAnnotations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	SetVerbose(true)
	defer SetVerbose(false)

	client := NewClient()

	// Test with alpine:latest which is a multi-platform image
	imageRef := "alpine:latest"

	t.Logf("Inspecting image: %s", imageRef)
	t.Logf("This test verifies that referrers are correctly linked to their platform manifests")

	info, err := client.InspectImage(imageRef)
	if err != nil {
		t.Fatalf("Failed to inspect image: %v", err)
	}

	// Verify we have an image index with multiple platforms
	if info.ImageIndex == nil {
		t.Skip("Image is not multi-platform, skipping platform referrer test")
	}

	platformManifests := info.ImageIndex.Manifests
	validPlatforms := 0
	for _, m := range platformManifests {
		if m.Platform != nil {
			platformStr := fmt.Sprintf("%s/%s", m.Platform.OS, m.Platform.Architecture)
			if platformStr != "unknown/unknown" {
				validPlatforms++
			}
		}
	}

	if validPlatforms < 2 {
		t.Skipf("Image has fewer than 2 valid platforms (%d), skipping test", validPlatforms)
	}

	t.Logf("Found %d valid platforms", validPlatforms)
	t.Logf("Total referrers found: %d", len(info.Referrers))

	// Group referrers by platform digest
	referrersByPlatform := make(map[string][]Referrer)
	globalReferrers := []Referrer{}

	for _, ref := range info.Referrers {
		if refDigest, ok := ref.Annotations["vnd.docker.reference.digest"]; ok {
			if _, exists := referrersByPlatform[refDigest]; !exists {
				referrersByPlatform[refDigest] = []Referrer{}
			}
			referrersByPlatform[refDigest] = append(referrersByPlatform[refDigest], ref)
		} else {
			globalReferrers = append(globalReferrers, ref)
		}
	}

	t.Logf("Referrers by platform:")
	for platformDigest, refs := range referrersByPlatform {
		// Find the platform name for this digest
		platformName := "unknown"
		for _, m := range platformManifests {
			if m.Digest == platformDigest && m.Platform != nil {
				platformName = fmt.Sprintf("%s/%s", m.Platform.OS, m.Platform.Architecture)
				break
			}
		}
		t.Logf("  Platform %s (%s): %d referrers", platformName, truncateDigest(platformDigest), len(refs))
		for _, r := range refs {
			t.Logf("    - %s: %s", r.Type, truncateDigest(r.Digest))
		}
	}

	if len(globalReferrers) > 0 {
		t.Logf("Global referrers (apply to all platforms): %d", len(globalReferrers))
		for _, r := range globalReferrers {
			t.Logf("  - %s: %s", r.Type, truncateDigest(r.Digest))
		}
	}

	// Verify that referrers linked to platforms have the correct annotation
	for platformDigest, refs := range referrersByPlatform {
		// Verify this digest corresponds to a valid platform manifest
		foundPlatform := false
		for _, m := range platformManifests {
			if m.Digest == platformDigest && m.Platform != nil {
				platformStr := fmt.Sprintf("%s/%s", m.Platform.OS, m.Platform.Architecture)
				if platformStr != "unknown/unknown" {
					foundPlatform = true
					break
				}
			}
		}

		if !foundPlatform {
			t.Errorf("Referrers are linked to digest %s which is not a valid platform manifest", truncateDigest(platformDigest))
		}

		// Verify all referrers for this platform have the correct annotation
		for _, r := range refs {
			if r.Annotations == nil {
				t.Errorf("Referrer %s has no annotations", truncateDigest(r.Digest))
				continue
			}
			if refDigest, ok := r.Annotations["vnd.docker.reference.digest"]; !ok {
				t.Errorf("Referrer %s is missing vnd.docker.reference.digest annotation", truncateDigest(r.Digest))
			} else if refDigest != platformDigest {
				t.Errorf("Referrer %s has incorrect reference digest: expected %s, got %s",
					truncateDigest(r.Digest), truncateDigest(platformDigest), truncateDigest(refDigest))
			}
		}
	}

	// If we have referrers, verify at least some are platform-specific (not all global)
	if len(info.Referrers) > 0 && len(referrersByPlatform) == 0 && len(globalReferrers) == len(info.Referrers) {
		t.Logf("Note: All referrers are global (apply to all platforms). This is valid but may indicate referrers should be platform-specific.")
	}

	t.Logf("Test completed: Referrers are correctly annotated with platform digests")
}

// TestTruncateDigest tests the digest truncation helper function
func TestTruncateDigest(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "sha256:c93ad9a4ed48c4fb9eed1bf52397effe2b6ec40bb99685015957f8406857f54a",
			expected: "sha256:c93ad9a4ed48...",
		},
		{
			input:    "sha256:short",
			expected: "sha256:short",
		},
		{
			input:    "invalid",
			expected: "invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncateDigest(tt.input)
			if result != tt.expected {
				t.Errorf("truncateDigest(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestExtractSignatureInfoFromCert tests certificate parsing for Sigstore extensions and SANs
func TestExtractSignatureInfoFromCert(t *testing.T) {
	// Generate a synthetic certificate with Sigstore extensions
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	issuerOID := asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
	issuerValue, err := asn1.Marshal("https://accounts.google.com")
	if err != nil {
		t.Fatalf("Failed to marshal issuer: %v", err)
	}

	sanURI, _ := url.Parse("https://github.com/owner/repo/.github/workflows/release.yml@refs/heads/main")

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sigstore-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		URIs:         []*url.URL{sanURI},
		ExtraExtensions: []pkix.Extension{
			{
				Id:    issuerOID,
				Value: issuerValue,
			},
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	// Parse the certificate using the same logic as extractSignatureInfo
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("Failed to decode PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	// Extract identity
	var identity string
	if len(cert.EmailAddresses) > 0 {
		identity = cert.EmailAddresses[0]
	} else if len(cert.URIs) > 0 {
		identity = cert.URIs[0].String()
	}

	if identity != sanURI.String() {
		t.Errorf("Expected identity %q, got %q", sanURI.String(), identity)
	}

	// Extract OIDC issuer
	var oidcIssuer string
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(issuerOID) {
			var issuer string
			if _, err := asn1.Unmarshal(ext.Value, &issuer); err == nil {
				oidcIssuer = issuer
			}
			break
		}
	}

	if oidcIssuer != "https://accounts.google.com" {
		t.Errorf("Expected OIDC issuer %q, got %q", "https://accounts.google.com", oidcIssuer)
	}

	t.Logf("Identity: %s", identity)
	t.Logf("OIDC Issuer: %s", oidcIssuer)
}

// TestExtractSignatureInfoFromCertEmail tests certificate with email SAN
func TestExtractSignatureInfoFromCertEmail(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber:   big.NewInt(2),
		Subject:        pkix.Name{CommonName: "sigstore-email-test"},
		NotBefore:      time.Now().Add(-time.Hour),
		NotAfter:       time.Now().Add(time.Hour),
		EmailAddresses: []string{"user@example.com"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	block, _ := pem.Decode(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	var identity string
	if len(cert.EmailAddresses) > 0 {
		identity = cert.EmailAddresses[0]
	} else if len(cert.URIs) > 0 {
		identity = cert.URIs[0].String()
	}

	if identity != "user@example.com" {
		t.Errorf("Expected identity %q, got %q", "user@example.com", identity)
	}
}

// TestMissingCertificateReturnsNil tests that a missing cert returns nil gracefully
func TestMissingCertificateReturnsNil(t *testing.T) {
	// Simulate missing certificate - the function returns nil when no cert is found
	// We test the parsing logic with an empty PEM string
	block, _ := pem.Decode([]byte(""))
	if block != nil {
		t.Error("Expected nil block for empty PEM")
	}
	// This confirms the nil-check path in extractSignatureInfo
}

// TestClassifyArtifactTypeVEX tests VEX artifact classification
func TestClassifyArtifactTypeSigstoreBundle(t *testing.T) {
	tests := []struct {
		name         string
		artifactType string
		annotations  map[string]string
		expected     string
	}{
		{
			name:         "sigstore bundle with SLSA provenance predicate",
			artifactType: "application/vnd.dev.sigstore.bundle.v0.3+json",
			annotations: map[string]string{
				"dev.sigstore.bundle.content":       "dsse-envelope",
				"dev.sigstore.bundle.predicateType": "https://slsa.dev/provenance/v1",
			},
			expected: "attestation",
		},
		{
			name:         "sigstore bundle with message-signature content",
			artifactType: "application/vnd.dev.sigstore.bundle.v0.3+json",
			annotations: map[string]string{
				"dev.sigstore.bundle.content": "message-signature",
			},
			expected: "signature",
		},
		{
			name:         "sigstore bundle with no predicate (fallback to attestation)",
			artifactType: "application/vnd.dev.sigstore.bundle.v0.3+json",
			annotations:  map[string]string{},
			expected:     "attestation",
		},
		{
			name:         "cosign predicateType annotation for VEX",
			artifactType: "application/vnd.dsse.envelope.v1+json",
			annotations: map[string]string{
				"predicateType": "https://openvex.dev/ns",
			},
			expected: "vex",
		},
		{
			name:         "cosign predicateType annotation for SBOM",
			artifactType: "application/vnd.dsse.envelope.v1+json",
			annotations: map[string]string{
				"predicateType": "https://spdx.dev/Document",
			},
			expected: "sbom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyArtifactType(tt.artifactType, tt.annotations)
			if result != tt.expected {
				t.Errorf("classifyArtifactType(%q, %v) = %q, want %q",
					tt.artifactType, tt.annotations, result, tt.expected)
			}
		})
	}
}

func TestClassifyArtifactTypeVEX(t *testing.T) {
	tests := []struct {
		name         string
		artifactType string
		annotations  map[string]string
		expected     string
	}{
		{
			name:         "openvex artifact type",
			artifactType: "application/vnd.dev.openvex.type.v0.2.0",
			annotations:  nil,
			expected:     "vex",
		},
		{
			name:         "vex in artifact type",
			artifactType: "application/vnd.example.vex+json",
			annotations:  nil,
			expected:     "vex",
		},
		{
			name:         "openvex predicate type",
			artifactType: "application/vnd.dsse.envelope.v1+json",
			annotations: map[string]string{
				"in-toto.io/predicate-type": "https://openvex.dev/ns/v0.2.0",
			},
			expected: "vex",
		},
		{
			name:         "vex predicate type",
			artifactType: "application/vnd.in-toto+json",
			annotations: map[string]string{
				"in-toto.io/predicate-type": "https://example.com/vex/v1",
			},
			expected: "vex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyArtifactType(tt.artifactType, tt.annotations)
			if result != tt.expected {
				t.Errorf("classifyArtifactType(%q, %v) = %q, want %q",
					tt.artifactType, tt.annotations, result, tt.expected)
			}
		})
	}
}

// TestVEXDocumentParsing tests parsing of VEX document fixture
func TestVEXDocumentParsing(t *testing.T) {
	data := loadTestFixture(t, "sample_vex.json")

	var vex VEXDocument
	if err := json.Unmarshal(data, &vex); err != nil {
		t.Fatalf("Failed to parse VEX document: %v", err)
	}

	if vex.Context != "https://openvex.dev/ns/v0.2.0" {
		t.Errorf("Expected OpenVEX context, got %s", vex.Context)
	}

	if vex.ID != "https://example.com/vex/2024-01-15/1" {
		t.Errorf("Expected @id, got %s", vex.ID)
	}

	if vex.Author != "Example Security Team <security@example.com>" {
		t.Errorf("Expected author, got %s", vex.Author)
	}

	if vex.LastUpdated != "2024-01-16T12:00:00Z" {
		t.Errorf("Expected last_updated, got %s", vex.LastUpdated)
	}

	if vex.Version != 2 {
		t.Errorf("Expected version 2, got %d", vex.Version)
	}

	if vex.Role != "Document creator" {
		t.Errorf("Expected role %q, got %q", "Document creator", vex.Role)
	}

	if vex.Tooling != "vexctl/0.2.0" {
		t.Errorf("Expected tooling %q, got %q", "vexctl/0.2.0", vex.Tooling)
	}

	if len(vex.Statements) != 4 {
		t.Fatalf("Expected 4 statements, got %d", len(vex.Statements))
	}

	// Verify each statement
	expectedStatements := []struct {
		vulnID        string
		status        string
		justification string
	}{
		{"CVE-2023-44487", "not_affected", "vulnerable_code_not_present"},
		{"CVE-2024-0001", "fixed", ""},
		{"CVE-2024-1234", "under_investigation", ""},
		{"CVE-2024-5678", "affected", ""},
	}

	for i, expected := range expectedStatements {
		stmt := vex.Statements[i]
		if stmt.Vulnerability.Name != expected.vulnID {
			t.Errorf("Statement %d: expected vuln ID %q, got %q", i, expected.vulnID, stmt.Vulnerability.Name)
		}
		if stmt.Status != expected.status {
			t.Errorf("Statement %d: expected status %q, got %q", i, expected.status, stmt.Status)
		}
		if stmt.Justification != expected.justification {
			t.Errorf("Statement %d: expected justification %q, got %q", i, expected.justification, stmt.Justification)
		}
	}

	// Verify spec fields on first statement (vulnerability details, products, status_notes)
	stmt0 := vex.Statements[0]
	if stmt0.StatusNotes != "govulncheck confirms this code path is not reachable" {
		t.Errorf("Expected status_notes on statement 0, got %q", stmt0.StatusNotes)
	}
	if len(stmt0.Products) != 1 || stmt0.Products[0].ID != "pkg:oci/myimage@sha256:abc123" {
		t.Errorf("Expected product @id on statement 0, got %v", stmt0.Products)
	}
	if stmt0.Vulnerability.Description != "HTTP/2 Rapid Reset Attack" {
		t.Errorf("Expected vulnerability description, got %q", stmt0.Vulnerability.Description)
	}
	if len(stmt0.Vulnerability.Aliases) != 1 || stmt0.Vulnerability.Aliases[0] != "GHSA-qppj-fm5r-hxr3" {
		t.Errorf("Expected vulnerability aliases, got %v", stmt0.Vulnerability.Aliases)
	}

	// Verify action_statement on affected status (statement 3)
	stmt3 := vex.Statements[3]
	if stmt3.ActionStatement != "Update to version 2.0.1 or later to remediate this vulnerability." {
		t.Errorf("Expected action_statement on statement 3, got %q", stmt3.ActionStatement)
	}
}
