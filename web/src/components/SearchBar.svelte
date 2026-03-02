<script lang="ts">
  import { appState } from '../lib/state.svelte';

  let { oninspect }: { oninspect: () => void } = $props();

  function handleKeypress(e: KeyboardEvent) {
    if (e.key === 'Enter') oninspect();
  }
</script>

<div class="mt-4 flex gap-2">
  <div class="flex-1 relative">
    <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
    </svg>
    <input
      type="text"
      bind:value={appState.searchQuery}
      placeholder="Enter image reference (e.g., nginx:latest, ghcr.io/org/image:tag)"
      class="w-full pl-10 pr-4 py-3 bg-slate-900 border border-slate-700 rounded-lg text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500 font-mono text-sm"
      onkeypress={handleKeypress}
    />
  </div>
  <button
    onclick={oninspect}
    disabled={appState.isLoading}
    class="px-6 py-3 bg-blue-600 hover:bg-blue-500 disabled:bg-blue-800 rounded-lg font-semibold transition-colors flex items-center gap-2"
  >
    {#if appState.isLoading}
      <svg class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
      <span>Loading...</span>
    {:else}
      <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
      </svg>
      <span>Inspect</span>
    {/if}
  </button>
</div>
