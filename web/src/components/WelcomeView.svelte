<script lang="ts">
  let { oninspect }: { oninspect: (image: string) => void } = $props();
  const examples = [
    'alpine:latest',
    'nginx:latest',
    'python:3.12-slim',
    'golang:1.21',
    'ghcr.io/hkolvenbach/oci-explorer:latest',
  ];
</script>

<div class="text-center py-20">
  <div class="w-20 h-20 mx-auto rounded-2xl bg-gradient-to-br from-blue-500/20 to-purple-600/20 border border-slate-700 flex items-center justify-center mb-6">
    <svg class="w-10 h-10 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4"></path>
    </svg>
  </div>
  <h2 class="text-2xl font-bold text-slate-200 mb-2">Welcome to OCI Image Explorer</h2>
  <p class="text-slate-400 max-w-md mx-auto mb-6">
    Enter a container image reference above to visualize its OCI structure, including layers, manifests, configurations, and supply chain artifacts.
  </p>
  <div class="flex flex-wrap justify-center gap-2">
    {#each examples as image}
      <button
        onclick={() => oninspect(image)}
        class="px-3 py-2 bg-slate-800 hover:bg-slate-700 border border-slate-700 rounded-lg text-sm font-mono"
      >
        {image}
      </button>
    {/each}
  </div>
</div>
