<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { Problem, PlatformType } from '@cpbridge/contracts';
  import { BookOpen, Search, ExternalLink, ArrowRight } from 'lucide-svelte';

  let problems: Problem[] = [];
  let total = 0;
  let loading = true;
  let error = '';
  let query = '';
  let selectedPlatform: PlatformType | '' = '';

  async function loadProblems() {
    loading = true;
    error = '';

    try {
      const params = new URLSearchParams({ limit: '50' });
      if (query.trim()) params.set('query', query.trim());
      if (selectedPlatform) params.set('platform', selectedPlatform);

      const response = await api.get<{ problems: Problem[]; total: number }>(`/problems?${params.toString()}`);
      problems = response.problems;
      total = response.total;
    } catch (err: any) {
      error = err.message || 'Failed to load problems';
    } finally {
      loading = false;
    }
  }

  function handleSearch(event: SubmitEvent) {
    event.preventDefault();
    void loadProblems();
  }

  onMount(() => {
    void loadProblems();
  });
</script>

<div class="space-y-6">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-white flex items-center space-x-2">
        <BookOpen class="w-7 h-7 text-zinc-300" />
        <span>Problems</span>
      </h1>
      <p class="text-sm text-zinc-400">Browse the problems available for practice and contests.</p>
    </div>
    <span class="text-xs text-zinc-500">{total} problem{total === 1 ? '' : 's'}</span>
  </div>

  <form on:submit={handleSearch} class="flex flex-col sm:flex-row gap-3">
    <div class="relative flex-1">
      <Search class="w-4 h-4 absolute left-3.5 top-3 text-zinc-500" />
      <input
        type="search"
        bind:value={query}
        placeholder="Search by title or problem ID..."
        class="w-full pl-10 pr-4 py-2.5 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-zinc-500 focus:outline-none text-zinc-100 text-sm placeholder-zinc-500"
      />
    </div>
    <select
      bind:value={selectedPlatform}
      on:change={() => void loadProblems()}
      class="px-4 py-2.5 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-zinc-500 focus:outline-none text-zinc-200 text-sm"
    >
      <option value="">All platforms</option>
      <option value="CODEFORCES">Codeforces</option>
      <option value="ATCODER">AtCoder</option>
    </select>
    <button type="submit" class="px-4 py-2.5 rounded-xl text-sm font-semibold bg-white hover:bg-zinc-200 text-black transition">
      Search
    </button>
  </form>

  {#if loading}
    <div class="h-64 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
  {:else if error}
    <div class="p-6 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm">{error}</div>
  {:else if problems.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center text-sm text-zinc-500">
      No problems found.
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      {#each problems as problem}
        <a
          href={`/problems/${problem.id}`}
          class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-800/40 hover:border-zinc-700 transition group"
        >
          <div class="flex items-start justify-between gap-4">
            <div class="min-w-0 space-y-2">
              <div class="flex items-center gap-2 text-xs text-zinc-500">
                <span class="font-semibold text-zinc-300">{problem.platform}</span>
                <span>•</span>
                <span class="font-mono">{problem.externalId}</span>
              </div>
              <h2 class="font-bold text-white truncate group-hover:text-zinc-300 transition">{problem.title}</h2>
              {#if problem.tags.length > 0}
                <div class="flex flex-wrap gap-1.5">
                  {#each problem.tags.slice(0, 4) as tag}
                    <span class="text-[11px] px-2 py-0.5 rounded-md bg-zinc-800 text-zinc-400">{tag}</span>
                  {/each}
                </div>
              {/if}
            </div>
            <ArrowRight class="w-4 h-4 text-zinc-600 group-hover:text-white group-hover:translate-x-0.5 transition shrink-0" />
          </div>
          <div class="mt-4 pt-3 border-t border-zinc-800 flex items-center justify-between text-xs text-zinc-500">
            <span>{problem.difficulty ? `Difficulty ${problem.difficulty}` : 'Difficulty not rated'}</span>
            <span class="flex items-center gap-1">Open problem <ExternalLink class="w-3 h-3" /></span>
          </div>
        </a>
      {/each}
    </div>
  {/if}
</div>
