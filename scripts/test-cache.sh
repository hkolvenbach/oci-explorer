#!/usr/bin/env bash
# Integration tests for the S3 response cache.
#
# Prerequisites:
#   docker compose -f docker-compose.dev.yml up --build -d
#
# Usage:
#   ./scripts/test-cache.sh [base_url]
#
# Defaults to http://localhost:8080 if no argument is given.

set -euo pipefail

BASE="${1:-http://localhost:8080}"
PASS=0
FAIL=0

green() { printf "\033[32m%s\033[0m\n" "$*"; }
red()   { printf "\033[31m%s\033[0m\n" "$*"; }
bold()  { printf "\033[1m%s\033[0m\n" "$*"; }

assert_eq() {
  local label="$1" got="$2" want="$3"
  if [ "$got" = "$want" ]; then
    green "  PASS: $label (got: $got)"
    PASS=$((PASS + 1))
  else
    red "  FAIL: $label (got: $got, want: $want)"
    FAIL=$((FAIL + 1))
  fi
}

assert_contains() {
  local label="$1" haystack="$2" needle="$3"
  if echo "$haystack" | grep -q "$needle"; then
    green "  PASS: $label"
    PASS=$((PASS + 1))
  else
    red "  FAIL: $label (expected to contain: $needle)"
    FAIL=$((FAIL + 1))
  fi
}

# ─── Wait for the app to be healthy ───────────────────────────────────────────

bold "Waiting for $BASE/api/health..."
for i in $(seq 1 30); do
  if curl -sf "$BASE/api/health" > /dev/null 2>&1; then
    break
  fi
  if [ "$i" -eq 30 ]; then
    red "App not healthy after 30s — aborting."
    exit 1
  fi
  sleep 1
done
green "App is healthy."
echo ""

# ─── 1. Health endpoint reports cache enabled ────────────────────────────────

bold "1. Health endpoint"
HEALTH=$(curl -sf "$BASE/api/health")
CACHE_ENABLED=$(echo "$HEALTH" | grep -o '"cacheEnabled":[a-z]*' | cut -d: -f2)
assert_eq "cacheEnabled is true" "$CACHE_ENABLED" "true"
echo ""

# ─── 2. Inspect: first request is MISS, second is HIT ───────────────────────

IMAGE="alpine:latest"
bold "2. Inspect cache (image: $IMAGE)"

HEADERS1=$(curl -sD - "$BASE/api/inspect?image=$IMAGE" -o /dev/null 2>&1)
XCACHE1=$(echo "$HEADERS1" | grep -i "^X-Cache:" | tr -d '\r' | awk '{print $2}')
assert_eq "first inspect is MISS" "$XCACHE1" "MISS"

HEADERS2=$(curl -sD - "$BASE/api/inspect?image=$IMAGE" -o /dev/null 2>&1)
XCACHE2=$(echo "$HEADERS2" | grep -i "^X-Cache:" | tr -d '\r' | awk '{print $2}')
assert_eq "second inspect is HIT" "$XCACHE2" "HIT"
echo ""

# ─── 3. Different image is a separate MISS ───────────────────────────────────

IMAGE2="busybox:latest"
bold "3. Different image is a MISS (image: $IMAGE2)"

HEADERS3=$(curl -sD - "$BASE/api/inspect?image=$IMAGE2" -o /dev/null 2>&1)
XCACHE3=$(echo "$HEADERS3" | grep -i "^X-Cache:" | tr -d '\r' | awk '{print $2}')
assert_eq "different image is MISS" "$XCACHE3" "MISS"
echo ""

# ─── 4. Digest-based request hits the same cache entry ──────────────────────

bold "4. Digest-based request (same image as test 2)"
DIGEST=$(curl -sf "$BASE/api/inspect?image=$IMAGE" | grep -o '"digest":"[^"]*"' | head -1 | cut -d'"' -f4)
if [ -n "$DIGEST" ]; then
  REPO=$(echo "$IMAGE" | cut -d: -f1)
  # For Docker Hub library images, the repository in the response includes the registry
  FULL_REF=$(curl -sf "$BASE/api/inspect?image=$IMAGE" | grep -o '"repository":"[^"]*"' | head -1 | cut -d'"' -f4)
  DIGEST_REF="${FULL_REF}@${DIGEST}"

  HEADERS4=$(curl -sD - "$BASE/api/inspect?image=$DIGEST_REF" -o /dev/null 2>&1)
  XCACHE4=$(echo "$HEADERS4" | grep -i "^X-Cache:" | tr -d '\r' | awk '{print $2}')
  assert_eq "digest-based request is HIT" "$XCACHE4" "HIT"
