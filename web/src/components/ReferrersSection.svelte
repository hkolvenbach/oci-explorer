<script lang="ts">
  import type { Referrer, VEXDocument } from '../lib/types';
  import { appState } from '../lib/state.svelte';
  import { formatBytes, downloadBlob } from '../lib/utils';
  import { referrerTypeClasses } from '../lib/constants';
  import * as api from '../lib/api';
  import CollapsibleSection from './CollapsibleSection.svelte';
  import CopyableDigest from './CopyableDigest.svelte';
  import VEXViewer from './VEXViewer.svelte';

  let { referrers, totalCount }: { referrers: Referrer[]; totalCount: number } = $props();

  let isFiltered = $derived(referrers.length !== totalCount);

  let expanded = $state<Record<number, boolean>>({});
  let vexData = $state<Record<number, VEXDocument>>({});
  let loadingActions = $state<Record<string, boolean>>({});
  let actionError = $state('');

  function toggleExpand(i: number) {
    expanded[i] = !expanded[i];
  }


  async function handleDownloadSBOM(e: MouseEvent, r: Referrer) {
    e.stopPropagation();
    const key = `sbom-${r.digest}`;
    loadingActions[key] = true;
    try {
      const blob = await api.downloadSBOM(appState.currentData!.repository, r.digest);
      downloadBlob(blob, `sbom-${r.digest.substring(7, 19)}.json`);
    } catch (err) {
      actionError = 'Error downloading SBOM: ' + (err as Error).message;
    } finally {
      loadingActions[key] = false;
    }
  }

  async function handleFetchVEX(e: MouseEvent, r: Referrer, index: number) {
    e.stopPropagation();
    const key = `vex-view-${r.digest}`;
    loadingActions[key] = true;
    try {
      const doc = await api.fetchVEX(appState.currentData!.repository, r.digest);
      vexData[index] = doc;
      expanded[index] = true;
    } catch (err) {
      actionError = 'Error fetching VEX: ' + (err as Error).message;
    } finally {
      loadingActions[key] = false;
    }
  }

  async function handleDownloadVEX(e: MouseEvent, r: Referrer) {
    e.stopPropagation();
    const key = `vex-dl-${r.digest}`;
    loadingActions[key] = true;
    try {
      const doc = await api.fetchVEX(appState.currentData!.repository, r.digest);
      downloadBlob(
        new Blob([JSON.stringify(doc, null, 2)], { type: 'application/json' }),
        `vex-${r.digest.substring(7, 19)}.json`,
      );
    } catch (err) {
      actionError = 'Error downloading VEX: ' + (err as Error).message;
    } finally {
      loadingActions[key] = false;
    }
  }
</script>

