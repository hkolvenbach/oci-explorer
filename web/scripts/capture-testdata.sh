#!/usr/bin/env bash
# Captures API responses from a running backend for frontend test fixtures.
# Requires the backend to be running on localhost:8080.
#
# Usage:
#   make run-go  # in another terminal
#   ./web/scripts/capture-testdata.sh
#
# The test image (dmitriylewen/alpine:3.21.2) has VEX, attestation referrers,
# and produces vulnerability scan results with CVSS scores.

set -euo pipefail

API="http://localhost:8080"
IMG="dmitriylewen/alpine:3.21.2"
DIR="$(dirname "$0")/../src/lib/testdata"

ENC=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$IMG'))")

echo "Capturing test data for $IMG..."
echo "Output: $DIR/"
echo ""

# Health
echo -n "  health.json... "
curl -sf "$API/api/health" | python3 -m json.tool > "$DIR/health.json"
echo "$(wc -c < "$DIR/health.json" | tr -d ' ') bytes"

# Inspect
echo -n "  inspect.json... "
curl -sf "$API/api/inspect?image=$ENC" | python3 -m json.tool > "$DIR/inspect.json"
echo "$(wc -c < "$DIR/inspect.json" | tr -d ' ') bytes"

# Matching tags
echo -n "  matching-tags.json... "
curl -sf "$API/api/matching-tags?image=$ENC" | python3 -m json.tool > "$DIR/matching-tags.json"
echo "$(wc -c < "$DIR/matching-tags.json" | tr -d ' ') bytes"

# VEX — find the VEX referrer digest from inspect data
REPO="dmitriylewen/alpine"
VEX_DIGEST=$(python3 -c "
import json
data = json.load(open('$DIR/inspect.json'))
for r in data['data']['referrers']:
    if r['type'] == 'vex':
        print(r['digest'])
        break
" 2>/dev/null || true)

if [ -n "$VEX_DIGEST" ]; then
  REPO_ENC=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$REPO'))")
  DIGEST_ENC=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$VEX_DIGEST'))")
  echo -n "  vex.json... "
  curl -sf "$API/api/vex?repository=$REPO_ENC&digest=$DIGEST_ENC" | python3 -m json.tool > "$DIR/vex.json"
  echo "$(wc -c < "$DIR/vex.json" | tr -d ' ') bytes"
else
  echo "  vex.json... SKIPPED (no VEX referrer found)"
fi

# Scan
echo -n "  scan.json... "
curl -sf "$API/api/scan?image=$ENC" | python3 -m json.tool > "$DIR/scan.json"
echo "$(wc -c < "$DIR/scan.json" | tr -d ' ') bytes"

echo ""
echo "Done. Verify with: npx svelte-check --tsconfig web/tsconfig.json"