else
  red "  SKIP: could not extract digest from response"
fi
echo ""

# ─── 5. Scan: MISS then HIT, force=1 bypasses cache ─────────────────────────

bold "5. Scan cache + force refresh (image: $IMAGE)"

# Check if Trivy is available
TRIVY_AVAILABLE=$(curl -sf "$BASE/api/health" | grep -o '"trivyAvailable":[a-z]*' | cut -d: -f2)
if [ "$TRIVY_AVAILABLE" = "true" ]; then
  SCAN_HEADERS1=$(curl -sD - "$BASE/api/scan?image=$IMAGE" -o /dev/null 2>&1)
  SCAN_CACHE1=$(echo "$SCAN_HEADERS1" | grep -i "^X-Cache:" | tr -d '\r' | awk '{print $2}')
  assert_eq "first scan is MISS" "$SCAN_CACHE1" "MISS"

  SCAN_HEADERS2=$(curl -sD - "$BASE/api/scan?image=$IMAGE" -o /dev/null 2>&1)
  SCAN_CACHE2=$(echo "$SCAN_HEADERS2" | grep -i "^X-Cache:" | tr -d '\r' | awk '{print $2}')
  assert_eq "second scan is HIT" "$SCAN_CACHE2" "HIT"

  # X-Cached-At should be present on HIT
  CACHED_AT=$(echo "$SCAN_HEADERS2" | grep -i "^X-Cached-At:" | tr -d '\r')
  if [ -n "$CACHED_AT" ]; then
    green "  PASS: X-Cached-At header present on scan HIT"
    PASS=$((PASS + 1))
  else
    red "  FAIL: X-Cached-At header missing on scan HIT"
    FAIL=$((FAIL + 1))
  fi

  # force=1 bypasses cache
  SCAN_HEADERS3=$(curl -sD - "$BASE/api/scan?image=$IMAGE&force=1" -o /dev/null 2>&1)
  SCAN_CACHE3=$(echo "$SCAN_HEADERS3" | grep -i "^X-Cache:" | tr -d '\r' | awk '{print $2}')
  # force=1 skips cache entirely, so no X-Cache header
  if [ -z "$SCAN_CACHE3" ]; then
    green "  PASS: force=1 bypasses cache (no X-Cache header)"
    PASS=$((PASS + 1))
  else
    red "  FAIL: force=1 should bypass cache (got X-Cache: $SCAN_CACHE3)"
    FAIL=$((FAIL + 1))
  fi
else
  echo "  SKIP: Trivy not available, skipping scan tests"
fi
echo ""

# ─── 6. Prometheus metrics include cache counters ────────────────────────────

bold "6. Prometheus metrics"

# Metrics may be on a separate port or /api/metrics
METRICS_URL="$BASE/api/metrics"
METRICS=$(curl -sf "$METRICS_URL" 2>/dev/null || echo "")
if [ -z "$METRICS" ]; then
  # Try port 9090 (METRICS_PORT default on Fly)
  METRICS_URL="${BASE%:*}:9090/metrics"
  METRICS=$(curl -sf "$METRICS_URL" 2>/dev/null || echo "")
fi

if [ -n "$METRICS" ]; then
  assert_contains "oci_cache_requests_total present" "$METRICS" "oci_cache_requests_total"
  assert_contains "cache hits recorded" "$METRICS" 'result="hit"'
  assert_contains "cache misses recorded" "$METRICS" 'result="miss"'
  assert_contains "oci_cache_latency_seconds present" "$METRICS" "oci_cache_latency_seconds"
else
  red "  SKIP: could not reach metrics endpoint"
fi
echo ""

# ─── Summary ─────────────────────────────────────────────────────────────────

bold "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ "$FAIL" -eq 0 ]; then
  green "All $PASS tests passed."
else
  red "$FAIL failed, $PASS passed."
fi
exit "$FAIL"
