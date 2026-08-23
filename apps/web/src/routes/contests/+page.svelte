<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { Contest } from '@cp-hub/contracts';
  import { Trophy, Plus, Clock, Users, ArrowRight } from 'lucide-svelte';

  let contests: Contest[] = [];
  let loading = true;
  let filterState: string = 'ALL';

  async function loadContests() {
    loading = true;
    try {
      contests = await api.get<Contest[]>('/contests');
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  }

  $: filteredContests = contests.filter((c) => {
    if (filterState === 'ALL') return true;
    return c.state === filterState;
  });

  onMount(() => {
    loadContests();
  });
</script>

<div class="space-y-6">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-white">Virtual Contests</h1>
      <p class="text-sm text-zinc-400">Compete with friends or practice solo with server-timed virtual contests.</p>
    </div>

    {#if $auth.user}
      <a
        href="/contests/new"
        class="px-4 py-2.5 rounded-xl font-semibold bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/20 transition flex items-center space-x-2 shrink-0 self-start sm:self-auto"
      >
        <Plus class="w-4 h-4" />
        <span>Create Virtual Contest</span>
      </a>
    {/if}
  </div>

  <!-- Filters -->
  <div class="flex items-center space-x-2 border-b border-zinc-800 pb-3">
    {#each ['ALL', 'ACTIVE', 'UPCOMING', 'FINISHED'] as s}
      <button
        on:click={() => (filterState = s)}
        class="px-3.5 py-1.5 rounded-xl text-xs font-semibold transition {
          filterState === s ? 'bg-indigo-600 text-white shadow-sm' : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60'
        }"
      >
        {s}
      </button>
    {/each}
  </div>

  <!-- Contest Grid -->
  {#if loading}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      {#each Array(6) as _}
        <div class="h-48 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      {/each}
    </div>
  {:else if filteredContests.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-4">
      <p class="text-zinc-400 text-base">No contests found.</p>
      {#if $auth.user}
        <a
          href="/contests/new"
          class="px-4 py-2 rounded-xl text-sm font-semibold bg-indigo-600 hover:bg-indigo-500 text-white transition inline-flex items-center space-x-1.5"
        >
          <Plus class="w-4 h-4" />
          <span>Host a Contest</span>
        </a>
      {/if}
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-5">
      {#each filteredContests as c}
        <a
          href={`/contests/${c.id}`}
          class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-800/40 hover:border-zinc-700 transition flex flex-col justify-between space-y-4 group"
        >
          <div class="space-y-3">
            <div class="flex items-center justify-between">
              <span class="text-xs px-2.5 py-0.5 rounded-full font-bold {
                c.state === 'UPCOMING' ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' :
                c.state === 'ACTIVE' ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
                'bg-zinc-800 text-zinc-400 border border-zinc-700'
              }">
                {c.state}
              </span>

              <span class="text-xs font-mono font-semibold text-zinc-400">{c.scoringType}</span>
            </div>

            <div>
              <h3 class="text-lg font-bold text-white group-hover:text-indigo-400 transition">{c.name}</h3>
              <p class="text-xs text-zinc-400 line-clamp-2 mt-1">{c.description || 'No description.'}</p>
            </div>

            <div class="space-y-1.5 text-xs text-zinc-400 pt-1">
              <div class="flex items-center space-x-1.5">
                <Clock class="w-3.5 h-3.5 text-zinc-500" />
                <span>Starts: {new Date(c.startAt).toLocaleString()}</span>
              </div>
              <div class="flex items-center space-x-1.5">
                <Users class="w-3.5 h-3.5 text-zinc-500" />
                <span>{c.participantCount} participant{c.participantCount === 1 ? '' : 's'}</span>
              </div>
            </div>
          </div>

          <div class="flex items-center justify-between pt-3 border-t border-zinc-800 text-xs">
            <span class="text-zinc-500">by {c.ownerUsername}</span>
            <span class="text-indigo-400 font-semibold flex items-center space-x-1 group-hover:translate-x-0.5 transition">
              <span>Enter Contest</span>
              <ArrowRight class="w-3.5 h-3.5" />
            </span>
          </div>
        </a>
      {/each}
    </div>
  {/if}
</div>
