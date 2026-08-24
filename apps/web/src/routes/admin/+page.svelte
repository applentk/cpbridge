<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { Contest } from '@cpbridge/contracts';
  import { Trophy, Code2, Layers, Users, Plus, ArrowRight } from 'lucide-svelte';

  interface AdminStats {
    totalProblems: number;
    totalProblemSets: number;
    totalContests: number;
    activeContests: number;
    upcomingContests: number;
    totalUsers: number;
  }

  let stats: AdminStats | null = null;
  let recentContests: Contest[] = [];
  let loading = true;
  let error = '';

  async function loadDashboard() {
    loading = true;
    try {
      const [statsRes, contestsRes] = await Promise.all([
        api.get<AdminStats>('/admin/stats'),
        api.get<Contest[]>('/admin/contests')
      ]);
      stats = statsRes;
      recentContests = contestsRes.slice(0, 5);
    } catch (err: any) {
      error = err.message || 'Failed to load dashboard statistics';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadDashboard();
  });
</script>

<div class="space-y-8">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-extrabold text-white">Admin Dashboard</h1>
      <p class="text-sm text-zinc-400">System overview, management tools, and quick actions.</p>
    </div>

    <div class="flex items-center space-x-2">
      <a
        href="/admin/contests/new"
        class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black shadow-sm transition flex items-center space-x-1.5"
      >
        <Plus class="w-4 h-4" />
        <span>Create Contest</span>
      </a>
      <a
        href="/admin/problems"
        class="px-4 py-2 rounded-xl text-sm font-semibold border border-zinc-700 bg-zinc-900/80 hover:bg-zinc-800 text-white transition flex items-center space-x-1.5"
      >
        <Code2 class="w-4 h-4" />
        <span>View Problems</span>
      </a>
    </div>
  </div>

  {#if loading}
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      {#each Array(4) as _}
        <div class="h-28 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      {/each}
    </div>
  {:else if error}
    <div class="p-6 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm">
      {error}
    </div>
  {:else if stats}
    <!-- Stats Cards -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/50 space-y-2">
        <div class="flex items-center justify-between text-zinc-400">
          <span class="text-xs font-semibold uppercase tracking-wider">Contests</span>
          <Trophy class="w-4 h-4 text-zinc-400" />
        </div>
        <div class="text-3xl font-extrabold text-white">{stats.totalContests}</div>
        <div class="text-xs text-zinc-400 flex items-center space-x-2">
          <span class="text-emerald-400 font-medium">{stats.activeContests} active</span>
          <span>•</span>
          <span>{stats.upcomingContests} upcoming</span>
        </div>
      </div>

      <a href="/admin/problems" class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/50 hover:bg-zinc-800/40 hover:border-zinc-700 transition space-y-2 block group">
        <div class="flex items-center justify-between text-zinc-400">
          <span class="text-xs font-semibold uppercase tracking-wider">Problems</span>
          <Code2 class="w-4 h-4 text-zinc-400 group-hover:text-white" />
        </div>
        <div class="text-3xl font-extrabold text-white">{stats.totalProblems}</div>
        <p class="text-xs text-zinc-400 group-hover:text-zinc-300">View problem library</p>
      </a>

      <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/50 space-y-2">
        <div class="flex items-center justify-between text-zinc-400">
          <span class="text-xs font-semibold uppercase tracking-wider">Problem Sets</span>
          <Layers class="w-4 h-4 text-zinc-400" />
        </div>
        <div class="text-3xl font-extrabold text-white">{stats.totalProblemSets}</div>
        <p class="text-xs text-zinc-400">Curated sets</p>
      </div>

      <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/50 space-y-2">
        <div class="flex items-center justify-between text-zinc-400">
          <span class="text-xs font-semibold uppercase tracking-wider">Registered Users</span>
          <Users class="w-4 h-4 text-zinc-400" />
        </div>
        <div class="text-3xl font-extrabold text-white">{stats.totalUsers}</div>
        <p class="text-xs text-zinc-400">Platform accounts</p>
      </div>
    </div>

    <!-- Quick Action Shortcuts -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      <a
        href="/admin/contests/new"
        class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/30 hover:bg-zinc-800/40 hover:border-zinc-700 transition flex items-center justify-between group"
      >
        <div class="space-y-1">
          <div class="font-bold text-white text-base">Create Contest</div>
          <p class="text-xs text-zinc-400">Snapshot problems and schedule start/end time</p>
        </div>
        <ArrowRight class="w-5 h-5 text-zinc-500 group-hover:text-white group-hover:translate-x-0.5 transition shrink-0" />
      </a>

      <a
        href="/admin/problems"
        class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/30 hover:bg-zinc-800/40 hover:border-zinc-700 transition flex items-center justify-between group"
      >
        <div class="space-y-1">
          <div class="font-bold text-white text-base">View Problems</div>
          <p class="text-xs text-zinc-400">Browse, import, and manage Codeforces or AtCoder problems</p>
        </div>
        <ArrowRight class="w-5 h-5 text-zinc-500 group-hover:text-white group-hover:translate-x-0.5 transition shrink-0" />
      </a>

      <a
        href="/admin/users"
        class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/30 hover:bg-zinc-800/40 hover:border-zinc-700 transition flex items-center justify-between group"
      >
        <div class="space-y-1">
          <div class="font-bold text-white text-base">Manage Users</div>
          <p class="text-xs text-zinc-400">Change roles and toggle active account status</p>
        </div>
        <ArrowRight class="w-5 h-5 text-zinc-500 group-hover:text-white group-hover:translate-x-0.5 transition shrink-0" />
      </a>
    </div>

    <!-- Recent Contests -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-bold text-white">Recent Contests</h2>
        <a href="/admin/contests" class="text-xs font-semibold text-zinc-400 hover:text-white flex items-center space-x-1">
          <span>Manage all</span>
          <ArrowRight class="w-3.5 h-3.5" />
        </a>
      </div>

      {#if recentContests.length === 0}
        <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center text-sm text-zinc-500">
          No contests created yet.
        </div>
      {:else}
        <div class="rounded-2xl border border-zinc-800 bg-zinc-900/40 overflow-hidden divide-y divide-zinc-800/60">
          {#each recentContests as c}
            <div class="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:bg-zinc-800/20 transition">
              <div class="space-y-1 min-w-0">
                <div class="flex items-center space-x-2">
                  <span class="text-xs px-2 py-0.5 rounded-full font-bold {
                    c.publicationStatus === 'PUBLISHED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' : 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                  }">
                    {c.publicationStatus}
                  </span>
                  <span class="text-xs px-2 py-0.5 rounded-full font-bold {
                    c.state === 'ACTIVE' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                    c.state === 'UPCOMING' ? 'bg-zinc-800 text-zinc-300 border border-zinc-700' :
                    'bg-zinc-950 text-zinc-500 border border-zinc-800'
                  }">
                    {c.state}
                  </span>
                  <span class="font-semibold text-white text-sm truncate">{c.name}</span>
                </div>
                <div class="text-xs text-zinc-400 flex items-center space-x-2">
                  <span>Starts: {new Date(c.startAt).toLocaleString()}</span>
                  <span>•</span>
                  <span>{c.participantCount} participants</span>
                </div>
              </div>

              <div class="flex items-center space-x-2 shrink-0">
                <a
                  href={`/admin/contests/${c.id}/edit`}
                  class="px-3 py-1.5 rounded-lg text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition"
                >
                  Edit
                </a>
                <a
                  href={`/contests/${c.id}`}
                  class="px-3 py-1.5 rounded-lg text-xs font-semibold border border-zinc-700 hover:bg-zinc-800 text-zinc-300 transition"
                >
                  View
                </a>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>
