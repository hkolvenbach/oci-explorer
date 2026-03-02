<script lang="ts">
  import type { Descriptor } from '../lib/types';
  import { formatBytes } from '../lib/utils';
  import CollapsibleSection from './CollapsibleSection.svelte';
  import CopyableDigest from './CopyableDigest.svelte';

  let { layers }: { layers: Descriptor[] } = $props();

  let expanded = $state<Record<number, boolean>>({});

  function toggleExpand(i: number) {
    expanded[i] = !expanded[i];
  }
</script>

{#snippet icon()}
  <svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"></path>
  </svg>
{/snippet}

<CollapsibleSection id="layers" title="Layers" badge="{layers.length} layers" {icon}>
  <div class="space-y-2">
    {#each layers as l, i}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div
        class="border rounded-lg p-3 hover:border-slate-500 transition-all cursor-pointer overflow-hidden {expanded[i] ? 'border-slate-500 bg-slate-700/50' : 'border-slate-700 bg-slate-800/30'}"
        onclick={() => toggleExpand(i)}
      >
        <div class="flex items-start justify-between gap-2">
          <div class="flex items-start gap-3 min-w-0">
            <div class="w-8 h-8 rounded-md flex items-center justify-center text-sm font-bold bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 flex-shrink-0">{i}</div>
            <div class="min-w-0">
              <div class="text-sm font-mono text-slate-300 {expanded[i] ? 'break-all' : 'truncate'}">{l.digest}</div>
              <div class="text-xs text-slate-500">{l.mediaType?.split('.').pop()?.replace('+', ' ') || ''}</div>
            </div>
          </div>
          <div class="text-right flex-shrink-0">
            <div class="text-sm font-medium text-slate-300 whitespace-nowrap">{formatBytes(l.size)}</div>
          </div>
        </div>
        {#if expanded[i]}
          <div class="mt-3 pt-3 border-t border-slate-700 text-sm space-y-2">
            <div class="flex items-start gap-2">
              <span class="text-slate-500 min-w-24">Media Type:</span>
              <code class="text-xs bg-slate-900 px-2 py-1 rounded text-slate-300 break-all">{l.mediaType}</code>
            </div>
            <div class="flex items-start gap-2">
              <span class="text-slate-500 min-w-24">Digest:</span>
              <CopyableDigest digest={l.digest} />
            </div>
            {#if l.annotations}
              <div class="mt-2">
                <span class="text-slate-500 text-xs">Annotations:</span>
                <div class="mt-1 space-y-1">
                  {#each Object.entries(l.annotations) as [k, v]}
                    <div class="flex items-start gap-2 pl-2">
                      <span class="text-xs text-slate-500">{k.split('.').pop()}:</span>
                      <span class="text-xs text-slate-400">{v}</span>
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        {/if}
      </div>
    {/each}
  </div>
</CollapsibleSection>
