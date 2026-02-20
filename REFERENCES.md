# SBOM Encoding in OCI Images - Official Guidelines and Specifications

This document describes the official methods for encoding Software Bill of Materials (SBOMs) into OCI container images.

## Overview

There are two primary approaches for attaching SBOMs to OCI images:

1. **OCI 1.1 Referrers API** - The standardized OCI approach using artifact references
2. **Docker BuildKit Attestations** - Docker's implementation embedded in image indexes

## 1. OCI 1.1 Reference Types (Referrers API)

The OCI Distribution Specification 1.1 introduces native support for artifact references through the `subject` field and Referrers API.

### Manifest Structure

SBOMs are stored as OCI artifacts with a `subject` field pointing to the target image:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "artifactType": "application/spdx+json",
  "subject": {
    "mediaType": "application/vnd.oci.image.manifest.v1+json",
    "digest": "sha256:5e140a61e16155b30356685a6801e5250339bfb11370e70573d28d4ff2dc89cf",
    "size": 477
  },
  "layers": [
    {
      "mediaType": "application/spdx+json",
      "digest": "sha256:...",
      "size": 12345
    }
  ]
}
```

### Referrers API

Query artifacts referencing a specific digest:

```
GET /v2/<name>/referrers/<digest>
```

Optional filtering by artifact type:
```
GET /v2/<name>/referrers/<digest>?artifactType=application/spdx+json
```

### Common SBOM Artifact Types

| Format | Artifact Type |
|--------|---------------|
| SPDX JSON | `application/spdx+json` |
| CycloneDX JSON | `application/vnd.cyclonedx+json` |
| CycloneDX XML | `application/vnd.cyclonedx+xml` |

### References

- [OCI Image Spec - Image Index](https://github.com/opencontainers/image-spec/blob/main/image-index.md)
- [OCI Distribution Spec - Referrers API](https://github.com/opencontainers/distribution-spec/blob/main/spec.md#listing-referrers)
- [ORAS Attached Artifacts](https://oras.land/docs/concepts/reftypes/)

---

## 2. Docker BuildKit Attestations

Docker BuildKit embeds attestations (including SBOMs) directly in the image index using a vendor-specific annotation scheme.

### Storage Structure

Attestations are stored as manifest entries in the image index with special annotations:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.index.v1+json",
  "manifests": [
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "digest": "sha256:c93ad9a4ed48...",
      "size": 677,
      "platform": {
        "architecture": "amd64",
        "os": "linux"
      }
    },
    {
      "mediaType": "application/vnd.oci.image.manifest.v1+json",
      "digest": "sha256:77ffcabd8242...",
      "size": 841,
      "platform": {
        "architecture": "unknown",
        "os": "unknown"
      },
      "annotations": {
        "vnd.docker.reference.type": "attestation-manifest",
        "vnd.docker.reference.digest": "sha256:c93ad9a4ed48..."
      }
    }
  ]
}
```

### Key Annotations

| Annotation | Description |
|------------|-------------|
| `vnd.docker.reference.type` | Set to `attestation-manifest` to identify attestation manifests |
| `vnd.docker.reference.digest` | The digest of the image manifest this attestation refers to |

### Platform Marker

Attestation manifests use a special platform marker to prevent container runtimes from accidentally executing them:

```json
"platform": {
  "architecture": "unknown",
  "os": "unknown"
}
```

### Attestation Manifest Layers

Each layer in the attestation manifest contains an in-toto attestation blob:

```json
{
  "schemaVersion": 2,
  "mediaType": "application/vnd.oci.image.manifest.v1+json",
  "layers": [
    {
      "mediaType": "application/vnd.in-toto+json",
      "digest": "sha256:9d184400e09a...",
      "size": 1234567,
      "annotations": {
        "in-toto.io/predicate-type": "https://spdx.dev/Document"
      }
    },
    {
      "mediaType": "application/vnd.in-toto+json",
      "digest": "sha256:f7dff822a9b0...",
      "size": 54321,
      "annotations": {
        "in-toto.io/predicate-type": "https://slsa.dev/provenance/v0.2"
      }
    }
  ]
}
```

### In-Toto Predicate Types

| Predicate Type | Description |
|----------------|-------------|
| `https://spdx.dev/Document` | SPDX SBOM |
| `https://cyclonedx.org/bom` | CycloneDX SBOM |
| `https://slsa.dev/provenance/v0.2` | SLSA Provenance v0.2 |
| `https://slsa.dev/provenance/v1` | SLSA Provenance v1 |

### Creating BuildKit Attestations

Generate SBOM during build:
```bash
docker buildx build --sbom=true --push -t myimage:latest .
```

Generate provenance attestation:
```bash
docker buildx build --attest type=provenance,mode=max --push -t myimage:latest .
```

### References

- [Docker Build Attestations](https://docs.docker.com/build/metadata/attestations/)
- [Docker SBOM Attestations](https://docs.docker.com/build/metadata/attestations/sbom/)
- [Docker Attestation Storage](https://docs.docker.com/build/metadata/attestations/attestation-storage/)
- [Docker Provenance Attestations](https://docs.docker.com/build/metadata/attestations/slsa-provenance/)

---

## 3. In-Toto Attestation Framework

Both approaches use the in-toto attestation format for the actual SBOM content.

### Statement Structure

```json
{
  "_type": "https://in-toto.io/Statement/v0.1",
  "subject": [
    {
      "name": "pkg:oci/myimage@sha256:...",
      "digest": {
        "sha256": "..."
      }
    }
  ],
  "predicateType": "https://spdx.dev/Document",
  "predicate": {
    "SPDXID": "SPDXRef-DOCUMENT",
    "spdxVersion": "SPDX-2.3",
    "creationInfo": {
      "created": "2025-01-13T12:00:00Z",
      "creators": ["Tool: syft-v1.29.0"]
    },
    "packages": [...]
  }
}
```

### References

- [In-Toto Attestation Framework](https://github.com/in-toto/attestation)
- [SPDX Predicate](https://github.com/in-toto/attestation/blob/main/spec/predicates/spdx.md)
- [CycloneDX Predicate](https://github.com/in-toto/attestation/blob/main/spec/predicates/cyclonedx.md)

---

## 4. SBOM Formats

### SPDX (Software Package Data Exchange)

- Standard: ISO/IEC 5962:2021
- Media Type: `application/spdx+json`
- Website: https://spdx.dev/

### CycloneDX

- Standard: OWASP CycloneDX
- Media Type: `application/vnd.cyclonedx+json`
- Website: https://cyclonedx.org/

---

## 5. Tools

| Tool | Description |
|------|-------------|
| [Syft](https://github.com/anchore/syft) | SBOM generator supporting SPDX and CycloneDX |
| [ORAS](https://oras.land/) | OCI Registry As Storage - attach artifacts to images |
| [Cosign](https://github.com/sigstore/cosign) | Sign and attach SBOMs to container images |
| [BuildKit](https://github.com/moby/buildkit) | Docker's build engine with attestation support |

---

## 6. Discovery Algorithm

To discover SBOMs for an OCI image:

1. **Check OCI Referrers API**: Query `/v2/<name>/referrers/<digest>` for artifacts with SBOM artifact types
2. **Check Referrers Tag Schema**: Look for `<name>:sha256-<hash>` tag (fallback for older registries)
3. **Check Image Index for BuildKit Attestations**: Parse image index for manifests with `vnd.docker.reference.type: attestation-manifest` annotation
4. **Inspect Attestation Manifest Layers**: Check layer annotations for `in-toto.io/predicate-type` containing SBOM predicates
