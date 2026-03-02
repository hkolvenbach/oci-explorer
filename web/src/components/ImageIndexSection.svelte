<script lang="ts">
  import type { ImageIndex } from '../lib/types';
  import { appState } from '../lib/state.svelte';
  import { formatBytes } from '../lib/utils';
  import CollapsibleSection from './CollapsibleSection.svelte';
  import CopyableDigest from './CopyableDigest.svelte';
  import JsonToggleButton from './JsonToggleButton.svelte';

  let { imageIndex }: { imageIndex: ImageIndex } = $props();

  let validManifests = $derived.by(() => {
    let manifests = imageIndex.manifests.filter((m) => {
      if (!m.platform) return false;
      return `${m.platform.os}/${m.platform.architecture}` !== 'unknown/unknown';
    });
    if (appState.selectedPlatform !== 'all') {
      manifests = manifests.filter((m) => m.digest === appState.selectedPlatform);
    }
    return manifests;
  });

  let expanded = $state<Record<number, boolean>>({});

  function toggleExpand(i: number) {
    expanded[i] = !expanded[i];
  }

  function toggleJsonView(e: MouseEvent) {
    e.stopPropagation();
    appState.viewToggles.imageIndex = !appState.viewToggles.imageIndex;
  }
</script>

{#snippet icon()}
  <svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4"></path>
  </svg>
{/snippet}

{#snippet headerRight()}
  <JsonToggleButton active={appState.viewToggles.imageIndex} onclick={toggleJsonView} />
{/snippet}

<CollapsibleSection
  id="image-index"
  title="Image Index"
  badge="{validManifests.length} platform{validManifests.length !== 1 ? 's' : ''}{appState.selectedPlatform !== 'all' ? ' (filtered)' : ''}"
  {icon}
  {headerRight}
>
  {#if appState.viewToggles.imageIndex}
    <pre class="bg-slate-900 p-4 rounded-lg overflow-auto max-h-96 scrollbar-thin text-xs font-mono text-slate-300">{JSON.stringify(imageIndex, null, 2)}</pre>
  {:else if validManifests.length === 0}
    <div class="text-center py-6 text-slate-500">
      <p>No platforms match the selected filter</p>
    </div>
  {:else}
    <div class="space-y-2">
      {#each validManifests as m, i}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="border rounded-lg p-3 hover:border-slate-500 transition-all cursor-pointer overflow-hidden {expanded[i] ? 'border-slate-500 bg-slate-700/50' : 'border-slate-700 bg-slate-800/30'}"
          onclick={() => toggleExpand(i)}
        >
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-start gap-3 min-w-0">
              <div class="w-8 h-8 rounded-md flex items-center justify-center text-sm font-bold bg-blue-500/20 text-blue-300 border border-blue-500/30 flex-shrink-0">{i}</div>
              <div class="min-w-0">
                <div class="text-sm font-mono text-slate-300 {expanded[i] ? 'break-all' : 'truncate'}">{m.digest}</div>
                <div class="text-xs text-slate-500">
                  {m.platform ? `${m.platform.os}/${m.platform.architecture}${m.platform.variant ? '/' + m.platform.variant : ''}` : 'unknown'}
                </div>
              </div>
            </div>
            <div class="text-right flex-shrink-0">
              <div class="text-sm font-medium text-slate-300 whitespace-nowrap">{formatBytes(m.size)}</div>
            </div>
          </div>
          {#if expanded[i]}
            <div class="mt-3 pt-3 border-t border-slate-700 text-sm space-y-2">
              <div class="flex items-start gap-2">
                <span class="text-slate-500 min-w-24">Media Type:</span>
                <code class="text-xs bg-slate-900 px-2 py-1 rounded text-slate-300 break-all">{m.mediaType}</code>
              </div>
              <div class="flex items-start gap-2">
                <span class="text-slate-500 min-w-24">Digest:</span>
                <CopyableDigest digest={m.digest} />
              </div>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</CollapsibleSection>
