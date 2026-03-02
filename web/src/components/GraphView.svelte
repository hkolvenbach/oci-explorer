<script lang="ts">
  import type { ImageInfo, Referrer } from '../lib/types';
  import { graphState, resetGraphState } from '../lib/state.svelte';
  import { truncateDigest, downloadBlob } from '../lib/utils';
  import { referrerTypeGraph } from '../lib/constants';
  import * as api from '../lib/api';

  let { data }: { data: ImageInfo } = $props();

  // Reset pan/zoom when data changes
  $effect(() => {
    data; // track dependency
    resetGraphState();
  });

  // Layout calculations
  let layout = $derived.by(() => {
    const allPlatforms = data.imageIndex?.manifests || [];
    const platforms = allPlatforms.filter(
      (m) => m.platform && !(m.platform.os === 'unknown' && m.platform.architecture === 'unknown'),
    );
    const layers = data.manifest?.layers || [];
    const referrers = data.referrers || [];

    // Group referrers by their target platform digest
    const referrersByPlatform: Record<string, Referrer[]> = {};
    const globalReferrers: Referrer[] = [];
    const platformDigests = new Set(platforms.map((p) => p.digest));

    for (const r of referrers) {
      const refDigest = r.annotations?.['vnd.docker.reference.digest'];
      if (refDigest && platformDigests.has(refDigest)) {
        if (!referrersByPlatform[refDigest]) referrersByPlatform[refDigest] = [];
        referrersByPlatform[refDigest].push(r);
      } else {
        globalReferrers.push(r);
      }
    }

    const platformCount = Math.max(platforms.length, 1);
    const platformWidth = 150;
    const platformSpacing = 280;
    const totalWidth = platformCount * platformSpacing;
    const startX = Math.max(50, (900 - totalWidth + platformSpacing - platformWidth) / 2);

    let maxReferrersPerPlatform = 0;
    for (const p of platforms) {
      const count = (referrersByPlatform[p.digest] || []).length;
      if (count > maxReferrersPerPlatform) maxReferrersPerPlatform = count;
    }

    const referrerRowHeight = 48;
    const configNodeHeight = 70;
    const baseHeight = 400;
    const extraHeight = Math.max(0, maxReferrersPerPlatform * referrerRowHeight) + configNodeHeight;
    const svgHeight = baseHeight + extraHeight + 50;
    const svgWidth = Math.max(900, totalWidth + 200 + (globalReferrers.length > 0 ? 150 : 0));

    const configLayersY = Math.max(205 + maxReferrersPerPlatform * referrerRowHeight + 20, 260);
    const firstPlatformX = platforms.length > 0 ? startX : 375;

    const globalRefStartY = 90;
    const globalRefX = Math.min(svgWidth - 170, startX + platformCount * platformSpacing + 30);

    return {
      platforms,
      layers,
      referrers,
      referrersByPlatform,
      globalReferrers,
      platformWidth,
      platformSpacing,
      startX,
      maxReferrersPerPlatform,
      referrerRowHeight,
      svgHeight,
      svgWidth,
      configLayersY,
      firstPlatformX,
      globalRefStartY,
      globalRefX,
    };
  });

  function getColor(type: string) {
    return referrerTypeGraph[type] || referrerTypeGraph.artifact;
  }

  // Pan/zoom handlers
  let svgEl: SVGSVGElement;

  function onWheel(e: WheelEvent) {
    e.preventDefault();
    const delta = e.deltaY > 0 ? -0.03 : 0.03;
    graphState.zoom = Math.max(0.25, Math.min(3, graphState.zoom + delta));
  }

  function onMouseDown(e: MouseEvent) {
    if (e.button === 0) {
      graphState.isPanning = true;
      graphState.panStartX = e.clientX - graphState.panX;
      graphState.panStartY = e.clientY - graphState.panY;
    }
  }

  function onMouseMove(e: MouseEvent) {
    if (graphState.isPanning) {
      graphState.panX = e.clientX - graphState.panStartX;
      graphState.panY = e.clientY - graphState.panStartY;
    }
  }

  function onMouseUp() {
    graphState.isPanning = false;
  }

  let lastTouchDist = 0;

  function onTouchStart(e: TouchEvent) {
    if (e.touches.length === 1) {
      graphState.isPanning = true;
      graphState.panStartX = e.touches[0].clientX - graphState.panX;
      graphState.panStartY = e.touches[0].clientY - graphState.panY;
    } else if (e.touches.length === 2) {
      lastTouchDist = Math.hypot(
        e.touches[0].clientX - e.touches[1].clientX,
        e.touches[0].clientY - e.touches[1].clientY,
      );
    }
  }

  function onTouchMove(e: TouchEvent) {
    if (e.touches.length === 1 && graphState.isPanning) {
      graphState.panX = e.touches[0].clientX - graphState.panStartX;
      graphState.panY = e.touches[0].clientY - graphState.panStartY;
    } else if (e.touches.length === 2) {
      const dist = Math.hypot(
        e.touches[0].clientX - e.touches[1].clientX,
        e.touches[0].clientY - e.touches[1].clientY,
      );
      const delta = (dist - lastTouchDist) * 0.005;
      graphState.zoom = Math.max(0.25, Math.min(3, graphState.zoom + delta));
      lastTouchDist = dist;
    }
  }

  function onTouchEnd() {
    graphState.isPanning = false;
  }

  function zoom(delta: number) {
    graphState.zoom = Math.max(0.25, Math.min(3, graphState.zoom + delta));
  }

  let downloadError = $state('');

  async function handleSBOMDownload(repo: string, digest: string) {
    try {
      downloadError = '';
      const blob = await api.downloadSBOM(repo, digest);
      downloadBlob(blob, `sbom-${digest.substring(7, 19)}.json`);
    } catch (err) {
      downloadError = 'Error downloading SBOM: ' + (err as Error).message;
    }
  }