{#snippet icon()}
  <svg class="w-5 h-5 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
  </svg>
{/snippet}

<CollapsibleSection
  id="referrers"
  title="Referrers"
  badge="{isFiltered ? `${referrers.length} of ${totalCount}` : String(referrers.length)} artifacts{isFiltered ? ' (filtered)' : ''}"
  {icon}
>
  {#if actionError}
    <div class="flex items-center justify-between gap-2 p-3 mb-3 bg-red-500/10 border border-red-500/30 rounded-lg text-xs text-red-300">
      <span>{actionError}</span>
      <button onclick={() => (actionError = '')} class="text-red-400 hover:text-red-200 flex-shrink-0">Dismiss</button>
    </div>
  {/if}
  {#if referrers.length === 0}
    <div class="text-center py-6 text-slate-500">
      <svg class="w-10 h-10 mx-auto mb-2 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
      </svg>
      <p>No referrers found</p>
      <p class="text-xs mt-1">No signatures, SBOMs, or attestations attached to this image</p>
    </div>
  {:else}
    <div class="space-y-3">
      {#each referrers as r, i}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div
          class="border rounded-lg p-3 hover:border-slate-500 transition-all cursor-pointer overflow-hidden {expanded[i] ? 'border-slate-500 bg-slate-700/50' : 'border-slate-700 bg-slate-800/30'}"
          onclick={() => toggleExpand(i)}
        >
          <div class="flex items-start justify-between gap-2">
            <div class="flex items-start gap-3 min-w-0">
              <div class="w-10 h-10 rounded-md flex items-center justify-center border flex-shrink-0 {referrerTypeClasses[r.type] || referrerTypeClasses.artifact}">
                {#if r.type === 'signature'}
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"></path></svg>
                {:else if r.type === 'sbom'}
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"></path></svg>
                {:else if r.type === 'attestation'}
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path></svg>
                {:else if r.type === 'vex'}
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
                {:else}
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path></svg>
                {/if}
              </div>
              <div class="min-w-0">
                <div class="text-sm font-semibold text-slate-200 capitalize">{r.type}</div>
                <div class="text-xs text-slate-500 font-mono {expanded[i] ? 'break-all' : 'truncate'}">{r.signatureInfo?.identity || r.digest}</div>
              </div>
            </div>
            <div class="text-sm text-slate-400 flex-shrink-0 whitespace-nowrap">{formatBytes(r.size)}</div>
          </div>

          <!-- Action buttons -->
          {#if r.type === 'sbom' || r.type === 'vex'}
            <div class="mt-2 ml-[52px] flex items-center gap-2 flex-wrap">
              {#if r.type === 'sbom'}
                <button
                  onclick={(e) => handleDownloadSBOM(e, r)}
                  disabled={loadingActions[`sbom-${r.digest}`]}
                  class="px-3 py-1.5 bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-300 rounded-md text-xs font-semibold flex items-center gap-1.5 transition-colors"
                >
                  {#if loadingActions[`sbom-${r.digest}`]}
                    <svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                  {:else}
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"></path></svg>
                  {/if}
                  Download
                </button>
              {/if}
              {#if r.type === 'vex'}
                <button
                  onclick={(e) => handleFetchVEX(e, r, i)}
                  disabled={loadingActions[`vex-view-${r.digest}`]}
                  class="px-3 py-1.5 bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 rounded-md text-xs font-semibold flex items-center gap-1.5 transition-colors"
                >
                  {#if loadingActions[`vex-view-${r.digest}`]}
                    <svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                  {:else}
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path></svg>
                  {/if}
                  View
                </button>
                <button
                  onclick={(e) => handleDownloadVEX(e, r)}
                  disabled={loadingActions[`vex-dl-${r.digest}`]}
                  class="px-3 py-1.5 bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 rounded-md text-xs font-semibold flex items-center gap-1.5 transition-colors"
                >
                  {#if loadingActions[`vex-dl-${r.digest}`]}
                    <svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" fill="none"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                  {:else}
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"></path></svg>
                  {/if}
                  Download
                </button>
              {/if}
            </div>
          {/if}

          {#if expanded[i]}
            <div class="mt-3 pt-3 border-t border-slate-700 space-y-3">
              <div class="grid grid-cols-2 gap-3 text-sm">
                <div>
                  <span class="text-slate-500 text-xs">Artifact Type</span>
                  <div class="text-xs text-slate-300 mt-1 break-all">{r.artifactType || 'N/A'}</div>
                </div>
                <div>
                  <span class="text-slate-500 text-xs">Media Type</span>
                  <div class="text-xs text-slate-300 mt-1 break-all">{r.mediaType}</div>
                </div>
              </div>
              <div class="flex items-start gap-2 text-sm">
                <span class="text-slate-500 min-w-24">Digest:</span>
                <CopyableDigest digest={r.digest} />
              </div>
              {#if r.signatureInfo}
                <div class="bg-amber-500/10 border border-amber-500/30 rounded-lg p-3">
                  <div class="flex items-center gap-2 mb-2">
                    <svg class="w-4 h-4 text-amber-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"></path>
                    </svg>
                    <span class="text-xs font-semibold text-amber-300">Sigstore Certificate</span>
                  </div>
                  {#if r.signatureInfo.identity}
                    <div class="text-xs mb-1">
                      <span class="text-slate-500">Identity:</span>
                      <span class="text-amber-200 ml-1 break-all">{r.signatureInfo.identity}</span>
                    </div>
                  {/if}
                  {#if r.signatureInfo.issuer}
                    <div class="text-xs">
                      <span class="text-slate-500">OIDC Issuer:</span>
                      <span class="text-amber-200 ml-1 break-all">{r.signatureInfo.issuer}</span>
                    </div>
                  {/if}
                </div>
              {/if}
              {#if r.annotations && Object.keys(r.annotations).length}
                <div>
                  <span class="text-slate-500 text-xs">Annotations</span>
                  <div class="mt-1 space-y-1">
                    {#each Object.entries(r.annotations) as [k, v]}
                      <div class="text-xs">
                        <span class="text-slate-500">{k.split(/[.#]/).pop()}:</span>
                        <span class="text-slate-400 ml-1">{v}</span>
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}
              {#if vexData[i]}
                <div class="mt-3">
                  <VEXViewer vex={vexData[i]} />
                </div>
              {/if}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</CollapsibleSection>
