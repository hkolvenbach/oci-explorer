package registry

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/sync/errgroup"
)

// verbose controls whether verbose logging is enabled
var verbose bool

// SetVerbose enables or disables verbose logging
func SetVerbose(v bool) {
	verbose = v
}

// logVerbose prints a message only if verbose mode is enabled
func logVerbose(format string, args ...interface{}) {
	if verbose {
		log.Printf("[VERBOSE] [registry] "+format, args...)
	}
}

// ImageInfo represents the full OCI image structure
type ImageInfo struct {
	Repository   string       `json:"repository"`
	Tag          string       `json:"tag"`
	Digest       string       `json:"digest"`
	Created      string       `json:"created"`
	Architecture string       `json:"architecture"`
	OS           string       `json:"os"`
	ImageIndex   *ImageIndex  `json:"imageIndex,omitempty"`
	Manifest     *Manifest    `json:"manifest"`
	Config       *ImageConfig `json:"config"`
	Tags         []string     `json:"tags"`
	Referrers    []Referrer   `json:"referrers"`
	// PlatformDigest is the digest of the resolved platform manifest (may differ from Digest for multi-platform images)
	PlatformDigest string `json:"platformDigest,omitempty"`
}

type ImageIndex struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Manifests     []IndexManifest   `json:"manifests"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

type IndexManifest struct {
	MediaType    string            `json:"mediaType"`
	Digest       string            `json:"digest"`
	Size         int64             `json:"size"`
	Platform     *Platform         `json:"platform,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	ArtifactType string            `json:"artifactType,omitempty"`
	Config       *ImageConfig      `json:"config,omitempty"`
}

type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
}

type Manifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType"`
	Config        Descriptor        `json:"config"`
	Layers        []Descriptor      `json:"layers"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

type Descriptor struct {
	MediaType   string            `json:"mediaType"`
	Digest      string            `json:"digest"`
	Size        int64             `json:"size"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Platform    *Platform         `json:"platform,omitempty"`
}

type ImageConfig struct {
	Created      string           `json:"created"`
	Author       string           `json:"author,omitempty"`
	Architecture string           `json:"architecture"`
	OS           string           `json:"os"`
	Config       *ContainerConfig `json:"config,omitempty"`
	RootFS       *RootFS          `json:"rootfs,omitempty"`
	History      []HistoryEntry   `json:"history,omitempty"`
}

type ContainerConfig struct {
	User         string              `json:"User,omitempty"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts,omitempty"`
	Env          []string            `json:"Env,omitempty"`
	Entrypoint   []string            `json:"Entrypoint,omitempty"`
	Cmd          []string            `json:"Cmd,omitempty"`
	WorkingDir   string              `json:"WorkingDir,omitempty"`
	Labels       map[string]string   `json:"Labels,omitempty"`
}

type RootFS struct {
	Type    string   `json:"type"`
	DiffIDs []string `json:"diff_ids"`
}

type HistoryEntry struct {
	Created    string `json:"created"`
	CreatedBy  string `json:"created_by"`
	EmptyLayer bool   `json:"empty_layer,omitempty"`
	Comment    string `json:"comment,omitempty"`
}

type Referrer struct {
	Type          string            `json:"type"`
	MediaType     string            `json:"mediaType"`
	Digest        string            `json:"digest"`
	Size          int64             `json:"size"`
	ArtifactType  string            `json:"artifactType"`
	Annotations   map[string]string `json:"annotations,omitempty"`
	SignatureInfo *SignatureInfo     `json:"signatureInfo,omitempty"`
}

// SignatureInfo contains cosign signature verification details.
type SignatureInfo struct {
	Issuer   string `json:"issuer"`
	Identity string `json:"identity"`
}

// Client handles OCI registry operations
type Client struct {
	keychain authn.Keychain
}

// NewClient creates a new registry client
func NewClient() *Client {
	logVerbose("Creating new registry client with default keychain")
	return &Client{
		keychain: authn.DefaultKeychain,
	}
}

