<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { Contest, ProblemSet, Submission } from '@cp-hub/contracts';
  import { Trophy, Layers, Cpu, Plus, ArrowRight, Clock } from 'lucide-svelte';

  let contests: Contest[] = [];
  let problemSets: ProblemSet[] = [];
  let submissions: Submission[] = [];
  let loading = true;

  onMount(async () => {
    try {
      const [cRes, psRes, sRes] = await Promise.all([
        api.get<Contest[]>('/contests'),
        api.get<ProblemSet[]>('/problem-sets'),
        api.get<Submission[]>('/submissions')
      ]);
      contests = cRes.slice(0, 3);
      problemSets = psRes.slice(0, 3);
      submissions = sRes.slice(0, 5);
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  });
</script>

<div class="space-y-8">
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-bold text-white">Dashboard</h1>
      <p class="text-sm text-zinc-400">Welcome back, {$auth.user?.username || 'Guest'}!</p>
    </div>
    <div class="flex items-center space-x-3">
      <a href="/contests/new" class="px-4 py-2 rounded-xl text-sm font-semibold bg-white hover:bg-zinc-200 text-black transition flex items-center space-x-1.5 shadow-sm">
        <Plus class="w-4 h-4" />
        <span>New Contest</span>
      </a>
      <a href="/problem-sets" class="px-4 py-2 rounded-xl text-sm font-semibold border border-zinc-700 bg-zinc-900/80 hover:bg-zinc-800 text-white transition flex items-center space-x-1.5">
        <Layers class="w-4 h-4" />
        <span>Create Problem Set</span>
      </a>
    </div>
  </div>

  {#if loading}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div class="h-40 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      <div class="h-40 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      <div class="h-40 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
    </div>
  {:else}
    <!-- Active / Recent Virtual Contests -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-xl font-bold text-white flex items-center space-x-2">
          <Trophy class="w-5 h-5 text-white" />
          <span>Virtual Contests</span>
        </h2>
        <a href="/contests" class="text-xs font-semibold text-zinc-400 hover:text-white flex items-center space-x-1">
          <span>View all</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </a>
      </div>

      {#if contests.length === 0}
        <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
          <p class="text-sm text-zinc-400">No virtual contests created yet.</p>
          <a href="/contests/new" class="inline-flex items-center space-x-1.5 text-xs font-semibold text-white hover:underline">
            <Plus class="w-3.5 h-3.5" />
            <span>Create your first contest</span>
          </a>
        </div>
      {:else}
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          {#each contests as c}
            <a href={`/contests/${c.id}`} class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-800/40 hover:border-zinc-700 transition space-y-3 block">
              <div class="flex items-center justify-between">
                <span class="text-xs px-2 py-0.5 rounded-full font-bold {
                  c.state === 'ACTIVE' ? 'bg-white text-black border border-white' :
                  c.state === 'UPCOMING' ? 'bg-zinc-800 text-zinc-300 border border-zinc-700' :
                  'bg-zinc-950 text-zinc-500 border border-zinc-800'
                }">
                  {c.state}
                </span>
                <span class="text-xs text-zinc-500 font-mono">{c.scoringType}</span>
              </div>
              <h3 class="font-bold text-white text-base truncate">{c.name}</h3>
              <div class="text-xs text-zinc-400 flex items-center space-x-1.5">
                <Clock class="w-3.5 h-3.5 text-zinc-500" />
                <span>{new Date(c.startAt).toLocaleString()}</span>
              </div>
            </a>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Problem Sets -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-xl font-bold text-white flex items-center space-x-2">
          <Layers class="w-5 h-5 text-white" />
          <span>Curated Problem Sets</span>
        </h2>
        <a href="/problem-sets" class="text-xs font-semibold text-zinc-400 hover:text-white flex items-center space-x-1">
          <span>View all</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </a>
      </div>

      {#if problemSets.length === 0}
        <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
          <p class="text-sm text-zinc-400">No problem sets available.</p>
          <a href="/problem-sets" class="inline-flex items-center space-x-1.5 text-xs font-semibold text-white hover:underline">
            <Plus class="w-3.5 h-3.5" />
            <span>Create a Problem Set</span>
          </a>
        </div>
      {:else}
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          {#each problemSets as ps}
            <a href={`/problem-sets/${ps.id}`} class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-800/40 hover:border-zinc-700 transition space-y-2 block">
              <h3 class="font-bold text-white text-base truncate">{ps.name}</h3>
              <p class="text-xs text-zinc-400 line-clamp-2">{ps.description || 'No description provided.'}</p>
              <div class="text-xs text-zinc-300 font-semibold pt-2">
                {ps.problemCount} problem{ps.problemCount === 1 ? '' : 's'}
              </div>
            </a>
          {/each}
        </div>
      {/if}
    </div>

    <!-- Recent Submissions -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-xl font-bold text-white flex items-center space-x-2">
          <Cpu class="w-5 h-5 text-white" />
          <span>Recent Submissions</span>
        </h2>
        <a href="/submissions" class="text-xs font-semibold text-zinc-400 hover:text-white flex items-center space-x-1">
          <span>View all</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </a>
      </div>

      {#if submissions.length === 0}
        <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center text-sm text-zinc-500">
          No recent submissions recorded.
        </div>
      {:else}
        <div class="rounded-xl border border-zinc-800 bg-zinc-900/40 divide-y divide-zinc-800/60 overflow-hidden">
          {#each submissions as sub}
            <div class="p-4 flex items-center justify-between hover:bg-zinc-800/20 transition">
              <div class="space-y-1">
                <a href={`/problems/${sub.problemId}`} class="font-semibold text-zinc-100 hover:text-white text-sm">
                  {sub.problemTitle || sub.problemId}
                </a>
                <div class="flex items-center space-x-2 text-xs text-zinc-400">
                  <span class="font-mono">{sub.platform}</span>
                  <span>•</span>
                  <span>{sub.language}</span>
                  <span>•</span>
                  <span>{new Date(sub.submittedAt).toLocaleTimeString()}</span>
                </div>
              </div>

              <span class="text-xs px-2.5 py-1 rounded-lg font-bold font-mono {
                sub.status === 'ACCEPTED' ? 'bg-white text-black border border-white' :
                sub.status === 'WRONG_ANSWER' ? 'bg-zinc-900 text-zinc-300 border border-zinc-600' :
                sub.status === 'JUDGING' || sub.status === 'PENDING' ? 'bg-zinc-800 text-zinc-200 border border-zinc-700 font-medium' :
                'bg-zinc-950 text-zinc-400 border border-zinc-800'
              }">
                {sub.status}
              </span>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>
