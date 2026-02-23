#!/usr/bin/env bash
# verify-readme-examples.sh — verify supply chain artifacts across reference images
#
# Tests OCI Explorer's own image plus a curated set of third-party images that
# exercise different supply chain patterns (cosign tag scheme, OCI referrers,
# BuildKit attestations, SLSA provenance, OpenVEX, SBOMs).
#
# If OCI Explorer is not already running, builds and starts it automatically.
set -euo pipefail

PASS=0
FAIL=0
SKIP=0
OCI_EXPLORER_PID=""

# ── Helpers ──────────────────────────────────────────────────────────────────

run_check() {
  local name="$1"
  shift
  echo "--- $name ---"
  if "$@" >/dev/null 2>&1; then
    echo "PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "FAIL: $name"
    FAIL=$((FAIL + 1))
  fi
  echo
}

skip_check() {
  local name="$1"
  local reason="$2"
  echo "--- $name ---"
  echo "SKIP: $name ($reason)"
  SKIP=$((SKIP + 1))
  echo
}

# Resolve the index digest for a multi-arch image.
resolve_index_digest() {
  local image="$1"
  crane digest "$image" 2>/dev/null || true
}

# Resolve the first real platform manifest digest from a multi-arch image.
# Uses negated equality that is safe across bash/zsh (avoids != which zsh
# history-expands the ! character, breaking jq).
resolve_platform_digest() {
  local image="$1"
  docker buildx imagetools inspect --raw "$image" 2>/dev/null \
    | jq -r '[.manifests[] | select(.platform.architecture | . == "unknown" | not)] | .[0].digest' 2>/dev/null || true
}

# Check if a cosign .att tag exists and contains an OpenVEX predicate.
# cosign verify-attestation only checks OCI referrers, not .att tags,
# so we inspect the .att tag manifest directly.
has_openvex_att_tag() {
  local repo="$1" digest="$2"
  local tag="sha256-${digest#sha256:}.att"
  crane manifest "${repo}:${tag}" 2>/dev/null \
    | jq -e '.layers[]? | select(.annotations.predicateType == "https://openvex.dev/ns")' >/dev/null 2>&1
}

