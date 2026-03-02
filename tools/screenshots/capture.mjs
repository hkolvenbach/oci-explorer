#!/usr/bin/env node
// Screenshot capture script for OCI Image Explorer README.
// Uses Playwright to start the server and take browser screenshots.
//
// Usage:
//   npx playwright install chromium   # one-time setup
//   node tools/screenshots/capture.mjs
//
// Images used:
//   ghcr.io/hkolvenbach/oci-explorer:latest — details, referrers, graph, matching tags (unsupported)
//   alpine:latest — matching tags (supported, Docker Hub)
//   golang:1.21 — vulnerability scanning

import { chromium } from "playwright";
import { execSync, spawn } from "child_process";
import { mkdirSync, writeFileSync } from "fs";
import { dirname, resolve } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "../..");
const OUT_DIR = resolve(ROOT, "docs/screenshots");

// Parse --url flag: if provided, skip build/start and use existing server
const urlArg = process.argv.find((a) => a.startsWith("--url="));
const externalUrl = urlArg ? urlArg.split("=")[1] : null;

const PORT = 9999;
const BASE = externalUrl || `http://localhost:${PORT}`;

mkdirSync(OUT_DIR, { recursive: true });

// Write placeholder files so go:embed docs/* succeeds on first build
for (const f of ["welcome.png", "details.png", "referrers.png", "graph.png", "scan.png", "cli-output.txt"]) {
  const p = resolve(OUT_DIR, f);
  try { writeFileSync(p, "", { flag: "wx" }); } catch { /* already exists */ }
}

let server = null;
let cliOutput = "";

if (!externalUrl) {
  // Build the server
  console.log("Building server...");
  execSync("make build", { cwd: ROOT, stdio: "inherit" });

  // Start the server and capture CLI output
  console.log(`Starting server on port ${PORT}...`);
  server = spawn("./build/oci-explorer", ["--port", String(PORT)], {
    cwd: ROOT,
    env: { ...process.env },
  });

  server.stdout.on("data", (d) => {
    const text = d.toString();
    process.stdout.write(text);
    cliOutput += text;
  });
  server.stderr.on("data", (d) => {
    process.stderr.write(d.toString());
  });
} else {
  console.log(`Using existing server at ${BASE}`);
}

// Wait for the server to be ready
async function waitForServer(url, timeoutMs = 15000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      const res = await fetch(`${url}/api/health`);
      if (res.ok) return;
    } catch {
      // not ready yet
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  throw new Error("Server did not start in time");
}

// Helper: navigate to an image by URL (avoids fragile button selectors)
async function inspectImage(page, imageRef) {
  const url = `${BASE}/?q=${encodeURIComponent(imageRef)}`;
  await page.goto(url, { waitUntil: "domcontentloaded" });
  // Wait for the image data to render (ImageSummary shows the repository name)
  const repoName = imageRef.split(":")[0];
  await page.waitForFunction(
    (name) => document.body.innerText.includes(name),
    repoName,
    { timeout: 120000 },
  );
  // Let rendering settle
  await page.waitForTimeout(2000);
}

