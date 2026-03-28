package score

import (
	"regexp"
	"strings"

	"github.com/hkolvenbach/oci-explorer/registry"
)

// Result holds the computed supply chain score with grade and color metadata.
type Result struct {
	Score    float64 `json:"score"`
	MaxScore float64 `json:"maxScore"`
	Grade    string  `json:"grade"`
	Color    string  `json:"color"`
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
	for _, t := range []string{"signature", "attestation", "sbom", "vex"} {
		if hasType(t) {
			s += 2
		}
	}

	// Minimal base traits — 0.5 points each.
	if manifest != nil {
		if len(manifest.Layers) <= 5 {
			s += 0.5
		}

		var totalSize int64
		for _, l := range manifest.Layers {
			totalSize += l.Size
		}
		if totalSize <= 50*1024*1024 {
			s += 0.5
		}
	}

	if config != nil && config.Config != nil {
		user := config.Config.User
		if user != "" && user != "0" && user != "root" {
			s += 0.5
		}

		ep := strings.Join(config.Config.Entrypoint, " ")
		if ep != "" && !shellRegexp.MatchString(ep) {
			s += 0.5
		}
	}

	return Result{
		Score:    s,
		MaxScore: 10,
		Grade:    grade(s),
		Color:    color(s),
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

func color(s float64) string {
	switch {
	case s >= 10:
		return "22c55e"
	case s >= 8:
		return "4ade80"
	case s >= 6:
		return "eab308"
	case s >= 4:
		return "fb923c"
	default:
		return "f87171"
	}
}
