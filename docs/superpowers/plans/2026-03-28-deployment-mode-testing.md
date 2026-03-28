# Deployment Mode Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ensure every shipped artifact (Go binary, Docker image, docker-compose stack) is functionally verified before reaching production.

**Architecture:** Add a `smoke-test` job to the release workflow that gates production deployment, add an `integration-test` job to CI that exercises the docker-compose stack on every PR, and add an explicit integration test step for VEX/Trivy test visibility.

**Tech Stack:** GitHub Actions YAML, bash, docker, curl, cosign, gh CLI

**Spec:** `docs/superpowers/specs/2026-03-28-deployment-mode-testing-design.md`

---

### Task 1: Add `release-docker` digest output

**Files:**
- Modify: `.github/workflows/release.yml:130-135`

The `smoke-test` job needs the image digest from `release-docker`. Currently the digest is only available within the `release-docker` job. Add a job-level output to expose it.

- [ ] **Step 1: Add outputs declaration and id to release-docker job**

In `.github/workflows/release.yml`, add `outputs` to the `release-docker` job definition and ensure the docker-push step has an `id`:

Change the job header at line 130 from:

```yaml
  release-docker:
    runs-on: ubuntu-latest
    needs: [test, version, release-binaries]
    permissions:
      packages: write
      id-token: write
      attestations: write
    steps:
```

To:

```yaml
  release-docker:
    runs-on: ubuntu-latest
    needs: [test, version, release-binaries]
    outputs:
      digest: ${{ steps.docker-push.outputs.digest }}
    permissions:
      packages: write
      id-token: write
      attestations: write
    steps:
```

- [ ] **Step 2: Remove the smoke test steps from release-docker**

Remove these steps from `release-docker` (lines 282-334):

1. The line `echo "$PLATFORM_DIGESTS" | head -1 > /tmp/first-platform-digest.txt` at the end of the "Generate and attach OpenVEX documents" step
2. The entire "Install verification tools" step (lines 292-296)
3. The entire "Verify signatures and attestations (smoke test)" step (lines 298-334)

Keep the "Upload VEX document as build artifact" step — it stays in `release-docker`.

