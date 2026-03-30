<script lang="ts">
  import type { ScanResult, VEXDocument, ImageInfo } from '../lib/types';
  import { appState } from '../lib/state.svelte';
  import * as api from '../lib/api';
  import ScanResults from './ScanResults.svelte';

  let { data }: { data: ImageInfo } = $props();

  let scanResult = $state<ScanResult | null>(null);
  let scanCachedAt = $state<string | undefined>(undefined);
  let isScanning = $state(false);
  let isPeeking = $state(true); // true while checking for cached results
  let scanError = $state('');
  let scanStep = $state('');
  let globalStatusFilter = $state<'all' | 'fixable' | 'nofix' | 'vexed'>('all');
  let elapsedSeconds = $state(0);
  let timerInterval = $state<ReturnType<typeof setInterval> | null>(null);

  function startTimer() {
    elapsedSeconds = 0;
    timerInterval = setInterval(() => { elapsedSeconds++; }, 1000);
  }

  function stopTimer() {
    if (timerInterval) { clearInterval(timerInterval); timerInterval = null; }
  }

  // Cross-reference scan results with VEX documents attached to the image.
  // Must be called for both fresh scans and cached results since the cache
  // stores raw scan data without VEX annotations.
  async function applyVEX(result: ScanResult) {
    const vexReferrers = (data.referrers || []).filter((r) => r.type === 'vex');
    if (vexReferrers.length === 0) return;

    const vexMap = new Map<string, { status: string; justification?: string; impact?: string }>();
    for (const vexRef of vexReferrers) {
      try {
        const doc: VEXDocument = await api.fetchVEX(data.repository, vexRef.digest);
        for (const stmt of doc.statements) {
          const entry = { status: stmt.status, justification: stmt.justification, impact: stmt.impact_statement };
          if (stmt.vulnerability.name) {
            vexMap.set(stmt.vulnerability.name, entry);
          }
          for (const alias of stmt.vulnerability.aliases || []) {
            vexMap.set(alias, entry);
          }
        }
      } catch (err) {
        console.warn('VEX fetch failed for', vexRef.digest, err);
      }
    }

    if (vexMap.size > 0) {
      for (const sevGroup of Object.values(result.bySeverity)) {
        for (const vuln of sevGroup) {
          const entry = vexMap.get(vuln.vulnerabilityID);
          if (entry) {
            vuln.vexStatus = entry.status;
            vuln.vexJustification = entry.justification;
            vuln.vexImpact = entry.impact;
          }
        }
      }
    }
  }

  // Track which image the current peek is for to discard stale responses
  let peekGeneration = 0;

  // On mount or image change, reset state and check for cached scan results
  $effect(() => {
    const imageRef = data.repository + (data.tag ? ':' + data.tag : '@' + data.digest);
    const gen = ++peekGeneration;
    scanResult = null;
    scanCachedAt = undefined;
    scanError = '';
    isPeeking = true;
    api.peekScan(imageRef).then(async (cached) => {
      if (gen !== peekGeneration) return; // stale response from previous image
      if (cached) {
        await applyVEX(cached.result);
        if (gen !== peekGeneration) return; // check again after async VEX fetch
        scanResult = cached.result;
        scanCachedAt = cached.cachedAt;
      }
    }).finally(() => {
      if (gen === peekGeneration) isPeeking = false;
    });
  });

  // Format "X hours ago" from an RFC3339 timestamp
  function formatTimeAgo(isoTimestamp: string): string {
    const then = new Date(isoTimestamp).getTime();
    const now = Date.now();
    const diffMs = now - then;
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return 'just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}h ago`;
    const diffDays = Math.floor(diffHr / 24);
    return `${diffDays}d ago`;
  }

  let summaryStats = $derived.by(() => {
    if (!scanResult) return null;
    let fixable = 0, vexed = 0, noFix = 0;
    for (const vulns of Object.values(scanResult.bySeverity)) {
      for (const v of vulns) {
        if (v.vexStatus) vexed++;
        else if (v.fixedVersion) fixable++;
        else noFix++;
      }
    }
    return { total: scanResult.totalCount, open: fixable + noFix, fixable, vexed, noFix };
  });

  // Determine highest severity for results border color
  let highestSeverity = $derived.by(() => {
    if (!scanResult || scanResult.totalCount === 0) return null;
    for (const sev of ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'UNKNOWN']) {
      if (scanResult.bySeverity[sev]?.length > 0) return sev;
    }
    return null;
  });

  const severityBorder: Record<string, string> = {
    CRITICAL: 'border-red-500/40',
    HIGH: 'border-orange-500/40',
    MEDIUM: 'border-yellow-500/40',
    LOW: 'border-blue-500/40',
    UNKNOWN: 'border-slate-500/40',
  };

  async function doScan(force = false) {
    const imageRef = data.repository + (data.tag ? ':' + data.tag : '@' + data.digest);
    isScanning = true;
    scanError = '';
    scanResult = null;
    scanCachedAt = undefined;
    startTimer();

    try {
      scanStep = 'Starting scan...';
      const scanResponse = await api.scanImageStream(imageRef, force, (msg) => {
        scanStep = msg;
      });
      const result = scanResponse.result;
      scanCachedAt = scanResponse.cachedAt;

      await applyVEX(result);
      scanResult = result;
      scanStep = '';
    } catch (err) {
      scanError = (err as Error).message;
      scanStep = '';
    } finally {
      stopTimer();
      isScanning = false;
    }
  }
