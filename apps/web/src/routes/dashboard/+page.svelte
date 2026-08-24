<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import { type Contest, type Submission, formatLanguageName } from '@cp-hub/contracts';
  import SubmissionModal from '$lib/components/SubmissionModal.svelte';
  import { Trophy, Cpu, ArrowRight, Clock, ShieldCheck, LayoutDashboard, Code2 } from 'lucide-svelte';

  let contests: Contest[] = [];
  let submissions: Submission[] = [];
  let loading = true;
  let viewingSubmission: Submission | null = null;

  onMount(async () => {
    try {
      const [cRes, sRes] = await Promise.all([
        api.get<Contest[]>('/contests'),
        api.get<Submission[]>('/submissions').catch(() => [])
      ]);
      contests = cRes.slice(0, 3);
      submissions = (sRes || []).slice(0, 5);
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  });
</script>

<div class="space-y-8">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-white">Dashboard</h1>
      <p class="text-sm text-zinc-400">Welcome back, {$auth.user?.username || 'Guest'}!</p>
    </div>

    {#if $auth.user?.role === 'ADMIN'}
      <a
        href="/admin"
        class="px-4 py-2 rounded-xl text-sm font-bold bg-amber-500 hover:bg-amber-400 text-black shadow-sm transition flex items-center space-x-1.5 self-start sm:self-auto"
      >
        <LayoutDashboard class="w-4 h-4" />
        <span>Admin Dashboard</span>
      </a>
    {/if}
  </div>

  {#if loading}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
      <div class="h-40 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      <div class="h-40 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
    </div>
  {:else}
    <!-- Active / Recent Contests -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-xl font-bold text-white flex items-center space-x-2">
          <Trophy class="w-5 h-5 text-white" />
          <span>Available Contests</span>
        </h2>
        <a href="/contests" class="text-xs font-semibold text-zinc-400 hover:text-white flex items-center space-x-1">
          <span>View all contests</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </a>
      </div>

      {#if contests.length === 0}
        <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
          <p class="text-sm text-zinc-400">No active or upcoming contests right now.</p>
        </div>
      {:else}
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
          {#each contests as c}
            <a href={`/contests/${c.id}`} class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-800/40 hover:border-zinc-700 transition space-y-3 block">
              <div class="flex items-center justify-between">
                <span class="text-xs px-2 py-0.5 rounded-full font-bold {
                  c.state === 'ACTIVE' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
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

    <!-- Recent Submissions -->
    {#if $auth.user}
      <div class="space-y-4">
        <div class="flex items-center justify-between">
          <h2 class="text-xl font-bold text-white flex items-center space-x-2">
            <Cpu class="w-5 h-5 text-white" />
            <span>My Recent Submissions</span>
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
              <button
                type="button"
                on:click={() => (viewingSubmission = sub)}
                class="w-full text-left p-4 flex items-center justify-between hover:bg-zinc-800/30 transition cursor-pointer group"
              >
                <div class="space-y-1">
                  <div class="font-semibold text-zinc-100 group-hover:text-white text-sm transition">
                    {sub.problemTitle || sub.problemId}
                  </div>
                  <div class="flex flex-wrap items-center gap-2 text-xs text-zinc-400">
                    <span class="font-mono">{sub.platform}</span>
                    <span>•</span>
                    <span class="text-zinc-300 font-semibold">{formatLanguageName(sub.language)}</span>
                    <span class="text-zinc-600">•</span>
                    <span class="font-mono text-[11px] text-zinc-500">{sub.id}</span>
                    <span>•</span>
                    <span>{new Date(sub.submittedAt).toLocaleTimeString()}</span>
                  </div>
                </div>

                <div class="flex items-center space-x-2.5">
                  <span class="text-xs px-2.5 py-1 rounded-lg font-bold font-mono {
                    sub.status === 'ACCEPTED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                    sub.status === 'WRONG_ANSWER' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30' :
                    sub.status === 'JUDGING' || sub.status === 'PENDING' ? 'bg-zinc-800 text-zinc-200 border border-zinc-700 font-medium' :
                    'bg-zinc-950 text-zinc-400 border border-zinc-800'
                  }">
                    {sub.status}
                  </span>
                  <Code2 class="w-4 h-4 text-zinc-600 group-hover:text-zinc-300 transition" />
                </div>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  {/if}
</div>

<SubmissionModal
  submission={viewingSubmission}
  open={!!viewingSubmission}
  onClose={() => (viewingSubmission = null)}
/>
