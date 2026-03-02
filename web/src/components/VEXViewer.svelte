<script lang="ts">
  import type { VEXDocument } from '../lib/types';
  import { vexStatusClasses, vexStatusLabels } from '../lib/constants';

  let { vex }: { vex: VEXDocument } = $props();

  let counts = $derived.by(() => {
    const c: Record<string, number> = {};
    for (const s of vex.statements || []) {
      c[s.status] = (c[s.status] || 0) + 1;
    }
    return c;
  });
</script>

<div class="bg-emerald-500/5 border border-emerald-500/20 rounded-lg p-3">
  <div class="flex items-center gap-2 mb-2">
    <svg class="w-4 h-4 text-emerald-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
    </svg>
    <span class="text-xs font-semibold text-emerald-300">VEX Document</span>
    {#if vex.author}
      <span class="text-xs text-slate-500">by {vex.author}</span>
    {/if}
  </div>
  <div class="grid grid-cols-2 gap-x-4 gap-y-1 text-xs mb-3 pl-6">
    {#if vex['@id']}
      <div><span class="text-slate-500">ID:</span> <span class="text-slate-400 break-all">{vex['@id']}</span></div>
    {/if}
    {#if vex.timestamp}
      <div><span class="text-slate-500">Issued:</span> <span class="text-slate-400">{new Date(vex.timestamp).toLocaleString()}</span></div>
    {/if}
    {#if vex.last_updated}
      <div><span class="text-slate-500">Updated:</span> <span class="text-slate-400">{new Date(vex.last_updated).toLocaleString()}</span></div>
    {/if}
    {#if vex.version}
      <div><span class="text-slate-500">Version:</span> <span class="text-slate-400">{vex.version}</span></div>
    {/if}
    {#if vex.role}
      <div><span class="text-slate-500">Role:</span> <span class="text-slate-400">{vex.role}</span></div>
    {/if}
    {#if vex.tooling}
      <div><span class="text-slate-500">Tooling:</span> <span class="text-slate-400">{vex.tooling}</span></div>
    {/if}
  </div>
  <div class="flex flex-wrap gap-2 mb-3 pl-6">
    {#each Object.entries(counts) as [status, count]}
      <span class="px-2 py-0.5 text-xs font-semibold rounded border {vexStatusClasses[status] || 'bg-slate-500/20 text-slate-300 border-slate-500/30'}">
        {count} {vexStatusLabels[status] || status}
      </span>
    {/each}
  </div>
  <div class="space-y-2 max-h-96 overflow-y-auto scrollbar-thin">
    {#each vex.statements || [] as stmt}
      <div class="flex items-start gap-2 p-2 bg-slate-800/50 rounded">
        <span class="px-2 py-0.5 text-xs font-semibold rounded border flex-shrink-0 {vexStatusClasses[stmt.status] || 'bg-slate-500/20 text-slate-300 border-slate-500/30'}">
          {vexStatusLabels[stmt.status] || stmt.status}
        </span>
        <div class="min-w-0 flex-1">
          <div class="text-xs font-mono font-semibold text-slate-200">
            {stmt.vulnerability?.name || 'Unknown'}
            {#if stmt.vulnerability?.aliases?.length}
              <span class="text-slate-500 font-normal ml-1">({stmt.vulnerability.aliases.join(', ')})</span>
            {/if}
          </div>
          {#if stmt.vulnerability?.description}
            <div class="text-xs text-slate-300 mt-0.5">{stmt.vulnerability.description}</div>
          {/if}
          {#if stmt.status_notes}
            <div class="text-xs text-slate-400 mt-0.5">{stmt.status_notes}</div>
          {/if}
          {#if stmt.justification}
            <div class="text-xs text-slate-400 mt-0.5">{stmt.justification.replace(/_/g, ' ')}</div>
          {/if}
          {#if stmt.impact_statement}
            <div class="text-xs text-slate-500 mt-0.5">{stmt.impact_statement}</div>
          {/if}
          {#if stmt.action_statement}
            <div class="text-xs text-amber-400/80 mt-0.5">{stmt.action_statement}</div>
          {/if}
          {#if stmt.products?.length}
            <div class="text-xs text-slate-500 mt-1 font-mono break-all">
              {stmt.products.map(p => p['@id'] || p.identifiers?.purl || '').filter(Boolean).join(', ')}
            </div>
          {/if}
        </div>
      </div>
    {/each}
  </div>
</div>
