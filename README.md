# OCI Image Explorer

Visualize OCI container image structures including layers, manifests, referrers, SBOMs, attestations, and other supply chain artifacts. Built with Go and Svelte.

![OCI Image Explorer](https://img.shields.io/badge/OCI-1.1-blue) ![Go](https://img.shields.io/badge/Go-1.26+-00ADD8) ![Svelte](https://img.shields.io/badge/Svelte-5-FF3E00) ![Trivy](https://img.shields.io/badge/Trivy-0.69+-1904DA) ![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg) ![supply chain score](https://ociexplorer.dev/badge/score.svg?image=ghcr.io/hkolvenbach/oci-explorer:latest)

---

<h3><img src="web/public/favicon.svg" width="22" height="22" align="absmiddle" alt="logo" />&nbsp; Try it live at <a href="https://ociexplorer.dev">ociexplorer.dev</a></h3>

<sub>You can also [run it locally](#quick-start) for private registries and vulnerability scanning.</sub>

---

### Add a supply chain score badge to your project

Show the supply chain security score of any public container image in your README:

```markdown
![supply chain score](https://ociexplorer.dev/badge/score.svg?image=YOUR_IMAGE:TAG)
```

Or use the [shields.io endpoint](https://shields.io/badges/endpoint-badge) for style customization:

```markdown
![supply chain score](https://img.shields.io/endpoint?url=https://ociexplorer.dev/badge/score.json?image=YOUR_IMAGE:TAG)
```

Replace `YOUR_IMAGE:TAG` with any public image reference (e.g., `ghcr.io/org/repo:latest`, `alpine:3.21`).

---

## Features

- **Multi-platform Image Index** - Visualize fat manifests with all architecture variants (linux/amd64, linux/arm64, etc.). Filter the entire UI by platform to see platform-specific layers, config, and referrers. The image summary shows total size, layer count, and platform count at a glance.
- **Layer Inspection** - View every layer's digest, compressed size, media type, and annotations. Layers are listed in stack order matching the image filesystem.
- **Configuration Details** - Full runtime configuration: architecture, OS, entrypoint, cmd, env vars, exposed ports, working directory, user, and labels. Build history traces each Dockerfile instruction that created a layer, including empty layers from `ENV` and `LABEL` commands.
- **Referrers (OCI 1.1)** - Discover and inspect supply chain artifacts attached via the OCI Referrers API:
  - **Signatures** (Notary, Cosign) with Sigstore certificate identity, OIDC issuer, and signature digest
  - **SBOMs** (CycloneDX, SPDX) with one-click download of the full SBOM document
  - **Attestations** (SLSA Provenance, In-Toto) with inline viewing of attestation payloads
  - **VEX** (OpenVEX) with parsed statements showing vulnerability status, justifications, and affected products
  - **Cosign Tag Discovery** — also finds `.sig` and `.att` cosign-style tags alongside the Referrers API
- **Vulnerability Scanning ([Trivy](https://trivy.dev))** - On-demand CVE scanning with rich detail:
  - CVSS scores from multiple sources (NVD, Red Hat, etc.) displayed per vulnerability and in expanded detail view
  - Severity-grouped collapsible sections (CRITICAL, HIGH, MEDIUM, LOW, UNKNOWN) with per-group counts
  - Two-level filtering: global status filter (header chips) across all groups, plus per-section filter overrides
  - Fixable / no-fix / VEXed status chips with counts at both the scan header and each severity group
  - Expandable CVE details with package metadata, installed and fixed versions, and full description with preserved formatting
  - Reference links to NVD, Red Hat, Debian, Ubuntu, GitHub Advisories, Aqua, and other vulnerability databases
  - Automatic VEX cross-referencing: if the image has OpenVEX referrers, scan results are annotated with VEX status (not affected, fixed, under investigation)
  - Deduplication of identical CVEs across multiple targets (e.g., Go stdlib vulnerabilities found in many binaries are merged into a single entry)
- **Matching Tags** - Discover which tags in a repository point to the same digest. For Docker Hub and GCR/Artifact Registry, shows all aliases (e.g., `alpine:latest` → also `3.23.3`, `3.23`, `3`) with the current tag highlighted. Unsupported registries show an explanatory note.
- **Tag Listing** - Browse all tags for a repository with clickable navigation to inspect any tag.
- **Supply Chain Security Score** - At-a-glance 0–10 score with animated ring and letter grade. Evaluates supply chain artifact presence: signatures, SBOMs, attestations, VEX documents, minimal base image characteristics (few layers, small size, non-root user, no shell entrypoint). Expandable detail panel shows each criterion with pass/fail status.
- **Embeddable Badges** - Shields.io-compatible badge endpoints for embedding supply chain scores in GitHub READMEs and websites. Self-rendered SVG (`/badge/score.svg`) and shields.io endpoint JSON (`/badge/score.json`) with the OCI Explorer icon.
- **Graph View** - Interactive directed graph with pan, zoom, and fit-to-view controls. Shows the full image structure: image index, platform manifests, configs, layers, and all referrer artifacts (SBOMs, VEX, attestations, signatures) with color-coded nodes and relationship edges.
- **Copyable Digests** - Click any SHA-256 digest in the UI to copy the full value to the clipboard.
- **Mobile Responsive** - Adaptive layout with stacked columns on small screens and side-by-side panels on desktop.
- **Authentication** - Uses Docker credential helpers (`~/.docker/config.json`) for private registries. Supports Docker Hub, GHCR, GCR, ECR, and any registry with a configured credential helper.
- **Response Cache** - Optional S3-compatible cache (Tigris, AWS S3, MinIO) for hosted deployments. Caches responses by SHA256 digest to reduce registry bandwidth and avoid Docker Hub rate limits. Previously scanned images load instantly. See [HOSTED.md](HOSTED.md) for setup.

## Quick Start

### Prerequisites

- Go 1.25 or later
- Node.js 22+ (for building the Svelte frontend)
- Make (optional, for build automation)
- [Trivy](https://trivy.dev) v0.69+ (optional, for vulnerability scanning)

### Build & Run

```bash
# Clone or download this project
git clone https://github.com/hkolvenbach/oci-explorer.git
cd oci-explorer

# Download dependencies
go mod tidy

# Build and run (Docker — includes Trivy)
make run

# Or manually (requires frontend build first)
cd web && npm ci && npm run build && cd ..
go build -o build/oci-explorer .
./build/oci-explorer
```

The application starts a web server at http://localhost:8080

## Screenshots

### CLI Startup

```
┌─────────────────────────────────────────────────┐
│           🐳 OCI Image Explorer                 │
├─────────────────────────────────────────────────┤
│  URL:      http://localhost:8080                │
│  Platform: darwin/arm64                         │
│  Version:  1.0.0                                │
│  Press Ctrl+C to stop                           │
└─────────────────────────────────────────────────┘
```

### Landing Page

![Landing page](docs/screenshots/welcome.png)

### Details View

Inspecting `ghcr.io/hkolvenbach/oci-explorer:latest` — shows supply chain security score, platforms, layers, configuration, and referrers at a glance:

![Details view](docs/screenshots/details.png)

### Referrers View

Supply chain artifacts discovered via the OCI Referrers API: SBOMs (CycloneDX), cosign signatures with Sigstore identity, attestations (SLSA Provenance), and VEX documents:

![Referrers view](docs/screenshots/referrers.png)

### Vulnerability Scan

On-demand Trivy scan of `golang:1.21` — 4,088 deduplicated vulnerabilities across 5 severity levels. Header chips show fixable/no-fix totals; each severity group has its own filter overrides. Expanding a CVE reveals CVSS scores by source (NVD, Red Hat, etc.), package metadata, description, and reference links to vulnerability databases. If the image carries OpenVEX referrers, affected CVEs are automatically annotated with VEX status:

![Vulnerability scan](docs/screenshots/scan.png)

### Graph View

Interactive graph visualization of the full image structure with SBOMs, VEX, attestations, and signatures:

![Graph view](docs/screenshots/graph.png)

### Embeddable Badges

Supply chain score badges for embedding in READMEs and websites:

![supply chain score](docs/screenshots/badge-score-a.svg) ![supply chain score error](docs/screenshots/badge-score-error.svg)

```markdown
<!-- Self-rendered SVG (direct) -->
![supply chain score](https://ociexplorer.dev/badge/score.svg?image=ghcr.io/hkolvenbach/oci-explorer:latest)

<!-- Via shields.io (supports style overrides like ?style=for-the-badge) -->
![supply chain score](https://img.shields.io/endpoint?url=https://ociexplorer.dev/badge/score.json?image=ghcr.io/hkolvenbach/oci-explorer:latest)
```

### Command Line Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--port` | | `8080` | HTTP server port |
| `--verbose` | `-v` | `false` | Enable verbose logging |

```bash
# Run on a different port
./build/oci-explorer --port 3000

# Run with verbose logging
./build/oci-explorer -v
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server port |

## Building for Multiple Platforms

Build binaries for all supported platforms:

```bash
make build-all
```

This creates binaries in the `build/` directory for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)

Create release archives:

```bash
make release
```

## API Endpoints

### GET /api/inspect

Inspect an OCI image and return its full structure.

**Query Parameters:**
- `image` (required) - Image reference (e.g., `nginx:latest`, `ghcr.io/org/repo:tag`)

**Response:**
```json
{
  "success": true,
  "data": {
    "repository": "library/nginx",
    "tag": "latest",
    "digest": "sha256:abc123...",
    "imageIndex": { ... },
    "manifest": { ... },
    "config": { ... },
    "tags": ["latest", "1.25", "1.25.3"],
    "referrers": [ ... ],
    "score": { "score": 8, "maxScore": 10, "grade": "A", "color": "4ade80" }
  }
}
```

### GET /api/tags

List all tags for a repository.

**Query Parameters:**
- `repository` (required) - Repository reference (e.g., `nginx`, `ghcr.io/org/repo`)

### GET /api/matching-tags

Find all tags in a repository that resolve to the same digest as the given image.

**Query Parameters:**
- `image` (required) - Image reference (e.g., `alpine:latest`, `gcr.io/google-containers/pause:3.2`)

**Registry support:**
| Registry | Strategy |
|---|---|
| Docker Hub | Paginate Hub API, match digests client-side |
| GCR / Artifact Registry | Extended `tags/list` with manifest map (1 request) |
| Other (GHCR, Quay, ECR) | Returns empty list + explanatory note |

**Response:**
```json
{
  "success": true,
  "data": {
    "repository": "index.docker.io/library/alpine",
    "digest": "sha256:25109184c71b...",
    "tags": ["latest", "3.23.3", "3.23", "3"]
  }
}
```

### GET /api/sbom

Download SBOM content from an attestation manifest.

**Query Parameters:**
- `repository` (required) - Full repository name
- `digest` (required) - Digest of the attestation manifest containing the SBOM

**Response:** Raw SBOM content (SPDX or CycloneDX JSON) with `Content-Disposition: attachment`

### GET /api/vex

Fetch and parse a VEX (Vulnerability Exploitability eXchange) document from an attestation.

**Query Parameters:**
- `repository` (required) - Full repository name
- `digest` (required) - Digest of the attestation manifest containing the VEX document

**Response:** Parsed OpenVEX document with statements, status, justifications, and product identifiers.

### GET /api/scan

Scan a container image for vulnerabilities using Trivy (must be installed locally).

**Query Parameters:**
- `image` (required) - Image reference (e.g., `nginx:latest`)

**Response:** Vulnerabilities grouped by severity with CVE details, affected packages, and fix versions.

### GET /api/health

Health check endpoint.

### GET /badge/score.svg

Embeddable supply chain score badge as a self-rendered SVG. Returns a shields.io flat-style badge with the OCI Explorer icon and letter grade (A+ through D).

**Query Parameters:**
- `image` (required) - Image reference (e.g., `alpine:latest`, `ghcr.io/org/repo:tag`)

**Response:** `image/svg+xml` with `Cache-Control: public, max-age=86400`

**Embed in Markdown:**
```markdown
![supply chain score](https://ociexplorer.dev/badge/score.svg?image=ghcr.io/hkolvenbach/oci-explorer:latest)
```

### GET /badge/score.json

Supply chain score as a [shields.io endpoint badge](https://shields.io/badges/endpoint-badge) JSON response. Includes the OCI Explorer logo via `logoSvg`.

**Query Parameters:**
- `image` (required) - Image reference

**Response:**
```json
{
  "schemaVersion": 1,
  "label": "supply chain score",
  "message": "A",
  "color": "4ade80",
  "logoSvg": "<svg>...</svg>"
}
```

**Embed via shields.io (supports style overrides):**
```markdown
![supply chain score](https://img.shields.io/endpoint?url=https://ociexplorer.dev/badge/score.json?image=ghcr.io/hkolvenbach/oci-explorer:latest)
```

Both badge endpoints return a valid gray error badge (never a broken image) when the image parameter is missing or the image cannot be found.

## Usage Examples

### Inspect Public Images

```bash
# Docker Hub
curl "http://localhost:8080/api/inspect?image=nginx:latest"
curl "http://localhost:8080/api/inspect?image=alpine:3.19"

# GitHub Container Registry
curl "http://localhost:8080/api/inspect?image=ghcr.io/sigstore/cosign/cosign:latest"

# Google Container Registry
curl "http://localhost:8080/api/inspect?image=gcr.io/distroless/static:latest"
```

### Find Matching Tags

**Supported registry** (Docker Hub) — shows all tags sharing the same digest, with the queried tag marked "current":

![Matching tags — supported registry](docs/screenshots/matching-tags-supported.png)

**Unsupported registry** (GHCR) — shows a warning explaining the limitation:

![Matching tags — unsupported registry](docs/screenshots/matching-tags-unsupported.png)

```bash
# Docker Hub — discover that alpine:latest is also tagged 3.23.3, 3.23, 3
curl "http://localhost:8080/api/matching-tags?image=alpine:latest"

# GCR — single-request lookup via extended tags/list
curl "http://localhost:8080/api/matching-tags?image=gcr.io/google-containers/pause:3.2"

# GHCR — returns note (unsupported registry)
curl "http://localhost:8080/api/matching-tags?image=ghcr.io/hkolvenbach/oci-explorer:0.2.2"
```

### Inspect Private Images

The application uses Docker's credential helpers. Log in first:

```bash
# Docker Hub
docker login

# GitHub Container Registry
docker login ghcr.io

# AWS ECR
aws ecr get-login-password | docker login --username AWS --password-stdin <account>.dkr.ecr.<region>.amazonaws.com
```

Then inspect:
```bash
curl "http://localhost:8080/api/inspect?image=ghcr.io/myorg/private-image:v1"
```

## Supply Chain Security

All release artifacts are signed and attested for end-to-end supply chain verification.

| Artifact | Protection |
|----------|-----------|
| Docker image | Cosign keyless signature (Sigstore OIDC) |
| Docker image | SLSA Build Provenance attestation |
| Docker image | Embedded SBOM (BuildKit) |
| Docker image | OpenVEX attestation (govulncheck-based) |
| Release binaries | GitHub Artifact Attestation (SLSA provenance) |
| Runtime base | `gcr.io/distroless/static-debian12` (zero CVEs, no shell) |

### Verify image signature

```bash
cosign verify \
  --certificate-identity-regexp="https://github.com/hkolvenbach/oci-explorer" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/hkolvenbach/oci-explorer:latest
```

### Verify SLSA provenance

```bash
cosign verify-attestation \
  --type slsaprovenance1 \
  --certificate-identity-regexp="https://github.com/hkolvenbach/oci-explorer" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/hkolvenbach/oci-explorer:latest
```

### Verify OpenVEX attestation

```bash
cosign verify-attestation \
  --type openvex \
  --certificate-identity-regexp="https://github.com/hkolvenbach/oci-explorer" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
  ghcr.io/hkolvenbach/oci-explorer:latest
```

### Verify binary provenance (GitHub Artifact Attestation)

```bash
gh release download v1.0.0 --repo hkolvenbach/oci-explorer \
  --pattern 'oci-explorer-*-linux-amd64.tar.gz'
gh attestation verify oci-explorer-1.0.0-linux-amd64.tar.gz \
  --repo hkolvenbach/oci-explorer
```

### Inspect embedded SBOM

```bash
docker buildx imagetools inspect ghcr.io/hkolvenbach/oci-explorer:latest \
  --format '{{ json .SBOM }}'
```

### Explore with OCI Image Explorer

You can also use OCI Image Explorer itself to visually inspect all of these supply chain artifacts — signatures, SBOMs, attestations, and provenance — by pointing it at its own image:

```
http://localhost:8080/?q=ghcr.io/hkolvenbach/oci-explorer:latest
```

## Project Structure

```
oci-explorer/
├── .devcontainer/       # Dev container configuration
├── cache/               # S3-compatible response cache package
├── docs/
│   ├── adrs/            # Architecture decision records
│   ├── screenshots/     # Browser screenshots for README
│   ├── api.md           # API reference (served at /docs/)
│   └── openapi.yaml     # OpenAPI specification (served at /api/openapi.yaml)
├── badge/               # Shields.io badge rendering (SVG + JSON)
│   ├── badge.go         # RenderSVG, RenderJSON, error badge functions
│   └── badge_test.go
├── docshandler/         # Documentation HTTP handlers (extracted from main.go)
├── registry/            # OCI registry client using go-containerregistry
│   └── testdata/        # Test fixtures (Alpine, Kairos, VEX sample data)
├── score/               # Supply chain security score computation
│   ├── score.go         # Compute(referrers, manifest, config) → Result
│   └── score_test.go
├── scanner/             # Trivy vulnerability scanner (subprocess-based)
├── scripts/             # Test and verification scripts
├── tools/
│   ├── download-alpine/ # Alpine test data downloader
│   └── sbom-extractor/  # Reference SBOM extraction tool
├── web/                 # Svelte 5 + TypeScript frontend (Vite)
│   ├── src/
│   │   ├── components/  # 22 Svelte components
│   │   ├── lib/         # API client, types, state, utilities
│   │   ├── App.svelte   # Root component
│   │   └── main.ts      # Entry point
│   └── package.json
├── docker-compose.dev.yml # Development stack (app + Tigris cache)
├── Dockerfile           # Multi-stage container build (distroless)
├── fly.toml             # Fly.io deployment config
├── main.go              # HTTP server and handlers
├── Makefile             # Build automation
├── HOSTED.md            # Hosted deployment guide (S3 cache, Fly.io)
├── DEVELOPER.md         # Developer guide
├── REFERENCES.md        # OCI and SBOM specification references
└── LICENSE              # Apache 2.0
```

## Dependencies

### Backend
- [google/go-containerregistry](https://github.com/google/go-containerregistry) - OCI registry client
- [gorilla/mux](https://github.com/gorilla/mux) - HTTP router
- [Trivy](https://github.com/aquasecurity/trivy) - Vulnerability scanner (Apache 2.0 license, bundled in Docker image)

### Frontend
- [Svelte 5](https://svelte.dev) - Reactive UI framework
- [Tailwind CSS 4](https://tailwindcss.com) - Utility-first CSS
- [Vite 6](https://vite.dev) - Build tool

## OCI Specification Support

This tool supports OCI Image Spec 1.1 features including:

- **Image Index** (fat manifests) for multi-platform images
- **Image Manifest** with config and layer descriptors
- **Image Configuration** with runtime settings and build history
- **Referrers API** for attached artifacts (signatures, SBOMs, attestations)
- **Annotations** on all descriptor types

## License

Apache License 2.0