// InspectImage fetches and parses image information from a registry
func (c *Client) InspectImage(imageRef string) (*ImageInfo, error) {
	logVerbose("Parsing image reference: %s", imageRef)
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		logVerbose("Failed to parse reference: %v", err)
		return nil, fmt.Errorf("invalid image reference: %w", err)
	}
	logVerbose("Parsed reference - Registry: %s, Repository: %s", ref.Context().Registry.Name(), ref.Context().RepositoryStr())

	logVerbose("Fetching image descriptor from registry...")
	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(c.keychain))
	if err != nil {
		logVerbose("Failed to fetch descriptor: %v", err)
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}
	logVerbose("Received descriptor - MediaType: %s, Digest: %s", desc.MediaType, desc.Digest.String())

	info := &ImageInfo{
		Repository: ref.Context().String(),
		Digest:     desc.Digest.String(),
		Tags:       []string{},
		Referrers:  []Referrer{},
	}

	if tag, ok := ref.(name.Tag); ok {
		info.Tag = tag.TagStr()
		info.Tags = append(info.Tags, tag.TagStr())
		logVerbose("Image tag: %s", tag.TagStr())
	}

	logVerbose("Processing manifest based on media type: %s", desc.MediaType)
	switch desc.MediaType {
	case "application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.index.v1+json":
		logVerbose("Detected multi-platform image index")
		if err := c.populateFromIndex(info, desc); err != nil {
			return nil, err
		}
	case "application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.oci.image.manifest.v1+json":
		logVerbose("Detected single-platform image manifest")
		if err := c.populateFromImage(info, desc); err != nil {
			return nil, err
		}
	default:
		logVerbose("Unknown media type: %s", desc.MediaType)
	}

	// Fetch referrers - check the index digest and all platform manifest digests in parallel
	logVerbose("Fetching referrers (OCI 1.1 artifacts)...")

	var (
		referrersMu     sync.Mutex
		referrers       []Referrer
		existingDigests = make(map[string]bool)
	)

	// Helper function to safely add referrers
	addReferrers := func(newReferrers []Referrer) {
		referrersMu.Lock()
		defer referrersMu.Unlock()
		for _, r := range newReferrers {
			if !existingDigests[r.Digest] {
				existingDigests[r.Digest] = true
				referrers = append(referrers, r)
			}
		}
	}

	// Helper function to safely add a single referrer
	addReferrer := func(r Referrer) {
		referrersMu.Lock()
		defer referrersMu.Unlock()
		if !existingDigests[r.Digest] {
			existingDigests[r.Digest] = true
			referrers = append(referrers, r)
		}
	}

	g, _ := errgroup.WithContext(context.Background())

	// Fetch referrers for the main digest (index for multi-platform images) in parallel
	g.Go(func() error {
		indexReferrers, _ := c.fetchReferrers(ref, desc.Digest.String())
		logVerbose("Found %d referrers for index digest %s", len(indexReferrers), truncateDigest(desc.Digest.String()))
		addReferrers(indexReferrers)
		return nil
	})

	// Check all platform manifest digests for referrers in parallel
	// SBOMs and attestations are often attached to specific platform manifests
	if info.ImageIndex != nil {
		for _, m := range info.ImageIndex.Manifests {
			m := m // capture loop variable

			// Check if this manifest entry itself is an artifact (has artifactType but no platform)
			// This is local processing, no network call needed
			if m.ArtifactType != "" && m.Platform == nil {
				refType := classifyArtifactType(m.ArtifactType, m.Annotations)
				addReferrer(Referrer{
					Type:         refType,
					MediaType:    m.MediaType,
					Digest:       m.Digest,
					Size:         m.Size,
					ArtifactType: m.ArtifactType,
					Annotations:  m.Annotations,
				})
				logVerbose("  Found artifact in index: type=%s, artifactType=%s, digest=%s",
					refType, m.ArtifactType, truncateDigest(m.Digest))
			}

			// Check for Docker BuildKit attestation manifests (used by Kairos and other images)
			// These have annotation vnd.docker.reference.type: attestation-manifest
			//
			// Mapping between com.docker.official-images.bashbrew.arch and platform tags:
			//   - "amd64"     -> "linux/amd64"
			//   - "arm32v6"   -> "linux/arm/v6"
			//   - "arm32v7"   -> "linux/arm/v7"
			//   - "arm64v8"   -> "linux/arm64/v8"
			//   - "i386"      -> "linux/386"
			//   - "ppc64le"   -> "linux/ppc64le"
			//   - "riscv64"   -> "linux/riscv64"
			//   - "s390x"     -> "linux/s390x"
			//
			// The attestation manifest should have the same com.docker.official-images.bashbrew.arch
			// annotation as its corresponding platform manifest.
			if refType, ok := m.Annotations["vnd.docker.reference.type"]; ok && refType == "attestation-manifest" {
				// Get the platform digest this attestation is linked to
				platformDigest := m.Annotations["vnd.docker.reference.digest"]
				if platformDigest == "" {
					logVerbose("Attestation manifest %s missing vnd.docker.reference.digest annotation", truncateDigest(m.Digest))
				} else {
					logVerbose("Found BuildKit attestation manifest: %s (linked to platform %s)",
						truncateDigest(m.Digest), truncateDigest(platformDigest))
				}

				g.Go(func() error {
					attestationDigest := m.Digest           // capture attestation digest
					attestationAnnotations := m.Annotations // capture annotations

					referrersMu.Lock()
					alreadyAdded := existingDigests[attestationDigest]
					referrersMu.Unlock()

					if !alreadyAdded {
						// Fetch the attestation manifest to inspect its layers for SBOM info
						attestationReferrers, err := c.extractAttestationInfo(ref, attestationDigest, m.Size, attestationAnnotations)
						if err != nil {
							logVerbose("Failed to extract attestation info: %v", err)
							return nil // Non-fatal
						}
						for _, r := range attestationReferrers {
							// Ensure the platform reference digest annotation is preserved
							if r.Annotations == nil {
								r.Annotations = make(map[string]string)
							}
							// Explicitly set the reference digest if it's in the attestation annotations
							if refDigest, ok := attestationAnnotations["vnd.docker.reference.digest"]; ok {
								r.Annotations["vnd.docker.reference.digest"] = refDigest
							}
							addReferrer(r)
							logVerbose("  Found attestation referrer: type=%s, digest=%s, platform=%s",
								r.Type, truncateDigest(r.Digest), truncateDigest(r.Annotations["vnd.docker.reference.digest"]))
						}
					}
					return nil
				})
			}

			// Check referrers API for this manifest (only for actual platform manifests)
			if m.Digest != desc.Digest.String() && m.Digest != "" && m.Platform != nil {
				// Skip attestation manifests - they don't have referrers of their own
				if _, isAttestation := m.Annotations["vnd.docker.reference.type"]; isAttestation {
					continue
				}

				g.Go(func() error {
					platformDigest := m.Digest // capture platform digest
					logVerbose("Checking referrers for platform manifest: %s (%s/%s)",
						truncateDigest(platformDigest), m.Platform.OS, m.Platform.Architecture)

					platformReferrers, _ := c.fetchReferrers(ref, platformDigest)
					for _, r := range platformReferrers {
						// Ensure referrers have the platform digest annotation for proper filtering
						if r.Annotations == nil {
							r.Annotations = make(map[string]string)
						}
						// Set the reference digest annotation to link this referrer to the platform manifest
						r.Annotations["vnd.docker.reference.digest"] = platformDigest
						addReferrer(r)
						logVerbose("  Found new referrer: type=%s, digest=%s, linked to platform %s",
							r.Type, truncateDigest(r.Digest), truncateDigest(platformDigest))
					}
					return nil
				})
			}
		}
	}

	// Wait for all referrer fetches to complete
	if err := g.Wait(); err != nil {
		logVerbose("Error fetching referrers: %v", err)
		// Continue anyway, some referrers may have been fetched successfully
	}

	// Discover cosign-tagged artifacts (.sig and .att tags)
	// Cosign stores signatures and attestations using a tag naming convention:
	//   sha256-<hex>.sig — cosign signatures
	//   sha256-<hex>.att — cosign attestations (may contain VEX, provenance, etc.)
	cosignDigests := []string{desc.Digest.String()}
	if info.ImageIndex != nil {
		for _, m := range info.ImageIndex.Manifests {
			// Only check real platform manifests, not attestation manifests (unknown/unknown)
			if m.Platform != nil && m.Annotations["vnd.docker.reference.type"] == "" {
				cosignDigests = append(cosignDigests, m.Digest)
			}
		}
	}
	for _, d := range cosignDigests {
		cosignRefs, err := c.fetchCosignTagArtifacts(ref, d)
		if err != nil {
			logVerbose("Error fetching cosign tag artifacts for %s: %v", truncateDigest(d), err)
			continue
		}
		for _, r := range cosignRefs {
			addReferrer(r)
		}
	}

	// Enrich signature referrers with certificate info
	for i := range referrers {
		if referrers[i].Type == "signature" {
			sigInfo, err := c.extractSignatureInfo(ref, referrers[i].Digest)
			if err != nil {
				logVerbose("Failed to extract signature info for %s: %v", truncateDigest(referrers[i].Digest), err)
				continue
			}
			referrers[i].SignatureInfo = sigInfo
		}
	}

	info.Referrers = referrers
	logVerbose("Total referrers found: %d", len(referrers))

	// Only show the tag that was requested, not all tags in the repository
	// (fetching all tags is expensive and usually not what users want)
	logVerbose("Using requested tag: %s", info.Tag)

	logVerbose("Image inspection complete")
	return info, nil
}

