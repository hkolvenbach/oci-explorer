<script lang="ts">
  import type { Snippet } from 'svelte';
  import { appState } from '../lib/state.svelte';

  let {
    id,
    title,
    badge = '',
    defaultOpen = true,
    children,
    headerRight,
    icon,
  }: {
    id: string;
    title: string;
    badge?: string;
    defaultOpen?: boolean;
    children: Snippet;
    headerRight?: Snippet;
    icon?: Snippet;
  } = $props();

  let isOpen = $derived(appState.collapseStates[id] ?? defaultOpen);

  function toggle() {
    appState.collapseStates[id] = !isOpen;
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      toggle();
    }
  }
</script>

<div class="border border-slate-700 rounded-lg overflow-hidden bg-slate-800/50 mb-4 fade-in">
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <div class="flex items-center justify-between bg-slate-800 hover:bg-slate-700 transition-colors cursor-pointer" role="button" tabindex="0" onclick={toggle} onkeydown={onKeydown}>
    <div class="flex-1 px-4 py-3 flex items-center justify-between">
      <div class="flex items-center gap-3">
        {#if icon}
          {@render icon()}
        {/if}
        <span class="font-semibold text-slate-100">{title}</span>
        {#if badge}
          <span class="px-2 py-0.5 text-xs bg-blue-500/20 text-blue-300 rounded-full">{badge}</span>
        {/if}
      </div>
      <svg
        class="w-5 h-5 text-slate-400 transition-transform"
        style:transform={isOpen ? 'rotate(90deg)' : 'rotate(0deg)'}
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
      >
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"></path>
      </svg>
    </div>
    {#if headerRight}
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div onclick={(e: MouseEvent) => e.stopPropagation()}>
        {@render headerRight()}
      </div>
    {/if}
  </div>
  {#if isOpen}
    <div class="p-4">
      {@render children()}
    </div>
  {/if}
</div>
