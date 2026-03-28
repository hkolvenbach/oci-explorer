# Shields.io Supply Chain Score Badge

Embeddable badge endpoints that expose the OCI Explorer supply chain score as shields.io-compatible badges for GitHub READMEs and other websites.

## Overview

OCI Explorer computes a supply chain security score (0-10, letter grade A+ through D) based on the presence of signatures, SBOMs, attestations, VEX documents, and minimal base image characteristics. This feature exposes that score as embeddable badges via two endpoints: a self-rendered SVG and a shields.io-compatible JSON endpoint.

## Badge Format

- **Label:** `supply chain score` (with OCI Explorer favicon icon)
- **Value:** Letter grade (A+, A, B, C, D)
- **Color:** Grade-matched (green → yellow → orange → red)
- **Style:** Shields.io flat style

### Grade Thresholds

| Grade | Score | Color (hex) |
|-------|-------|-------------|
| A+ | 10 | 22c55e |
| A | >= 8 | 4ade80 |
| B | >= 6 | eab308 |
| C | >= 4 | fb923c |
| D | < 4 | f87171 |

### Scoring Criteria

**Supply chain artifacts (2 points each):**
- Signature (Cosign/Notary)
- Attestation (SLSA Provenance)
- SBOM (CycloneDX/SPDX)
- VEX (OpenVEX)

**Minimal base image (0.5 points each, max 2):**
- Few layers (<= 5)
- Small size (<= 50 MB)
- Non-root user
- No shell entrypoint

**Maximum score: 10**

## Architecture

### New Packages

```
score/
  score.go       — Compute(referrers, manifest, config) → Result
  score_test.go

badge/
  badge.go       — RenderSVG(Result) → []byte, RenderJSON(Result) → []byte
  badge_test.go
  favicon.svg    — Embedded icon for SVG badges
```

- `score` is pure computation: no HTTP, no registry, no cache.
- `badge` is pure rendering: takes a score result, returns bytes.
- `main.go` wires them together in HTTP handlers.

### Request Flow

```
GET /badge/score.svg?image=alpine:latest
    │
    ├─ resolveDigest(imageRef)
    │
    ├─ cacheStore.GetOrCompute(
    │     "inspect/" + digest,
    │     30-day TTL,
    │     func() { registry.InspectImage(imageRef) }
    │  )
    │
    ├─ json.Unmarshal → ImageInfo
    │
    ├─ score.Compute(imageInfo.Referrers, imageInfo.Manifest, imageInfo.Config)
    │
    └─ badge.RenderSVG(scoreResult)
        → Content-Type: image/svg+xml
        → Cache-Control: max-age=86400
```

The badge endpoint reuses the existing inspect + S3 cache infrastructure. Same cache key (`inspect/{digest}`), same 30-day TTL. A badge request shares cached data with the web UI — if an image was already inspected, the badge is instant.

No separate badge cache is needed. Score computation and SVG rendering are microseconds; only the registry inspect is expensive, and that's already cached.

## API Endpoints

### `GET /badge/score.svg?image={ref}`

Self-rendered SVG badge with embedded OCI Explorer icon.

**Response headers:**
- `Content-Type: image/svg+xml`
- `Cache-Control: max-age=86400`

**Embedding:**
```markdown
![supply chain score](https://ociexplorer.dev/badge/score.svg?image=alpine:latest)
```

### `GET /badge/score.json?image={ref}`

Shields.io endpoint badge JSON.

**Response:**
```json
{
  "schemaVersion": 1,
  "label": "supply chain score",
  "message": "A",
  "color": "4ade80",
  "logoSvg": "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'>...</svg>"
}
```

**Embedding via shields.io:**
```markdown
![supply chain score](https://img.shields.io/endpoint?url=https://ociexplorer.dev/badge/score.json?image=alpine:latest)
```

The shields.io path gives users free style overrides (e.g., `&style=for-the-badge`).

## Score Package

### Type

```go
package score

type Result struct {
    Score    float64 // 0-10
    MaxScore float64 // always 10
    Grade    string  // "A+", "A", "B", "C", "D"
    Color    string  // hex without # prefix
}
```

### Function

```go
func Compute(referrers []registry.Referrer, manifest *registry.Manifest, config *registry.ImageConfig) Result
```

Takes only the data it needs from ImageInfo. Same scoring logic as the current frontend `computeSecurityScore()` in `web/src/lib/utils.ts`, ported to Go.

## Badge Package

### SVG Rendering

SVG template as a Go string constant using `text/template`. The OCI Explorer favicon is embedded as a constant in the template. Shields.io flat style: gray label + colored value, `rx="3"` corners, Verdana font, shadow text for depth.

The icon is the only logo option — no customization parameter.

### JSON Rendering

Returns the shields.io endpoint badge schema with `logoSvg` containing the inline favicon SVG.

## API Response Change

`/api/inspect` response gains a `score` field computed server-side:

```json
{
  "success": true,
  "data": {
    "repository": "...",
    "score": {
      "score": 8,
      "maxScore": 10,
      "grade": "A",
      "color": "4ade80"
    },
    ...
  }
}
```

## Frontend Changes

- Remove `computeSecurityScore()` from `web/src/lib/utils.ts`
- Remove local score-related types that are replaced by the API response
- `SecurityScore.svelte` reads `data.score` from the API instead of computing locally
- Criteria breakdown (expandable details showing which artifacts are present) stays, reading from `data.referrers` directly

## Error Handling

All errors return a valid badge — never a 500 or broken image.

| Condition | SVG | JSON |
|-----------|-----|------|
| Missing/invalid `image` param | gray `supply chain score \| error` | `{"isError": true, "message": "error", "color": "gray"}` |
| Registry unreachable / image not found | gray `supply chain score \| not found` | `{"isError": true, "message": "not found", "color": "gray"}` |

## URL Structure

Badge endpoints live under `/badge/` (not `/api/badge/`) to separate badge rendering from the data API. This accommodates future badge types (e.g., `/badge/vulns.svg`).

## Scope Boundaries

**In scope:**
- `score` Go package (ported from frontend)
- `badge` Go package (SVG + JSON rendering)
- Two new HTTP handlers in `main.go`
- `score` field added to `/api/inspect` response
- Frontend simplified to consume server-computed score

**Out of scope (future):**
- Authentication for private registries via badge URL
- Vulnerability count badges (`/badge/vulns.svg`)
- Custom logo override on self-rendered SVG
- Badge style parameter on self-rendered SVG
