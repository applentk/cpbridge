<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { ContestState } from '@cp-hub/contracts';
  import { Clock, Play, CheckCircle } from 'lucide-svelte';

  export let startAt: string;
  export let endAt: string;
  export let state: ContestState;

  let remainingText = '';
  let interval: any;

  function update() {
    const now = Date.now();
    const start = new Date(startAt).getTime();
    const end = new Date(endAt).getTime();

    if (now < start) {
      state = 'UPCOMING';
      remainingText = formatDiff(start - now);
    } else if (now < end) {
      state = 'ACTIVE';
      remainingText = formatDiff(end - now);
    } else {
      state = 'FINISHED';
      remainingText = 'Contest Finished';
    }
  }

  function formatDiff(ms: number): string {
    if (ms <= 0) return '00:00:00';
    const totalSec = Math.floor(ms / 1000);
    const hrs = Math.floor(totalSec / 3600);
    const mins = Math.floor((totalSec % 3600) / 60);
    const secs = totalSec % 60;
    return `${String(hrs).padStart(2, '0')}:${String(mins).padStart(2, '0')}:${String(secs).padStart(2, '0')}`;
  }

  onMount(() => {
    update();
    interval = setInterval(update, 1000);
  });

  onDestroy(() => {
    if (interval) clearInterval(interval);
  });
</script>

<div class="flex items-center space-x-3 px-4 py-2 rounded-xl border bg-zinc-900/80 backdrop-blur-md shadow-sm {
  state === 'UPCOMING' ? 'border-amber-500/30 text-amber-300' :
  state === 'ACTIVE' ? 'border-emerald-500/30 text-emerald-300' :
  'border-zinc-700 text-zinc-400'
}">
  {#if state === 'UPCOMING'}
    <Clock class="w-5 h-5 animate-pulse text-amber-400" />
    <div>
      <div class="text-xs uppercase font-bold tracking-wider text-amber-500/90">Starts In</div>
      <div class="text-lg font-mono font-bold tracking-tight">{remainingText}</div>
    </div>
  {:else if state === 'ACTIVE'}
    <Play class="w-5 h-5 text-emerald-400" />
    <div>
      <div class="text-xs uppercase font-bold tracking-wider text-emerald-500/90">Time Remaining</div>
      <div class="text-lg font-mono font-bold tracking-tight">{remainingText}</div>
    </div>
  {:else}
    <CheckCircle class="w-5 h-5 text-zinc-400" />
    <div>
      <div class="text-xs uppercase font-bold tracking-wider text-zinc-500">Status</div>
      <div class="text-lg font-mono font-bold tracking-tight">Finished</div>
    </div>
  {/if}
</div>
