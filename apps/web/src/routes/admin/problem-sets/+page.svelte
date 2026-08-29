<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import type { ImportContestResult, ProblemSet } from '@cpbridge/contracts';
  import { Plus, Trash2, ArrowRight, X, Check, Download } from 'lucide-svelte';

  let problemSets: ProblemSet[] = [];
  let loading = true;
  let error = '';
  let successMsg = '';

  // Create Modal
  let showCreateModal = false;
  let newName = '';
  let newDescription = '';
  let newVisibility = 'PUBLIC';
  let creating = false;
  let createError = '';

  // External contest import modal
  let showImportModal = false;
  let contestUrl = '';
  let importName = '';
  let importDescription = '';
  let importVisibility: ProblemSet['visibility'] = 'PUBLIC';
  let importing = false;
  let importError = '';

  async function loadProblemSets() {
    loading = true;
    error = '';
    try {
      problemSets = await api.get<ProblemSet[]>('/admin/problem-sets');
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load problem sets';
    } finally {
      loading = false;
    }
  }

  async function handleCreate() {
    if (!newName.trim()) {
      createError = 'Name is required';
      return;
    }
    creating = true;
    createError = '';
    try {
      await api.post<ProblemSet>('/admin/problem-sets', {
        name: newName.trim(),
        description: newDescription.trim(),
        visibility: newVisibility
      });
      showCreateModal = false;
      newName = '';
      newDescription = '';
      successMsg = 'Problem Set created successfully!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadProblemSets();
    } catch (err) {
      createError = err instanceof Error ? err.message : 'Failed to create problem set';
    } finally {
      creating = false;
    }
  }

  async function handleDelete(ps: ProblemSet) {
    if (!confirm(`Are you sure you want to delete Problem Set "${ps.name}"?`)) return;
    try {
      await api.delete(`/admin/problem-sets/${ps.id}`);
      successMsg = 'Problem Set deleted!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadProblemSets();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to delete problem set');
    }
  }

  async function handleContestImport() {
    if (!contestUrl.trim()) {
      importError = 'A contest URL is required';
      return;
    }
    importing = true;
    importError = '';
    try {
      const result = await api.post<ImportContestResult>('/admin/problem-sets/import', {
        contestUrl: contestUrl.trim(),
        name: importName.trim(),
        description: importDescription.trim(),
        visibility: importVisibility
      });
      showImportModal = false;
      await goto(`/admin/problem-sets/${result.problemSet.id}`);
    } catch (err) {
      importError = err instanceof Error ? err.message : 'Failed to import contest';
    } finally {
      importing = false;
    }
  }

  onMount(() => {
    loadProblemSets();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold text-white">Problem Sets</h1>
      <p class="text-sm text-zinc-400">Curate and group problems into sets that can be snapshotted into contests.</p>
    </div>

    <div class="flex flex-wrap items-center gap-2 self-start sm:self-auto">
      <button
        on:click={() => (showImportModal = true)}
        class="px-4 py-2 rounded-xl text-sm font-bold bg-zinc-800 hover:bg-zinc-700 text-white transition flex items-center space-x-1.5 shadow-sm"
      >
        <Download class="w-4 h-4" />
        <span>Import Contest</span>
      </button>
      <button
        on:click={() => (showCreateModal = true)}
        class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black transition flex items-center space-x-1.5 shadow-sm"
      >
        <Plus class="w-4 h-4" />
        <span>Create Problem Set</span>
      </button>
    </div>
  </div>

  {#if successMsg}
    <div class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center space-x-2">
      <Check class="w-4 h-4 text-emerald-400" />
      <span>{successMsg}</span>
    </div>
  {/if}

  {#if loading}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      {#each Array(6) as _}
        <div class="h-40 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      {/each}
    </div>
  {:else if error}
    <div class="p-6 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm">
      {error}
    </div>
  {:else if problemSets.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
      <p class="text-zinc-400 text-sm">No problem sets created yet.</p>
      <button
        on:click={() => (showCreateModal = true)}
        class="px-4 py-2 rounded-xl text-xs font-bold bg-white text-black hover:bg-zinc-200 transition"
      >
        Create your first Problem Set
      </button>
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      {#each problemSets as ps}
        <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:border-zinc-700 transition flex flex-col justify-between space-y-4">
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs px-2 py-0.5 rounded-md font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                {ps.visibility}
              </span>
              <span class="text-xs text-zinc-400">{ps.problemCount} problem{ps.problemCount === 1 ? '' : 's'}</span>
            </div>

            <h3 class="text-lg font-bold text-white truncate">{ps.name}</h3>
            <p class="text-xs text-zinc-400 line-clamp-2">{ps.description || 'No description provided.'}</p>
          </div>

          <div class="flex items-center justify-between pt-3 border-t border-zinc-800/80">
            <button
              on:click={() => handleDelete(ps)}
              class="p-1.5 rounded-lg text-zinc-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
              title="Delete Set"
            >
              <Trash2 class="w-4 h-4" />
            </button>

            <a
              href={`/admin/problem-sets/${ps.id}`}
              class="px-3.5 py-1.5 rounded-lg text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-white transition flex items-center space-x-1"
            >
              <span>Manage Problems</span>
              <ArrowRight class="w-3 h-3" />
            </a>
          </div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<!-- Create Modal -->
{#if showCreateModal}
  <div class="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
    <div class="w-full max-w-md bg-zinc-900 border border-zinc-800 rounded-2xl p-6 shadow-2xl space-y-4">
      <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
        <h3 class="font-bold text-white text-lg">Create Problem Set</h3>
        <button on:click={() => (showCreateModal = false)} class="text-zinc-500 hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      {#if createError}
        <div class="p-3 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-xs">
          {createError}
        </div>
      {/if}

      <div class="space-y-3 text-sm">
        <div>
          <label for="set-name" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Set Name</label>
          <input
            id="set-name"
            type="text"
            bind:value={newName}
            placeholder="e.g. DP & Graphs Practice Set"
            class="w-full px-3.5 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="set-desc" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Description (Optional)</label>
          <textarea
            id="set-desc"
            bind:value={newDescription}
            rows="3"
            placeholder="Topic overview, difficulty level..."
            class="w-full px-3.5 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          ></textarea>
        </div>

        <div>
          <label for="set-visibility" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Visibility</label>
          <select
            id="set-visibility"
            bind:value={newVisibility}
            class="w-full px-3.5 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            <option value="PUBLIC">PUBLIC</option>
            <option value="UNLISTED">UNLISTED</option>
            <option value="PRIVATE">PRIVATE</option>
          </select>
        </div>
      </div>

      <div class="flex items-center justify-end space-x-2 pt-2 border-t border-zinc-800">
        <button
          on:click={() => (showCreateModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold text-zinc-400 hover:text-white"
        >
          Cancel
        </button>
        <button
          on:click={handleCreate}
          disabled={creating || !newName.trim()}
          class="px-5 py-2 rounded-xl text-xs font-bold bg-white hover:bg-zinc-200 text-black disabled:opacity-50 transition"
        >
          {creating ? 'Creating...' : 'Create Set'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Import Contest Modal -->
{#if showImportModal}
  <div class="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
    <div class="w-full max-w-lg bg-zinc-900 border border-zinc-800 rounded-2xl p-6 shadow-2xl space-y-4">
      <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
        <div>
          <h3 class="font-bold text-white text-lg">Import Contest</h3>
          <p class="text-xs text-zinc-400 mt-1">Creates an ordered problem set from a public, revealed contest.</p>
        </div>
        <button on:click={() => (showImportModal = false)} class="text-zinc-500 hover:text-white" aria-label="Close import modal">
          <X class="w-5 h-5" />
        </button>
      </div>

      {#if importError}
        <div class="p-3 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-xs">
          {importError}
        </div>
      {/if}

      <div class="space-y-3 text-sm">
        <div>
          <label for="contest-url" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Contest URL</label>
          <input
            id="contest-url"
            type="text"
            bind:value={contestUrl}
            placeholder="e.g. https://codeforces.com/contest/1931, https://codeforces.com/gym/105053, or https://atcoder.jp/contests/abc350"
            class="w-full px-3.5 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="import-set-name" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Set Name (Optional)</label>
          <input
            id="import-set-name"
            type="text"
            bind:value={importName}
            placeholder="Uses the official contest name"
            class="w-full px-3.5 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="import-set-description" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Description (Optional)</label>
          <textarea
            id="import-set-description"
            bind:value={importDescription}
            rows="2"
            placeholder="Defaults to the official contest source link"
            class="w-full px-3.5 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          ></textarea>
        </div>

        <div>
          <label for="import-set-visibility" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Visibility</label>
          <select
            id="import-set-visibility"
            bind:value={importVisibility}
            class="w-full px-3.5 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            <option value="PUBLIC">PUBLIC</option>
            <option value="UNLISTED">UNLISTED</option>
            <option value="PRIVATE">PRIVATE</option>
          </select>
        </div>
      </div>

      <div class="flex items-center justify-end space-x-2 pt-2 border-t border-zinc-800">
        <button
          on:click={() => (showImportModal = false)}
          disabled={importing}
          class="px-4 py-2 rounded-xl text-xs font-semibold text-zinc-400 hover:text-white disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          on:click={handleContestImport}
          disabled={importing || !contestUrl.trim()}
          class="px-5 py-2 rounded-xl text-xs font-bold bg-white hover:bg-zinc-200 text-black disabled:opacity-50 transition flex items-center space-x-1.5"
        >
          <Download class="w-3.5 h-3.5" />
          <span>{importing ? 'Importing...' : 'Import Contest'}</span>
        </button>
      </div>
    </div>
  </div>
{/if}
