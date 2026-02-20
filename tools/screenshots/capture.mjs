#!/usr/bin/env node
// Screenshot capture script for OCI Image Explorer README.
// Uses Playwright to start the server and take browser screenshots.
//
// Usage:
//   npx playwright install chromium   # one-time setup
//   node tools/screenshots/capture.mjs

import { chromium } from "playwright";
import { execSync, spawn } from "child_process";
import { mkdirSync, writeFileSync } from "fs";
import { dirname, resolve } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "../..");
const OUT_DIR = resolve(ROOT, "docs/screenshots");
const PORT = 9999;
const URL = `http://localhost:${PORT}`;

mkdirSync(OUT_DIR, { recursive: true });

// Write placeholder files so go:embed docs/* succeeds on first build
for (const f of ["welcome.png", "details.png", "graph.png", "cli-output.txt"]) {
  const p = resolve(OUT_DIR, f);
  try { writeFileSync(p, "", { flag: "wx" }); } catch { /* already exists */ }
}

// Build the server
console.log("Building server...");
execSync("make build", { cwd: ROOT, stdio: "inherit" });

// Start the server and capture CLI output
console.log(`Starting server on port ${PORT}...`);
let cliOutput = "";
const server = spawn("./build/oci-explorer", ["--port", String(PORT)], {
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

try {
  await waitForServer(URL);
  console.log("Server is ready.");

  // Save CLI startup output, replacing the capture port with the default port
  const normalizedOutput = cliOutput.replace(String(PORT), "8080");
  writeFileSync(resolve(OUT_DIR, "cli-output.txt"), normalizedOutput.trimEnd() + "\n");
  console.log("Saved cli-output.txt");

  // Launch browser
  const browser = await chromium.launch();
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  // 1. Welcome / landing page
  await page.goto(URL, { waitUntil: "networkidle" });
  await page.waitForSelector("#welcome-container:not(.hidden)", { timeout: 5000 });
  await page.screenshot({ path: resolve(OUT_DIR, "welcome.png") });
  console.log("Captured welcome.png");

  // 2. Details view — inspect alpine:latest
  await page.fill("#image-input", "alpine:latest");
  await page.click("#inspect-btn");
  await page.waitForSelector("#image-container:not(.hidden)", { timeout: 30000 });
  // Give rendering a moment to settle
  await page.waitForTimeout(1000);
  await page.screenshot({ path: resolve(OUT_DIR, "details.png") });
  console.log("Captured details.png");

  // 3. Graph view
  await page.click("#btn-graph");
  await page.waitForTimeout(1500);
  await page.screenshot({ path: resolve(OUT_DIR, "graph.png") });
  console.log("Captured graph.png");

  await browser.close();
  console.log("Done!");
} finally {
  server.kill("SIGTERM");
}
