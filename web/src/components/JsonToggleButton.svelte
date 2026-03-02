<script lang="ts">
  let { active, onclick }: { active: boolean; onclick: (e: MouseEvent) => void } = $props();
</script>

<button {onclick} class="px-3 py-2 mr-2 rounded text-xs font-medium transition-colors {active ? 'bg-blue-500/30 text-blue-300' : 'bg-slate-700 text-slate-400 hover:text-slate-200'}" title="Toggle JSON view">
  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"></path>
  </svg>
</button>
