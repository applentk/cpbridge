<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { Problem, PlatformType } from '@cp-hub/contracts';
  import ProblemCard from '$lib/components/ProblemCard.svelte';
  import { Search, Plus, ExternalLink, Filter, AlertCircle, CheckCircle2 } from 'lucide-svelte';

  let problems: Problem[] = [];
  let total = 0;
  let query = '';
  let selectedPlatform: string = '';
  let loading = true;

  let showImportModal = false;
  let importUrl = '';
  let importError = '';
  let importLoading = false;
  let importSuccess = '';

  async function loadProblems() {
    loading = true;
    try {
      let path = `/problems?query=${encodeURIComponent(query)}`;
      if (selectedPlatform) path += `&platform=${selectedPlatform}`;
      const res = await api.get<{ problems: Problem[]; total: number }>(path);
      problems = res.problems || [];
      total = res.total || 0;
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  }

  async function handleImport() {
    importError = '';
    importSuccess = '';
    importLoading = true;
    try {
      const p = await api.post<Problem>('/problems/import', { url: importUrl });
      importSuccess = `Successfully imported "${p.title}"!`;
      importUrl = '';
      await loadProblems();
      setTimeout(() => {
        showImportModal = false;
        importSuccess = '';
      }, 1500);
    } catch (err: any) {
      importError = err.message || 'Import failed';
    } finally {
      importLoading = false;
    }
  }

  onMount(() => {
    loadProblems();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-white">Problem Library</h1>
      <p class="text-sm text-zinc-400">Explore and import problems from Codeforces, AtCoder, and LeetCode.</p>
    </div>

    <button
      on:click={() => (showImportModal = true)}
      class="px-4 py-2.5 rounded-xl font-semibold bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/20 transition flex items-center space-x-2 shrink-0 self-start sm:self-auto"
    >
      <Plus class="w-4 h-4" />
      <span>Import by URL</span>
    </button>
  </div>

  <!-- Search and Filters -->
  <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
    <div class="sm:col-span-2 relative">
      <Search class="w-4 h-4 text-zinc-500 absolute left-3.5 top-1/2 -translate-y-1/2" />
      <input
        type="text"
        bind:value={query}
        on:input={loadProblems}
        placeholder="Search problems by title, ID, or tag..."
        class="w-full pl-10 pr-4 py-2.5 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-indigo-500 focus:outline-none text-zinc-100 placeholder-zinc-500 text-sm transition"
      />
    </div>

    <div>
      <select
        bind:value={selectedPlatform}
        on:change={loadProblems}
        class="w-full px-4 py-2.5 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-indigo-500 focus:outline-none text-zinc-100 text-sm transition"
      >
        <option value="">All Platforms</option>
        <option value="CODEFORCES">Codeforces</option>
        <option value="ATCODER">AtCoder</option>
        <option value="LEETCODE">LeetCode</option>
      </select>
    </div>
  </div>

  <!-- Problems List -->
  {#if loading}
    <div class="space-y-3">
      {#each Array(5) as _}
        <div class="h-20 rounded-xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      {/each}
    </div>
  {:else if problems.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-4">
      <p class="text-zinc-400 text-base">No problems found matching your criteria.</p>
      <button
        on:click={() => (showImportModal = true)}
        class="px-4 py-2 rounded-xl text-sm font-semibold bg-indigo-600 hover:bg-indigo-500 text-white transition inline-flex items-center space-x-1.5"
      >
        <Plus class="w-4 h-4" />
        <span>Import a Problem</span>
      </button>
    </div>
  {:else}
    <div class="space-y-3">
      {#each problems as prob}
        <ProblemCard problem={prob} />
      {/each}
    </div>
  {/if}

  <!-- Import Modal -->
  {#if showImportModal}
    <div class="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
      <div class="max-w-lg w-full rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl space-y-5">
        <div class="space-y-1">
          <h3 class="text-xl font-bold text-white">Import Problem by URL</h3>
          <p class="text-xs text-zinc-400">
            Paste any supported URL from Codeforces, AtCoder, or LeetCode.
          </p>
        </div>

        {#if importError}
          <div class="p-3 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm flex items-center space-x-2">
            <AlertCircle class="w-4 h-4 shrink-0" />
            <span>{importError}</span>
          </div>
        {/if}

        {#if importSuccess}
          <div class="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center space-x-2">
            <CheckCircle2 class="w-4 h-4 shrink-0" />
            <span>{importSuccess}</span>
          </div>
        {/if}

        <div class="space-y-2">
          <label for="import-url" class="block text-xs font-semibold uppercase text-zinc-400">Official URL</label>
          <input
            id="import-url"
            type="url"
            bind:value={importUrl}
            placeholder="https://codeforces.com/problemset/problem/1900/A"
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-indigo-500 focus:outline-none text-zinc-100 text-sm placeholder-zinc-600 transition"
          />
          <div class="text-[11px] text-zinc-500 space-y-0.5">
            <div>Examples:</div>
            <div>• https://codeforces.com/problemset/problem/1900/A</div>
            <div>• https://atcoder.jp/contests/abc350/tasks/abc350_f</div>
            <div>• https://leetcode.com/problems/two-sum/</div>
          </div>
        </div>

        <div class="flex items-center justify-end space-x-3 pt-2">
          <button
            on:click={() => (showImportModal = false)}
            class="px-4 py-2 rounded-xl text-sm font-semibold text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition"
          >
            Cancel
          </button>
          <button
            on:click={handleImport}
            disabled={importLoading || !importUrl}
            class="px-4 py-2 rounded-xl text-sm font-semibold bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white transition flex items-center space-x-2"
          >
            <span>{importLoading ? 'Fetching...' : 'Import'}</span>
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
