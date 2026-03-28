package score

import (
	"regexp"
	"strings"

	"github.com/hkolvenbach/oci-explorer/registry"
)

// Result holds the computed supply chain score with grade and criteria breakdown.
type Result struct {
	Score       float64           `json:"score"`
	MaxScore    float64           `json:"maxScore"`
	Grade       string            `json:"grade"`
	Criteria    []Criterion       `json:"criteria"`
	MinimalBase MinimalBaseDetail `json:"minimalBaseDetails"`
}

// Criterion describes a single scoring factor with pass/fail status.
type Criterion struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Desc    string `json:"desc"`
	Present bool   `json:"present"`
}

// MinimalBaseDetail holds the pass/fail breakdown for minimal base image traits.
type MinimalBaseDetail struct {
	FewLayers        bool `json:"fewLayers"`
	SmallSize        bool `json:"smallSize"`
	NonRoot          bool `json:"nonRoot"`
	NoShellEntrypoint bool `json:"noShellEntrypoint"`
}

var shellRegexp = regexp.MustCompile(`\b(sh|bash|ash|zsh)\b`)

// Compute calculates a supply chain security score (0–10) based on attached
// referrer artifacts and minimal-base image traits.
//
// Supply chain artifacts (2 points each): signature, attestation, sbom, vex.
// Minimal base traits (0.5 points each, max 2): few layers, small size,
// non-root user, no shell entrypoint.
func Compute(referrers []registry.Referrer, manifest *registry.Manifest, config *registry.ImageConfig) Result {
	var s float64

	// Supply chain artifacts — 2 points each.
	hasType := func(t string) bool {
		for _, r := range referrers {
			if r.Type == t {
				return true
			}
		}
		return false
	}

	hasSig := hasType("signature")
	hasAtt := hasType("attestation")
	hasSbom := hasType("sbom")
	hasVex := hasType("vex")

	for _, present := range []bool{hasSig, hasAtt, hasSbom, hasVex} {
		if present {
			s += 2
		}
	}

	// Minimal base traits — 0.5 points each.
	var mb MinimalBaseDetail

	if manifest != nil {
		mb.FewLayers = len(manifest.Layers) <= 5
		var totalSize int64
		for _, l := range manifest.Layers {
			totalSize += l.Size
		}
		mb.SmallSize = totalSize <= 50*1024*1024
	}

	if config != nil && config.Config != nil {
		user := config.Config.User
		mb.NonRoot = user != "" && user != "0" && user != "root"

		ep := strings.Join(config.Config.Entrypoint, " ")
		mb.NoShellEntrypoint = ep != "" && !shellRegexp.MatchString(ep)
	}

	var baseScore float64
	for _, pass := range []bool{mb.FewLayers, mb.SmallSize, mb.NonRoot, mb.NoShellEntrypoint} {
		if pass {
			baseScore += 0.5
		}
	}
	s += baseScore

	criteria := []Criterion{
		{Key: "signature", Label: "Signature", Desc: "Image is signed (Cosign/Notary)", Present: hasSig},
		{Key: "attestation", Label: "Attestation", Desc: "Build provenance attestation (SLSA)", Present: hasAtt},
		{Key: "sbom", Label: "SBOM", Desc: "Software Bill of Materials attached", Present: hasSbom},
		{Key: "vex", Label: "VEX", Desc: "Vulnerability Exploitability eXchange document", Present: hasVex},
		{Key: "minimalBase", Label: "Minimal Base", Desc: "Few layers, small size, non-root, no shell entrypoint", Present: baseScore >= 1},
	}

	return Result{
		Score:       s,
		MaxScore:    10,
		Grade:       grade(s),
		Criteria:    criteria,
		MinimalBase: mb,
	}
}

func grade(s float64) string {
	switch {
	case s >= 10:
		return "A+"
	case s >= 8:
		return "A"
	case s >= 6:
		return "B"
	case s >= 4:
		return "C"
	default:
		return "D"
	}
}

// GradeColor returns the hex color (without #) for a grade.
// Used by the badge package for rendering; the frontend maps grades to colors itself.
func GradeColor(grade string) string {
	switch grade {
	case "A+":
		return "22c55e"
	case "A":
		return "4ade80"
	case "B":
		return "eab308"
	case "C":
		return "fb923c"
	default:
		return "f87171"
	}
}
