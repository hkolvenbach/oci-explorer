<script lang="ts">
  import type { SecurityScoreResult } from '../lib/types';

  let { score: sr, showDetails = $bindable(false) }: { score: SecurityScoreResult; showDetails?: boolean } = $props();

  const circumference = 282.7;
  let offset = $derived(circumference - (sr.score / sr.maxScore) * circumference);
  let scoreDisplay = $derived(sr.score % 1 === 0 ? String(sr.score) : sr.score.toFixed(1));
  let btnColor = $derived(sr.score >= 8 ? 'green' : sr.score >= 6 ? 'yellow' : sr.score >= 4 ? 'orange' : 'red');
</script>

<div class="flex flex-col items-center">
  <div class="text-lg font-semibold text-slate-100 mb-2">Supply Chain Score</div>
  <svg width="80" height="80" viewBox="0 0 100 100">
    <circle cx="50" cy="50" r="45" fill="none" stroke="#334155" stroke-width="8" />
    <circle
      cx="50" cy="50" r="45" fill="none" stroke={sr.color} stroke-width="8"
      stroke-dasharray={circumference} stroke-dashoffset={offset}
      stroke-linecap="round" transform="rotate(-90 50 50)" class="score-ring"
    />
    <text x="50" y="50" text-anchor="middle" dominant-baseline="central"
      fill={sr.color} font-size="28" font-weight="bold">{scoreDisplay}</text>
  </svg>
  <button
    onclick={() => (showDetails = !showDetails)}
    class="mt-2 px-3 py-1.5 rounded-md text-xs font-semibold flex items-center gap-1.5 transition-colors border
      {btnColor === 'green' ? 'bg-green-500/20 hover:bg-green-500/30 text-green-300 border-green-500/30' :
       btnColor === 'yellow' ? 'bg-yellow-500/20 hover:bg-yellow-500/30 text-yellow-300 border-yellow-500/30' :
       btnColor === 'orange' ? 'bg-orange-500/20 hover:bg-orange-500/30 text-orange-300 border-orange-500/30' :
       'bg-red-500/20 hover:bg-red-500/30 text-red-300 border-red-500/30'}"
  >
    {sr.grade} ({scoreDisplay}/{sr.maxScore})
    <svg
      class="w-3.5 h-3.5 transition-transform"
      style:transform={showDetails ? 'rotate(180deg)' : ''}
      fill="none" stroke="currentColor" viewBox="0 0 24 24"
    >
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
    </svg>
  </button>
</div>
