<script lang="ts">
  import type { ImageInfo } from '../lib/types';
  import { formatBytes, truncateDigest, computeSecurityScore } from '../lib/utils';
  import SecurityScore from './SecurityScore.svelte';
  import CopyableDigest from './CopyableDigest.svelte';

  let { data }: { data: ImageInfo } = $props();

  let scoreResult = $derived(computeSecurityScore(data));
  let totalSize = $derived(data.manifest?.layers?.reduce((sum, l) => sum + l.size, 0) || 0);
  let validPlatforms = $derived(
    (data.imageIndex?.manifests || []).filter((m) => {
      if (!m.platform) return false;
      return `${m.platform.os}/${m.platform.architecture}` !== 'unknown/unknown';
    }),
  );
  let platformCount = $derived(validPlatforms.length || 1);
  let layerCount = $derived(data.manifest?.layers?.length || 0);
  let referrerCount = $derived(data.referrers?.length || 0);
  let tagCount = $derived(data.tags?.length || 0);

  let showScoreDetails = $state(false);
</script>

<div class="bg-slate-800 border border-slate-700 rounded-lg p-5 mb-6 fade-in overflow-hidden">
  <div class="flex flex-col md:flex-row md:items-start gap-6 md:gap-8">
    <!-- Security Score -->
    <div class="flex-shrink-0 md:border-r border-b md:border-b-0 border-slate-700 pb-4 md:pb-0 md:pr-8">
      <SecurityScore score={scoreResult} bind:showDetails={showScoreDetails} />
    </div>
    <!-- Image Info -->
    <div class="flex-1 min-w-0">
      <div class="flex items-start justify-between flex-wrap gap-4">
        <div class="min-w-0">
          <div class="flex items-start gap-3 min-w-0">
            <svg class="w-6 h-6 text-blue-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"></path>
            </svg>
            <h2 class="text-lg font-semibold text-slate-100 break-all">{data.repository}</h2>
          </div>
          <div class="flex items-center gap-2 mt-2 flex-wrap">
            {#each (data.tags || []).slice(0, 5) as tag}
              <span class="px-2 py-1 rounded text-xs font-mono {tag === data.tag ? 'bg-blue-500/30 text-blue-200 ring-1 ring-blue-500/50' : 'bg-blue-500/20 text-blue-300'}">{tag}</span>
            {/each}
            {#if (data.tags?.length || 0) > 5}
              <span class="text-xs text-slate-500">+{data.tags.length - 5} more</span>
            {/if}
          </div>
        </div>
        <div class="text-right">
          <CopyableDigest digest={data.digest} label={truncateDigest(data.digest, 16)} classes="text-sm" />
          <div class="flex items-center gap-2 text-sm text-slate-500 mt-1">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
            <span>{data.created ? new Date(data.created).toLocaleString() : 'Unknown'}</span>
          </div>
        </div>
      </div>

      <!-- Quick Stats -->
      <div class="grid grid-cols-2 md:grid-cols-5 gap-4 mt-4 pt-4 border-t border-slate-700">
        <div class="text-center">
          <div class="text-2xl font-bold text-amber-400">{formatBytes(totalSize)}</div>
          <div class="text-xs text-slate-500">Total Size</div>
        </div>
        <div class="text-center">
          <div class="text-2xl font-bold text-purple-400">{platformCount}</div>
          <div class="text-xs text-slate-500">Platforms</div>
        </div>
        <div class="text-center">
          <div class="text-2xl font-bold text-emerald-400">{layerCount}</div>
          <div class="text-xs text-slate-500">Layers</div>
        </div>
        <div class="text-center">
          <div class="text-2xl font-bold text-cyan-400">{referrerCount}</div>
          <div class="text-xs text-slate-500">Referrers</div>
        </div>
        <div class="text-center">
          <div class="text-2xl font-bold text-blue-400">{tagCount}</div>
          <div class="text-xs text-slate-500">Tags</div>
        </div>
      </div>
    </div>
  </div>
</div>

{#if showScoreDetails}
  <div class="bg-slate-800/50 border border-slate-700 rounded-lg p-4 mb-6 fade-in">
    <div class="flex items-center justify-between mb-3">
      <div class="flex items-center gap-2">
        <span class="text-sm font-semibold text-slate-200">Security Score Breakdown</span>
        <span class="px-2 py-0.5 text-xs rounded-full font-bold" style:background="{scoreResult.color}20" style:color={scoreResult.color}>{scoreResult.grade} ({scoreResult.score}/{scoreResult.maxScore})</span>
      </div>
      <button onclick={() => (showScoreDetails = false)} class="text-slate-400 hover:text-slate-200 text-xs">Close</button>
    </div>
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
      {#each scoreResult.criteria as c}
        <div class="flex items-start gap-3 p-3 rounded-lg border {c.present ? 'border-green-500/30 bg-green-500/5' : 'border-red-500/30 bg-red-500/5'}">
          {#if c.present}
            <svg class="w-4 h-4 text-green-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>
          {:else}
            <svg class="w-4 h-4 text-red-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
          {/if}
          <div>
            <div class="text-sm font-semibold {c.present ? 'text-green-300' : 'text-red-300'}">{c.label}</div>
            <div class="text-xs text-slate-400">{c.desc}</div>
            {#if c.key === 'minimalBase' && scoreResult.minimalBaseDetails}
              {@const details = scoreResult.minimalBaseDetails}
              {@const items = [
                { pass: details.fewLayers, label: 'Few layers (\u2264 5)' },
                { pass: details.smallSize, label: 'Small size (\u2264 50 MB)' },
                { pass: details.nonRoot, label: 'Non-root user' },
                { pass: details.noShellEntrypoint, label: 'No shell entrypoint' },
              ]}
              <div class="mt-2 pt-2 border-t border-slate-700 text-xs text-slate-400 space-y-1">
                <div class="font-semibold text-slate-300 mb-1">Minimal Base Breakdown:</div>
                {#each items as item}
                  <div class="flex items-center gap-1.5">
                    {#if item.pass}
                      <svg class="w-4 h-4 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>
                    {:else}
                      <svg class="w-4 h-4 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
                    {/if}
                    <span>{item.label}</span>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  </div>
{/if}
