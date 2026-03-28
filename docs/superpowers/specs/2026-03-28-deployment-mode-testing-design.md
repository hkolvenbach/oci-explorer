# Deployment Mode Testing

**Date:** 2026-03-28
**Status:** Approved

## Problem

The release pipeline builds four deployment artifacts (Go binaries, GitHub release tarballs, Docker image, docker-compose stack) but never functionally verifies that any of them start and serve HTTP. The only safety net before production is Fly.io's post-deploy health check. Additionally, the docker-compose stack is never tested in CI, and the multi-arch VEX smoke test only verifies one platform.

## Design

### 1. New `smoke-test` job in `release.yml`

**Position in workflow graph:**

```
test -> release-binaries -> release-docker -> smoke-test -> deploy-fly
```

`needs: [version, release-binaries, release-docker]`. Skipped when `inputs.dry-run == true` (same as deploy-fly). The deploy-fly job changes its `needs` from `[version, release-docker]` to `[version, smoke-test]`, gating production deployment on functional verification.

**Permissions:** `packages: read` (pull image from GHCR), `contents: read` (download release tarball).

#### 1a. Container startup test

- `docker run -d --name smoke-test -p 8080:8080 ghcr.io/${{ github.repository }}:${{ needs.version.outputs.version }}`
- Wait up to 30s for `/api/health` to return 200
- Verify response JSON contains `version` field matching the release version
- Verify response JSON contains `trivyAvailable` field
- `docker stop smoke-test && docker rm smoke-test`

#### 1b. Binary execution test

- Download the linux-amd64 tarball from the GitHub release using `gh release download`
- Extract the binary from the tarball
- Run the binary on a non-conflicting port (`PORT=8081`)
- Wait up to 15s for `/api/health` to return 200
- Verify response JSON contains `version` field matching the release version
- Kill the process

#### 1c. Both-arch VEX verification

- Extract ALL platform digests from the multi-arch manifest (filtering out `unknown` architecture)
- For each platform digest, run `cosign verify-attestation --type openvex` with the same certificate identity and OIDC issuer checks as the existing smoke test
- Fail if any platform is missing its VEX attestation

The existing single-platform VEX smoke test steps in `release-docker` are removed since this job supersedes them. Specifically, the following steps move from `release-docker` to `smoke-test`:
- "Install verification tools" (Trivy install)
- "Verify signatures and attestations (smoke test)" (cosign verify on index + single-platform VEX check)
- The `first-platform-digest.txt` file is no longer needed

The cosign index signature verification (step 1 of the old smoke test) is preserved in the new job. The VEX verification is upgraded to cover all platforms.

The `smoke-test` job needs the image digest from `release-docker`, so `release-docker` must expose it as a job output via `outputs.digest: ${{ steps.docker-push.outputs.digest }}`.

### 2. New `integration-test` job in `ci.yml`

Runs in parallel with `test`, `lint`, and `vulncheck` — no dependencies between them. Runs on every push to main and every PR.

Steps:
1. Checkout code
2. `docker compose -f docker-compose.dev.yml up --build -d`
3. Run `scripts/test-cache.sh` (already handles health check wait, cache MISS/HIT, Prometheus metrics, scan endpoint)
4. `docker compose -f docker-compose.dev.yml down -v`

The `test-cache.sh` script works as-is with no modifications. The default `BASE=http://localhost:8080` matches the docker-compose port mapping.

### 3. Explicit integration test visibility in CI

Add a new step in the `test` job of `ci.yml`, after the existing "Run tests" step:

```yaml
- name: Run integration tests (VEX + Trivy)
  run: go test -v -count=1 -run 'TestTrivyNativeVEX|TestAppSideVEXCrossReference|TestCompareApproaches' ./scanner/
```

This makes integration test execution visible as a distinct step in the workflow UI. The tests already run as part of `go test ./...`, but this step makes it explicit and easy to spot if they're skipped.

## Files changed

| File | Change |
|------|--------|
| `.github/workflows/release.yml` | Add `smoke-test` job; update `deploy-fly` needs; remove single-platform VEX smoke test from `release-docker` |
| `.github/workflows/ci.yml` | Add `integration-test` job; add explicit VEX integration test step |

## Files NOT changed

- `Dockerfile`, `docker-compose.dev.yml` — work as-is
- `Makefile` — no changes needed
- `scripts/test-cache.sh` — works as-is
- `scanner/vex_integration_test.go` — no changes to skip logic
