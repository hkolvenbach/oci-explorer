<script lang="ts">
  import { appState } from '../lib/state.svelte';
  import type { MatchingTagsResult } from '../lib/types';
  import CollapsibleSection from './CollapsibleSection.svelte';

  let {
    tags,
    matchingTags,
    oninspect,
  }: {
    tags: string[];
    matchingTags: MatchingTagsResult | null;
    oninspect: (image: string) => void;
  } = $props();

  let currentTag = $derived(appState.currentData?.tag || '');
  let displayTags = $derived(matchingTags?.tags?.length ? matchingTags.tags : tags);
  let badgeText = $derived(matchingTags?.tags?.length ? `${matchingTags.tags.length} matching` : `${tags.length} tags`);
</script>

{#snippet icon()}
  <svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z"></path>
  </svg>
{/snippet}

<CollapsibleSection id="tags" title="Tags" badge={badgeText} {icon}>
  {#if matchingTags?.note}
    <div class="flex items-start gap-3 p-3 mb-3 rounded-lg bg-amber-500/10 border border-amber-500/30">
      <svg class="w-5 h-5 text-amber-400 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.072 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
      </svg>
      <span class="text-sm text-amber-300">{matchingTags.note}</span>
    </div>
  {/if}

  <div class="flex flex-wrap gap-2 max-h-48 overflow-y-auto scrollbar-thin">
    {#each displayTags as tag}
      <button
        onclick={() => oninspect(appState.currentData?.repository + ':' + tag)}
        class="px-3 py-2 bg-slate-800 border rounded-lg hover:border-blue-500 cursor-pointer transition-colors {tag === currentTag ? 'border-blue-500 bg-blue-500/20' : 'border-slate-700'}"
      >
        <span class="font-mono text-sm {tag === currentTag ? 'text-blue-300 font-semibold' : 'text-slate-300'}">{tag}</span>
      </button>
    {/each}
  </div>
</CollapsibleSection>
