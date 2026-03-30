#!/usr/bin/env bash
# Generate 1280x640 PNG OG images from SVG sources.
# Two-step: Chrome screenshot at native SVG size, then resize with sips.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

WIDTH=1280
HEIGHT=640
SVGS=(og-v1-minimal og-v2-layers og-v3-score og-v4-centered)

# Find Chrome
CHROME=""
for candidate in \
  "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
  "/Applications/Chromium.app/Contents/MacOS/Chromium" \
  "$(which google-chrome-stable 2>/dev/null || true)" \
  "$(which chromium 2>/dev/null || true)"; do
  if [[ -n "$candidate" && -x "$candidate" ]]; then
    CHROME="$candidate"
    break
  fi
done

if [[ -z "$CHROME" ]]; then
  echo "ERROR: Chrome/Chromium not found."
  exit 1
fi

echo "Using Chrome: $CHROME"

for name in "${SVGS[@]}"; do
  svg="$SCRIPT_DIR/${name}.svg"
  png="$SCRIPT_DIR/${name}.png"

  if [[ ! -f "$svg" ]]; then
    echo "SKIP: $svg not found"
    continue
  fi

  # Step 1: Screenshot SVG directly at its native viewBox size (1200x630)
  # Use a wrapper HTML that fills the viewport exactly with the SVG
  tmp_html=$(mktemp /tmp/og-XXXXXX.html)
  cat > "$tmp_html" <<HTMLEOF
<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  html, body { width: ${WIDTH}px; height: ${HEIGHT}px; overflow: hidden; background: #0f172a; }
  body { display: flex; align-items: center; justify-content: center; }
  img { width: ${WIDTH}px; height: ${HEIGHT}px; object-fit: fill; }
</style>
</head>
<body>
<img src="file://${svg}">
</body>
</html>
HTMLEOF

  "$CHROME" \
    --headless=new \
    --disable-gpu \
    --no-sandbox \
    "--screenshot=$png" \
    "--window-size=${WIDTH},${HEIGHT}" \
    --hide-scrollbars \
    --force-device-scale-factor=1 \
    "file://$tmp_html" 2>&1 | grep "bytes written" || true

  rm -f "$tmp_html"

  # Step 2: Ensure exact dimensions with sips (macOS) or skip
  if [[ -f "$png" ]] && command -v sips &>/dev/null; then
    sips --resampleWidth "$WIDTH" --resampleHeight "$HEIGHT" "$png" --out "$png" >/dev/null 2>&1 || true
  fi

  if [[ -f "$png" ]]; then
    dims=$(sips -g pixelWidth -g pixelHeight "$png" 2>/dev/null | grep pixel | awk '{print $2}' | tr '\n' 'x' | sed 's/x$//')
    echo "OK: ${name}.png  ${dims}  $(du -h "$png" | cut -f1)"
  else
    echo "FAIL: $name"
  fi
done

echo ""
echo "Done. PNGs are in: $SCRIPT_DIR"