</script>

<!-- Same style as other sections: slate-700 border, slate-800/50 bg -->
<div class="border border-slate-700 rounded-md overflow-hidden bg-slate-800/50 mb-6 fade-in">
  <!-- Header bar - matches CollapsibleSection header style -->
  <div class="flex items-center justify-between px-4 py-3 bg-slate-800">
    <div class="flex items-center gap-3">
      <svg class="w-5 h-5 text-orange-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path>
      </svg>
      <span class="font-semibold text-slate-100">Vulnerability Scan</span>
      {#if isScanning}
        <span class="text-sm text-slate-400">{scanStep}{#if elapsedSeconds > 0}{' '}{elapsedSeconds}s{/if}</span>
      {:else if scanResult && summaryStats}
        <div class="flex items-center gap-1.5 flex-wrap">
          {#if summaryStats.total === 0}
            <span class="px-2 py-0.5 text-xs rounded bg-green-500/15 text-green-400 font-medium">No vulnerabilities</span>
          {:else}
            <span class="px-2 py-0.5 text-xs rounded bg-slate-500/15 text-slate-300 font-medium">{summaryStats.total} vulnerabilities</span>
            <span class="px-2 py-0.5 text-xs rounded font-medium {summaryStats.open > 0 ? 'bg-orange-500/15 text-orange-400' : 'bg-green-500/15 text-green-400'}">{summaryStats.open} open</span>
            {#if summaryStats.fixable > 0}
              <button
                onclick={(e: MouseEvent) => { e.stopPropagation(); globalStatusFilter = globalStatusFilter === 'fixable' ? 'all' : 'fixable'; }}
                class="px-2 py-0.5 text-xs rounded cursor-pointer transition-colors {globalStatusFilter === 'fixable' ? 'bg-green-500/30 text-green-300 ring-1 ring-green-500/50' : 'bg-green-500/15 text-green-400'}"
              >{summaryStats.fixable} fixable</button>
            {/if}
            {#if summaryStats.noFix > 0}
              <button
                onclick={(e: MouseEvent) => { e.stopPropagation(); globalStatusFilter = globalStatusFilter === 'nofix' ? 'all' : 'nofix'; }}
                class="px-2 py-0.5 text-xs rounded cursor-pointer transition-colors {globalStatusFilter === 'nofix' ? 'bg-slate-500/30 text-slate-200 ring-1 ring-slate-400/50' : 'bg-slate-500/15 text-slate-400'}"
              >{summaryStats.noFix} no fix</button>
            {/if}
            {#if summaryStats.vexed > 0}
              <button
                onclick={(e: MouseEvent) => { e.stopPropagation(); globalStatusFilter = globalStatusFilter === 'vexed' ? 'all' : 'vexed'; }}
                class="px-2 py-0.5 text-xs rounded cursor-pointer transition-colors {globalStatusFilter === 'vexed' ? 'bg-purple-500/30 text-purple-300 ring-1 ring-purple-500/50' : 'bg-purple-500/15 text-purple-400'}"
              >{summaryStats.vexed} VEXed</button>
            {/if}
          {/if}
        </div>
      {/if}
    </div>
    <div class="flex items-center gap-2">
      {#if !appState.trivyAvailable}
        <span class="text-xs text-yellow-400">Trivy not available</span>
      {:else if isScanning}
        <div class="w-5 h-5 border-2 border-orange-400 border-t-transparent rounded-full animate-spin"></div>
      {:else if isPeeking}
        <div class="w-4 h-4 border-2 border-slate-500 border-t-transparent rounded-full animate-spin"></div>
      {:else if scanResult}
        <div class="flex items-center gap-2">
          {#if scanCachedAt}
            <span class="text-xs text-slate-500">Scanned {formatTimeAgo(scanCachedAt)}</span>
          {/if}
          <button
            onclick={(_e: MouseEvent) => doScan(true)}
            class="px-3 py-1.5 bg-slate-700 hover:bg-slate-600 border border-slate-600 text-slate-300 rounded-lg text-xs font-medium transition-colors"
          >
            Rescan
          </button>
        </div>
      {:else}
        <button
          onclick={() => doScan()}
          class="px-4 py-1.5 bg-orange-400 hover:bg-orange-300 text-slate-900 rounded-lg text-sm font-medium transition-colors flex items-center gap-2"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path>
          </svg>
          Scan
        </button>
      {/if}
    </div>
  </div>

  {#if scanError}
    <div class="p-4">
      <div class="p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-red-300 text-sm">
        {scanError}
      </div>
    </div>
  {:else if scanResult}
    <div class="p-4 {scanResult.totalCount > 0 && highestSeverity ? severityBorder[highestSeverity] + ' border-t' : 'border-t border-slate-700'}">
      <ScanResults result={scanResult} {globalStatusFilter} />
    </div>
  {/if}
</div>
