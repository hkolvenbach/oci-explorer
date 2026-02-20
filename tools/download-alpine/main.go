package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func main() {
	imageRef := "alpine:latest"
	outputDir := "registry/testdata/alpine"

	// Create output directory
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Fetching image index for %s...\n", imageRef)

	// Parse reference
	ref, err := name.ParseReference(imageRef)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse reference: %v\n", err)
		os.Exit(1)
	}

	// Fetch descriptor
	desc, err := remote.Get(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to fetch image: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Media type: %s\n", desc.MediaType)
	fmt.Printf("Digest: %s\n", desc.Digest.String())

	// Save the raw manifest (image index) - pretty print it
	var indexJSON interface{}
	if err := json.Unmarshal(desc.Manifest, &indexJSON); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse index JSON: %v\n", err)
		os.Exit(1)
	}

	indexFile := filepath.Join(outputDir, "image_index.json")
	prettyJSON, err := json.MarshalIndent(indexJSON, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal JSON: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(indexFile, prettyJSON, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write index file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Saved image index to %s\n", indexFile)

	// Parse the image index
	idx, err := desc.ImageIndex()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get image index: %v\n", err)
		os.Exit(1)
	}

	indexManifest, err := idx.IndexManifest()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get index manifest: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d manifests in index\n", len(indexManifest.Manifests))

	// Download each manifest
	for i, m := range indexManifest.Manifests {
		digest := m.Digest.String()
		fmt.Printf("\n[%d] Processing manifest: %s\n", i, digest[:20]+"...")

		// Create filename from digest
		filename := fmt.Sprintf("manifest_%s.json", digest[7:19]) // Use first 12 chars of hash
		
		// Determine subdirectory based on type
		var subdir string
		// Check if it's an attestation manifest first (even if it has unknown/unknown platform)
		if refType, ok := m.Annotations["vnd.docker.reference.type"]; ok && refType == "attestation-manifest" {
			refDigest := m.Annotations["vnd.docker.reference.digest"]
			if refDigest != "" {
				subdir = filepath.Join(outputDir, "attestations", refDigest[7:19])
			} else {
				subdir = filepath.Join(outputDir, "attestations", "unknown")
			}
		} else if m.Platform != nil {
			// Skip unknown/unknown platforms (these are attestation manifests)
			if m.Platform.OS == "unknown" && m.Platform.Architecture == "unknown" {
				subdir = filepath.Join(outputDir, "other")
			} else {
				platformStr := fmt.Sprintf("%s_%s", m.Platform.OS, m.Platform.Architecture)
				if m.Platform.Variant != "" {
					platformStr += "_" + m.Platform.Variant
				}
				subdir = filepath.Join(outputDir, "platforms", platformStr)
			}
		} else {
			subdir = filepath.Join(outputDir, "other")
		}

		if err := os.MkdirAll(subdir, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create subdirectory: %v\n", err)
			continue
		}

		manifestPath := filepath.Join(subdir, filename)

		// Fetch the manifest
		manifestRef, err := name.NewDigest(fmt.Sprintf("%s@%s", ref.Context().String(), digest))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create manifest reference: %v\n", err)
			continue
		}

		manifestDesc, err := remote.Get(manifestRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch manifest: %v\n", err)
			continue
		}

		// Pretty print the JSON
		var manifestJSON interface{}
		if err := json.Unmarshal(manifestDesc.Manifest, &manifestJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse manifest JSON: %v\n", err)
			continue
		}

		prettyJSON, err := json.MarshalIndent(manifestJSON, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to marshal manifest JSON: %v\n", err)
			continue
		}

		if err := os.WriteFile(manifestPath, prettyJSON, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write manifest file: %v\n", err)
			continue
		}

		fmt.Printf("  Saved to %s\n", manifestPath)

		// If it's a platform manifest (not an attestation), download the config
		if m.Platform != nil && m.Platform.OS != "unknown" && m.Platform.Architecture != "unknown" {
			// Try to parse as image to get config
			img, err := manifestDesc.Image()
			if err == nil {
				configFile, err := img.ConfigFile()
				if err == nil {
					// Convert config to JSON
					configJSON, err := json.MarshalIndent(configFile, "", "  ")
					if err == nil {
						configPath := filepath.Join(subdir, "config.json")
						if err := os.WriteFile(configPath, configJSON, 0644); err == nil {
							fmt.Printf("  Saved config to %s\n", configPath)
						} else {
							fmt.Fprintf(os.Stderr, "  Failed to write config file: %v\n", err)
						}
					} else {
						fmt.Fprintf(os.Stderr, "  Failed to marshal config JSON: %v\n", err)
					}
				} else {
					fmt.Fprintf(os.Stderr, "  Failed to get config file: %v\n", err)
				}
			} else {
				fmt.Fprintf(os.Stderr, "  Failed to parse as image: %v\n", err)
			}
		}

		// If it's an attestation manifest, try to download SBOM layers
		if refType, ok := m.Annotations["vnd.docker.reference.type"]; ok && refType == "attestation-manifest" {
			fmt.Printf("  This is an attestation manifest\n")
			
			// Try to parse as image to get layers
			img, err := manifestDesc.Image()
			if err == nil {
				manifest, err := img.Manifest()
				if err == nil {
					for j, layer := range manifest.Layers {
						predicateType := layer.Annotations["in-toto.io/predicate-type"]
						if predicateType != "" {
							fmt.Printf("  Layer %d: predicate-type=%s\n", j, predicateType)
							
							// Download the layer
							layerRef, err := name.NewDigest(fmt.Sprintf("%s@%s", ref.Context().String(), layer.Digest.String()))
							if err == nil {
								layerBlob, err := remote.Layer(layerRef, remote.WithAuthFromKeychain(authn.DefaultKeychain))
								if err == nil {
									reader, err := layerBlob.Compressed()
									if err == nil {
										layerFile := filepath.Join(subdir, fmt.Sprintf("layer_%d_%s.json", j, layer.Digest.String()[7:19]))
										layerData, err := io.ReadAll(reader)
										reader.Close()
										if err == nil {
											// Try to pretty print if it's JSON
											var layerJSON interface{}
											if err := json.Unmarshal(layerData, &layerJSON); err == nil {
												prettyJSON, err := json.MarshalIndent(layerJSON, "", "  ")
												if err == nil {
													layerData = prettyJSON
												}
											}
											if err := os.WriteFile(layerFile, layerData, 0644); err == nil {
												fmt.Printf("    Saved layer to %s\n", layerFile)
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("\nDone! All manifests saved to %s/\n", outputDir)
}
