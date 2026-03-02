<script lang="ts">
  import { appState, getFilteredReferrers } from '../lib/state.svelte';
  import type { ImageInfo } from '../lib/types';

  let { data }: { data: ImageInfo } = $props();

  let hasReferrers = $derived((data.referrers?.length || 0) > 0);
  let filteredCount = $derived(getFilteredReferrers(data).length);
  let totalCount = $derived(data.referrers?.length || 0);
</script>

<div class="bg-slate-800/50 border border-slate-700 rounded-lg p-4 mb-6 fade-in">
  <div class="flex items-center gap-4 flex-wrap">
    <div class="flex items-center gap-2">
      <svg class="w-5 h-5 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"></path>
      </svg>
      <span class="text-sm font-semibold text-slate-300">Select Platform:</span>
    </div>
    <div class="flex items-center gap-2 flex-wrap">
      <button
        onclick={() => (appState.selectedPlatform = 'all')}
        class="px-3 py-1.5 rounded-lg text-sm font-medium transition-colors {appState.selectedPlatform === 'all' ? 'bg-purple-500/30 text-purple-300 border border-purple-500/50' : 'bg-slate-700 text-slate-400 border border-slate-600 hover:text-slate-200'}"
      >
        All Platforms
      </button>
      {#each Object.entries(appState.platformDigestMap) as [platform, digest]}
        <button
          onclick={() => (appState.selectedPlatform = digest)}
          class="px-3 py-1.5 rounded-lg text-sm font-mono transition-colors {appState.selectedPlatform === digest ? 'bg-purple-500/30 text-purple-300 border border-purple-500/50' : 'bg-slate-700 text-slate-400 border border-slate-600 hover:text-slate-200'}"
        >
          {platform}
        </button>
      {/each}
    </div>
    <div class="text-xs text-slate-500">
      {#if hasReferrers}
        Showing {filteredCount} of {totalCount} referrers
      {:else}
        Configuration updates with platform selection
      {/if}
    </div>
  </div>
</div>
