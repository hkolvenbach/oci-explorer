<script lang="ts">
  import { appState } from '../lib/state.svelte';
</script>

<footer class="border-t border-slate-800 py-4 mt-8">
  <div class="max-w-7xl mx-auto px-4 text-center text-sm text-slate-500">
    OCI Image Explorer - Visualize OCI Image Spec 1.1 structures{#if appState.version} | {appState.version}{/if} | <a href="/docs/" class="text-slate-400 hover:text-slate-300">API Docs</a>
  </div>
</footer>