# Check OCI Explorer API for referrer types (requires running server at $API_BASE).
# Usage: api_has_referrer_type <image> <type>
API_BASE="${OCI_EXPLORER_URL:-http://localhost:8888}"
api_has_referrer_type() {
  local image="$1" type="$2"
  curl -sf "${API_BASE}/api/inspect?image=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$image'))")" \
    | jq -e "[.data.referrers[]? | select(.type == \"$type\")] | length > 0" >/dev/null 2>&1
}

# ── Prerequisite checks ─────────────────────────────────────────────────────

for cmd in cosign crane gh docker jq curl python3; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: $cmd is required but not found" >&2
    exit 1
  fi
done

TMPDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMPDIR"
  if [ -n "$OCI_EXPLORER_PID" ]; then
    echo "Stopping OCI Explorer (PID $OCI_EXPLORER_PID)..."
    kill "$OCI_EXPLORER_PID" 2>/dev/null || true
    wait "$OCI_EXPLORER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# ── Start OCI Explorer if needed ─────────────────────────────────────────────

if ! curl -sf "${API_BASE}/api/inspect?image=alpine:3.19" >/dev/null 2>&1; then
  echo "OCI Explorer not running at $API_BASE — building and starting..."
  SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
  PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
  OCI_EXPLORER_BIN="$TMPDIR/oci-explorer"

  (cd "$PROJECT_ROOT" && go build -o "$OCI_EXPLORER_BIN" .) || {
    echo "ERROR: failed to build OCI Explorer" >&2
    exit 1
  }

  PORT="${API_BASE##*:}"
  "$OCI_EXPLORER_BIN" -port "$PORT" &
  OCI_EXPLORER_PID=$!

  # Wait for server to be ready (up to 10 seconds)
  for i in $(seq 1 20); do
    if curl -sf "${API_BASE}/api/inspect?image=alpine:3.19" >/dev/null 2>&1; then
      echo "OCI Explorer started (PID $OCI_EXPLORER_PID) on port $PORT"
      break
    fi
    if [ "$i" -eq 20 ]; then
      echo "ERROR: OCI Explorer failed to start within 10 seconds" >&2
      exit 1
    fi
    sleep 0.5
  done
  echo
fi

# ════════════════════════════════════════════════════════════════════════════
# 1. OCI Explorer's own image — full supply chain verification
# ════════════════════════════════════════════════════════════════════════════

echo "========================================================"
echo "  ghcr.io/hkolvenbach/oci-explorer (own image)"
echo "========================================================"
echo

OCI_IMAGE="ghcr.io/hkolvenbach/oci-explorer:latest"
OCI_REPO="hkolvenbach/oci-explorer"
OCI_RELEASE_TAG="v0.2.2"
OCI_CERT_ID="https://github.com/hkolvenbach/oci-explorer"
OCI_OIDC_ISSUER="https://token.actions.githubusercontent.com"

# 1a. Cosign signature
run_check "oci-explorer: cosign signature" \
  cosign verify \
    --certificate-identity-regexp="$OCI_CERT_ID" \
    --certificate-oidc-issuer="$OCI_OIDC_ISSUER" \
    "$OCI_IMAGE"

# 1b. SLSA provenance (via OCI referrers — sigstore bundle)
run_check "oci-explorer: SLSA provenance" \
  cosign verify-attestation \
    --type slsaprovenance1 \
    --certificate-identity-regexp="$OCI_CERT_ID" \
    --certificate-oidc-issuer="$OCI_OIDC_ISSUER" \
    "$OCI_IMAGE"

# 1c. OpenVEX attestation (via cosign .att tag on the index digest)
# Note: cosign verify-attestation only checks OCI referrers, not .att tags.
# VEX is attached via `cosign attest --type openvex` which writes to .att tags,
# so we verify by inspecting the .att tag manifest directly.
echo "--- oci-explorer: OpenVEX attestation ---"
OCI_INDEX_DIGEST=$(resolve_index_digest "$OCI_IMAGE")
OCI_IMAGE_REPO="${OCI_IMAGE%%:*}"
if [ -n "$OCI_INDEX_DIGEST" ] && has_openvex_att_tag "$OCI_IMAGE_REPO" "$OCI_INDEX_DIGEST"; then
  echo "PASS: oci-explorer: OpenVEX attestation (index ${OCI_INDEX_DIGEST:0:19}...)"
  PASS=$((PASS + 1))
else
  echo "FAIL: oci-explorer: OpenVEX attestation (no .att tag with openvex predicate on index)"
  FAIL=$((FAIL + 1))
fi
echo

# 1d. Binary provenance
ASSET="oci-explorer-${OCI_RELEASE_TAG#v}-linux-amd64.tar.gz"
echo "--- oci-explorer: binary provenance ---"
if ! gh release download "$OCI_RELEASE_TAG" --repo "$OCI_REPO" \
     --pattern 'oci-explorer-*-linux-amd64.tar.gz' \
     --dir "$TMPDIR" >/dev/null 2>&1; then
  echo "FAIL: oci-explorer: binary provenance (could not download release asset)"
  FAIL=$((FAIL + 1))
else
  VERIFY_OUT="$(gh attestation verify "$TMPDIR/$ASSET" --repo "$OCI_REPO" 2>&1)" && VERIFY_RC=0 || VERIFY_RC=$?
  if [ "$VERIFY_RC" -eq 0 ]; then
    echo "PASS: oci-explorer: binary provenance"
    PASS=$((PASS + 1))
  elif echo "$VERIFY_OUT" | grep -q "unsupported tlog public key type"; then
    echo "SKIP: oci-explorer: binary provenance (gh CLI too old — upgrade to >= 2.63.0)"
    SKIP=$((SKIP + 1))
  else
    echo "FAIL: oci-explorer: binary provenance"
    echo "  $VERIFY_OUT"
    FAIL=$((FAIL + 1))
  fi
fi
echo

# 1e. Embedded SBOM
echo "--- oci-explorer: embedded SBOM ---"
if docker buildx imagetools inspect "$OCI_IMAGE" --format '{{ json .SBOM }}' >/dev/null 2>&1; then
  echo "PASS: oci-explorer: embedded SBOM"
  PASS=$((PASS + 1))
else
  echo "FAIL: oci-explorer: embedded SBOM"
  FAIL=$((FAIL + 1))
fi
echo

# 1f. OCI Explorer API — verify all expected referrer types are discovered
echo "--- oci-explorer: API referrer discovery ---"
for TYPE in signature sbom vex attestation; do
  if api_has_referrer_type "$OCI_IMAGE" "$TYPE"; then
    echo "PASS: oci-explorer: API discovers '$TYPE' referrers"
    PASS=$((PASS + 1))
  else
    echo "FAIL: oci-explorer: API discovers '$TYPE' referrers"
    FAIL=$((FAIL + 1))
  fi
done
echo

# ════════════════════════════════════════════════════════════════════════════
# 2. ghcr.io/hkolvenbach/oci-explorer:0.2.2 — pinned release
#    Expected: signature, SBOM (SPDX), attestation (sigstore bundle), VEX
# ════════════════════════════════════════════════════════════════════════════

echo "========================================================"
echo "  ghcr.io/hkolvenbach/oci-explorer:0.2.2 (pinned)"
echo "========================================================"
echo

OCI_PINNED="ghcr.io/hkolvenbach/oci-explorer:0.2.2"

run_check "oci-explorer:0.2.2: cosign signature" \
  cosign verify \
    --certificate-identity-regexp="$OCI_CERT_ID" \
    --certificate-oidc-issuer="$OCI_OIDC_ISSUER" \
    "$OCI_PINNED"

# 2b. OpenVEX attestation on pinned release (via .att tag on index)
echo "--- oci-explorer:0.2.2: OpenVEX attestation ---"
PINNED_INDEX_DIGEST=$(resolve_index_digest "$OCI_PINNED")
PINNED_REPO="${OCI_PINNED%%:*}"
if [ -n "$PINNED_INDEX_DIGEST" ] && has_openvex_att_tag "$PINNED_REPO" "$PINNED_INDEX_DIGEST"; then
  echo "PASS: oci-explorer:0.2.2: OpenVEX attestation (index ${PINNED_INDEX_DIGEST:0:19}...)"
  PASS=$((PASS + 1))
else
  echo "FAIL: oci-explorer:0.2.2: OpenVEX attestation"
  FAIL=$((FAIL + 1))
fi
echo

# 2c. OCI Explorer API — verify all expected referrer types
echo "--- oci-explorer:0.2.2: API referrer discovery ---"
for TYPE in signature sbom vex attestation; do
  if api_has_referrer_type "$OCI_PINNED" "$TYPE"; then
    echo "PASS: oci-explorer:0.2.2: API discovers '$TYPE' referrers"
    PASS=$((PASS + 1))
  else
    echo "FAIL: oci-explorer:0.2.2: API discovers '$TYPE' referrers"
    FAIL=$((FAIL + 1))
  fi
done
echo

# ════════════════════════════════════════════════════════════════════════════
# 3. dmitriylewen/alpine:3.21.2 — cosign tag scheme VEX
#    VEX is attached via cosign .att tag, signature via OCI Referrers API.
#    Reference: https://github.com/aquasecurity/trivy/discussions/9833
# ════════════════════════════════════════════════════════════════════════════

echo "========================================================"
echo "  dmitriylewen/alpine:3.21.2 (cosign tag scheme VEX)"
echo "========================================================"
echo

DMITRI_IMAGE="dmitriylewen/alpine:3.21.2"

# cosign tree shows: .att tag (VEX) + referrers API (sigstore bundle)
echo "--- dmitriylewen/alpine:3.21.2: cosign tree ---"
if cosign tree "$DMITRI_IMAGE" >/dev/null 2>&1; then
  echo "PASS: dmitriylewen/alpine:3.21.2: cosign tree"
  PASS=$((PASS + 1))
else
  echo "FAIL: dmitriylewen/alpine:3.21.2: cosign tree"
  FAIL=$((FAIL + 1))
fi
echo

# 3b. OpenVEX via .att tag
echo "--- dmitriylewen/alpine:3.21.2: OpenVEX attestation ---"
DMITRI_DIGEST=$(resolve_index_digest "$DMITRI_IMAGE")
DMITRI_REPO="${DMITRI_IMAGE%%:*}"
if [ -n "$DMITRI_DIGEST" ] && has_openvex_att_tag "$DMITRI_REPO" "$DMITRI_DIGEST"; then
  echo "PASS: dmitriylewen/alpine:3.21.2: OpenVEX attestation (.att tag)"
  PASS=$((PASS + 1))
else
  echo "FAIL: dmitriylewen/alpine:3.21.2: OpenVEX attestation (.att tag)"
  FAIL=$((FAIL + 1))
fi
echo

# 3c. OCI Explorer API
echo "--- dmitriylewen/alpine:3.21.2: API referrer discovery ---"
# Must discover VEX from cosign .att tag scheme
if api_has_referrer_type "$DMITRI_IMAGE" "vex"; then
  echo "PASS: dmitriylewen/alpine:3.21.2: API discovers VEX (cosign tag scheme)"
  PASS=$((PASS + 1))
else
  echo "FAIL: dmitriylewen/alpine:3.21.2: API discovers VEX (cosign tag scheme)"
  FAIL=$((FAIL + 1))
fi
# Must discover attestation from OCI Referrers API
if api_has_referrer_type "$DMITRI_IMAGE" "attestation"; then
  echo "PASS: dmitriylewen/alpine:3.21.2: API discovers attestation (OCI referrers)"
  PASS=$((PASS + 1))
else
  echo "FAIL: dmitriylewen/alpine:3.21.2: API discovers attestation (OCI referrers)"
  FAIL=$((FAIL + 1))
fi
echo

# ════════════════════════════════════════════════════════════════════════════
# 4. ghcr.io/aquasecurity/trivy:0.69.1 — Trivy image
#    Expected: cosign signatures (via OCI referrers), multi-arch, NO VEX.
#    Trivy distributes VEX via VEX Hub, not OCI attestations.
# ════════════════════════════════════════════════════════════════════════════

echo "========================================================"
echo "  ghcr.io/aquasecurity/trivy:0.69.1"
echo "========================================================"
echo

TRIVY_IMAGE="ghcr.io/aquasecurity/trivy:0.69.1"

run_check "trivy:0.69.1: cosign signature" \
  cosign verify \
    --certificate-identity-regexp=".*aquasecurity.*" \
    --certificate-oidc-issuer="https://token.actions.githubusercontent.com" \
    "$TRIVY_IMAGE"

# 4b. Confirm NO VEX attestation (Trivy uses VEX Hub, not OCI attestations)
echo "--- trivy:0.69.1: no OpenVEX attestation (expected) ---"
TRIVY_DIGEST=$(resolve_index_digest "$TRIVY_IMAGE")
TRIVY_REPO="${TRIVY_IMAGE%%:*}"
if [ -n "$TRIVY_DIGEST" ] && has_openvex_att_tag "$TRIVY_REPO" "$TRIVY_DIGEST"; then
  echo "FAIL: trivy:0.69.1: unexpected OpenVEX .att tag found"
  FAIL=$((FAIL + 1))
else
  echo "PASS: trivy:0.69.1: no OpenVEX attestation (correct — uses VEX Hub)"
  PASS=$((PASS + 1))
fi
echo

# 4c. OCI Explorer API
echo "--- trivy:0.69.1: API referrer discovery ---"
# Trivy images have referrers (cosign signatures appear as artifacts)
REFERRER_COUNT=$(curl -sf "${API_BASE}/api/inspect?image=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$TRIVY_IMAGE'))")" \
  | jq '[.data.referrers[]?] | length')
if [ "$REFERRER_COUNT" -gt 0 ]; then
  echo "PASS: trivy:0.69.1: API discovers referrers ($REFERRER_COUNT found)"
  PASS=$((PASS + 1))
else
  echo "FAIL: trivy:0.69.1: API discovers referrers (none found)"
  FAIL=$((FAIL + 1))
fi
# Confirm no VEX via API either
if api_has_referrer_type "$TRIVY_IMAGE" "vex"; then
  echo "FAIL: trivy:0.69.1: API unexpectedly discovers VEX referrers"
  FAIL=$((FAIL + 1))
else
  echo "PASS: trivy:0.69.1: API correctly reports no VEX referrers"
  PASS=$((PASS + 1))
fi
echo

# ── Summary ──────────────────────────────────────────────────────────────────

echo "========================================================"
echo "  Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "========================================================"
[ "$FAIL" -eq 0 ]
