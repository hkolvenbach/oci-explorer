#!/usr/bin/env bash
# Fetches supply chain artifact badges for each example image on the welcome page.
# Requires the backend to be running on localhost:8080.
#
# Usage:
#   make run  # in another terminal
#   ./web/scripts/update-badges.sh
#
# Then update the `examples` array in web/src/components/WelcomeView.svelte
# with the output.

set -euo pipefail

API="http://localhost:8080/api/inspect"

IMAGES=(
  "alpine:latest"
  "nginx:latest"
  "python:3.12-slim"
  "golang:1.21"
  "ghcr.io/hkolvenbach/oci-explorer:latest"
  "dmitriylewen/alpine:3.21.2"
)

echo "Fetching badges for welcome page examples..."
echo ""
echo "  const examples: Example[] = ["

for img in "${IMAGES[@]}"; do
  encoded=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$img'))")
  badges=$(curl -sf "${API}?image=${encoded}" | python3 -c "
import sys, json
data = json.load(sys.stdin)
if not data.get('success'):
    sys.exit(0)
refs = data.get('data', {}).get('referrers', [])
order = ['SBOM', 'VEX', 'ATT', 'SIG']
mapping = {'sbom': 'SBOM', 'vex': 'VEX', 'attestation': 'ATT', 'signature': 'SIG'}
found = set()
for r in refs:
    t = r.get('type', '')
    if t in mapping:
        found.add(mapping[t])
result = [b for b in order if b in found]
print(', '.join(repr(b) for b in result))
" 2>/dev/null || echo "")

  if [ -n "$badges" ]; then
    echo "    { image: '$img', badges: [$badges] },"
  else
    echo "    { image: '$img' },"
  fi
done

echo "  ];"
echo ""
echo "Copy the above into WelcomeView.svelte"
