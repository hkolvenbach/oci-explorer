# OCI Image Test Data

This directory contains test fixtures for OCI (Open Container Initiative) images, including multi-platform images and their associated artifacts (SBOMs, attestations, signatures).

## Multi-Platform Image Structure

A typical multi-platform OCI image has the following structure:

```
Image Index (application/vnd.oci.image.index.v1+json)
│
├── Platform Manifest 1 (linux/amd64)
│   ├── Config Blob (image configuration)
│   │   ├── Architecture: amd64
│   │   ├── OS: linux
│   │   ├── Runtime Config (Cmd, Env, WorkingDir, etc.)
│   │   ├── Build History (Dockerfile commands)
│   │   └── RootFS (layer diff_ids)
│   └── Layer Blobs (filesystem layers)
│
├── Platform Manifest 2 (linux/arm64)
│   ├── Config Blob
│   └── Layer Blobs
│
├── Platform Manifest N (linux/arm/v6, linux/386, etc.)
│   ├── Config Blob
│   └── Layer Blobs
│
└── Attestation Manifests (platform: unknown/unknown)
    ├── Attestation for Platform 1
    │   ├── Annotation: vnd.docker.reference.type = "attestation-manifest"
    │   ├── Annotation: vnd.docker.reference.digest = <platform-1-digest>
    │   └── Layers:
    │       ├── SBOM Layer (in-toto.io/predicate-type: https://spdx.dev/Document)
    │       └── Provenance Layer (in-toto.io/predicate-type: https://slsa.dev/provenance/v0.2)
    │
    ├── Attestation for Platform 2
    │   ├── Annotation: vnd.docker.reference.digest = <platform-2-digest>
    │   └── Layers: (SBOM + Provenance)
    │
    └── Attestation for Platform N
        ├── Annotation: vnd.docker.reference.digest = <platform-n-digest>
        └── Layers: (SBOM + Provenance)
```

## Referrers and Platform Association

### How Referrers Link to Platforms

Referrers (SBOMs, attestations, signatures) can be associated with images in two ways:

#### 1. **Attestation Manifests in Image Index** (Docker BuildKit style)

Attestation manifests are embedded directly in the image index with:
- `platform: {architecture: "unknown", os: "unknown"}`
- `annotations.vnd.docker.reference.type: "attestation-manifest"`
- `annotations.vnd.docker.reference.digest: "<platform-manifest-digest>"`

**Example from alpine:latest:**

```
Image Index
├── linux/amd64 manifest (sha256:1882fa4569e0...)
│   └── Referrers:
│       └── Attestation manifest (sha256:f9fd905ebc9c...)
│           ├── vnd.docker.reference.digest: sha256:1882fa4569e0...
│           └── Layers:
│               ├── SBOM (SPDX)
│               └── Provenance (SLSA v0.2)
│
├── linux/arm/v6 manifest (sha256:cd194307b351...)
│   └── Referrers:
│       └── Attestation manifest (sha256:12dea40f86f3...)
│           ├── vnd.docker.reference.digest: sha256:cd194307b351...
│           └── Layers:
│               └── Provenance (SLSA v0.2)
│
└── ... (6 more platforms, each with its own attestation)
```

#### 2. **OCI 1.1 Referrers API** (Standard OCI way)

Referrers are discovered via the Referrers API endpoint:
- `GET /v2/<name>/referrers/<digest>`
- Each referrer has annotations linking it to the subject digest

**Example:**

```
Platform Manifest (sha256:abc123...)
└── Referrers API (/v2/library/alpine/referrers/sha256:abc123...)
    ├── SBOM Referrer
    │   ├── artifactType: application/spdx+json
    │   └── annotations: (may include vnd.docker.reference.digest)
    ├── Signature Referrer
    │   ├── artifactType: application/vnd.dev.cosign.artifact.sig.v1+json
    │   └── annotations: (may include vnd.docker.reference.digest)
    └── Attestation Referrer
        ├── artifactType: application/vnd.in-toto+json
        └── annotations: (may include vnd.docker.reference.digest)
```

## Test Fixtures

### alpine:latest

Located in `alpine/` directory. A multi-platform image with 8 platforms, each with its own attestation manifest.

