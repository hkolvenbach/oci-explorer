<script lang="ts">
  import type { ScanResult, VulnSummary } from '../lib/types';
  import { appState } from '../lib/state.svelte';

  let { result, globalStatusFilter = 'all' }: { result: ScanResult; globalStatusFilter?: 'all' | 'fixable' | 'nofix' | 'vexed' } = $props();

  const severityOrder = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'UNKNOWN'];
  const severityColors: Record<string, { bg: string; text: string; border: string; badge: string; badgeOff: string }> = {
    CRITICAL: { bg: 'bg-red-500/10', text: 'text-red-400', border: 'border-red-500/30', badge: 'bg-red-500/20 text-red-300', badgeOff: 'bg-red-500/5 text-red-400/40' },
    HIGH: { bg: 'bg-orange-500/10', text: 'text-orange-400', border: 'border-orange-500/30', badge: 'bg-orange-500/20 text-orange-300', badgeOff: 'bg-orange-500/5 text-orange-400/40' },
    MEDIUM: { bg: 'bg-yellow-500/10', text: 'text-yellow-400', border: 'border-yellow-500/30', badge: 'bg-yellow-500/20 text-yellow-300', badgeOff: 'bg-yellow-500/5 text-yellow-400/40' },
    LOW: { bg: 'bg-blue-500/10', text: 'text-blue-400', border: 'border-blue-500/30', badge: 'bg-blue-500/20 text-blue-300', badgeOff: 'bg-blue-500/5 text-blue-400/40' },
    UNKNOWN: { bg: 'bg-slate-500/10', text: 'text-slate-400', border: 'border-slate-500/30', badge: 'bg-slate-500/20 text-slate-300', badgeOff: 'bg-slate-500/5 text-slate-400/40' },
  };

  // All severities present in the results
  let availableSeverities = $derived(
    severityOrder.filter((s) => result.bySeverity[s]?.length > 0),
  );

  // Filter state: which severities are enabled (all on by default)
  let enabledSeverities = $state(new Set(severityOrder));

  // Per-section status filter overrides (when a user clicks chips inside a severity group header)
  let sectionFilters = $state<Record<string, 'all' | 'fixable' | 'nofix' | 'vexed'>>({});

  // Clear section overrides when global filter changes
  $effect(() => {
    globalStatusFilter; // track dependency
    sectionFilters = {};
  });

  // Effective filter for a severity group: section-level override takes priority, then global
  function effectiveFilter(severity: string): 'all' | 'fixable' | 'nofix' | 'vexed' {
    return sectionFilters[severity] ?? globalStatusFilter;
  }

  function vulnMatchesFilter(v: VulnSummary, filter: 'all' | 'fixable' | 'nofix' | 'vexed'): boolean {
    if (filter === 'all') return true;
    if (filter === 'fixable') return !v.vexStatus && !!v.fixedVersion;
    if (filter === 'nofix') return !v.vexStatus && !v.fixedVersion;
    if (filter === 'vexed') return !!v.vexStatus;
    return true;
  }

  // Filtered view
  let visibleSeverities = $derived(
    availableSeverities.filter((s) => enabledSeverities.has(s)),
  );

  // Filtered vulns per severity (apply both severity + effective status filter per section)
  let filteredBySeverity = $derived.by(() => {
    const map: Record<string, VulnSummary[]> = {};
    for (const sev of visibleSeverities) {
      const filter = effectiveFilter(sev);
      const filtered = (result.bySeverity[sev] || []).filter((v) => vulnMatchesFilter(v, filter));
      if (filtered.length > 0) map[sev] = filtered;
    }
    return map;
  });

  let filteredTotal = $derived(
    Object.values(filteredBySeverity).reduce((sum, vulns) => sum + vulns.length, 0),
  );

  function toggleSeverityFilter(severity: string) {
    if (enabledSeverities.has(severity)) {
      enabledSeverities.delete(severity);
    } else {
      enabledSeverities.add(severity);
    }
    enabledSeverities = new Set(enabledSeverities);
  }

  function toggleSectionFilter(severity: string, filter: 'fixable' | 'nofix' | 'vexed') {
    const current = sectionFilters[severity];
    if (current === filter) {
      // Toggle off: remove section override so it falls back to global
      const { [severity]: _, ...rest } = sectionFilters;
      sectionFilters = rest;
    } else {
      sectionFilters = { ...sectionFilters, [severity]: filter };
    }
  }

  // Section collapse state
  function isSectionOpen(severity: string): boolean {
    return appState.collapseStates[`scan-section-${severity}`] ?? true;
  }

  function toggleSection(severity: string) {
    appState.collapseStates[`scan-section-${severity}`] = !isSectionOpen(severity);
  }

  function toggleVuln(id: string) {
    appState.collapseStates[`vuln-${id}`] = !appState.collapseStates[`vuln-${id}`];
  }

  function isVulnOpen(id: string): boolean {
    return appState.collapseStates[`vuln-${id}`] ?? false;
  }

  function getVexBadge(vuln: VulnSummary): { label: string; class: string } | null {
    if (!vuln.vexStatus) return null;
    switch (vuln.vexStatus) {
      case 'not_affected':
        return { label: 'Not Affected', class: 'bg-purple-500/20 text-purple-300' };
      case 'fixed':
        return { label: 'VEX: Fixed', class: 'bg-green-500/20 text-green-300' };
      case 'under_investigation':
        return { label: 'Investigating', class: 'bg-yellow-500/20 text-yellow-300' };
      default:
        return { label: `VEX: ${vuln.vexStatus}`, class: 'bg-purple-500/20 text-purple-300' };
    }
  }

  function groupStats(vulns: VulnSummary[]): { fixable: number; noFix: number; vexed: number } {
    let fixable = 0, noFix = 0, vexed = 0;
    for (const v of vulns) {
      if (v.vexStatus) vexed++;
      else if (v.fixedVersion) fixable++;
      else noFix++;
    }
    return { fixable, noFix, vexed };
  }

  interface RefLink { label: string; url: string; color: string }

  function categorizeRefs(vuln: VulnSummary): RefLink[] {
    const links: RefLink[] = [];
    const seen = new Set<string>();

    if (vuln.primaryURL) {
      const primary = categorizeUrl(vuln.primaryURL);
      links.push(primary);
      seen.add(vuln.primaryURL);
    }

    for (const url of vuln.references || []) {
      if (seen.has(url)) continue;
      seen.add(url);
      links.push(categorizeUrl(url));
    }

    return links;
  }

  function categorizeUrl(url: string): RefLink {
    if (url.includes('nvd.nist.gov')) return { label: 'NVD', url, color: 'text-blue-400 hover:text-blue-300' };
    if (url.includes('cve.org') || url.includes('cve.mitre.org')) return { label: 'CVE', url, color: 'text-blue-400 hover:text-blue-300' };
    if (url.includes('access.redhat.com')) return { label: 'Red Hat', url, color: 'text-red-400 hover:text-red-300' };
    if (url.includes('security-tracker.debian.org')) return { label: 'Debian', url, color: 'text-pink-400 hover:text-pink-300' };
    if (url.includes('ubuntu.com/security')) return { label: 'Ubuntu', url, color: 'text-orange-400 hover:text-orange-300' };
    if (url.includes('github.com') && url.includes('advisories')) return { label: 'GitHub Advisory', url, color: 'text-slate-300 hover:text-slate-200' };
    if (url.includes('github.com')) return { label: 'GitHub', url, color: 'text-slate-300 hover:text-slate-200' };
    if (url.includes('avd.aquasec.com')) return { label: 'Aqua', url, color: 'text-cyan-400 hover:text-cyan-300' };
    if (url.includes('lists.apache.org') || url.includes('apache.org')) return { label: 'Apache', url, color: 'text-red-400 hover:text-red-300' };
    if (url.includes('bugs.chromium.org')) return { label: 'Chromium', url, color: 'text-green-400 hover:text-green-300' };
    if (url.includes('alpinelinux.org')) return { label: 'Alpine', url, color: 'text-blue-400 hover:text-blue-300' };
    try {
      const domain = new URL(url).hostname.replace('www.', '');
      return { label: domain, url, color: 'text-slate-400 hover:text-slate-300' };
    } catch {
      return { label: 'Link', url, color: 'text-slate-400 hover:text-slate-300' };
    }
  }
