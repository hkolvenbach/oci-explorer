# Trivy DB S3 Caching

**Date:** 2026-03-30
**Status:** Approved

## Problem

OCI Explorer runs on Fly.io with `min_machines_running = 0`. Every cold start forces Trivy to download its vulnerability DB from upstream (~30-60s), delaying the first scan. The Java DB adds further delay on first Java image scan.

## Solution

Cache the Trivy vulnerability DB and Java DB in S3. On startup, restore from S3 (~2-5s). A background goroutine refreshes from upstream hourly and uploads the new DBs to S3. Scans never trigger their own DB downloads.

## S3 Layout

Two fixed keys in the existing `CACHE_S3_BUCKET`, overwritten on each refresh:

```
trivy-db/vuln-db.tar.gz    (~40MB, contains db/trivy.db + db/metadata.json)
trivy-db/java-db.tar.gz    (~10MB, contains java-db/trivy-java.db + java-db/metadata.json)
```

S3 PutObject is atomic for single-part uploads (<5GB). Combined with S3's strong read-after-write consistency, a GET during a PUT returns the previous complete version. No versioning or multi-key rotation needed.

## New Package: `trivydb`

### Manager

```go
type Manager struct {
    s3client  s3API       // S3 GetObject/PutObject (same interface as cache pkg)
    bucket    string
    cacheDir  string      // Trivy's --cache-dir
    trivyPath string      // resolved path to trivy binary
    cancel    context.CancelFunc
    wg        sync.WaitGroup
}

func New(ctx context.Context, bucket, cacheDir string) (*Manager, error)
func (m *Manager) Start(ctx context.Context) error
func (m *Manager) Stop()
func (m *Manager) CacheDir() string
func (m *Manager) Ready() bool
func (m *Manager) DBAge() time.Duration
```

### Startup Flow (`Start`)

1. Resolve Trivy binary path via `exec.LookPath`.
2. Try restore vuln DB from S3 key `trivy-db/vuln-db.tar.gz` -> extract to `<cacheDir>/db/`.
3. Try restore Java DB from S3 key `trivy-db/java-db.tar.gz` -> extract to `<cacheDir>/java-db/`.
4. If either S3 key is missing (first-ever deploy):
   - Run `trivy image --download-db-only --cache-dir <cacheDir>` for vuln DB.
   - Run `trivy image --download-java-db-only --cache-dir <cacheDir>` for Java DB.
   - Upload the newly downloaded DBs to S3.
5. Mark manager as ready.
6. Launch background goroutine with hourly ticker.

### Hourly Refresh (non-blocking, Approach A)

1. Create a temp directory.
2. Run `trivy image --download-db-only --cache-dir <tempDir>`.
3. Run `trivy image --download-java-db-only --cache-dir <tempDir>`.
4. Tar+gzip `<tempDir>/db/` and `<tempDir>/java-db/`.
5. Upload both tarballs to S3.
6. Swap: `os.Rename` the temp subdirectories over the live ones.
7. Remove temp directory.
8. On failure: log warning, keep existing DB, retry next tick.

### Tar/Extract

Implemented in Go using `archive/tar` + `compress/gzip` (distroless has no shell).

- `tarDir(srcDir string) ([]byte, error)` — walks a directory, produces tar.gz bytes.
- `extractTar(data []byte, destDir string) error` — extracts tar.gz into destination.

Both preserve file permissions but strip absolute paths for safety.

## Scanner Changes

In `scanner.go`, the `ScanImage` function accepts an optional cache dir:

```go
func ScanImage(ctx context.Context, imageRef string, opts ...ScanOption) (*ScanResult, error)
```

`ScanOption` is a functional option:

```go
type ScanOption func(*scanConfig)

type scanConfig struct {
    cacheDir       string
    skipDBUpdate   bool
    skipJavaUpdate bool
}

func WithCacheDir(dir string) ScanOption
func WithSkipDBUpdate() ScanOption
func WithSkipJavaDBUpdate() ScanOption
```

When a `trivydb.Manager` is active, `main.go` passes all three options. Without the manager, current behavior is preserved (Trivy manages its own DB downloads).

## Integration in `main.go`

After cache store initialization:

```go
var trivyDBManager *trivydb.Manager

if cacheStore != nil {
    cacheDir := filepath.Join(os.TempDir(), "trivy-cache")
    mgr, err := trivydb.New(ctx, bucket, cacheDir)
    if err != nil {
        slog.Warn("trivy db cache disabled", "error", err)
    } else {
        if err := mgr.Start(ctx); err != nil {
            slog.Warn("trivy db restore failed, will use upstream", "error", err)
        }
        trivyDBManager = mgr
        defer mgr.Stop()
    }
}
```

Scan handler passes options when manager is active:

```go
var opts []scanner.ScanOption
if trivyDBManager != nil && trivyDBManager.Ready() {
    opts = append(opts,
        scanner.WithCacheDir(trivyDBManager.CacheDir()),
        scanner.WithSkipDBUpdate(),
        scanner.WithSkipJavaDBUpdate(),
    )
}
scanner.ScanImage(ctx, imageRef, opts...)
```

## Health Endpoint

When the manager is active, the `/api/health` response includes:

```json
{
  "trivyDBCached": true,
  "trivyDBAge": "42m15s"
}
```

## Local Testing with MinIO

The existing docker-compose MinIO setup works unchanged. The `trivy-db/` prefix coexists with `scan/`, `inspect/`, etc. in the same bucket.

Manual verification:
```bash
# After first scan, check MinIO for DB files:
mc ls local/oci-cache/trivy-db/
# Should show vuln-db.tar.gz and java-db.tar.gz
```

## Graceful Degradation

| Scenario | Behavior |
|----------|----------|
| No `CACHE_S3_BUCKET` | Feature disabled, current behavior preserved |
| S3 restore fails on startup | Trivy downloads from upstream (logs warning) |
| S3 upload fails after refresh | Log warning, local DB still works, retry next hour |
| Hourly refresh fails (network) | Log warning, keep existing DB, retry next hour |
| Trivy binary missing | Manager init fails, feature disabled |

## Files Changed

| File | Change |
|------|--------|
| `trivydb/trivydb.go` | New — Manager, startup, refresh, tar/extract |
| `trivydb/trivydb_test.go` | New — unit tests with mock S3 |
| `scanner/scanner.go` | Add ScanOption, WithCacheDir, WithSkipDBUpdate, WithSkipJavaDBUpdate |
| `main.go` | Init Manager, pass scan options, health endpoint additions |
| `go.mod` / `go.sum` | No new dependencies (uses stdlib + existing AWS SDK) |
