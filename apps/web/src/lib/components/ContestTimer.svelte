<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import type { ContestState } from '@cpbridge/contracts';
  import { Clock, Play, CheckCircle } from 'lucide-svelte';

  export let startAt: string;
  export let endAt: string;
  export let state: ContestState;

  let remainingText = '';
  let interval: ReturnType<typeof setInterval> | undefined;

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

<div class="flex items-center space-x-3 px-4 py-2 rounded-xl border backdrop-blur-md shadow-sm {
  state === 'ACTIVE' ? 'border-emerald-500/40 text-emerald-300 bg-emerald-500/10 shadow-sm' :
  state === 'UPCOMING' ? 'border-zinc-700 text-zinc-200 bg-zinc-900/80' :
  'border-zinc-800 text-zinc-500 bg-zinc-950'
}">
  {#if state === 'UPCOMING'}
    <Clock class="w-5 h-5 animate-pulse text-zinc-300" />
    <div>
      <div class="text-xs uppercase font-bold tracking-wider text-zinc-400">Starts In</div>
      <div class="text-lg font-mono font-bold tracking-tight text-white">{remainingText}</div>
    </div>
  {:else if state === 'ACTIVE'}
    <Play class="w-5 h-5 text-emerald-400 animate-pulse" />
    <div>
      <div class="text-xs uppercase font-bold tracking-wider text-emerald-400/90">Time Remaining</div>
      <div class="text-lg font-mono font-bold tracking-tight text-white">{remainingText}</div>
    </div>
  {:else}
    <CheckCircle class="w-5 h-5 text-zinc-500" />
    <div>
      <div class="text-xs uppercase font-bold tracking-wider text-zinc-500">Status</div>
      <div class="text-lg font-mono font-bold tracking-tight text-zinc-400">Finished</div>
    </div>
  {/if}
</div>
