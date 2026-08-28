<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { Contest, ContestProblem, Submission } from '@cpbridge/contracts';
  import ContestTimer from '$lib/components/ContestTimer.svelte';
  import ProblemCard from '$lib/components/ProblemCard.svelte';
  import { Trophy, Users, Lock, Edit3 } from 'lucide-svelte';

  let contestId = $page.params.id!;
  let contest: Contest | null = null;
  let problems: ContestProblem[] = [];
  let solvedProblemIds: Set<string> = new Set();
  let wrongProblemIds: Set<string> = new Set();
  let loading = true;
  let error = '';
  let interval: ReturnType<typeof setInterval> | undefined;
  let lastLoadedAuthUserId: string | null = null;

  async function loadContest(options: { fetchSubmissions?: boolean } = {}) {
    const { fetchSubmissions = false } = options;
    try {
      contest = await api.get<Contest>(`/contests/${contestId}`);
      problems = contest.problems || [];

      // Stop polling once the contest has started and problems are unlocked
      if (contest.state !== 'UPCOMING' && interval) {
        clearInterval(interval);
        interval = undefined;
      }

      if (fetchSubmissions && $auth.user) {
        await loadSubmissions();
      }
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load contest';
    } finally {
      loading = false;
    }
  }

  async function loadSubmissions() {
    if (!$auth.user) {
      solvedProblemIds = new Set();
      wrongProblemIds = new Set();
      return;
    }
    try {
      const params = new URLSearchParams({
        contestId,
        userId: $auth.user.id,
      });
      const subsRes = await api.get<Submission[]>(`/submissions?${params.toString()}`);
      const solved = new Set<string>();
      const wrong = new Set<string>();
      if (Array.isArray(subsRes)) {
        for (const sub of subsRes) {
          if (sub.status === 'ACCEPTED') {
            solved.add(sub.problemId);
          } else if (sub.status === 'WRONG_ANSWER') {
            wrong.add(sub.problemId);
          }
        }
      }
      solvedProblemIds = solved;
      wrongProblemIds = new Set([...wrong].filter((id) => !solved.has(id)));
    } catch {}
  }

  $: {
    const currentUserId = $auth.user?.id ?? null;
    if (currentUserId !== lastLoadedAuthUserId) {
      lastLoadedAuthUserId = currentUserId;
      if (contest) {
        void loadSubmissions();
      }
    }
  }

  async function handleJoin() {
    if (!$auth.user) {
      alert('Please sign in to join this contest');
      return;
    }
    try {
      await api.post(`/contests/${contestId}/join`);
      await loadContest({ fetchSubmissions: true });
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to join contest');
    }
  }

  onMount(async () => {
    await loadContest({ fetchSubmissions: true });
    // Poll contest state only if UPCOMING to auto-reveal problems when contest starts
    if (contest?.state === 'UPCOMING') {
      interval = setInterval(() => loadContest({ fetchSubmissions: false }), 5000);
    }
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
              contest.state === 'ACTIVE' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
              contest.state === 'UPCOMING' ? 'bg-zinc-800 text-zinc-300 border border-zinc-700' :
              'bg-zinc-950 text-zinc-500 border border-zinc-800'
            }">
              {contest.state}
            </span>

            <span class="text-xs px-2 py-0.5 rounded-md font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
              {contest.scoringType} Scoring
            </span>

            <span class="text-xs text-zinc-500">by {contest.ownerUsername}</span>

            {#if $auth.user?.role === 'ADMIN'}
              <span class="text-xs px-2 py-0.5 rounded font-mono font-bold {
                contest.publicationStatus === 'PUBLISHED' ? 'bg-emerald-500/20 text-emerald-300' : 'bg-amber-500/20 text-amber-300'
              }">
                {contest.publicationStatus}
              </span>
            {/if}
          </div>

          <h1 class="text-3xl sm:text-4xl font-extrabold text-white">{contest.name}</h1>
          <p class="text-sm text-zinc-400 max-w-2xl">{contest.description || 'No contest description provided.'}</p>
        </div>

        <!-- Timer & Controls -->
        <div class="flex flex-col sm:flex-row items-stretch sm:items-center gap-3 shrink-0">
          <ContestTimer startAt={contest.startAt} endAt={contest.endAt} state={contest.state} />

          <a
            href={`/contests/${contest.id}/standings`}
            class="px-5 py-3 rounded-xl font-bold bg-white hover:bg-zinc-200 text-black shadow-sm transition flex items-center justify-center space-x-2 text-sm"
          >
            <Trophy class="w-4 h-4" />
            <span>Scoreboard</span>
          </a>

          {#if $auth.user?.role === 'ADMIN'}
            <a
              href={`/admin/contests/${contest.id}/edit`}
              class="px-4 py-3 rounded-xl font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition flex items-center justify-center space-x-1.5 text-sm"
            >
              <Edit3 class="w-4 h-4" />
              <span>Edit Contest</span>
            </a>
          {/if}
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
            class="px-3.5 py-1.5 rounded-lg font-bold bg-emerald-600 hover:bg-emerald-500 text-white transition text-xs shadow-sm"
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
          {#if contest.state !== 'UPCOMING' || $auth.user?.role === 'ADMIN'}
            <span class="text-xs font-mono text-zinc-500">({problems.length})</span>
          {/if}
        </h2>
      </div>

      {#if contest.state === 'UPCOMING' && $auth.user?.role !== 'ADMIN'}
        <!-- Pre-contest problem lock screen for normal users -->
        <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/50 text-center space-y-3">
          <div class="w-12 h-12 rounded-full bg-zinc-800 border border-zinc-700 text-white flex items-center justify-center mx-auto">
            <Lock class="w-6 h-6" />
          </div>
          <h3 class="text-lg font-bold text-white">Problems are Locked</h3>
          <p class="text-xs text-zinc-400 max-w-md mx-auto">
            Problem statements and titles will automatically unlock when the contest starts.
          </p>
        </div>
      {:else if problems.length === 0}
        <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center text-sm text-zinc-500">
          No problems have been added to this contest yet.
        </div>
      {:else}
        <!-- Unlocked problems list -->
        <div class="flex flex-col gap-4">
          {#each problems as cp}
            {#if cp.problem}
              <ProblemCard
                problem={cp.problem}
                label={cp.label}
                contestId={contest.id}
                isSolved={solvedProblemIds.has(cp.problemId)}
                isWrong={wrongProblemIds.has(cp.problemId)}
              />
            {/if}
          {/each}
        </div>
      {/if}
    </div>
  </div>
{/if}
