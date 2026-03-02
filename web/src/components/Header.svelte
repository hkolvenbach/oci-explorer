<script lang="ts">
  import { appState, type ViewMode } from '../lib/state.svelte';
  import SearchBar from './SearchBar.svelte';

  let { oninspect }: { oninspect: () => void } = $props();

  function setView(view: ViewMode) {
    appState.currentView = view;
  }
</script>

<header class="bg-slate-800 border-b border-slate-700 sticky top-0 z-10">
  <div class="max-w-7xl mx-auto px-4 py-4">
    <div class="flex items-center justify-between flex-wrap gap-4">
      <a href="/" class="flex items-center gap-3 hover:opacity-80 transition-opacity" onclick={(e) => { e.preventDefault(); appState.currentData = null; appState.error = ''; appState.searchQuery = 'alpine:latest'; history.pushState(null, '', '/'); }}>
        <div class="w-10 h-10 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600 flex items-center justify-center">
          <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"></path>
          </svg>
        </div>
        <div>
          <h1 class="text-xl font-bold">OCI Image Explorer</h1>
          <p class="text-xs text-slate-400">Visualize container image structures</p>
        </div>
      </a>
      <div class="flex items-center gap-2">
        <button
          onclick={() => setView('details')}
          class="px-3 py-2 rounded-lg flex items-center gap-2 text-sm {appState.currentView === 'details' ? 'bg-blue-500/20 text-blue-300' : 'text-slate-400 hover:text-slate-200'}"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"></path>
          </svg>
          Details
        </button>
        <button
          onclick={() => setView('graph')}
          class="px-3 py-2 rounded-lg flex items-center gap-2 text-sm {appState.currentView === 'graph' ? 'bg-blue-500/20 text-blue-300' : 'text-slate-400 hover:text-slate-200'}"
        >
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z"></path>
          </svg>
          Graph
        </button>
      </div>
    </div>
    <SearchBar {oninspect} />
  </div>
</header>