**Structure:**
```
alpine/
├── image_index.json                    # Main image index (16 manifests)
├── platforms/
│   ├── linux_amd64/
│   │   ├── manifest_1882fa4569e0.json  # Platform manifest
│   │   └── config.json                 # Image config
│   ├── linux_arm_v6/
│   │   ├── manifest_cd194307b351.json
│   │   └── config.json
│   ├── linux_arm_v7/
│   │   ├── manifest_d320aee52817.json
│   │   └── config.json
│   ├── linux_arm64_v8/
│   │   ├── manifest_410dabcd6f1d.json
│   │   └── config.json
│   ├── linux_386/
│   │   ├── manifest_16ff8a639f58.json
│   │   └── config.json
│   ├── linux_ppc64le/
│   │   ├── manifest_a78fa7e4e732.json
│   │   └── config.json
│   ├── linux_riscv64/
│   │   ├── manifest_6b283ad1125d.json
│   │   └── config.json
│   └── linux_s390x/
│       ├── manifest_e1fe68c8d560.json
│       └── config.json
└── attestations/
    ├── 1882fa4569e0/                   # Linked to linux/amd64
    │   ├── manifest_f9fd905ebc9c.json  # Attestation manifest
    │   ├── layer_0_5c1f58ba4e0d.json   # SBOM (SPDX)
    │   └── layer_1_644afed44dca.json   # Provenance (SLSA)
    ├── cd194307b351/                   # Linked to linux/arm/v6
    │   ├── manifest_12dea40f86f3.json
    │   └── layer_0_c29e4c1977b4.json   # Provenance only
    ├── d320aee52817/                   # Linked to linux/arm/v7
    │   ├── manifest_823ade00a6dc.json
    │   ├── layer_0_8c23028141b7.json   # SBOM
    │   └── layer_1_a576b4c646f1.json   # Provenance
    ├── 410dabcd6f1d/                   # Linked to linux/arm64/v8
    │   ├── manifest_b8b03df6fb6f.json
    │   ├── layer_0_0d38fd5b3194.json   # SBOM
    │   └── layer_1_4fbdf2544c91.json   # Provenance
    ├── 16ff8a639f58/                   # Linked to linux/386
    │   ├── manifest_10557b46ac59.json
    │   ├── layer_0_93d91cf859d4.json   # SBOM
    │   └── layer_1_ac10394e9a1e.json   # Provenance
    ├── a78fa7e4e732/                   # Linked to linux/ppc64le
    │   ├── manifest_bd761fcd6f08.json
    │   ├── layer_0_6825a608256a.json  # SBOM
    │   └── layer_1_b4b137ac9ee8.json  # Provenance
    ├── 6b283ad1125d/                   # Linked to linux/riscv64
    │   ├── manifest_89dcdffaf28d.json
    │   ├── layer_0_3ba01e8490eb.json   # SBOM
    │   └── layer_1_dbbd7c856a8c.json   # Provenance
    └── e1fe68c8d560/                   # Linked to linux/s390x
        ├── manifest_00dba905f263.json
        ├── layer_0_0ab41aa147da.json   # SBOM
        └── layer_1_85ca2f88d269.json   # Provenance
```

**Key Observations:**
- Each platform has exactly one attestation manifest
- Attestation manifests are linked via `vnd.docker.reference.digest` annotation
- Most attestations contain both SBOM and Provenance layers
- linux/arm/v6 attestation only has Provenance (no SBOM)

### kairos/ubuntu

Located in `kairos_*.json` files. Example of an image using the OCI 1.1 Referrers API.

**Structure:**
```
kairos_image_index.json
├── linux/amd64 manifest
└── attestation manifest (unknown/unknown)
    ├── vnd.docker.reference.type: attestation-manifest
    ├── vnd.docker.reference.digest: <platform-digest>
    └── Layers:
        ├── SBOM (SPDX)
        └── Provenance (SLSA v0.2)
```

## Referrer Types

### SBOM (Software Bill of Materials)
- **Artifact Types:**
  - `application/spdx+json` (SPDX format)
  - `application/vnd.cyclonedx+json` (CycloneDX format)
  - `application/vnd.example.sbom.v1` (Generic SBOM)
- **Predicate Types (in-toto):**
  - `https://spdx.dev/Document`
  - `https://cyclonedx.org/bom`
- **Purpose:** Lists all software components and dependencies

### Attestation/Provenance
- **Artifact Types:**
  - `application/vnd.in-toto+json`
  - `application/vnd.dsse.envelope.v1+json`
- **Predicate Types:**
  - `https://slsa.dev/provenance/v0.2`
  - `https://slsa.dev/provenance/v1`
- **Purpose:** Documents how the image was built (build provenance)

### Signatures
- **Artifact Types:**
  - `application/vnd.dev.cosign.artifact.sig.v1+json` (Cosign)
  - `application/vnd.cncf.notary.signature` (Notary)
- **Purpose:** Cryptographic signatures for image verification

### Vulnerability Scans
- **Artifact Types:**
  - `application/vnd.security.vuln.report+json`
- **Purpose:** Security vulnerability scan results

## Platform-Specific Referrer Filtering

When a referrer has the `vnd.docker.reference.digest` annotation, it is linked to a specific platform manifest. This allows:

1. **Filtering referrers by platform** - Show only referrers for the selected platform
2. **Platform-specific SBOMs** - Each platform may have different dependencies
3. **Platform-specific attestations** - Build provenance may differ per platform

**Example filtering logic:**
```javascript
// Show referrers for linux/amd64 platform
const platformDigest = "sha256:1882fa4569e0...";
const filteredReferrers = referrers.filter(r => 
  r.annotations?.['vnd.docker.reference.digest'] === platformDigest
);
```

## Testing with These Fixtures

These test fixtures can be used to verify:

1. **Image Index Parsing**
   - Multi-platform manifest parsing
   - Platform detection and filtering
   - Attestation manifest identification

2. **Referrer Association**
   - Platform-specific referrer linking
   - Attestation manifest extraction
   - SBOM and provenance layer parsing

3. **Config Extraction**
   - Platform-specific config retrieval
   - Build history parsing
   - Runtime configuration extraction

4. **UI Filtering**
   - Platform-based referrer filtering
   - Config display per platform
   - Referrer count per platform

## Downloading New Test Data

Use the download tool to fetch new test fixtures:

```bash
go run tools/download-alpine/main.go
```

This will download manifests, configs, and attestation layers for the specified image.
