# ADR-001: Digest-Based Response Cache with S3-Compatible Storage

- **Status:** Accepted
- **Date:** 2026-03-27

## Context

The publicly hosted OCI Explorer instance on Fly.io faces three scaling problems:

1. **Bandwidth** -- every image inspection makes 10+ HTTP calls to container registries (manifest, platform configs, referrers, cosign signatures). Every vulnerability scan spawns a Trivy subprocess that pulls image layers.
2. **Rate limits** -- Docker Hub allows 100 anonymous pulls per 6 hours per IP address. A shared public server burns through this quickly since all users share one outbound IP.
3. **Redundancy** -- most users explore the same popular images (`alpine`, `nginx`, `node`, `golang`, etc.), repeating identical expensive operations.

## Decision Drivers

- Must scale horizontally (not tied to a single machine)
- Compatible with Fly.io deployment (`auto_stop_machines`, `min_machines_running = 0`)
- Low cost and operational simplicity
- Correct cache invalidation (tags like `latest` must not serve stale data)
- No vendor lock-in

## Decision

Cache API responses in an **S3-compatible object store** (Tigris as default, swappable to AWS S3 / Cloudflare R2 / MinIO via endpoint config), keyed by **SHA256 content digest**.

### Cache Key Strategy

Tags are mutable (`latest` can point to different digests over time). To ensure correctness:

1. Every request first resolves the image reference to its SHA256 digest via a lightweight HEAD request (single HTTP call).
2. The digest is used as the cache key (`inspect/sha256:abc123...`).
3. If a tag moves to a new digest, the resolution returns a different key -- automatic cache miss.
4. Digest-based references (`image@sha256:...`) skip resolution entirely.

### What Gets Cached

| Endpoint | Key prefix | TTL | Rationale |
|----------|-----------|-----|-----------|
| `/api/inspect` | `inspect/` | 30 days | Immutable for a given digest (same manifest, config, layers) |
| `/api/scan` | `scan/` | 24 hours | Trivy vulnerability DB updates daily; new CVEs may apply |
| `/api/sbom` | `sbom/` | 30 days | Content-addressed, truly immutable |
| `/api/vex` | `vex/` | 30 days | Content-addressed, truly immutable |

Not cached: `/api/tags`, `/api/matching-tags` (mutable tag lists), `/api/health`.

Errors are never cached -- failed requests retry on the next attempt.

### Thundering Herd Protection

A per-machine `singleflight.Group` ensures that concurrent requests for the same uncached digest only trigger one computation. Other goroutines wait and receive the same result.

### Compression

All cached values are gzip-compressed at `gzip.BestSpeed` (~3x size reduction with negligible CPU overhead). Stored with `Content-Encoding: gzip` and `x-amz-meta-expires-at` metadata for TTL checking.

### Scan Staleness UX

Inspect, SBOM, and VEX results are immutable per digest -- caching is fully transparent to the user.

Scan results may become stale as new CVEs are published. The UI shows "Scanned X hours ago" with a "Rescan" button that bypasses the cache (`?force=1`). This balances performance with user control over freshness.

### Trivy DB Lifecycle

- Trivy downloads its vulnerability database (~30MB) on the first scan after machine start. This adds 30-60s to the first uncached scan on a cold start.
- The DB is stored in the container's ephemeral filesystem. It is lost on every machine stop/restart.
- The S3 scan cache mitigates this: most popular images are served from cache without invoking Trivy.
- Trivy auto-checks for DB updates every 12 hours while the machine is running.
- Future improvement: pre-populate the Trivy DB at Docker build time to eliminate the cold-start penalty.

## Considered Alternatives

### Tigris (chosen)

Fly-native S3-compatible object storage. Provisioned via `fly storage create`, credentials auto-injected as Fly secrets. Globally distributed with automatic edge caching. 5GB free tier, ~2-5ms latency from Fly machines. Uses standard S3 API, so switching to any other S3-compatible provider is a configuration change (endpoint URL + credentials).

### AWS S3

Proven, cheap (~$0.023/GB/mo), same Frankfurt data center as Fly `fra` region (~5-20ms latency). Requires a separate AWS account and IAM configuration. Same implementation via S3 API -- the only difference is the endpoint URL and credential source.

### bbolt on Fly Volume

Embedded pure-Go key-value store on a Fly persistent volume. Zero external dependencies, crash-safe, very fast local reads (~1ms). **Rejected**: volumes are pinned to a single machine and region. Does not scale horizontally -- each machine would have its own cold cache. Adding a second machine means duplicated storage and no cache sharing.

### DynamoDB

Low-latency key-value store with built-in TTL. **Rejected**: 400KB item size limit. Scan results for large images (e.g., `golang:1.21` with 4000+ CVEs) regularly exceed this compressed. Would require S3 as overflow storage, creating a two-tier architecture with unnecessary complexity.

### Redis / Upstash

Fast, TTL built-in, Fly has native Upstash integration. **Rejected**: free tier is only 256MB (10K commands/day), insufficient for scan result storage. Ongoing cost for a cache that S3 handles essentially for free.

### In-memory LRU

Simplest possible cache. **Rejected**: the Fly deployment uses `auto_stop_machines = 'stop'` with `min_machines_running = 0`. The machine stops when idle, destroying all in-memory state. Every restart would be a complete cold cache.

### Neon / Postgres

Queryable, robust, serverless Postgres. **Rejected**: storing large compressed binary blobs (scan results up to 1MB+) in Postgres is awkward. The workload is pure key-value with no relational queries -- Postgres adds complexity without benefit.

## Consequences

- Adds `aws-sdk-go-v2` dependency (config + S3 client modules)
- Requires S3-compatible bucket provisioning (Tigris via `fly storage create` or AWS S3 manually)
- Introduces ~2-20ms latency per cache check (vs seconds or minutes saved on cache hit)
- Small frontend change for scan staleness indicator
- Feature is off by default (`CACHE_S3_BUCKET` env var); no impact on local development