func (c *Client) populateFromIndex(info *ImageInfo, desc *remote.Descriptor) error {
	logVerbose("Parsing image index...")
	idx, err := desc.ImageIndex()
	if err != nil {
		logVerbose("Failed to get image index: %v", err)
		return fmt.Errorf("failed to get image index: %w", err)
	}

	indexManifest, err := idx.IndexManifest()
	if err != nil {
		logVerbose("Failed to get index manifest: %v", err)
		return fmt.Errorf("failed to get index manifest: %w", err)
	}
	logVerbose("Image index contains %d platform manifests", len(indexManifest.Manifests))

	info.ImageIndex = &ImageIndex{
		SchemaVersion: int(indexManifest.SchemaVersion),
		MediaType:     string(indexManifest.MediaType),
		Manifests:     make([]IndexManifest, 0, len(indexManifest.Manifests)),
		Annotations:   indexManifest.Annotations,
	}

	for i, m := range indexManifest.Manifests {
		im := IndexManifest{
			MediaType:    string(m.MediaType),
			Digest:       m.Digest.String(),
			Size:         m.Size,
			Annotations:  m.Annotations,
			ArtifactType: string(m.ArtifactType),
		}
		if m.Platform != nil {
			im.Platform = &Platform{
				Architecture: m.Platform.Architecture,
				OS:           m.Platform.OS,
				Variant:      m.Platform.Variant,
			}
			logVerbose("  Platform %d: %s/%s%s (digest: %s)",
				i, m.Platform.OS, m.Platform.Architecture,
				func() string {
					if m.Platform.Variant != "" {
						return "/" + m.Platform.Variant
					}
					return ""
				}(),
				truncateDigest(m.Digest.String()))
		} else if m.ArtifactType != "" {
			// This is an artifact manifest (e.g., SBOM, attestation) embedded in the index
			logVerbose("  Artifact %d: artifactType=%s (digest: %s)", i, m.ArtifactType, truncateDigest(m.Digest.String()))
		} else {
			logVerbose("  Manifest %d: unknown platform (digest: %s)", i, truncateDigest(m.Digest.String()))
		}
		info.ImageIndex.Manifests = append(info.ImageIndex.Manifests, im)
	}

	// Fetch config for all valid platforms in parallel (excluding unknown/unknown attestation manifests)
	var (
		firstValidPlatformIdx = -1
		firstValidPlatformMu  sync.Mutex
	)

	// Collect valid platform indices first
	type platformTask struct {
		index    int
		manifest v1.Descriptor
	}
	var validPlatforms []platformTask
	for i, m := range indexManifest.Manifests {
		// Skip manifests without platform info (artifacts) or with unknown/unknown platform (attestations)
		if m.Platform == nil {
			logVerbose("Skipping manifest %d: no platform info (artifact)", i)
			continue
		}
		if m.Platform.OS == "unknown" && m.Platform.Architecture == "unknown" {
			logVerbose("Skipping manifest %d: unknown/unknown platform (attestation)", i)
			continue
		}
		validPlatforms = append(validPlatforms, platformTask{index: i, manifest: m})
	}

	// Fetch configs in parallel
	g, _ := errgroup.WithContext(context.Background())
	for _, task := range validPlatforms {
		task := task // capture loop variable
		g.Go(func() error {
			i := task.index
			m := task.manifest

			logVerbose("Fetching config for platform %d: %s/%s%s (digest: %s)",
				i, m.Platform.OS, m.Platform.Architecture,
				func() string {
					if m.Platform.Variant != "" {
						return "/" + m.Platform.Variant
					}
					return ""
				}(),
				truncateDigest(m.Digest.String()))

			img, err := idx.Image(m.Digest)
			if err != nil {
				logVerbose("Failed to get image for platform %d: %v", i, err)
				return nil // Non-fatal, continue with other platforms
			}

			configFile, err := img.ConfigFile()
			if err != nil {
				logVerbose("Failed to get config file for platform %d: %v", i, err)
				return nil // Non-fatal, continue with other platforms
			}

			// Build the config for this platform
			platformConfig := &ImageConfig{
				Created:      configFile.Created.Time.Format("2006-01-02T15:04:05Z"),
				Author:       configFile.Author,
				Architecture: configFile.Architecture,
				OS:           configFile.OS,
				Config: &ContainerConfig{
					User:         configFile.Config.User,
					ExposedPorts: configFile.Config.ExposedPorts,
					Env:          configFile.Config.Env,
					Entrypoint:   configFile.Config.Entrypoint,
					Cmd:          configFile.Config.Cmd,
					WorkingDir:   configFile.Config.WorkingDir,
					Labels:       configFile.Config.Labels,
				},
				RootFS: &RootFS{
					Type:    configFile.RootFS.Type,
					DiffIDs: make([]string, 0, len(configFile.RootFS.DiffIDs)),
				},
				History: make([]HistoryEntry, 0, len(configFile.History)),
			}

			for _, diffID := range configFile.RootFS.DiffIDs {
				platformConfig.RootFS.DiffIDs = append(platformConfig.RootFS.DiffIDs, diffID.String())
			}

			for _, h := range configFile.History {
				platformConfig.History = append(platformConfig.History, HistoryEntry{
					Created:    h.Created.Time.Format("2006-01-02T15:04:05Z"),
					CreatedBy:  h.CreatedBy,
					EmptyLayer: h.EmptyLayer,
					Comment:    h.Comment,
				})
			}

			// Store the config in the IndexManifest (thread-safe since each goroutine writes to different index)
			info.ImageIndex.Manifests[i].Config = platformConfig
			logVerbose("Stored config for platform %s/%s: %d history entries",
				configFile.OS, configFile.Architecture, len(configFile.History))

			// Track the first valid platform for default display (thread-safe)
			firstValidPlatformMu.Lock()
			if firstValidPlatformIdx == -1 {
				firstValidPlatformIdx = i
			}
			firstValidPlatformMu.Unlock()

			return nil
		})
	}

	// Wait for all config fetches to complete
	if err := g.Wait(); err != nil {
		logVerbose("Error fetching platform configs: %v", err)
		// Continue anyway, some configs may have been fetched successfully
	}

	// Use the first valid platform's details for the top-level ImageInfo fields
	if firstValidPlatformIdx >= 0 {
		firstManifest := indexManifest.Manifests[firstValidPlatformIdx]
		logVerbose("Using first valid platform manifest for default display: %s", truncateDigest(firstManifest.Digest.String()))

		// Store the platform digest for referrer lookup
		info.PlatformDigest = firstManifest.Digest.String()

		img, err := idx.Image(firstManifest.Digest)
		if err != nil {
			logVerbose("Failed to get image for first valid platform: %v", err)
			return nil // Non-fatal
		}

		manifest, err := img.Manifest()
		if err != nil {
			logVerbose("Failed to get manifest: %v", err)
			return nil
		}
		logVerbose("Manifest has %d layers", len(manifest.Layers))

		info.Manifest = &Manifest{
			SchemaVersion: int(manifest.SchemaVersion),
			MediaType:     string(manifest.MediaType),
			Config: Descriptor{
				MediaType: string(manifest.Config.MediaType),
				Digest:    manifest.Config.Digest.String(),
				Size:      manifest.Config.Size,
			},
			Layers:      make([]Descriptor, 0, len(manifest.Layers)),
			Annotations: manifest.Annotations,
		}

		for layerIdx, layer := range manifest.Layers {
			logVerbose("  Layer %d: %s (%d bytes)", layerIdx, truncateDigest(layer.Digest.String()), layer.Size)
			info.Manifest.Layers = append(info.Manifest.Layers, Descriptor{
				MediaType:   string(layer.MediaType),
				Digest:      layer.Digest.String(),
				Size:        layer.Size,
				Annotations: layer.Annotations,
			})
		}

		// Use the stored config for the first valid platform
		if info.ImageIndex.Manifests[firstValidPlatformIdx].Config != nil {
			info.Config = info.ImageIndex.Manifests[firstValidPlatformIdx].Config
			info.Created = info.Config.Created
			info.Architecture = info.Config.Architecture
			info.OS = info.Config.OS
			logVerbose("Image config - OS: %s, Arch: %s, Created: %s", info.OS, info.Architecture, info.Created)
		}
	}

	return nil
}

