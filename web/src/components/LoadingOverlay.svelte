<script lang="ts">
  let { visible }: { visible: boolean } = $props();
</script>

{#if visible}
  <div class="absolute inset-0 bg-slate-900/70 flex items-center justify-center z-10 rounded-lg">
    <div class="text-center">
      <svg class="w-10 h-10 mx-auto text-blue-500 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
      <p class="mt-3 text-slate-400 text-sm">Fetching image information...</p>
    </div>
  </div>
{/if}