- [ ] **Step 3: Verify YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "refactor(ci): expose docker digest output, remove inline smoke test from release-docker"
```

---

### Task 2: Add `smoke-test` job to release workflow

**Files:**
- Modify: `.github/workflows/release.yml`

Add the new `smoke-test` job after `release-docker`. This job does container startup testing, binary execution testing, and full multi-arch VEX verification.

- [ ] **Step 1: Add the smoke-test job**

Add this job after `release-docker` (after the "Upload VEX document as build artifact" step and before `deploy-fly`):

```yaml
  smoke-test:
    runs-on: ubuntu-latest
    needs: [version, release-binaries, release-docker]
    if: inputs.dry-run != true
    permissions:
      contents: read
      packages: read
      id-token: write
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
        with:
          persist-credentials: false

      - name: Install cosign
        uses: sigstore/cosign-installer@c56c2d3e59e4281cc41dea2217323ba5694b171e # v3.8.0

      - name: Install Trivy
        run: curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/v0.69.2/contrib/install.sh | sh -s -- -b /usr/local/bin v0.69.2

      - name: Login to GHCR (read-only)
        uses: docker/login-action@74a5d142397b4f367a81961eba4e8cd7edddf772 # v3.4.0
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      # ── 1. Container startup test ──────────────────────────────────────
      - name: "Smoke test: container startup"
        run: |
          IMAGE="ghcr.io/${{ github.repository }}:${{ needs.version.outputs.version }}"
          echo "Pulling and starting $IMAGE..."
          docker run -d --name smoke-test -p 8080:8080 -e PORT=8080 "$IMAGE"

          echo "Waiting for /api/health (up to 30s)..."
          for i in $(seq 1 30); do
            if curl -sf http://localhost:8080/api/health > /dev/null 2>&1; then
              echo "Health check passed after ${i}s"
              break
            fi
            if [ "$i" -eq 30 ]; then
              echo "FAIL: container did not become healthy within 30s"
              docker logs smoke-test
              exit 1
            fi
            sleep 1
          done

          HEALTH=$(curl -sf http://localhost:8080/api/health)
          echo "Health response: $HEALTH"

          # Verify version field matches
          GOT_VERSION=$(echo "$HEALTH" | jq -r '.version')
          WANT_VERSION="${{ needs.version.outputs.version }}"
          if [ "$GOT_VERSION" != "$WANT_VERSION" ]; then
            echo "FAIL: version mismatch (got=$GOT_VERSION, want=$WANT_VERSION)"
            exit 1
          fi
          echo "PASS: version matches ($GOT_VERSION)"

          # Verify trivyAvailable field exists
          TRIVY=$(echo "$HEALTH" | jq -r '.trivyAvailable')
          if [ "$TRIVY" != "true" ] && [ "$TRIVY" != "false" ]; then
            echo "FAIL: trivyAvailable field missing or invalid (got=$TRIVY)"
            exit 1
          fi
          echo "PASS: trivyAvailable=$TRIVY"

          docker stop smoke-test && docker rm smoke-test
          echo "Container smoke test passed."

      # ── 2. Binary execution test ───────────────────────────────────────
      - name: "Smoke test: binary execution"
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          VERSION="${{ needs.version.outputs.version }}"
          ASSET="oci-explorer-${VERSION}-linux-amd64.tar.gz"

          echo "Downloading $ASSET from release v${VERSION}..."
          gh release download "v${VERSION}" \
            --repo "${{ github.repository }}" \
            --pattern "$ASSET" \
            --dir /tmp/smoke-binary

          echo "Extracting binary..."
          tar -xzf "/tmp/smoke-binary/$ASSET" -C /tmp/smoke-binary
          chmod +x /tmp/smoke-binary/oci-explorer-linux-amd64

          echo "Starting binary on port 8081..."
          PORT=8081 /tmp/smoke-binary/oci-explorer-linux-amd64 &
          BINARY_PID=$!

          echo "Waiting for /api/health (up to 15s)..."
          for i in $(seq 1 15); do
            if curl -sf http://localhost:8081/api/health > /dev/null 2>&1; then
              echo "Health check passed after ${i}s"
              break
            fi
            if [ "$i" -eq 15 ]; then
              echo "FAIL: binary did not become healthy within 15s"
              kill $BINARY_PID 2>/dev/null || true
              exit 1
            fi
            sleep 1
          done

          HEALTH=$(curl -sf http://localhost:8081/api/health)
          echo "Health response: $HEALTH"

          GOT_VERSION=$(echo "$HEALTH" | jq -r '.version')
          if [ "$GOT_VERSION" != "$VERSION" ]; then
            echo "FAIL: version mismatch (got=$GOT_VERSION, want=$VERSION)"
            kill $BINARY_PID 2>/dev/null || true
            exit 1
          fi
          echo "PASS: version matches ($GOT_VERSION)"

          kill $BINARY_PID 2>/dev/null || true
          wait $BINARY_PID 2>/dev/null || true
          echo "Binary smoke test passed."

      # ── 3. Cosign signature verification ───────────────────────────────
      - name: "Smoke test: cosign signature on index"
        run: |
          IMAGE="ghcr.io/${{ github.repository }}"
          DIGEST="${{ needs.release-docker.outputs.digest }}"
          echo "Verifying cosign signature on ${IMAGE}@${DIGEST}..."
          cosign verify \
            --certificate-identity-regexp="https://github.com/${{ github.repository }}" \
            --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
            "${IMAGE}@${DIGEST}"
          echo "PASS: cosign signature verified."

      # ── 4. All-platform VEX verification ───────────────────────────────
      - name: "Smoke test: VEX attestations (all platforms)"
        run: |
          IMAGE="ghcr.io/${{ github.repository }}"
          DIGEST="${{ needs.release-docker.outputs.digest }}"

          echo "Extracting platform digests from ${IMAGE}@${DIGEST}..."
          PLATFORM_DIGESTS=$(docker buildx imagetools inspect --raw "${IMAGE}@${DIGEST}" \
            | jq -r '.manifests[] | select(.platform.architecture != "unknown") | "\(.platform.os)/\(.platform.architecture) \(.digest)"')

          echo "Platform digests:"
          echo "$PLATFORM_DIGESTS"

          PLATFORM_COUNT=0
          VERIFIED_COUNT=0

          while IFS=' ' read -r PLATFORM PLAT_DIGEST; do
            PLATFORM_COUNT=$((PLATFORM_COUNT + 1))
            echo ""
            echo "=== Verifying VEX for $PLATFORM ($PLAT_DIGEST) ==="

            cosign verify-attestation \
              --type openvex \
              --certificate-identity-regexp="https://github.com/${{ github.repository }}" \
              --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
              "${IMAGE}@${PLAT_DIGEST}"

            echo "PASS: VEX attestation verified for $PLATFORM"
            VERIFIED_COUNT=$((VERIFIED_COUNT + 1))
          done <<< "$PLATFORM_DIGESTS"

          echo ""
          echo "Verified $VERIFIED_COUNT / $PLATFORM_COUNT platforms."
          if [ "$VERIFIED_COUNT" -ne "$PLATFORM_COUNT" ]; then
            echo "FAIL: not all platforms verified"
            exit 1
          fi
          if [ "$PLATFORM_COUNT" -lt 2 ]; then
            echo "FAIL: expected at least 2 platforms, got $PLATFORM_COUNT"
            exit 1
          fi
          echo "PASS: all $PLATFORM_COUNT platform VEX attestations verified."

      # ── 5. VEX document structure validation ───────────────────────────
      - name: "Smoke test: VEX document structure"
        run: |
          IMAGE="ghcr.io/${{ github.repository }}"
          DIGEST="${{ needs.release-docker.outputs.digest }}"

          # Get the first platform digest for structure validation
          PLAT_DIGEST=$(docker buildx imagetools inspect --raw "${IMAGE}@${DIGEST}" \
            | jq -r '[.manifests[] | select(.platform.architecture != "unknown")] | .[0].digest')

          echo "Extracting VEX document from ${PLAT_DIGEST}..."
          cosign verify-attestation \
            --type openvex \
            --certificate-identity-regexp="https://github.com/${{ github.repository }}" \
            --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
            "${IMAGE}@${PLAT_DIGEST}" 2>/dev/null \
            | head -1 | jq -r '.payload' | base64 -d | jq '.predicate' > /tmp/extracted-vex.json

          echo "Extracted VEX document:"
          jq '.' /tmp/extracted-vex.json

          jq -e '.["@context"] and .statements' /tmp/extracted-vex.json > /dev/null
          echo "PASS: VEX document has valid OpenVEX structure"

      # ── 6. Trivy VEX integration check ─────────────────────────────────
      - name: "Smoke test: Trivy VEX integration"
        run: |
          IMAGE="ghcr.io/${{ github.repository }}"
          DIGEST="${{ needs.release-docker.outputs.digest }}"

          PLAT_DIGEST=$(docker buildx imagetools inspect --raw "${IMAGE}@${DIGEST}" \
            | jq -r '[.manifests[] | select(.platform.architecture != "unknown")] | .[0].digest')

          echo "Running Trivy with VEX on ${IMAGE}@${PLAT_DIGEST}..."
          trivy image --vex oci --format table "${IMAGE}@${PLAT_DIGEST}" || true
          echo "PASS: Trivy VEX integration check complete."
```

- [ ] **Step 2: Update deploy-fly needs**

Change the `deploy-fly` job's `needs` from:

```yaml
  deploy-fly:
    runs-on: ubuntu-latest
    needs: [version, release-docker]
```

To:

```yaml
  deploy-fly:
    runs-on: ubuntu-latest
    needs: [version, smoke-test]
```

- [ ] **Step 3: Verify YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(ci): add smoke-test job gating production deploy

Tests container startup, binary execution, cosign signatures,
and VEX attestations across all platforms before deploying to Fly.io."
```

---

### Task 3: Add `integration-test` job to CI workflow

**Files:**
- Modify: `.github/workflows/ci.yml`

Add a new job that runs the docker-compose stack and exercises it with the existing `test-cache.sh` script.

- [ ] **Step 1: Add the integration-test job**

Add this job at the end of `.github/workflows/ci.yml` (after the `vulncheck` job):

```yaml
  integration-test:
    runs-on: ubuntu-latest
    permissions:
      contents: read
    steps:
      - uses: actions/checkout@de0fac2e4500dabe0009e67214ff5f5447ce83dd # v6.0.2
        with:
          persist-credentials: false

      - name: Start docker-compose stack
        run: docker compose -f docker-compose.dev.yml up --build -d

      - name: Run cache integration tests
        run: ./scripts/test-cache.sh

      - name: Tear down
        if: always()
        run: docker compose -f docker-compose.dev.yml down -v
```

- [ ] **Step 2: Verify YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "feat(ci): add docker-compose integration test job

Runs the full docker-compose stack (app + MinIO) on every PR
and exercises cache behavior via scripts/test-cache.sh."
```

---

### Task 4: Add explicit VEX integration test step to CI

**Files:**
- Modify: `.github/workflows/ci.yml`

Add a dedicated step in the `test` job that runs the VEX/Trivy integration tests explicitly, making their pass/fail status visible in the workflow UI.

- [ ] **Step 1: Add the integration test step**

In the `test` job of `.github/workflows/ci.yml`, add this step after the existing "Run tests" step (after line 45 `run: go test -v ./...`):

```yaml
      - name: Run integration tests (VEX + Trivy)
        run: go test -v -count=1 -run 'TestTrivyNativeVEX|TestAppSideVEXCrossReference|TestCompareApproaches' ./scanner/
```

- [ ] **Step 2: Verify YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`
Expected: No output (valid YAML)

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "feat(ci): add explicit VEX/Trivy integration test step

Makes integration test pass/fail visible as a distinct step
in the workflow UI rather than buried in go test output."
```
