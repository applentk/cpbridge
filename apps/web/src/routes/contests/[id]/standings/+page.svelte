<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import type { Standings, Contest } from '@cp-hub/contracts';
  import Scoreboard from '$lib/components/Scoreboard.svelte';
  import ContestTimer from '$lib/components/ContestTimer.svelte';
  import { Trophy, ArrowLeft, RefreshCw } from 'lucide-svelte';

  let contestId = $page.params.id;
  let contest: Contest | null = null;
  let standings: Standings | null = null;
  let loading = true;
  let error = '';
  let interval: any;

  async function loadData() {
    try {
      const [cRes, sRes] = await Promise.all([
        api.get<Contest>(`/contests/${contestId}`),
        api.get<Standings>(`/contests/${contestId}/standings`)
      ]);
      contest = cRes;
      standings = sRes;
    } catch (err: any) {
      error = err.message || 'Failed to load standings';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
    interval = setInterval(loadData, 5000);
  });

  onDestroy(() => {
    if (interval) clearInterval(interval);
  });
</script>

{#if loading}
  <div class="h-96 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
{:else if error || !contest || !standings}
  <div class="p-8 rounded-2xl border border-red-500/30 bg-red-500/10 text-red-300">
    <h2 class="text-xl font-bold">Error</h2>
    <p class="text-sm">{error || 'Standings not available.'}</p>
  </div>
{:else}
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="space-y-1">
        <div class="flex items-center space-x-2">
          <a href={`/contests/${contest.id}`} class="text-xs font-semibold text-zinc-400 hover:text-white flex items-center space-x-1">
            <ArrowLeft class="w-3.5 h-3.5" />
            <span>Back to Contest Lobby</span>
          </a>
        </div>
        <h1 class="text-3xl font-extrabold text-white flex items-center space-x-2">
          <Trophy class="w-7 h-7 text-indigo-400" />
          <span>{contest.name} — Standings</span>
        </h1>
      </div>

      <div class="flex items-center space-x-3 shrink-0">
        <ContestTimer startAt={contest.startAt} endAt={contest.endAt} state={contest.state} />
        <button
          on:click={loadData}
          class="p-2.5 rounded-xl border border-zinc-800 bg-zinc-900 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition"
          title="Refresh standings"
        >
          <RefreshCw class="w-4 h-4" />
        </button>
      </div>
    </div>

    <!-- Scoreboard Table -->
    <Scoreboard {standings} />
  </div>
{/if}
