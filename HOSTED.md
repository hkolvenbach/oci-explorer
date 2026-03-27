# Hosted Instance Setup

This document covers deploying OCI Explorer as a publicly hosted service on Fly.io, including the S3-compatible response cache that reduces registry bandwidth and avoids Docker Hub rate limits.

For local development, see [DEVELOPER.md](DEVELOPER.md).

## Architecture Overview

```
User -> Fly.io (fra) -> OCI Explorer
                            |
                            +-- ResolveDigest (1 HEAD request to registry)
                            |
                            +-- S3 cache lookup (Tigris, ~2-5ms)
                            |     HIT  -> return cached response
                            |     MISS -> fetch from registry / run Trivy -> store in S3
                            |
                            +-- Return JSON response
```

The cache keys responses by **SHA256 content digest**, not by tag. When a user requests `alpine:latest`, the server resolves the tag to its current digest via a single HEAD request, then checks the cache for that digest. If `latest` moves to a new image, the digest changes and the cache misses automatically.

## Prerequisites

- [Fly CLI](https://fly.io/docs/flyctl/install/) (`flyctl`)
- A Fly.io account

## Initial Deployment

### 1. Create the Fly app

```bash
fly apps create oci-explorer
```

### 2. Provision Tigris storage (S3-compatible cache)

```bash
fly storage create --name oci-explorer-cache
```

This automatically sets the following secrets on the app:

| Secret | Description |
|--------|-------------|
| `AWS_ACCESS_KEY_ID` | Tigris access key |
| `AWS_SECRET_ACCESS_KEY` | Tigris secret key |
| `AWS_ENDPOINT_URL_S3` | Tigris endpoint URL |
| `AWS_REGION` | Tigris region |
| `BUCKET_NAME` | Bucket name (informational) |

Verify with:

```bash
fly secrets list
```

### 3. Configure Docker Hub authentication (recommended)

Docker Hub limits anonymous pulls to 100 per 6 hours per IP. Since all users of the hosted instance share one outbound IP, this can be exhausted quickly.

Adding a Docker Hub access token raises the limit to 200 pulls/6h (free account) or unlimited (paid plan). Combined with the S3 cache, rate limits become a non-issue.

1. Create a read-only access token at https://hub.docker.com/settings/security
2. Set the credentials as Fly secrets:

```bash
fly secrets set DOCKER_HUB_USER=yourusername DOCKER_HUB_TOKEN=dckr_pat_...
```

The app detects these at startup and uses them for all Docker Hub registry operations. Other registries (GHCR, GCR, ECR) continue to use the default credential chain.

### 4. Deploy

```bash
fly deploy
```

The app uses a pre-built image from GHCR by default (see `fly.toml`). To build from source, change `[build]` to `dockerfile = 'Dockerfile'`.

### 5. Verify

```bash
# Health check (should show cacheEnabled: true)
curl -s https://oci-explorer.fly.dev/api/health | jq .

# First request (cache MISS)
curl -sD - 'https://oci-explorer.fly.dev/api/inspect?image=alpine:latest' -o /dev/null 2>&1 | grep X-Cache

# Second request (cache HIT)
curl -sD - 'https://oci-explorer.fly.dev/api/inspect?image=alpine:latest' -o /dev/null 2>&1 | grep X-Cache
```

## Configuration

All configuration is via environment variables in `fly.toml` or Fly secrets.

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `CACHE_S3_BUCKET` | No | _(empty = cache disabled)_ | S3 bucket name. Set this to enable caching. |
| `LOG_FORMAT` | No | `text` | Set to `json` for structured log output. |
| `PORT` | No | `8080` | HTTP server port. |
| `METRICS_PORT` | No | _(empty = serve on main port)_ | Serve Prometheus metrics on a separate port. Set on Fly.io to keep metrics off the public endpoint. |

### Secrets (set via `fly secrets set`)

| Secret | Set by | Description |
|--------|--------|-------------|
| `AWS_ACCESS_KEY_ID` | `fly storage create` | S3 credentials |
| `AWS_SECRET_ACCESS_KEY` | `fly storage create` | S3 credentials |
| `AWS_ENDPOINT_URL_S3` | `fly storage create` | S3 endpoint (Tigris) |
| `AWS_REGION` | `fly storage create` | S3 region |
| `DOCKER_HUB_USER` | `fly secrets set` | Docker Hub username (for rate limit increase) |
| `DOCKER_HUB_TOKEN` | `fly secrets set` | Docker Hub access token (read-only PAT) |

## Response Cache

### How It Works

The cache stores pre-serialized, gzip-compressed JSON responses in an S3-compatible object store. Each object's metadata includes an expiry timestamp; expired entries are treated as misses and overwritten.

A per-machine `singleflight.Group` prevents thundering herd: if 10 users request the same uncached image simultaneously, only one registry fetch runs.

### Cache TTLs

| Endpoint | S3 key prefix | TTL | Rationale |
|----------|--------------|-----|-----------|
| `/api/inspect` | `inspect/` | 30 days | Image manifests and configs are immutable for a given digest |
| `/api/scan` | `scan/` | 24 hours | Trivy vulnerability DB updates daily |
| `/api/sbom` | `sbom/` | 30 days | Content-addressed, truly immutable |
| `/api/vex` | `vex/` | 30 days | Content-addressed, truly immutable |

Not cached: `/api/tags`, `/api/matching-tags` (mutable tag lists), `/api/health`.

### Response Headers

| Header | Values | Description |
|--------|--------|-------------|
| `X-Cache` | `HIT` / `MISS` | Whether the response was served from cache |
| `X-Cached-At` | RFC3339 timestamp | When the cached response was originally computed (scan endpoint only) |

### Force Refresh

The scan endpoint accepts `?force=1` to bypass the cache and run a fresh Trivy scan:

```
/api/scan?image=alpine:latest&force=1
```

The UI exposes this as a "Rescan" button with a "Scanned X hours ago" indicator.

### Disabling the Cache

Remove `CACHE_S3_BUCKET` from `fly.toml` and redeploy. The app falls back to direct registry calls with no S3 dependency.

## Using AWS S3 Instead of Tigris

The cache uses the standard AWS SDK v2, so any S3-compatible backend works. To switch to AWS S3:

```bash
# Create an S3 bucket in the same region as Fly (Frankfurt)
aws s3 mb s3://oci-explorer-cache --region eu-central-1

# Create an IAM user with minimal permissions
# Policy: s3:GetObject + s3:PutObject on arn:aws:s3:::oci-explorer-cache/*

# Set credentials as Fly secrets
fly secrets set \
  AWS_ACCESS_KEY_ID=AKIA... \
  AWS_SECRET_ACCESS_KEY=... \
  AWS_REGION=eu-central-1

# Remove the Tigris endpoint so the SDK uses the default AWS endpoint
fly secrets unset AWS_ENDPOINT_URL_S3

# Optional: add lifecycle rules for automatic cleanup
aws s3api put-bucket-lifecycle-configuration \
  --bucket oci-explorer-cache \
  --lifecycle-configuration file://lifecycle.json
```

Example `lifecycle.json`:

```json
{
  "Rules": [
    {"ID": "expire-inspect", "Filter": {"Prefix": "inspect/"}, "Status": "Enabled", "Expiration": {"Days": 30}},
    {"ID": "expire-scan", "Filter": {"Prefix": "scan/"}, "Status": "Enabled", "Expiration": {"Days": 1}},
    {"ID": "expire-sbom", "Filter": {"Prefix": "sbom/"}, "Status": "Enabled", "Expiration": {"Days": 30}},
    {"ID": "expire-vex", "Filter": {"Prefix": "vex/"}, "Status": "Enabled", "Expiration": {"Days": 30}}
  ]
}
```

## Fly.io Machine Configuration

The current `fly.toml` configures:

- **Region**: `fra` (Frankfurt)
- **VM**: 512MB RAM, 1 CPU
- **Auto-scaling**: stops when idle (`min_machines_running = 0`), starts on incoming requests
- **Concurrency**: 100 hard limit, 50 soft limit per machine
- **Health check**: `GET /api/health` every 30s

### Trivy Cold Start

Trivy downloads its vulnerability database (~30MB) on the first scan after a machine starts. This adds 30-60s to the first uncached scan. The S3 cache mitigates this since most popular images are served from cache without invoking Trivy.

## Monitoring

### Health endpoint

The health endpoint includes request counts and cache hit/miss stats:

```bash
curl -s https://oci-explorer.fly.dev/api/health | jq .
```

Example response:

```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "cacheEnabled": true,
    "requests": {
      "inspect": 142,
      "scan": 23,
      "sbom": 5,
      "tags": 8
    },
    "cache": {
      "inspect": {"hits": 130, "misses": 12, "errors": 0, "hitRate": 0.915},
      "scan": {"hits": 19, "misses": 4, "errors": 0, "hitRate": 0.826}
    }
  }
}
```

### Prometheus metrics

Metrics are served on a separate port (`METRICS_PORT=9090`) to keep them off the public endpoint. Access via `fly proxy`:

```bash
# Forward the metrics port locally
fly proxy 9090:9090 &

# Query metrics
curl -s http://localhost:9090/metrics | grep oci_cache
```

Available metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `oci_cache_requests_total` | counter | `endpoint`, `result` | Cache requests (hit/miss/error per endpoint) |
| `oci_cache_latency_seconds` | histogram | `endpoint`, `operation` | Cache get and compute latency |
| `oci_cache_bytes_total` | counter | `endpoint` | Bytes stored (uncompressed) |

### Other commands

```bash
# Live logs
fly logs

# List cached objects (Tigris uses S3 API)
aws s3 ls s3://oci-explorer-cache/ --recursive \
  --endpoint-url "$AWS_ENDPOINT_URL_S3"
```

## Architecture Decision Records

- [ADR-001: Response Cache](docs/adrs/001-response-cache.md) -- why S3, alternatives considered, cache key design
- [ADR-002: Registry Auth / OIDC](docs/adrs/002-registry-auth-oidc.md) -- proposed feature for private repo access
