package score_test

import (
	"testing"

	"github.com/hkolvenbach/oci-explorer/registry"
	"github.com/hkolvenbach/oci-explorer/score"
)

func mb(n int64) int64 { return n * 1024 * 1024 }

func TestCompute(t *testing.T) {
	allArtifactReferrers := []registry.Referrer{
		{Type: "signature"},
		{Type: "attestation"},
		{Type: "sbom"},
		{Type: "vex"},
	}

	fewLayersManifest := &registry.Manifest{
		Layers: []registry.Descriptor{
			{Size: mb(5)},
			{Size: mb(5)},
			{Size: mb(5)},
		},
	}

	nonRootAppConfig := &registry.ImageConfig{
		Config: &registry.ContainerConfig{
			User:       "nonroot",
			Entrypoint: []string{"/app/server"},
		},
	}

	tests := []struct {
		name      string
		referrers []registry.Referrer
		manifest  *registry.Manifest
		config    *registry.ImageConfig
		wantScore float64
		wantGrade string
	}{
		{
			name:      "Perfect score (10): all 4 artifacts + all 4 base image traits",
			referrers: allArtifactReferrers,
			manifest:  fewLayersManifest,
			config:    nonRootAppConfig,
			wantScore: 10,
			wantGrade: "A+",

		},
		{
			name:      "Artifacts only (8): all 4 artifacts, no base image traits (10 layers, no config)",
			referrers: allArtifactReferrers,
			manifest: &registry.Manifest{
				Layers: func() []registry.Descriptor {
					layers := make([]registry.Descriptor, 10)
					for i := range layers {
						layers[i] = registry.Descriptor{Size: mb(10)}
					}
					return layers
				}(),
			},
			config:    nil,
			wantScore: 8,
			wantGrade: "A",
		},
		{
			name:      "Minimal base only (2): no artifacts, all base traits",
			referrers: nil,
			manifest:  fewLayersManifest,
			config:    nonRootAppConfig,
			wantScore: 2,
			wantGrade: "D",
		},
		{
			name: "Partial (5): sig + sbom + few layers + small size",
			referrers: []registry.Referrer{
				{Type: "signature"},
				{Type: "sbom"},
			},
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{
					{Size: mb(5)},
					{Size: mb(5)},
				},
			},
			config:    nil,
			wantScore: 5,
			wantGrade: "C",
		},
		{
			name:      "Empty (0): nil referrers, nil manifest, nil config",
			referrers: nil,
			manifest:  nil,
			config:    nil,
			wantScore: 0,
			wantGrade: "D",
		},
		{
			name:      "Single signature (2): one signature referrer only",
			referrers: []registry.Referrer{{Type: "signature"}},
			manifest:  nil,
			config:    nil,
			wantScore: 2,
			wantGrade: "D",
		},
		{
			name: "Boundary B (6): sig + att + sbom, no base traits",
			referrers: []registry.Referrer{
				{Type: "signature"},
				{Type: "attestation"},
				{Type: "sbom"},
			},
			manifest:  nil,
			config:    nil,
			wantScore: 6,
			wantGrade: "B",
		},
		{
			name:      "Boundary A (8): all 4 artifacts, no base traits",
			referrers: allArtifactReferrers,
			manifest:  nil,
			config:    nil,
			wantScore: 8,
			wantGrade: "A",
		},
		{
			name:      "Nil manifest with config (1): non-root user + /app entrypoint = 1 point",
			referrers: nil,
			manifest:  nil,
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "nonroot",
					Entrypoint: []string{"/app/server"},
				},
			},
			wantScore: 1,
			wantGrade: "D",
		},
		{
			name:      "Shell entrypoint (1.5): few layers + small + non-root but shell entrypoint",
			referrers: nil,
			manifest:  fewLayersManifest,
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "nonroot",
					Entrypoint: []string{"/bin/sh", "-c", "/app/run.sh"},
				},
			},
			wantScore: 1.5,
			wantGrade: "D",
		},
		{
			name:      "Root user (1.5): few layers + small + /app entrypoint but root user",
			referrers: nil,
			manifest:  fewLayersManifest,
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "root",
					Entrypoint: []string{"/app/server"},
				},
			},
			wantScore: 1.5,
			wantGrade: "D",
		},
		{
			name:      `User "0" (1.5): few layers + small + /app entrypoint but user "0"`,
			referrers: nil,
			manifest:  fewLayersManifest,
			config: &registry.ImageConfig{
				Config: &registry.ContainerConfig{
					User:       "0",
					Entrypoint: []string{"/app/server"},
				},
			},
			wantScore: 1.5,
			wantGrade: "D",
		},
		{
			name:      "Size exactly 50MB (1): scores (<=50MB)",
			referrers: nil,
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{
					{Size: mb(25)},
					{Size: mb(25)},
				},
			},
			config:    nil,
			wantScore: 1,
			wantGrade: "D",
		},
		{
			name:      "Size over 50MB (0.5): only few layers scores",
			referrers: nil,
			manifest: &registry.Manifest{
				Layers: []registry.Descriptor{
					{Size: mb(30)},
					{Size: mb(30)},
				},
			},
			config:    nil,
			wantScore: 0.5,
			wantGrade: "D",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := score.Compute(tt.referrers, tt.manifest, tt.config)

			if result.Score != tt.wantScore {
				t.Errorf("Score = %v, want %v", result.Score, tt.wantScore)
			}
			if result.MaxScore != 10 {
				t.Errorf("MaxScore = %v, want 10", result.MaxScore)
			}
			if result.Grade != tt.wantGrade {
				t.Errorf("Grade = %q, want %q", result.Grade, tt.wantGrade)
			}
		})
	}
}