try {
  await waitForServer(BASE);
  console.log("Server is ready.");

  // Save CLI startup output (only when we started the server ourselves)
  if (!externalUrl && cliOutput) {
    const normalizedOutput = cliOutput.replace(String(PORT), "8080");
    writeFileSync(resolve(OUT_DIR, "cli-output.txt"), normalizedOutput.trimEnd() + "\n");
    console.log("Saved cli-output.txt");
  }

  // Launch browser
  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  // ── 1. Welcome / landing page ──
  await page.goto(BASE, { waitUntil: "networkidle" });
  await page.waitForSelector('text=Welcome to OCI Image Explorer', { timeout: 5000 });
  await page.screenshot({ path: resolve(OUT_DIR, "welcome.png") });
  console.log("Captured welcome.png");

  // ── 2. Details view — ghcr.io/hkolvenbach/oci-explorer:latest ──
  await inspectImage(page, "ghcr.io/hkolvenbach/oci-explorer:latest");
  await page.screenshot({ path: resolve(OUT_DIR, "details.png") });
  console.log("Captured details.png");

  // Helper: find a CollapsibleSection by its title text and return the outer container
  async function findSection(title) {
    return page.evaluateHandle((t) => {
      // CollapsibleSection renders: <div class="border ..."><div>...<span class="font-semibold">{title}</span>...
      for (const span of document.querySelectorAll('span.font-semibold')) {
        if (span.textContent.trim() === t) {
          // Walk up to the outer border container
          let el = span.closest('.border');
          return el;
        }
      }
      return null;
    }, title);
  }

  // ── 3. Referrers section close-up ──
  const referrersEl = await findSection("Referrers");
  if (referrersEl.asElement()) {
    await referrersEl.asElement().scrollIntoViewIfNeeded();
    await page.waitForTimeout(500);
    await referrersEl.asElement().screenshot({ path: resolve(OUT_DIR, "referrers.png") });
    console.log("Captured referrers.png");
  } else {
    console.log("WARN: referrers section not found, skipping referrers.png");
  }

  // ── 4. Matching tags — unsupported registry (GHCR) ──
  //    Already on ghcr.io/hkolvenbach/oci-explorer:latest, wait for matching tags
  await page.waitForFunction(() => {
    return document.body.innerText.includes('not supported');
  }, { timeout: 30000 }).catch(() => {});
  await page.waitForTimeout(500);
  const tagsUnsupported = await findSection("Tags");
  if (tagsUnsupported.asElement()) {
    await tagsUnsupported.asElement().scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
    await tagsUnsupported.asElement().screenshot({ path: resolve(OUT_DIR, "matching-tags-unsupported.png") });
    console.log("Captured matching-tags-unsupported.png");
  } else {
    console.log("WARN: tags section not found, skipping matching-tags-unsupported.png");
  }

  // ── 5. Matching tags — supported registry (Docker Hub alpine:latest) ──
  await inspectImage(page, "alpine:latest");
  // Wait for matching tags to resolve ("current" badge appears)
  await page.waitForFunction(() => {
    return document.body.innerText.includes('current');
  }, { timeout: 60000 }).catch(() => {});
  await page.waitForTimeout(500);
  const tagsSupported = await findSection("Tags");
  if (tagsSupported.asElement()) {
    await tagsSupported.asElement().scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
    await tagsSupported.asElement().screenshot({ path: resolve(OUT_DIR, "matching-tags-supported.png") });
    console.log("Captured matching-tags-supported.png");
  } else {
    console.log("WARN: tags section not found, skipping matching-tags-supported.png");
  }

  // ── 6. Graph view — ghcr.io/hkolvenbach/oci-explorer:latest ──
  await inspectImage(page, "ghcr.io/hkolvenbach/oci-explorer:latest");
  await page.locator('button:has-text("Graph")').click();
  await page.waitForTimeout(2000);
  await page.screenshot({ path: resolve(OUT_DIR, "graph.png") });
  console.log("Captured graph.png");

  // ── 7. Vulnerability scan — golang:1.21 ──
  await inspectImage(page, "golang:1.21");
  // Click the Scan button
  const scanBtn = page.locator('button:has-text("Scan")').first();
  await scanBtn.click();
  // Wait for scan to complete (can take a few minutes)
  console.log("Waiting for vulnerability scan to complete (this may take a few minutes)...");
  await page.waitForFunction(() => {
    return document.body.innerText.includes('vulnerabilities') &&
           document.body.innerText.includes('CRITICAL');
  }, { timeout: 600000 });
  await page.waitForTimeout(1000);
  // Scroll so the Vulnerability Scan header + severity chips + some results are visible.
  // Account for the sticky header (~140px) by scrolling the element to view with offset.
  await page.evaluate(() => {
    for (const span of document.querySelectorAll('span.font-semibold')) {
      if (span.textContent.trim() === 'Vulnerability Scan') {
        const el = span.closest('.border');
        if (el) {
          const rect = el.getBoundingClientRect();
          window.scrollBy(0, rect.top - 140);
        }
        break;
      }
    }
  });
  await page.waitForTimeout(500);
  await page.screenshot({ path: resolve(OUT_DIR, "scan.png") });
  console.log("Captured scan.png");

  await browser.close();
  console.log("\nDone! Screenshots saved to docs/screenshots/");
} finally {
  if (server) server.kill("SIGTERM");
}