</script>

<svelte:window onmousemove={onMouseMove} onmouseup={onMouseUp} />

{#if downloadError}
  <div class="flex items-center justify-between gap-2 p-3 mb-4 bg-red-500/10 border border-red-500/30 rounded-lg text-xs text-red-300">
    <span>{downloadError}</span>
    <button onclick={() => (downloadError = '')} class="text-red-400 hover:text-red-200 flex-shrink-0">Dismiss</button>
  </div>
{/if}
<div class="bg-slate-800 rounded-lg overflow-hidden fade-in">
  <!-- Zoom Controls -->
  <div class="flex items-center justify-between px-4 py-2 bg-slate-700/50 border-b border-slate-600">
    <div class="flex items-center gap-2">
      <svg class="w-4 h-4 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7"></path>
      </svg>
      <span class="text-xs text-slate-400">Scroll to zoom, drag to pan</span>
    </div>
    <div class="flex items-center gap-2">
      <button onclick={() => zoom(-0.25)} class="p-1.5 rounded bg-slate-600 hover:bg-slate-500 text-slate-300 transition-colors" title="Zoom Out">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 12H4"></path></svg>
      </button>
      <span class="text-xs text-slate-300 font-mono w-12 text-center">{Math.round(graphState.zoom * 100)}%</span>
      <button onclick={() => zoom(0.25)} class="p-1.5 rounded bg-slate-600 hover:bg-slate-500 text-slate-300 transition-colors" title="Zoom In">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"></path></svg>
      </button>
      <button onclick={resetGraphState} class="p-1.5 rounded bg-slate-600 hover:bg-slate-500 text-slate-300 transition-colors ml-2" title="Reset View">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path></svg>
      </button>
    </div>
  </div>

  <!-- Graph Container -->
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="p-4 overflow-hidden" style:height="{Math.max(500, layout.svgHeight - 50)}px" onwheel={onWheel}>
    <svg
      bind:this={svgEl}
      viewBox="0 0 {layout.svgWidth} {layout.svgHeight}"
      class="w-full h-full"
      style:cursor={graphState.isPanning ? 'grabbing' : 'grab'}
      onmousedown={onMouseDown}
      ontouchstart={onTouchStart}
      ontouchmove={onTouchMove}
      ontouchend={onTouchEnd}
    >
      <g transform="translate({graphState.panX}, {graphState.panY}) scale({graphState.zoom})">
        <!-- Image Index -->
        <rect x="375" y="20" width="150" height="60" rx="8" fill="#1e293b" stroke="#3b82f6" stroke-width="2" />
        <rect x="375" y="20" width="150" height="22" rx="8" fill="#3b82f6" />
        <text x="450" y="36" text-anchor="middle" fill="white" font-size="12" font-weight="bold">Image Index</text>
        <text x="450" y="58" text-anchor="middle" fill="#94a3b8" font-size="10" font-family="monospace">{truncateDigest(data.digest, 8)}</text>

        <!-- Global referrer connection lines -->
        {#each layout.globalReferrers.slice(0, 6) as r, i}
          {@const refY = layout.globalRefStartY + i * 48}
          {@const color = getColor(r.type)}
          <line x1="525" y1="50" x2={layout.globalRefX} y2={refY + 20} stroke={color.fill} stroke-width="1.5" stroke-dasharray="4,3" opacity="0.7" />
        {/each}

        <!-- Global referrers -->
        {#if layout.globalReferrers.length > 0}
          <text x={layout.globalRefX + 60} y={layout.globalRefStartY - 15} text-anchor="middle" fill="#22d3ee" font-size="10" font-weight="bold">Global (all platforms)</text>
          {#each layout.globalReferrers.slice(0, 6) as r, i}
            {@const refY = layout.globalRefStartY + i * 48}
            {@const color = getColor(r.type)}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
            <g
              class={r.type === 'sbom' ? 'hover:opacity-80 cursor-pointer' : ''}
              onclick={r.type === 'sbom' ? () => handleSBOMDownload(data.repository, r.digest) : undefined}
              role={r.type === 'sbom' ? 'button' : undefined}
              tabindex={r.type === 'sbom' ? 0 : undefined}
            >
              <rect x={layout.globalRefX} y={refY} width="120" height="40" rx="6" fill={color.fill} fill-opacity="0.15" stroke={color.fill} stroke-width="1.5" />
              <text x={layout.globalRefX + 60} y={refY + 15} text-anchor="middle" fill={color.fill} font-size="10" font-weight="bold">{color.text}</text>
              <text x={layout.globalRefX + 60} y={refY + 30} text-anchor="middle" fill="#94a3b8" font-size="8" font-family="monospace">{truncateDigest(r.digest, 10)}</text>
            </g>
          {/each}
          {#if layout.globalReferrers.length > 6}
            <text x={layout.globalRefX + 60} y={layout.globalRefStartY + 6 * 48 + 15} text-anchor="middle" fill="#94a3b8" font-size="9">+{layout.globalReferrers.length - 6} more</text>
          {/if}
        {/if}

        <!-- Lines from Index to Platform Manifests -->
        {#if layout.platforms.length > 0}
          {#each layout.platforms as p, i}
            {@const x = layout.startX + i * layout.platformSpacing}
            <line x1="450" y1="80" x2={x + layout.platformWidth / 2} y2="130" stroke="#475569" stroke-width="2" />
          {/each}
        {:else}
          <line x1="450" y1="80" x2="450" y2="130" stroke="#475569" stroke-width="2" />
        {/if}

        <!-- Platform Manifests -->
        {#if layout.platforms.length > 0}
          {#each layout.platforms as p, i}
            {@const x = layout.startX + i * layout.platformSpacing}
            {@const platformReferrers = layout.referrersByPlatform[p.digest] || []}
            {@const refCount = platformReferrers.length}
            <rect x={x} y="130" width={layout.platformWidth} height="60" rx="8" fill="#1e293b" stroke="#8b5cf6" stroke-width="2" />
            <rect x={x} y="130" width={layout.platformWidth} height="22" rx="8" fill="#8b5cf6" />
            <text x={x + layout.platformWidth / 2} y="146" text-anchor="middle" fill="white" font-size="11" font-weight="bold">
              {p.platform?.os || 'unknown'}/{p.platform?.architecture || 'unknown'}{p.platform?.variant ? '/' + p.platform.variant : ''}
            </text>
            <text x={x + layout.platformWidth / 2} y="168" text-anchor="middle" fill="#94a3b8" font-size="9" font-family="monospace">{truncateDigest(p.digest, 8)}</text>
            {#if refCount > 0}
              <text x={x + layout.platformWidth / 2} y="182" text-anchor="middle" fill="#06b6d4" font-size="8">{refCount} referrer{refCount > 1 ? 's' : ''}</text>
            {/if}

            <!-- Platform referrer lines + boxes -->
            {#each platformReferrers as r, ri}
              {@const refX = x + 15}
              {@const refY = 205 + ri * layout.referrerRowHeight}
              {@const color = getColor(r.type)}
              <line x1={x + layout.platformWidth / 2} y1="190" x2={refX + 60} y2={refY} stroke={color.fill} stroke-width="1.5" stroke-dasharray="4,3" opacity="0.7" />
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
              <g
                class={r.type === 'sbom' ? 'hover:opacity-80 cursor-pointer' : ''}
                onclick={r.type === 'sbom' ? () => handleSBOMDownload(data.repository, r.digest) : undefined}
                role={r.type === 'sbom' ? 'button' : undefined}
                tabindex={r.type === 'sbom' ? 0 : undefined}
              >
                <rect x={refX} y={refY} width="120" height="40" rx="6" fill={color.fill} fill-opacity="0.15" stroke={color.fill} stroke-width="1.5" />
                <text x={refX + 60} y={refY + 15} text-anchor="middle" fill={color.fill} font-size="10" font-weight="bold">{color.text}</text>
                <text x={refX + 60} y={refY + 30} text-anchor="middle" fill="#94a3b8" font-size="8" font-family="monospace">{truncateDigest(r.digest, 10)}</text>
              </g>
            {/each}

            <!-- Config node per platform -->
            {@const refHeight = platformReferrers.length * layout.referrerRowHeight}
            {@const configY = Math.max(205 + refHeight + 20, layout.configLayersY)}
            {@const platformConfig = p.config || data.config}
            {@const platformLayers = platformConfig?.rootfs?.diff_ids?.length || layout.layers.length}
            <line x1={x + layout.platformWidth / 2} y1="190" x2={x + layout.platformWidth / 2} y2={configY} stroke="#475569" stroke-width="1.5" stroke-dasharray="3,2" />
            <rect x={x - 5} y={configY} width="160" height="50" rx="6" fill="#1e293b" stroke="#10b981" stroke-width="1.5" />
            <rect x={x - 5} y={configY} width="160" height="18" rx="6" fill="#10b981" />
            <text x={x + layout.platformWidth / 2} y={configY + 13} text-anchor="middle" fill="white" font-size="10" font-weight="bold">Config</text>
            <text x={x + layout.platformWidth / 2} y={configY + 32} text-anchor="middle" fill="#94a3b8" font-size="8">
              {platformConfig?.architecture || p.platform?.architecture || 'unknown'}/{platformConfig?.os || p.platform?.os || 'unknown'}
            </text>
            <text x={x + layout.platformWidth / 2} y={configY + 44} text-anchor="middle" fill="#f59e0b" font-size="8">{platformLayers} layers</text>
          {/each}
        {:else}
          <!-- Single platform -->
          {@const fpx = layout.firstPlatformX}
          {@const pw = layout.platformWidth}
          {@const cly = layout.configLayersY}
          <rect x="375" y="130" width={pw} height="60" rx="8" fill="#1e293b" stroke="#8b5cf6" stroke-width="2" />
          <rect x="375" y="130" width={pw} height="22" rx="8" fill="#8b5cf6" />
          <text x="450" y="146" text-anchor="middle" fill="white" font-size="11" font-weight="bold">{data.architecture || 'unknown'}/{data.os || 'unknown'}</text>
          <text x="450" y="168" text-anchor="middle" fill="#94a3b8" font-size="9" font-family="monospace">{truncateDigest(data.digest, 8)}</text>
          <line x1={fpx + pw / 2} y1="190" x2={fpx + pw / 2} y2={cly} stroke="#475569" stroke-width="2" />
          <line x1={fpx + pw / 2} y1={cly} x2={fpx - 20} y2={cly + 50} stroke="#475569" stroke-width="2" />
          <line x1={fpx + pw / 2} y1={cly} x2={fpx + pw + 40} y2={cly + 50} stroke="#475569" stroke-width="2" />
          <rect x={fpx - 90} y={cly + 50} width="140" height="60" rx="8" fill="#1e293b" stroke="#10b981" stroke-width="2" />
          <rect x={fpx - 90} y={cly + 50} width="140" height="22" rx="8" fill="#10b981" />
          <text x={fpx - 20} y={cly + 66} text-anchor="middle" fill="white" font-size="12" font-weight="bold">Config</text>
          <text x={fpx - 20} y={cly + 90} text-anchor="middle" fill="#94a3b8" font-size="9">{data.config?.architecture || 'unknown'}/{data.config?.os || 'unknown'}</text>
          <text x={fpx + pw + 40} y={cly + 40} fill="#94a3b8" font-size="11" font-weight="bold">Layers ({layout.layers.length})</text>
          {#each layout.layers.slice(0, 4) as l, i}
            <rect x={fpx + pw - 30} y={cly + 55 + i * 30} width="140" height="26" rx="4" fill="#f59e0b" fill-opacity={0.2 + i * 0.2} stroke="#f59e0b" />
            <text x={fpx + pw + 40} y={cly + 72 + i * 30} text-anchor="middle" fill="#fcd34d" font-size="10">
              Layer {i}{i === 0 ? ' (base)' : i === layout.layers.length - 1 ? ' (top)' : ''}
            </text>
          {/each}
          {#if layout.layers.length > 4}
            <text x={fpx + pw + 40} y={cly + 55 + 4 * 30 + 15} text-anchor="middle" fill="#94a3b8" font-size="10">+{layout.layers.length - 4} more layers</text>
          {/if}
        {/if}

        <!-- Legend -->
        <g transform="translate(30, {layout.svgHeight - 50})">
          <text x="0" y="0" fill="#94a3b8" font-size="11" font-weight="bold">Legend:</text>
          <rect x="0" y="10" width="16" height="16" rx="2" fill="#3b82f6" />
          <text x="22" y="22" fill="#94a3b8" font-size="10">Index</text>
          <rect x="70" y="10" width="16" height="16" rx="2" fill="#8b5cf6" />
          <text x="92" y="22" fill="#94a3b8" font-size="10">Manifest</text>
          <rect x="160" y="10" width="16" height="16" rx="2" fill="#10b981" />
          <text x="182" y="22" fill="#94a3b8" font-size="10">Config</text>
          <rect x="230" y="10" width="16" height="16" rx="2" fill="#f59e0b" />
          <text x="252" y="22" fill="#94a3b8" font-size="10">Layers</text>
          <rect x="300" y="10" width="16" height="16" rx="2" fill="#06b6d4" />
          <text x="322" y="22" fill="#94a3b8" font-size="10">SBOM</text>
          <rect x="370" y="10" width="16" height="16" rx="2" fill="#8b5cf6" fill-opacity="0.5" />
          <text x="392" y="22" fill="#94a3b8" font-size="10">Attestation</text>
          <rect x="470" y="10" width="16" height="16" rx="2" fill="#2dd4bf" />
          <text x="492" y="22" fill="#94a3b8" font-size="10">VEX</text>
          <rect x="530" y="10" width="16" height="16" rx="2" fill="#f59e0b" />
          <text x="552" y="22" fill="#94a3b8" font-size="10">Signature</text>
        </g>
      </g>
    </svg>
  </div>
</div>
