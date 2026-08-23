<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { Contest, ContestProblem } from '@cp-hub/contracts';
  import ContestTimer from '$lib/components/ContestTimer.svelte';
  import ProblemCard from '$lib/components/ProblemCard.svelte';
  import { Trophy, Users, Shield, ArrowRight, Lock, Play, AlertCircle, RefreshCw } from 'lucide-svelte';

  let contestId = $page.params.id;
  let contest: Contest | null = null;
  let problems: ContestProblem[] = [];
  let loading = true;
  let error = '';
  let interval: any;

  async function loadContest() {
    try {
      contest = await api.get<Contest>(`/contests/${contestId}`);
      problems = contest.problems || [];
    } catch (err: any) {
      error = err.message || 'Failed to load contest';
    } finally {
      loading = false;
    }
  }

  async function handleJoin() {
    if (!$auth.user) {
      alert('Please sign in to join this contest');
      return;
    }
    try {
      await api.post(`/contests/${contestId}/join`);
      await loadContest();
    } catch (err: any) {
      alert(err.message || 'Failed to join contest');
    }
  }

  onMount(() => {
    loadContest();
    // Poll contest state every 5 seconds to auto-reveal problems when contest starts
    interval = setInterval(loadContest, 5000);
  });

  onDestroy(() => {
    if (interval) clearInterval(interval);
  });
</script>

{#if loading}
  <div class="h-96 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
{:else if error || !contest}
  <div class="p-8 rounded-2xl border border-red-500/30 bg-red-500/10 text-red-300">
    <h2 class="text-xl font-bold">Error</h2>
    <p class="text-sm">{error || 'Contest not found.'}</p>
  </div>
{:else}
  <div class="space-y-8">
    <!-- Contest Header Card -->
    <div class="p-6 sm:p-8 rounded-2xl border border-zinc-800 bg-zinc-900/40 space-y-6">
      <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-6">
        <div class="space-y-2">
          <div class="flex items-center space-x-2.5">
            <span class="text-xs px-2.5 py-0.5 rounded-full font-bold {
              contest.state === 'UPCOMING' ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' :
              contest.state === 'ACTIVE' ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
              'bg-zinc-800 text-zinc-400 border border-zinc-700'
            }">
              {contest.state}
            </span>

            <span class="text-xs px-2 py-0.5 rounded-md font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
              {contest.scoringType} Scoring
            </span>

            <span class="text-xs text-zinc-500">by {contest.ownerUsername}</span>
          </div>

          <h1 class="text-3xl sm:text-4xl font-extrabold text-white">{contest.name}</h1>
          <p class="text-sm text-zinc-400 max-w-2xl">{contest.description || 'No contest description provided.'}</p>
        </div>

        <!-- Timer & Controls -->
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 shrink-0">
          <ContestTimer startAt={contest.startAt} endAt={contest.endAt} state={contest.state} />

          <a
            href={`/contests/${contest.id}/standings`}
            class="px-5 py-3 rounded-xl font-bold bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/20 transition flex items-center justify-center space-x-2 text-sm"
          >
            <Trophy class="w-4 h-4" />
            <span>Scoreboard</span>
          </a>
        </div>
      </div>

      <!-- Participation Bar -->
      <div class="flex items-center justify-between pt-4 border-t border-zinc-800/80 text-xs text-zinc-400">
        <div class="flex items-center space-x-4">
          <div class="flex items-center space-x-1.5">
            <Users class="w-4 h-4 text-zinc-500" />
            <span>{contest.participantCount} registered participant{contest.participantCount === 1 ? '' : 's'}</span>
          </div>
        </div>

        {#if $auth.user && !contest.isParticipant && contest.state !== 'FINISHED'}
          <button
            on:click={handleJoin}
            class="px-3.5 py-1.5 rounded-lg font-semibold bg-emerald-600 hover:bg-emerald-500 text-white transition text-xs"
          >
            Join Contest
          </button>
        {:else if contest.isParticipant}
          <span class="text-emerald-400 font-medium">✓ You are participating</span>
        {/if}
      </div>
    </div>

    <!-- Problems Section -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-xl font-bold text-white flex items-center space-x-2">
          <span>Contest Problems</span>
          <span class="text-xs font-mono text-zinc-500">({problems.length})</span>
        </h2>

        <button
          on:click={loadContest}
          class="p-2 rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition"
          title="Refresh"
        >
          <RefreshCw class="w-4 h-4" />
        </button>
      </div>

      {#if contest.state === 'UPCOMING'}
        <!-- Pre-contest problem lock screen -->
        <div class="p-12 rounded-2xl border border-amber-500/20 bg-amber-500/5 text-center space-y-3">
          <div class="w-12 h-12 rounded-full bg-amber-500/10 border border-amber-500/30 text-amber-400 flex items-center justify-center mx-auto">
            <Lock class="w-6 h-6" />
          </div>
          <h3 class="text-lg font-bold text-white">Problems are Hidden</h3>
          <p class="text-xs text-zinc-400 max-w-md mx-auto">
            Problem statements and titles will automatically unlock when the countdown timer reaches zero.
          </p>
          <div class="flex justify-center space-x-2 pt-2">
            {#each problems as p}
              <div class="w-10 h-10 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center font-bold text-zinc-500 text-sm">
                {p.label}
              </div>
            {/each}
          </div>
        </div>
      {:else}
        <!-- Unlocked problems list -->
        <div class="space-y-3">
          {#each problems as cp}
            {#if cp.problem}
              <ProblemCard problem={cp.problem} label={cp.label} />
            {/if}
          {/each}
        </div>
      {/if}
    </div>
  </div>
{/if}
