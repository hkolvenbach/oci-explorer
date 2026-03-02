<script lang="ts">
  let { digest, label = '', classes = '' }: { digest: string; label?: string; classes?: string } = $props();
  let showToast = $state(false);

  function copy() {
    navigator.clipboard.writeText(digest);
    showToast = true;
    setTimeout(() => (showToast = false), 1500);
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events -->
<!-- svelte-ignore a11y_no_static_element_interactions -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<code
  class="text-xs bg-slate-900 px-2 py-1 rounded text-slate-300 break-all cursor-pointer hover:text-slate-100 hover:bg-slate-800 transition-colors relative group/copy inline-flex items-center gap-1.5 {classes}"
  title="Click to copy"
  onclick={copy}
>
  {label || digest}
  <svg class="w-3.5 h-3.5 flex-shrink-0 opacity-0 group-hover/copy:opacity-100 transition-opacity" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
  </svg>
  {#if showToast}
    <span class="absolute top-full mt-1 left-1/2 -translate-x-1/2 px-2 py-1 bg-slate-700 text-xs text-green-400 rounded whitespace-nowrap z-50 shadow-lg">Copied!</span>
  {/if}
</code>
