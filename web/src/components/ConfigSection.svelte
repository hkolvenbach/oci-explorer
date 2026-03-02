<script lang="ts">
  import type { ImageConfig } from '../lib/types';
  import { appState } from '../lib/state.svelte';
  import CollapsibleSection from './CollapsibleSection.svelte';
  import JsonToggleButton from './JsonToggleButton.svelte';

  let { config, platformName = null }: { config: ImageConfig; platformName?: string | null } = $props();

  function toggleJsonView(e: MouseEvent) {
    e.stopPropagation();
    appState.viewToggles.config = !appState.viewToggles.config;
  }
</script>

{#snippet icon()}
  <svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4"></path>
  </svg>
{/snippet}

{#snippet headerRight()}
  <JsonToggleButton active={appState.viewToggles.config} onclick={toggleJsonView} />
{/snippet}

<CollapsibleSection
  id="config"
  title="Configuration"
  badge={platformName || ''}
  {icon}
  {headerRight}
>
  {#if appState.viewToggles.config}
    <pre class="bg-slate-900 p-4 rounded-lg overflow-auto max-h-96 scrollbar-thin text-xs font-mono text-slate-300">{JSON.stringify(config, null, 2)}</pre>
  {:else}
    <!-- Architecture / OS / Created / Author grid -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
      <div class="bg-slate-800/50 rounded-lg p-3">
        <div class="text-xs text-slate-500">Architecture</div>
        <div class="text-sm text-slate-200 font-mono mt-1">{config.architecture || 'N/A'}</div>
      </div>
      <div class="bg-slate-800/50 rounded-lg p-3">
        <div class="text-xs text-slate-500">OS</div>
        <div class="text-sm text-slate-200 font-mono mt-1">{config.os || 'N/A'}</div>
      </div>
      <div class="bg-slate-800/50 rounded-lg p-3">
        <div class="text-xs text-slate-500">Created</div>
        <div class="text-sm text-slate-200 mt-1">{config.created ? new Date(config.created).toLocaleDateString() : 'N/A'}</div>
      </div>
      <div class="bg-slate-800/50 rounded-lg p-3">
        <div class="text-xs text-slate-500">Author</div>
        <div class="text-sm text-slate-200 mt-1 truncate">{config.author || 'N/A'}</div>
      </div>
    </div>

    {#if config.config}
      <div class="bg-slate-800/50 rounded-lg p-4 mb-4">
        <div class="text-sm font-semibold text-slate-300 mb-3">Runtime Configuration</div>
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
          {#if config.config.User}
            <div><span class="text-slate-500">User:</span><span class="text-slate-300 ml-2 font-mono">{config.config.User}</span></div>
          {/if}
          {#if config.config.WorkingDir}
            <div><span class="text-slate-500">Working Dir:</span><span class="text-slate-300 ml-2 font-mono">{config.config.WorkingDir}</span></div>
          {/if}
          {#if config.config.Entrypoint?.length}
            <div class="col-span-2">
              <span class="text-slate-500">Entrypoint:</span>
              <code class="text-xs bg-slate-900 px-2 py-1 rounded text-emerald-300 ml-2">{config.config.Entrypoint.join(' ')}</code>
            </div>
          {/if}
          {#if config.config.Cmd?.length}
            <div class="col-span-2">
              <span class="text-slate-500">Cmd:</span>
              <code class="text-xs bg-slate-900 px-2 py-1 rounded text-blue-300 ml-2">{config.config.Cmd.join(' ')}</code>
            </div>
          {/if}
          {#if config.config.ExposedPorts && Object.keys(config.config.ExposedPorts).length}
            <div class="col-span-2">
              <span class="text-slate-500">Exposed Ports:</span>
              <div class="flex gap-2 mt-1 flex-wrap">
                {#each Object.keys(config.config.ExposedPorts) as port}
                  <span class="text-xs bg-amber-500/20 text-amber-300 px-2 py-1 rounded">{port}</span>
                {/each}
              </div>
            </div>
          {/if}
          {#if config.config.Env?.length}
            <div class="col-span-2">
              <span class="text-slate-500">Environment:</span>
              <div class="mt-1 space-y-1 max-h-32 overflow-y-auto scrollbar-thin">
                {#each config.config.Env as env}
                  <code class="block text-xs bg-slate-900 px-2 py-1 rounded text-slate-400">{env}</code>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      </div>
    {/if}

    {#if config.history?.length}
      <div class="bg-slate-800/50 rounded-lg p-4">
        <div class="text-sm font-semibold text-slate-300 mb-3">Build History</div>
        <div class="space-y-2 max-h-64 overflow-y-auto scrollbar-thin">
          {#each config.history as entry, i}
            <div class="flex items-start gap-3 text-sm">
              <div class="w-6 h-6 rounded-full bg-slate-700 flex items-center justify-center text-xs text-slate-400 flex-shrink-0">{i + 1}</div>
              <div class="flex-1 min-w-0">
                <code class="text-xs text-slate-300 break-all">{entry.created_by || 'Unknown'}</code>
                <div class="text-xs text-slate-500 mt-0.5">
                  {entry.created ? new Date(entry.created).toLocaleString() : ''}
                  {#if entry.empty_layer}
                    <span class="ml-2 text-slate-600">(empty layer)</span>
                  {/if}
                </div>
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  {/if}
</CollapsibleSection>