func (c *Client) populateFromImage(info *ImageInfo, desc *remote.Descriptor) error {
	logVerbose("Parsing single-platform image...")
	img, err := desc.Image()
	if err != nil {
		logVerbose("Failed to get image: %v", err)
		return fmt.Errorf("failed to get image: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		logVerbose("Failed to get manifest: %v", err)
		return fmt.Errorf("failed to get manifest: %w", err)
	}
	logVerbose("Manifest has %d layers", len(manifest.Layers))

	info.Manifest = &Manifest{
		SchemaVersion: int(manifest.SchemaVersion),
		MediaType:     string(manifest.MediaType),
		Config: Descriptor{
			MediaType: string(manifest.Config.MediaType),
			Digest:    manifest.Config.Digest.String(),
			Size:      manifest.Config.Size,
		},
		Layers:      make([]Descriptor, 0, len(manifest.Layers)),
		Annotations: manifest.Annotations,
	}

	for i, layer := range manifest.Layers {
		logVerbose("  Layer %d: %s (%d bytes)", i, truncateDigest(layer.Digest.String()), layer.Size)
		info.Manifest.Layers = append(info.Manifest.Layers, Descriptor{
			MediaType:   string(layer.MediaType),
			Digest:      layer.Digest.String(),
			Size:        layer.Size,
			Annotations: layer.Annotations,
		})
	}

	logVerbose("Fetching image configuration...")
	configFile, err := img.ConfigFile()
	if err != nil {
		logVerbose("Failed to get config file: %v", err)
		return fmt.Errorf("failed to get config: %w", err)
	}

	info.Created = configFile.Created.Time.Format("2006-01-02T15:04:05Z")
	info.Architecture = configFile.Architecture
	info.OS = configFile.OS
	logVerbose("Image config - OS: %s, Arch: %s, Created: %s", configFile.OS, configFile.Architecture, info.Created)

	info.Config = &ImageConfig{
		Created:      configFile.Created.Time.Format("2006-01-02T15:04:05Z"),
		Author:       configFile.Author,
		Architecture: configFile.Architecture,
		OS:           configFile.OS,
		Config: &ContainerConfig{
			User:         configFile.Config.User,
			ExposedPorts: configFile.Config.ExposedPorts,
			Env:          configFile.Config.Env,
			Entrypoint:   configFile.Config.Entrypoint,
			Cmd:          configFile.Config.Cmd,
			WorkingDir:   configFile.Config.WorkingDir,
			Labels:       configFile.Config.Labels,
		},
		RootFS: &RootFS{
			Type:    configFile.RootFS.Type,
			DiffIDs: make([]string, 0, len(configFile.RootFS.DiffIDs)),
		},
		History: make([]HistoryEntry, 0, len(configFile.History)),
	}

	for _, diffID := range configFile.RootFS.DiffIDs {
		info.Config.RootFS.DiffIDs = append(info.Config.RootFS.DiffIDs, diffID.String())
	}

	logVerbose("Build history has %d entries", len(configFile.History))
	for _, h := range configFile.History {
		info.Config.History = append(info.Config.History, HistoryEntry{
			Created:    h.Created.Time.Format("2006-01-02T15:04:05Z"),
			CreatedBy:  h.CreatedBy,
			EmptyLayer: h.EmptyLayer,
			Comment:    h.Comment,
		})
	}

	return nil
}

func (c *Client) fetchReferrers(ref name.Reference, digest string) ([]Referrer, error) {
	referrers := []Referrer{}

	repo := ref.Context()
	digestParts := strings.Split(digest, ":")
	if len(digestParts) != 2 {
		logVerbose("Invalid digest format for referrers lookup: %s", digest)
		return referrers, nil
	}

	// First, try the OCI 1.1 Referrers API endpoint: GET /v2/<name>/referrers/<digest>
	logVerbose("Trying OCI 1.1 Referrers API: /v2/%s/referrers/%s", repo.RepositoryStr(), digest)
	apiReferrers, err := c.fetchReferrersViaAPI(repo, digest)
	if err == nil && len(apiReferrers) > 0 {
		logVerbose("Found %d referrers via Referrers API", len(apiReferrers))
		return apiReferrers, nil
	}
	if err != nil {
		logVerbose("Referrers API not available or failed: %v", err)
	} else {
		logVerbose("Referrers API returned empty result, trying tag schema fallback")
	}

	// Fallback: Try referrers tag schema (sha256-<hash>)
	referrersTag := fmt.Sprintf("sha256-%s", digestParts[1])
	logVerbose("Falling back to tag schema: %s:%s", repo.String(), referrersTag)

	tagRef, err := name.NewTag(fmt.Sprintf("%s:%s", repo.String(), referrersTag))
	if err != nil {
		logVerbose("Failed to create referrers tag reference: %v", err)
		return referrers, nil
	}

	desc, err := remote.Get(tagRef, remote.WithAuthFromKeychain(c.keychain))
	if err != nil {
		logVerbose("No referrers found via tag schema (this is normal for images without attached artifacts)")
		return referrers, nil
	}
	logVerbose("Found referrers index at tag %s", referrersTag)

	idx, err := desc.ImageIndex()
	if err != nil {
		logVerbose("Failed to parse referrers index: %v", err)
		return referrers, nil
	}

	indexManifest, err := idx.IndexManifest()
	if err != nil {
		logVerbose("Failed to get referrers index manifest: %v", err)
		return referrers, nil
	}
	logVerbose("Referrers index contains %d artifacts", len(indexManifest.Manifests))

	for i, m := range indexManifest.Manifests {
		refType := classifyArtifactType(string(m.ArtifactType), m.Annotations)
		logVerbose("  Referrer %d: type=%s, artifactType=%s, digest=%s, size=%d",
			i, refType, m.ArtifactType, truncateDigest(m.Digest.String()), m.Size)

		referrers = append(referrers, Referrer{
			Type:         refType,
			MediaType:    string(m.MediaType),
			Digest:       m.Digest.String(),
			Size:         m.Size,
			ArtifactType: string(m.ArtifactType),
			Annotations:  m.Annotations,
		})
	}

	return referrers, nil
}

// ReferrersResponse represents the response from the OCI Referrers API
type ReferrersResponse struct {
	SchemaVersion int                     `json:"schemaVersion"`
	MediaType     string                  `json:"mediaType"`
	Manifests     []ReferrerManifestEntry `json:"manifests"`
}

// ReferrerManifestEntry represents a single referrer in the API response
type ReferrerManifestEntry struct {
	MediaType    string            `json:"mediaType"`
	Digest       string            `json:"digest"`
	Size         int64             `json:"size"`
	ArtifactType string            `json:"artifactType"`
	Annotations  map[string]string `json:"annotations,omitempty"`
}

// fetchReferrersViaAPI uses the OCI 1.1 Referrers API endpoint
func (c *Client) fetchReferrersViaAPI(repo name.Repository, digest string) ([]Referrer, error) {
	// Build the referrers API URL
	// Format: GET /v2/<name>/referrers/<digest>
	registry := repo.Registry
	scheme := "https"
	if registry.Scheme() != "" {
		scheme = registry.Scheme()
	}

	url := fmt.Sprintf("%s://%s/v2/%s/referrers/%s",
		scheme,
		registry.RegistryStr(),
		repo.RepositoryStr(),
		digest,
	)
	logVerbose("Referrers API URL: %s", url)

	// Get authentication for the registry
	auth, err := c.keychain.Resolve(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth: %w", err)
	}

	// Create transport with authentication
	tr, err := transport.NewWithContext(
		context.Background(),
		repo.Registry,
		auth,
		http.DefaultTransport,
		[]string{repo.Scope(transport.PullScope)},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	// Make the request
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set Accept header for OCI index
	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json")

	logVerbose("Sending GET request to Referrers API...")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	logVerbose("Referrers API response status: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode == http.StatusNotFound {
		// 404 means the registry doesn't support the Referrers API
		return nil, fmt.Errorf("referrers API not supported (404)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	// Read and parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	logVerbose("Referrers API response body length: %d bytes", len(body))

	var referrersResp ReferrersResponse
	if err := json.Unmarshal(body, &referrersResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	logVerbose("Referrers API returned %d manifests", len(referrersResp.Manifests))

	// Convert to our Referrer type
	referrers := make([]Referrer, 0, len(referrersResp.Manifests))
	for i, m := range referrersResp.Manifests {
		refType := classifyArtifactType(m.ArtifactType, m.Annotations)
		logVerbose("  Referrer %d: type=%s, artifactType=%s, digest=%s, size=%d",
			i, refType, m.ArtifactType, truncateDigest(m.Digest), m.Size)

		referrers = append(referrers, Referrer{
			Type:         refType,
			MediaType:    m.MediaType,
			Digest:       m.Digest,
			Size:         m.Size,
			ArtifactType: m.ArtifactType,
			Annotations:  m.Annotations,
		})
	}

	return referrers, nil
}

// fetchCosignTagArtifacts discovers cosign-style tagged artifacts (.sig and .att suffixes).
// Cosign uses a tag naming convention: sha256-<hex>.sig for signatures, sha256-<hex>.att for attestations.
func (c *Client) fetchCosignTagArtifacts(ref name.Reference, digest string) ([]Referrer, error) {
	var allReferrers []Referrer
	repo := ref.Context()

	digestParts := strings.Split(digest, ":")
	if len(digestParts) != 2 {
		return nil, nil
	}

	// Check for cosign signature tag (.sig)
	sigTag := fmt.Sprintf("sha256-%s.sig", digestParts[1])
	logVerbose("Checking for cosign signature tag: %s:%s", repo.String(), sigTag)

	sigTagRef, err := name.NewTag(fmt.Sprintf("%s:%s", repo.String(), sigTag))
	if err == nil {
		sigDesc, err := remote.Get(sigTagRef, remote.WithAuthFromKeychain(c.keychain))
		if err == nil {
			logVerbose("Found cosign signature manifest at tag %s", sigTag)
			allReferrers = append(allReferrers, Referrer{
				Type:         "signature",
				MediaType:    string(sigDesc.MediaType),
				Digest:       sigDesc.Digest.String(),
				Size:         sigDesc.Size,
				ArtifactType: "application/vnd.dev.cosign.simplesigning.v1+json",
				Annotations: map[string]string{
					"vnd.docker.reference.digest": digest,
				},
			})
		} else {
			logVerbose("No cosign signature tag found (this is normal)")
		}
	}

	// Check for cosign attestation tag (.att)
	attTag := fmt.Sprintf("sha256-%s.att", digestParts[1])
	logVerbose("Checking for cosign attestation tag: %s:%s", repo.String(), attTag)

	attTagRef, err := name.NewTag(fmt.Sprintf("%s:%s", repo.String(), attTag))
	if err == nil {
		attDesc, err := remote.Get(attTagRef, remote.WithAuthFromKeychain(c.keychain))
		if err == nil {
			logVerbose("Found cosign attestation manifest at tag %s", attTag)
			// Extract individual attestation layers (may contain VEX, provenance, etc.)
			attReferrers, err := c.extractAttestationInfo(ref, attDesc.Digest.String(), attDesc.Size, map[string]string{
				"vnd.docker.reference.digest": digest,
			})
			if err != nil {
				logVerbose("Failed to extract cosign attestation info: %v", err)
			} else {
				allReferrers = append(allReferrers, attReferrers...)
			}
		} else {
			logVerbose("No cosign attestation tag found (this is normal)")
		}
	}

	return allReferrers, nil
}

// FetchSBOMContent retrieves the actual SBOM content from an attestation manifest
func (c *Client) FetchSBOMContent(repository string, digest string) ([]byte, string, error) {
	logVerbose("Fetching SBOM content from %s@%s", repository, digest)

	// Create reference to the attestation manifest
	manifestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", repository, digest))
	if err != nil {
		return nil, "", fmt.Errorf("invalid manifest reference: %w", err)
	}

	// Fetch the attestation manifest
	desc, err := remote.Get(manifestRef, remote.WithAuthFromKeychain(c.keychain))
	if err != nil {
		return nil, "", fmt.Errorf("failed to fetch attestation manifest: %w", err)
	}

	img, err := desc.Image()
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse attestation: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get manifest: %w", err)
	}

	logVerbose("Searching for SBOM layer in %d layers", len(manifest.Layers))

	// Find the SBOM layer
	for _, layer := range manifest.Layers {
		predicateType := ""
		if pt, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
			predicateType = pt
		}

		predicateLower := strings.ToLower(predicateType)
		if strings.Contains(predicateLower, "spdx") ||
			strings.Contains(predicateLower, "cyclonedx") ||
			strings.Contains(predicateLower, "sbom") ||
			strings.Contains(predicateLower, "syft") {

			logVerbose("Found SBOM layer: %s (predicate: %s)", truncateDigest(layer.Digest.String()), predicateType)

			// Fetch the layer blob
			repo := manifestRef.Context()
			layerRef, err := name.NewDigest(fmt.Sprintf("%s@%s", repo.String(), layer.Digest.String()))
			if err != nil {
				return nil, "", fmt.Errorf("invalid layer reference: %w", err)
			}

			blob, err := remote.Layer(layerRef, remote.WithAuthFromKeychain(c.keychain))
			if err != nil {
				return nil, "", fmt.Errorf("failed to fetch layer: %w", err)
			}

			reader, err := blob.Compressed()
			if err != nil {
				return nil, "", fmt.Errorf("failed to read layer: %w", err)
			}
			defer reader.Close()

			// Read the attestation data
			attestationData, err := io.ReadAll(reader)
			if err != nil {
				return nil, "", fmt.Errorf("failed to read attestation data: %w", err)
			}

			logVerbose("Read %d bytes from SBOM layer", len(attestationData))

			// Try to parse as in-toto attestation and extract the predicate
			var attestation struct {
				Type          string          `json:"_type"`
				Subject       json.RawMessage `json:"subject"`
				PredicateType string          `json:"predicateType"`
				Predicate     json.RawMessage `json:"predicate"`
			}

			if err := json.Unmarshal(attestationData, &attestation); err == nil && len(attestation.Predicate) > 0 {
				// Return just the predicate (the actual SBOM)
				logVerbose("Extracted SBOM predicate from in-toto attestation")
				// Pretty print the JSON
				var prettyJSON bytes.Buffer
				if err := json.Indent(&prettyJSON, attestation.Predicate, "", "  "); err == nil {
					return prettyJSON.Bytes(), "application/json", nil
				}
				return attestation.Predicate, "application/json", nil
			}

			// Return raw data if not in-toto format
			return attestationData, "application/json", nil
		}
	}

	return nil, "", fmt.Errorf("no SBOM layer found in attestation manifest")
}

// VEXDocument represents a parsed OpenVEX document.
type VEXDocument struct {
	Context     string         `json:"@context"`
	ID          string         `json:"@id"`
	Author      string         `json:"author"`
	Timestamp   string         `json:"timestamp"`
	LastUpdated string         `json:"last_updated,omitempty"`
	Version     int            `json:"version,omitempty"`
	Statements  []VEXStatement `json:"statements"`
}

// VEXStatement represents a single VEX statement.
type VEXStatement struct {
	Vulnerability   VEXVulnerability `json:"vulnerability"`
	Products        []VEXProduct     `json:"products,omitempty"`
	Status          string           `json:"status"`
	StatusNotes     string           `json:"status_notes,omitempty"`
	Justification   string           `json:"justification,omitempty"`
	ImpactStatement string           `json:"impact_statement,omitempty"`
	Timestamp       string           `json:"timestamp,omitempty"`
}

// VEXVulnerability identifies the vulnerability in a VEX statement.
type VEXVulnerability struct {
	ID string `json:"name"`
}

// VEXProduct identifies a product affected by a VEX statement.
type VEXProduct struct {
	ID string `json:"@id"`
}

// FetchVEXContent retrieves and parses VEX content from an attestation manifest.
func (c *Client) FetchVEXContent(repository string, digest string) (*VEXDocument, error) {
	logVerbose("Fetching VEX content from %s@%s", repository, digest)

	ref, err := name.NewDigest(fmt.Sprintf("%s@%s", repository, digest))
	if err != nil {
		return nil, fmt.Errorf("invalid reference: %w", err)
	}

	// First, try fetching as a manifest (the digest might point to an attestation manifest)
	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(c.keychain))
	if err == nil {
		// Successfully fetched as manifest — iterate layers to find VEX
		img, imgErr := desc.Image()
		if imgErr == nil {
			manifest, manErr := img.Manifest()
			if manErr == nil {
				logVerbose("Searching for VEX layer in %d layers", len(manifest.Layers))
				for _, layer := range manifest.Layers {
					predicateType := layer.Annotations["in-toto.io/predicate-type"]
					if predicateType == "" {
						predicateType = layer.Annotations["predicateType"]
					}
					predicateLower := strings.ToLower(predicateType)
					if strings.Contains(predicateLower, "vex") || strings.Contains(predicateLower, "openvex") {
						logVerbose("Found VEX layer: %s (predicate: %s)", truncateDigest(layer.Digest.String()), predicateType)
						return c.fetchAndParseVEXBlob(ref.Context(), layer.Digest.String())
					}
				}
			}
		}
	}

	// If manifest fetch failed or no VEX layer found, try as a blob directly.
	// This handles the case where the digest is a layer blob (from extractAttestationInfo).
	logVerbose("Trying to fetch VEX as blob at %s", truncateDigest(digest))
	return c.fetchAndParseVEXBlob(ref.Context(), digest)
}

// fetchAndParseVEXBlob fetches a blob by digest and parses it as a VEX document,
// handling both raw VEX JSON and in-toto/DSSE wrapped attestations.
func (c *Client) fetchAndParseVEXBlob(repo name.Repository, digest string) (*VEXDocument, error) {
	blobRef, err := name.NewDigest(fmt.Sprintf("%s@%s", repo.String(), digest))
	if err != nil {
		return nil, fmt.Errorf("invalid blob reference: %w", err)
	}

	blob, err := remote.Layer(blobRef, remote.WithAuthFromKeychain(c.keychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch VEX blob: %w", err)
	}

	reader, err := blob.Compressed()
	if err != nil {
		return nil, fmt.Errorf("failed to read VEX blob: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read VEX data: %w", err)
	}

	logVerbose("Read %d bytes from VEX blob", len(data))

	// Try to parse as in-toto/DSSE attestation and extract the predicate
	var attestation struct {
		Type          string          `json:"_type"`
		PredicateType string          `json:"predicateType"`
		Predicate     json.RawMessage `json:"predicate"`
		// DSSE envelope fields
		PayloadType string `json:"payloadType"`
		Payload     string `json:"payload"`
	}

	var vexData []byte
	if err := json.Unmarshal(data, &attestation); err == nil {
		if len(attestation.Predicate) > 0 {
			// Standard in-toto format with direct predicate
			logVerbose("Extracted VEX from in-toto attestation predicate")
			vexData = attestation.Predicate
		} else if attestation.Payload != "" {
			// DSSE envelope — payload is base64-encoded in-toto statement
			logVerbose("Detected DSSE envelope, decoding payload")
			decoded, decErr := base64.StdEncoding.DecodeString(attestation.Payload)
			if decErr != nil {
				return nil, fmt.Errorf("failed to decode DSSE payload: %w", decErr)
			}
			// Parse the inner in-toto statement
			var inner struct {
				Predicate json.RawMessage `json:"predicate"`
			}
			if err := json.Unmarshal(decoded, &inner); err == nil && len(inner.Predicate) > 0 {
				vexData = inner.Predicate
			} else {
				vexData = decoded
			}
		} else {
			// Try raw data as VEX document
			vexData = data
		}
	} else {
		vexData = data
	}

	var vexDoc VEXDocument
	if err := json.Unmarshal(vexData, &vexDoc); err != nil {
		return nil, fmt.Errorf("failed to parse VEX document: %w", err)
	}

	logVerbose("Parsed VEX document: %d statements", len(vexDoc.Statements))
	return &vexDoc, nil
}

// extractAttestationInfo fetches a Docker BuildKit attestation manifest and extracts SBOM/attestation info
func (c *Client) extractAttestationInfo(ref name.Reference, digest string, size int64, indexAnnotations map[string]string) ([]Referrer, error) {
	var referrers []Referrer

	// Create a reference to the attestation manifest
	repo := ref.Context()
	manifestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", repo.String(), digest))
	if err != nil {
		return nil, fmt.Errorf("invalid manifest digest: %w", err)
	}

	logVerbose("Fetching attestation manifest: %s", truncateDigest(digest))

	// Fetch the manifest
	desc, err := remote.Get(manifestRef, remote.WithAuthFromKeychain(c.keychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch attestation manifest: %w", err)
	}

	// Parse the manifest to get its layers
	img, err := desc.Image()
	if err != nil {
		return nil, fmt.Errorf("failed to parse attestation as image: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get attestation manifest: %w", err)
	}

	logVerbose("Attestation manifest has %d layers", len(manifest.Layers))

	// Check each layer for SBOM or attestation info
	// Each layer gets its own referrer with its own digest to avoid deduplication
	for _, layer := range manifest.Layers {
		predicateType := ""
		// Check both in-toto standard annotation and cosign's annotation key
		if pt, ok := layer.Annotations["in-toto.io/predicate-type"]; ok {
			predicateType = pt
		} else if pt, ok := layer.Annotations["predicateType"]; ok {
			predicateType = pt
		}
		if predicateType != "" {
			logVerbose("  Layer %s has predicate-type: %s", truncateDigest(layer.Digest.String()), predicateType)
		}

		predicateLower := strings.ToLower(predicateType)

		// Check for SBOM predicate types
		if strings.Contains(predicateLower, "spdx") ||
			strings.Contains(predicateLower, "cyclonedx") ||
			strings.Contains(predicateLower, "sbom") ||
			strings.Contains(predicateLower, "syft") {
			// Create a referrer for this SBOM layer
			// Use the layer digest (not the manifest digest) to ensure uniqueness
			annotations := make(map[string]string)
			for k, v := range indexAnnotations {
				annotations[k] = v
			}
			annotations["in-toto.io/predicate-type"] = predicateType

			referrers = append(referrers, Referrer{
				Type:         "sbom",
				MediaType:    string(layer.MediaType),
				Digest:       layer.Digest.String(), // Use layer digest for uniqueness
				Size:         layer.Size,
				ArtifactType: predicateType,
				Annotations:  annotations,
			})
			logVerbose("  Found SBOM layer: predicate=%s, digest=%s, size=%d", predicateType, truncateDigest(layer.Digest.String()), layer.Size)
		}

		// Check for VEX predicate types (must come before provenance/attestation)
		if strings.Contains(predicateLower, "vex") ||
			strings.Contains(predicateLower, "openvex") {
			annotations := make(map[string]string)
			for k, v := range indexAnnotations {
				annotations[k] = v
			}
			annotations["in-toto.io/predicate-type"] = predicateType

			referrers = append(referrers, Referrer{
				Type:         "vex",
				MediaType:    string(layer.MediaType),
				Digest:       layer.Digest.String(),
				Size:         layer.Size,
				ArtifactType: predicateType,
				Annotations:  annotations,
			})
			logVerbose("  Found VEX layer: predicate=%s, digest=%s, size=%d", predicateType, truncateDigest(layer.Digest.String()), layer.Size)
		}

		// Check for provenance/attestation predicate types
		if strings.Contains(predicateLower, "provenance") ||
			strings.Contains(predicateLower, "slsa") {
			// Create a referrer for this attestation/provenance layer
			// Use the layer digest (not the manifest digest) to ensure uniqueness
			annotations := make(map[string]string)
			for k, v := range indexAnnotations {
				annotations[k] = v
			}
			annotations["in-toto.io/predicate-type"] = predicateType

			referrers = append(referrers, Referrer{
				Type:         "attestation",
				MediaType:    string(layer.MediaType),
				Digest:       layer.Digest.String(), // Use layer digest for uniqueness
				Size:         layer.Size,
				ArtifactType: predicateType,
				Annotations:  annotations,
			})
			logVerbose("  Found provenance attestation layer: predicate=%s, digest=%s, size=%d", predicateType, truncateDigest(layer.Digest.String()), layer.Size)
		}
	}

	// If no specific types found but this is an attestation manifest, add it as generic attestation
	if len(referrers) == 0 {
		logVerbose("No specific SBOM/provenance found in attestation, adding as generic attestation")
		referrers = append(referrers, Referrer{
			Type:         "attestation",
			MediaType:    string(manifest.MediaType),
			Digest:       digest,
			Size:         size,
			ArtifactType: "attestation",
			Annotations:  indexAnnotations,
		})
	}

	return referrers, nil
}

// Sigstore OIDC issuer OIDs
var (
	oidSigstoreIssuerV1 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 1}
	oidSigstoreIssuerV2 = asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 57264, 1, 8}
)

// extractSignatureInfo extracts cosign signature verification details from
// the certificate embedded in the signature manifest annotation.
func (c *Client) extractSignatureInfo(ref name.Reference, digest string) (*SignatureInfo, error) {
	repo := ref.Context()
	manifestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", repo.String(), digest))
	if err != nil {
		return nil, fmt.Errorf("invalid manifest reference: %w", err)
	}

	desc, err := remote.Get(manifestRef, remote.WithAuthFromKeychain(c.keychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch signature manifest: %w", err)
	}

	// Parse manifest to get layers/annotations
	var manifestData struct {
		Layers []struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"layers"`
	}
	if err := json.Unmarshal(desc.Manifest, &manifestData); err != nil {
		return nil, fmt.Errorf("failed to parse signature manifest: %w", err)
	}

	// Look for the cosign certificate annotation on layers
	var certPEM string
	for _, layer := range manifestData.Layers {
		if cert, ok := layer.Annotations["dev.sigstore.cosign/certificate"]; ok {
			certPEM = cert
			break
		}
	}

	if certPEM == "" {
		logVerbose("No cosign certificate found in signature manifest %s", truncateDigest(digest))
		return nil, nil
	}

	// Parse PEM certificate
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	info := &SignatureInfo{}

	// Extract identity from SAN (email or URI)
	if len(cert.EmailAddresses) > 0 {
		info.Identity = cert.EmailAddresses[0]
	} else if len(cert.URIs) > 0 {
		info.Identity = cert.URIs[0].String()
	}

	// Extract OIDC issuer from Sigstore extension
	for _, ext := range cert.Extensions {
		if ext.Id.Equal(oidSigstoreIssuerV1) || ext.Id.Equal(oidSigstoreIssuerV2) {
			// The value is an ASN.1 UTF8String
			var issuer string
			if _, err := asn1.Unmarshal(ext.Value, &issuer); err != nil {
				// Try as raw string
				info.Issuer = string(ext.Value)
			} else {
				info.Issuer = issuer
			}
			break
		}
	}

	if info.Identity == "" && info.Issuer == "" {
		return nil, nil
	}

	logVerbose("Extracted signature info: identity=%s, issuer=%s", info.Identity, info.Issuer)
	return info, nil
}

func classifyArtifactType(artifactType string, annotations map[string]string) string {
	artifactTypeLower := strings.ToLower(artifactType)

	if strings.Contains(artifactTypeLower, "signature") || strings.Contains(artifactTypeLower, "notary") || strings.Contains(artifactTypeLower, "cosign") {
		logVerbose("Classified artifact as 'signature' based on artifactType: %s", artifactType)
		return "signature"
	}
	if strings.Contains(artifactTypeLower, "sbom") || strings.Contains(artifactTypeLower, "cyclonedx") || strings.Contains(artifactTypeLower, "spdx") {
		logVerbose("Classified artifact as 'sbom' based on artifactType: %s", artifactType)
		return "sbom"
	}
	// VEX check must come before attestation to avoid classifying VEX as generic attestation
	if strings.Contains(artifactTypeLower, "vex") || strings.Contains(artifactTypeLower, "openvex") {
		logVerbose("Classified artifact as 'vex' based on artifactType: %s", artifactType)
		return "vex"
	}
	if strings.Contains(artifactTypeLower, "vuln") || strings.Contains(artifactTypeLower, "scan") {
		logVerbose("Classified artifact as 'vulnerability-scan' based on artifactType: %s", artifactType)
		return "vulnerability-scan"
	}

	// Check annotations for predicate type before generic envelope type checks.
	// In-toto/DSSE envelope types need annotation inspection to determine the specific type.
	// Also check Sigstore bundle predicateType annotation.
	predType := annotations["in-toto.io/predicate-type"]
	if predType == "" {
		predType = annotations["dev.sigstore.bundle.predicateType"]
	}
	if predType == "" {
		predType = annotations["predicateType"]
	}
	if predType != "" {
		logVerbose("Checking predicate type annotation: %s", predType)
		predTypeLower := strings.ToLower(predType)
		if strings.Contains(predTypeLower, "vex") || strings.Contains(predTypeLower, "openvex") {
			logVerbose("Classified artifact as 'vex' based on predicate-type annotation")
			return "vex"
		}
		if strings.Contains(predTypeLower, "sbom") || strings.Contains(predTypeLower, "cyclonedx") || strings.Contains(predTypeLower, "spdx") {
			logVerbose("Classified artifact as 'sbom' based on predicate-type annotation")
			return "sbom"
		}
		if strings.Contains(predTypeLower, "provenance") || strings.Contains(predTypeLower, "slsa") {
			logVerbose("Classified artifact as 'attestation' based on predicate-type annotation")
			return "attestation"
		}
		if strings.Contains(predTypeLower, "vuln") {
			logVerbose("Classified artifact as 'vulnerability-scan' based on predicate-type annotation")
			return "vulnerability-scan"
		}
	}

	// Sigstore bundle artifact type — classify based on content annotation
	if strings.Contains(artifactTypeLower, "sigstore.bundle") {
		if content, ok := annotations["dev.sigstore.bundle.content"]; ok && strings.Contains(strings.ToLower(content), "message-signature") {
			logVerbose("Classified Sigstore bundle as 'signature' based on content annotation")
			return "signature"
		}
		logVerbose("Classified Sigstore bundle as 'attestation' based on artifactType: %s", artifactType)
		return "attestation"
	}

	if strings.Contains(artifactTypeLower, "attestation") || strings.Contains(artifactTypeLower, "in-toto") || strings.Contains(artifactTypeLower, "provenance") {
		logVerbose("Classified artifact as 'attestation' based on artifactType: %s", artifactType)
		return "attestation"
	}

	logVerbose("Could not classify artifact type '%s', defaulting to 'artifact'", artifactType)
	return "artifact"
}

// truncateDigest shortens a digest for logging purposes
func truncateDigest(digest string) string {
	parts := strings.Split(digest, ":")
	if len(parts) == 2 && len(parts[1]) > 12 {
		return parts[0] + ":" + parts[1][:12] + "..."
	}
	return digest
}
