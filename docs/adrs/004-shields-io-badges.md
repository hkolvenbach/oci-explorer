# ADR-004: Embeddable Shields.io Supply Chain Score Badges

- **Status:** Accepted
- **Date:** 2026-03-28

## Context

OCI Explorer computes a supply chain security score (0-10, letter grade A+ through D) based on the presence of signatures, SBOMs, attestations, VEX documents, and minimal base image characteristics. This score was previously computed entirely in the frontend from the `/api/inspect` response.

Users want to embed this score in GitHub READMEs, project dashboards, and documentation sites -- anywhere a shields.io badge can appear.

## Decision Drivers

- Badge must render in GitHub READMEs (where only `<img>` tags work, no JavaScript)
- Should not depend on shields.io availability for the primary use case
- Must reuse the existing inspect + S3 cache infrastructure (no separate badge cache)
- Score computation must be single source of truth (not duplicated across Go and TypeScript)
- Colors are a presentation concern -- the backend should not dictate them

## Decision

### Two badge endpoints

| Endpoint | Format | Use case |
|----------|--------|----------|
| `GET /badge/score.svg?image={ref}` | Self-rendered SVG | Direct embedding, no external dependency |
| `GET /badge/score.json?image={ref}` | Shields.io endpoint JSON | Embedding via shields.io with style overrides |

Both return a valid badge on every request -- errors produce a gray badge, never a broken image or 500.

### Score computation moved to Go backend

New `score` package with `Compute(referrers, manifest, config) Result`. The frontend drops its local `computeSecurityScore()` and reads `data.score` from the API response. This eliminates the TypeScript/Go logic duplication.

`Result` includes the full criteria breakdown (`criteria` array + `minimalBaseDetails`) so the frontend can render the expandable detail panel without local computation.

### Colors stay in the presentation layer

`score.Result` returns grade and score but not colors. The badge package calls `score.GradeColor(grade)` for SVG rendering. The frontend has its own `gradeColor()` utility. This keeps the backend free of presentation concerns while allowing each consumer to map grades to colors in its own format.

### Shared inspect flow

A shared `inspectImage()` helper in `main.go` is used by both `handleInspect` and the badge handlers. Same cache key (`inspect/{digest}`), same TTL (30 days), same response format. A badge request for an image that was already inspected in the UI is instant.

## Architecture

```
GET /badge/score.svg?image=alpine:latest
    |
    +-- inspectImage()           <-- shared with /api/inspect
    |   +-- resolveDigest()
    |   +-- cacheStore.GetOrCompute("inspect/" + digest)
    |   +-- registry.InspectImage() on miss
    |
    +-- score.Compute()          <-- score/ package
    |
    +-- badge.RenderSVG()        <-- badge/ package
```

### Package structure

| Package | Responsibility |
|---------|----------------|
| `score/` | Pure computation: `Compute()` -> `Result` with grade, criteria, breakdown |
| `badge/` | Pure rendering: `RenderSVG()`, `RenderJSON()`, error variants |
| `main.go` | HTTP handlers, cache wiring, shared `inspectImage()` |

### Badge format

- Label: `supply chain score` with OCI Explorer favicon icon
- Value: letter grade (A+, A, B, C, D), color-coded by grade
- Style: shields.io flat (gray label, colored value, `rx="3"` corners, Verdana font)

## Alternatives Considered

### Badge-only cache

Store rendered SVG/JSON in a separate cache. Rejected -- the inspect data is already cached and score computation + SVG rendering takes microseconds. Adding a badge cache would mean two cache entries for the same image with different TTLs and invalidation semantics.

### Keep score computation in frontend

Have the badge endpoint duplicate the scoring logic. Rejected -- two independently maintained implementations (Go + TypeScript) with the same logic is a maintenance and divergence risk. The code review caught a color discrepancy (Go used `eab308` for grade B, TypeScript used `facc15`) that proves this point.

### Backend returns colors

Include `color` and `colorClass` in `score.Result`. Rejected -- colors are presentation concerns. The badge needs hex without `#`, the frontend needs hex with `#` plus Tailwind classes. Having the backend dictate colors couples it to rendering details.

## Consequences

- Any public container image can have a supply chain score badge in its README
- Frontend score display is now consistent with badges (single computation in Go)
- Score is available in the `/api/inspect` response for API consumers
- Old cached inspect responses (without score) will serve without score until the 30-day TTL expires
- Private registry images are not supported by badge endpoints (future work)