</script>

<div class="space-y-4">
  <!-- Filter chips -->
  <div class="flex flex-wrap items-center gap-2 mb-3">
    {#each availableSeverities as severity}
      {@const colors = severityColors[severity]}
      {@const active = enabledSeverities.has(severity)}
      <button
        onclick={() => toggleSeverityFilter(severity)}
        class="px-2.5 py-1 rounded-md text-xs font-semibold transition-colors cursor-pointer {active ? colors.badge : colors.badgeOff + ' line-through'}"
      >
        {severity}: {result.severityCounts[severity]}
      </button>
    {/each}
    <span class="px-2.5 py-1 rounded-md text-xs font-semibold bg-slate-600/30 text-slate-300">
      {#if filteredTotal === result.totalCount}
        Total: {result.totalCount}
      {:else}
        Showing: {filteredTotal} / {result.totalCount}
      {/if}
    </span>
  </div>

  <!-- Severity groups -->
  {#each visibleSeverities as severity}
    {@const vulns = filteredBySeverity[severity]}
    {@const colors = severityColors[severity]}
    {@const allVulns = result.bySeverity[severity] || []}
    {@const stats = groupStats(allVulns)}
    {@const sectionOpen = isSectionOpen(severity)}
    {@const eff = effectiveFilter(severity)}
    {#if vulns}
    <div class="border {colors.border} rounded-lg overflow-hidden">
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="px-4 py-2.5 {colors.bg} flex items-center justify-between cursor-pointer"
        onclick={() => toggleSection(severity)}
      >
        <div class="flex items-center gap-2">
          <svg
            class="w-4 h-4 {colors.text} transition-transform flex-shrink-0"
            style:transform={sectionOpen ? 'rotate(90deg)' : 'rotate(0deg)'}
            fill="none" stroke="currentColor" viewBox="0 0 24 24"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"></path>
          </svg>
          <span class="font-semibold {colors.text}">{severity}</span>
          <span class="text-xs text-slate-400">({vulns.length}{#if vulns.length !== allVulns.length}/{allVulns.length}{/if})</span>
        </div>
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="flex gap-1.5" onclick={(e: MouseEvent) => e.stopPropagation()}>
          {#if stats.fixable > 0}
            <button
              onclick={() => toggleSectionFilter(severity, 'fixable')}
              class="px-2 py-0.5 text-xs rounded cursor-pointer transition-colors {eff === 'fixable' ? 'bg-green-500/30 text-green-300 ring-1 ring-green-500/50' : 'bg-green-500/15 text-green-400'}"
            >{stats.fixable} fixable</button>
          {/if}
          {#if stats.noFix > 0}
            <button
              onclick={() => toggleSectionFilter(severity, 'nofix')}
              class="px-2 py-0.5 text-xs rounded cursor-pointer transition-colors {eff === 'nofix' ? 'bg-slate-500/30 text-slate-200 ring-1 ring-slate-400/50' : 'bg-slate-500/15 text-slate-400'}"
            >{stats.noFix} no fix</button>
          {/if}
          {#if stats.vexed > 0}
            <button
              onclick={() => toggleSectionFilter(severity, 'vexed')}
              class="px-2 py-0.5 text-xs rounded cursor-pointer transition-colors {eff === 'vexed' ? 'bg-purple-500/30 text-purple-300 ring-1 ring-purple-500/50' : 'bg-purple-500/15 text-purple-400'}"
            >{stats.vexed} VEXed</button>
          {/if}
        </div>
      </div>
      {#if sectionOpen}
        <div class="divide-y divide-slate-700/50">
          {#each vulns as vuln}
            {@const vexBadge = getVexBadge(vuln)}
            {@const expanded = isVulnOpen(vuln.vulnerabilityID + vuln.pkgName)}
            <div class="px-4 py-2.5 hover:bg-slate-800/50 transition-colors">
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <div
                class="flex items-center justify-between cursor-pointer gap-3"
                onclick={() => toggleVuln(vuln.vulnerabilityID + vuln.pkgName)}
              >
                <div class="flex-1 min-w-0">
                  <div class="flex items-center gap-2">
                    <a
                      href={vuln.primaryURL}
                      target="_blank"
                      rel="noopener noreferrer"
                      class="font-mono text-sm {colors.text} hover:underline flex-shrink-0"
                      onclick={(e: MouseEvent) => e.stopPropagation()}
                    >
                      {vuln.vulnerabilityID}
                    </a>
                    {#if vuln.cvssScore}
                      <span class="px-1.5 py-0.5 text-[10px] rounded font-mono bg-slate-600/30 text-slate-300">{vuln.cvssScore.toFixed(1)}</span>
                    {/if}
                    <span class="text-xs text-slate-400 font-mono truncate">{vuln.pkgName}</span>
                  </div>
                  {#if vuln.title}
                    <p class="text-xs text-slate-500 mt-0.5 truncate">{vuln.title}</p>
                  {/if}
                </div>
                <!-- Badges on the right -->
                <div class="flex items-center gap-1.5 flex-shrink-0">
                  {#if vexBadge}
                    <span class="px-1.5 py-0.5 text-xs rounded {vexBadge.class}">{vexBadge.label}</span>
                  {/if}
                  {#if vuln.fixedVersion}
                    <span class="px-1.5 py-0.5 text-xs rounded bg-green-500/15 text-green-400">fix: {vuln.fixedVersion}</span>
                  {/if}
                  <svg
                    class="w-4 h-4 text-slate-500 transition-transform ml-1"
                    style:transform={expanded ? 'rotate(90deg)' : 'rotate(0deg)'}
                    fill="none" stroke="currentColor" viewBox="0 0 24 24"
                  >
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"></path>
                  </svg>
                </div>
              </div>
              {#if expanded}
                <div class="mt-2 ml-2 p-3 bg-slate-900/50 rounded-md text-xs space-y-3">
                  <!-- CVSS scores header -->
                  {#if vuln.cvssSources?.length}
                    <div class="flex flex-wrap gap-2 pb-2 border-b border-slate-700/50">
                      {#each vuln.cvssSources as src}
                        <div class="flex items-center gap-1.5 px-2 py-1 rounded bg-slate-800/80 border border-slate-700/50">
                          <span class="text-slate-400 capitalize">{src.source}</span>
                          {#if src.v3Score}
                            <span class="font-mono font-semibold {src.v3Score >= 9 ? 'text-red-400' : src.v3Score >= 7 ? 'text-orange-400' : src.v3Score >= 4 ? 'text-yellow-400' : 'text-blue-400'}">{src.v3Score.toFixed(1)}</span>
                          {:else if src.v2Score}
                            <span class="font-mono font-semibold text-slate-300">{src.v2Score.toFixed(1)}</span>
                            <span class="text-slate-500">v2</span>
                          {/if}
                        </div>
                      {/each}
                    </div>
                  {/if}
                  <!-- Package metadata -->
                  <div class="grid grid-cols-2 gap-2 text-slate-400">
                    <div><span class="text-slate-500">Package:</span> {vuln.pkgName}</div>
                    <div><span class="text-slate-500">Installed:</span> <span class="font-mono">{vuln.installedVersion}</span></div>
                    <div><span class="text-slate-500">Target:</span> {vuln.target}{#if vuln.targets && vuln.targets.length > 1} <span class="text-slate-500">(+{vuln.targets.length - 1} more)</span>{/if}</div>
                    {#if vuln.fixedVersion}
                      <div><span class="text-slate-500">Fixed in:</span> <span class="font-mono text-green-400">{vuln.fixedVersion}</span></div>
                    {/if}
                  </div>
                  <!-- Description + References: side by side on large screens -->
                  <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 pt-2 border-t border-slate-700/50">
                    {#if vuln.description}
                      <div>
                        <div class="text-slate-500 mb-1.5">Description:</div>
                        <p class="text-slate-300 leading-relaxed whitespace-pre-line">{vuln.description}</p>
                      </div>
                    {/if}
                    {#if (vuln.primaryURL || vuln.references?.length)}
                      <div>
                        <div class="text-slate-500 mb-1.5">References:</div>
                        <div class="flex flex-col gap-0.5">
                          {#each categorizeRefs(vuln) as ref}
                            <a
                              href={ref.url}
                              target="_blank"
                              rel="noopener noreferrer"
                              class="text-blue-400 hover:text-blue-300 hover:underline text-xs font-mono truncate"
                            >{ref.url}</a>
                          {/each}
                        </div>
                      </div>
                    {/if}
                  </div>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      {/if}
    </div>
    {/if}
  {/each}

  <!-- Scan metadata -->
  <div class="text-xs text-slate-500 mt-2">
    Scanned {result.artifactName} at {new Date(result.scanTime).toLocaleString()}
    ({result.targets.length} {result.targets.length === 1 ? 'target' : 'targets'} analyzed)
  </div>
</div>
