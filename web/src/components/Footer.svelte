<script lang="ts">
  import { appState } from '../lib/state.svelte';
</script>

<footer class="border-t border-slate-800 py-4 mt-8">
  <div class="max-w-7xl mx-auto px-4 text-center text-sm text-slate-500 space-y-1">
    <div>
      {#if appState.version}<span class="text-slate-600">{appState.version}</span> |&nbsp;{/if}<a href="/docs/" class="text-slate-400 hover:text-slate-300">API Documentation</a>
    </div>
    <div class="text-slate-600">
      Free &amp; open-source. Provided as-is, no warranty. <a href="https://github.com/hkolvenbach/oci-explorer" class="text-slate-500 hover:text-slate-400" target="_blank" rel="noopener noreferrer">GitHub</a>
    </div>
  </div>
</footer>
