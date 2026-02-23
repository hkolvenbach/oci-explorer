#!/usr/bin/env bash
# verify-readme-examples.sh — run the public-registry verification examples from README.md
set -euo pipefail

IMAGE="ghcr.io/hkolvenbach/oci-explorer:latest"
REPO="hkolvenbach/oci-explorer"
RELEASE_TAG="v0.2.2"
CERT_ID="https://github.com/hkolvenbach/oci-explorer"
OIDC_ISSUER="https://token.actions.githubusercontent.com"

PASS=0
FAIL=0

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

# Prerequisite checks
for cmd in cosign gh docker; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: $cmd is required but not found" >&2
    exit 1
  fi
done

# 1. Cosign signature verification
run_check "Cosign signature" \
  cosign verify \
    --certificate-identity-regexp="$CERT_ID" \
    --certificate-oidc-issuer="$OIDC_ISSUER" \
    "$IMAGE"

# 2. SLSA provenance attestation (v1)
run_check "SLSA provenance (slsaprovenance1)" \
  cosign verify-attestation \
    --type slsaprovenance1 \
    --certificate-identity-regexp="$CERT_ID" \
    --certificate-oidc-issuer="$OIDC_ISSUER" \
    "$IMAGE"

# 3. OpenVEX attestation (may not be present for every image; treated as SKIP if missing)
SKIP=0
echo "--- OpenVEX attestation ---"
if cosign verify-attestation \
    --type openvex \
    --certificate-identity-regexp="$CERT_ID" \
    --certificate-oidc-issuer="$OIDC_ISSUER" \
    "$IMAGE" >/dev/null 2>&1; then
  echo "PASS: OpenVEX attestation"
  PASS=$((PASS + 1))
else
  echo "SKIP: OpenVEX attestation (not found — VEX may not be attached to current image)"
  SKIP=$((SKIP + 1))
fi
echo

# 4. Binary provenance (download release asset, verify, cleanup)
ASSET="oci-explorer-${RELEASE_TAG#v}-linux-amd64.tar.gz"
TMPDIR="$(mktemp -d)"
cleanup() { rm -rf "$TMPDIR"; }
trap cleanup EXIT

echo "--- Binary provenance (gh attestation) ---"
if ! gh release download "$RELEASE_TAG" --repo "$REPO" \
     --pattern 'oci-explorer-*-linux-amd64.tar.gz' \
     --dir "$TMPDIR" >/dev/null 2>&1; then
  echo "FAIL: Binary provenance (could not download release asset)"
  FAIL=$((FAIL + 1))
else
  VERIFY_OUT="$(gh attestation verify "$TMPDIR/$ASSET" --repo "$REPO" 2>&1)" && VERIFY_RC=0 || VERIFY_RC=$?
  if [ "$VERIFY_RC" -eq 0 ]; then
    echo "PASS: Binary provenance"
    PASS=$((PASS + 1))
  elif echo "$VERIFY_OUT" | grep -q "unsupported tlog public key type"; then
    echo "SKIP: Binary provenance (gh CLI too old — upgrade to >= 2.63.0 for Ed25519 support)"
    SKIP=$((SKIP + 1))
  else
    echo "FAIL: Binary provenance"
    echo "  $VERIFY_OUT"
    FAIL=$((FAIL + 1))
  fi
fi
echo

# 5. Embedded SBOM inspection
echo "--- Embedded SBOM inspection ---"
if docker buildx imagetools inspect "$IMAGE" --format '{{ json .SBOM }}' >/dev/null 2>&1; then
  echo "PASS: Embedded SBOM inspection"
  PASS=$((PASS + 1))
else
  echo "FAIL: Embedded SBOM inspection"
  FAIL=$((FAIL + 1))
fi
echo

# Summary
echo "========================="
echo "Results: $PASS passed, $FAIL failed, $SKIP skipped"
echo "========================="
[ "$FAIL" -eq 0 ]
