<script lang="ts">
  import { appState } from '../lib/state.svelte';
  import CollapsibleSection from './CollapsibleSection.svelte';
  import JsonToggleButton from './JsonToggleButton.svelte';

  let { annotations }: { annotations: Record<string, string> } = $props();

  let entries = $derived(Object.entries(annotations || {}));

  function toggleJsonView(e: MouseEvent) {
    e.stopPropagation();
    appState.viewToggles.annotations = !appState.viewToggles.annotations;
  }
</script>

{#snippet icon()}
  <svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
  </svg>
{/snippet}

{#snippet headerRight()}
  <JsonToggleButton active={appState.viewToggles.annotations} onclick={toggleJsonView} />
{/snippet}

<CollapsibleSection
  id="annotations"
  title="Annotations"
  badge={String(entries.length)}
  defaultOpen={false}
  {icon}
  {headerRight}
>
  {#if appState.viewToggles.annotations}
    <pre class="bg-slate-900 p-4 rounded-lg overflow-auto max-h-64 scrollbar-thin text-xs font-mono text-slate-300">{JSON.stringify(annotations, null, 2)}</pre>
  {:else}
    <div class="space-y-2">
      {#each entries as [k, v]}
        <div class="flex items-start gap-2 p-2 bg-slate-800/50 rounded">
          <span class="text-xs text-slate-500 min-w-32 break-all">{k.split('.').pop()}:</span>
          <span class="text-sm text-slate-300 break-all">{v}</span>
        </div>
      {/each}
    </div>
  {/if}
</CollapsibleSection>
