<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { Problem, PlatformType } from '@cpbridge/contracts';
  import { Plus, Upload, Search, Trash2, ExternalLink, Eye, X, Edit3, Check } from 'lucide-svelte';

  let problems: Problem[] = [];
  let _total = 0;
  let loading = true;
  let error = '';
  let successMsg = '';

  // Filter
  let query = '';
  let selectedPlatform = '';
  let limit = 50;
  let offset = 0;

  // Modals
  let showImportModal = false;
  let importUrl = '';
  let importing = false;
  let importError = '';

  let showCreateModal = false;
  let createReq = {
    title: '',
    platform: 'CODEFORCES' as PlatformType,
    externalId: '',
    url: '',
    difficulty: null as number | null,
    statement: '',
    timeLimit: '1.0s',
    memoryLimit: '256MB',
    tags: 'greedy, math'
  };
  let creating = false;
  let createError = '';

  let showEditModal = false;
  let editingProblem: Problem | null = null;
  let editForm = {
    title: '',
    url: '',
    difficulty: null as number | null,
    tags: ''
  };
  let updating = false;
  let editError = '';

  async function loadProblems() {
    loading = true;
    error = '';
    try {
      let url = `/admin/problems?limit=${limit}&offset=${offset}`;
      if (query.trim()) url += `&query=${encodeURIComponent(query.trim())}`;
      if (selectedPlatform) url += `&platform=${selectedPlatform}`;

      const res = await api.get<{ problems: Problem[]; total: number }>(url);
      problems = res.problems;
      _total = res.total;
    } catch (err: any) {
      error = err.message || 'Failed to load problems';
    } finally {
      loading = false;
    }
  }

  async function handleImport() {
    if (!importUrl.trim()) return;
    importing = true;
    importError = '';
    try {
      await api.post<Problem>('/admin/problems/import', { url: importUrl.trim() });
      showImportModal = false;
      importUrl = '';
      successMsg = 'Problem imported successfully!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadProblems();
    } catch (err: any) {
      importError = err.message || 'Failed to import problem';
    } finally {
      importing = false;
    }
  }

  async function handleCreateCustom() {
    if (!createReq.title.trim()) {
      createError = 'Title is required';
      return;
    }
    creating = true;
    createError = '';
    try {
      const tagList = createReq.tags.split(',').map((t) => t.trim()).filter(Boolean);
      await api.post<Problem>('/admin/problems', {
        ...createReq,
        tags: tagList
      });
      showCreateModal = false;
      createReq = {
        title: '',
        platform: 'CODEFORCES',
        externalId: '',
        url: '',
        difficulty: null,
        statement: '',
        timeLimit: '1.0s',
        memoryLimit: '256MB',
        tags: 'greedy, math'
      };
      successMsg = 'Custom problem created successfully!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadProblems();
    } catch (err: any) {
      createError = err.message || 'Failed to create problem';
    } finally {
      creating = false;
    }
  }

  function openEdit(p: Problem) {
    editingProblem = p;
    editForm = {
      title: p.title,
      url: p.url,
      difficulty: p.difficulty,
      tags: p.tags.join(', ')
    };
    editError = '';
    showEditModal = true;
  }

  async function handleUpdate() {
    if (!editingProblem) return;
    updating = true;
    editError = '';
    try {
      const tagList = editForm.tags.split(',').map((t) => t.trim()).filter(Boolean);
      await api.patch(`/admin/problems/${editingProblem.id}`, {
        title: editForm.title,
        url: editForm.url,
        difficulty: editForm.difficulty ? Number(editForm.difficulty) : null,
        tags: tagList
      });
      showEditModal = false;
      editingProblem = null;
      successMsg = 'Problem updated successfully!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadProblems();
    } catch (err: any) {
      editError = err.message || 'Failed to update problem';
    } finally {
      updating = false;
    }
  }

  async function handleDelete(p: Problem) {
    if (!confirm(`Are you sure you want to delete problem "${p.title}"?`)) return;
    try {
      await api.delete(`/admin/problems/${p.id}`);
      successMsg = 'Problem deleted successfully!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadProblems();
    } catch (err: any) {
      if (err.message === 'PROBLEM_IN_USE') {
        alert('Cannot delete this problem because it is currently used in an active or scheduled contest.');
      } else {
        alert(err.message || 'Failed to delete problem');
      }
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
      <h1 class="text-2xl font-bold text-white">Problem Library</h1>
      <p class="text-sm text-zinc-400">Manage, import, and organize problems available for contests.</p>
    </div>

    <div class="flex items-center space-x-2">
      <button
        on:click={() => (showImportModal = true)}
        class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black transition flex items-center space-x-1.5 shadow-sm"
      >
        <Upload class="w-4 h-4" />
        <span>Import Problem</span>
      </button>
      <button
        on:click={() => (showCreateModal = true)}
        class="px-4 py-2 rounded-xl text-sm font-semibold border border-zinc-700 bg-zinc-900/80 hover:bg-zinc-800 text-white transition flex items-center space-x-1.5"
      >
        <Plus class="w-4 h-4" />
        <span>Create Problem</span>
      </button>
    </div>
  </div>

  {#if successMsg}
    <div class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center space-x-2">
      <Check class="w-4 h-4 text-emerald-400" />
      <span>{successMsg}</span>
    </div>
  {/if}

  <!-- Filters -->
  <div class="flex flex-col sm:flex-row items-center gap-3">
    <div class="relative flex-1 w-full">
      <Search class="w-4 h-4 absolute left-3.5 top-3 text-zinc-500" />
      <input
        type="text"
        bind:value={query}
        on:input={() => { offset = 0; loadProblems(); }}
        placeholder="Search problems by title, index, or external ID..."
        class="w-full pl-10 pr-4 py-2 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-zinc-500 focus:outline-none text-zinc-100 text-sm placeholder-zinc-500"
      />
    </div>

    <select
      bind:value={selectedPlatform}
      on:change={() => { offset = 0; loadProblems(); }}
      class="px-4 py-2 rounded-xl bg-zinc-900/60 border border-zinc-800 focus:border-zinc-500 focus:outline-none text-zinc-200 text-sm w-full sm:w-auto"
    >
      <option value="">All Platforms</option>
      <option value="CODEFORCES">Codeforces</option>
      <option value="ATCODER">AtCoder</option>
    </select>
  </div>

  <!-- Problems Table -->
  {#if loading}
    <div class="h-64 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
  {:else if error}
    <div class="p-6 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm">
      {error}
    </div>
  {:else if problems.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
      <p class="text-zinc-400 text-sm">No problems found.</p>
      <button
        on:click={() => (showImportModal = true)}
        class="px-4 py-2 rounded-xl text-xs font-bold bg-white text-black hover:bg-zinc-200 transition"
      >
        Import your first problem
      </button>
    </div>
  {:else}
    <div class="rounded-2xl border border-zinc-800 bg-zinc-900/40 overflow-hidden">
      <table class="w-full text-left text-sm text-zinc-300">
        <thead class="bg-zinc-900/80 border-b border-zinc-800 text-xs text-zinc-400 uppercase font-semibold">
          <tr>
            <th class="px-5 py-3.5">Platform</th>
            <th class="px-5 py-3.5">Problem</th>
            <th class="px-5 py-3.5">Difficulty</th>
            <th class="px-5 py-3.5">Tags</th>
            <th class="px-5 py-3.5 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-zinc-800/60 font-medium">
          {#each problems as p}
            <tr class="hover:bg-zinc-800/30 transition">
              <td class="px-5 py-3.5 whitespace-nowrap">
                <span class="text-xs px-2.5 py-0.5 rounded font-mono font-bold {
                  p.platform === 'CODEFORCES' ? 'bg-blue-500/15 text-blue-300 border border-blue-500/30' : 'bg-red-500/15 text-red-300 border border-red-500/30'
                }">
                  {p.platform}
                </span>
              </td>
              <td class="px-5 py-3.5">
                <div class="space-y-0.5">
                  <div class="text-white font-semibold flex items-center space-x-1.5">
                    <span>{p.title}</span>
                    {#if p.url}
                      <a href={p.url} target="_blank" class="text-zinc-500 hover:text-zinc-300" title="Official link">
                        <ExternalLink class="w-3 h-3" />
                      </a>
                    {/if}
                  </div>
                  <div class="text-xs font-mono text-zinc-500">{p.externalId}</div>
                </div>
              </td>
              <td class="px-5 py-3.5 whitespace-nowrap">
                {#if p.difficulty}
                  <span class="text-xs font-mono px-2 py-0.5 rounded bg-zinc-800 text-zinc-300 border border-zinc-700">
                    {p.difficulty}
                  </span>
                {:else}
                  <span class="text-xs text-zinc-600">—</span>
                {/if}
              </td>
              <td class="px-5 py-3.5">
                <div class="flex flex-wrap gap-1 max-w-xs">
                  {#each p.tags.slice(0, 3) as tag}
                    <span class="text-[11px] px-1.5 py-0.2 rounded bg-zinc-950 text-zinc-400 border border-zinc-800">
                      {tag}
                    </span>
                  {/each}
                  {#if p.tags.length > 3}
                    <span class="text-[11px] text-zinc-500">+{p.tags.length - 3}</span>
                  {/if}
                </div>
              </td>
              <td class="px-5 py-3.5 text-right whitespace-nowrap">
                <div class="flex items-center justify-end space-x-2">
                  <a
                    href={`/problems/${p.id}`}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
                    title="View Problem"
                  >
                    <Eye class="w-4 h-4" />
                  </a>
                  <button
                    on:click={() => openEdit(p)}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
                    title="Edit Metadata"
                  >
                    <Edit3 class="w-4 h-4" />
                  </button>
                  <button
                    on:click={() => handleDelete(p)}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                    title="Delete Problem"
                  >
                    <Trash2 class="w-4 h-4" />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<!-- Import Problem Modal -->
{#if showImportModal}
  <div class="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
    <div class="w-full max-w-md bg-zinc-900 border border-zinc-800 rounded-2xl p-6 shadow-2xl space-y-4">
      <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
        <h3 class="font-bold text-white text-lg">Import Problem</h3>
        <button on:click={() => (showImportModal = false)} class="text-zinc-500 hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      {#if importError}
        <div class="p-3 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-xs">
          {importError}
        </div>
      {/if}

      <div class="space-y-3">
        <div>
          <label for="import-url" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Problem URL</label>
          <input
            id="import-url"
            type="text"
            bind:value={importUrl}
            placeholder="e.g. https://codeforces.com/problemset/problem/1900/A"
            class="w-full px-3.5 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm"
          />
          <p class="text-[11px] text-zinc-500 mt-1">
            Supported platforms: Codeforces, AtCoder
          </p>
        </div>
      </div>

      <div class="flex items-center justify-end space-x-2 pt-2 border-t border-zinc-800">
        <button
          on:click={() => (showImportModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold text-zinc-400 hover:text-white"
        >
          Cancel
        </button>
        <button
          on:click={handleImport}
          disabled={importing || !importUrl.trim()}
          class="px-5 py-2 rounded-xl text-xs font-bold bg-white hover:bg-zinc-200 text-black disabled:opacity-50 transition"
        >
          {importing ? 'Importing...' : 'Import'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Create Custom Problem Modal -->
{#if showCreateModal}
  <div class="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
    <div class="w-full max-w-lg bg-zinc-900 border border-zinc-800 rounded-2xl p-6 shadow-2xl space-y-4 max-h-[90vh] overflow-y-auto">
      <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
        <h3 class="font-bold text-white text-lg">Create Custom Problem</h3>
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
          <label for="create-title" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Title</label>
          <input
            id="create-title"
            type="text"
            bind:value={createReq.title}
            placeholder="e.g. Array Summation"
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label for="create-platform" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Platform</label>
            <select
              id="create-platform"
              bind:value={createReq.platform}
              class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
            >
              <option value="CODEFORCES">Codeforces</option>
              <option value="ATCODER">AtCoder</option>
            </select>
          </div>

          <div>
            <label for="create-difficulty" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Difficulty (Optional)</label>
            <input
              id="create-difficulty"
              type="number"
              bind:value={createReq.difficulty}
              placeholder="e.g. 1200"
              class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
            />
          </div>
        </div>

        <div>
          <label for="create-tags" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Tags (Comma-separated)</label>
          <input
            id="create-tags"
            type="text"
            bind:value={createReq.tags}
            placeholder="greedy, dp, math"
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="create-statement" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Statement</label>
          <textarea
            id="create-statement"
            bind:value={createReq.statement}
            rows="4"
            placeholder="Problem statement text or HTML..."
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          ></textarea>
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
          on:click={handleCreateCustom}
          disabled={creating || !createReq.title.trim()}
          class="px-5 py-2 rounded-xl text-xs font-bold bg-white hover:bg-zinc-200 text-black disabled:opacity-50 transition"
        >
          {creating ? 'Creating...' : 'Create Problem'}
        </button>
      </div>
    </div>
  </div>
{/if}

<!-- Edit Metadata Modal -->
{#if showEditModal && editingProblem}
  <div class="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
    <div class="w-full max-w-md bg-zinc-900 border border-zinc-800 rounded-2xl p-6 shadow-2xl space-y-4">
      <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
        <h3 class="font-bold text-white text-lg">Edit Problem Metadata</h3>
        <button on:click={() => (showEditModal = false)} class="text-zinc-500 hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      {#if editError}
        <div class="p-3 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-xs">
          {editError}
        </div>
      {/if}

      <div class="space-y-3 text-sm">
        <div>
          <label for="edit-title" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Title</label>
          <input
            id="edit-title"
            type="text"
            bind:value={editForm.title}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="edit-url" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Official URL</label>
          <input
            id="edit-url"
            type="text"
            bind:value={editForm.url}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="edit-difficulty" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Difficulty</label>
          <input
            id="edit-difficulty"
            type="number"
            bind:value={editForm.difficulty}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="edit-tags" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Tags (Comma-separated)</label>
          <input
            id="edit-tags"
            type="text"
            bind:value={editForm.tags}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>
      </div>

      <div class="flex items-center justify-end space-x-2 pt-2 border-t border-zinc-800">
        <button
          on:click={() => (showEditModal = false)}
          class="px-4 py-2 rounded-xl text-xs font-semibold text-zinc-400 hover:text-white"
        >
          Cancel
        </button>
        <button
          on:click={handleUpdate}
          disabled={updating || !editForm.title.trim()}
          class="px-5 py-2 rounded-xl text-xs font-bold bg-white hover:bg-zinc-200 text-black disabled:opacity-50 transition"
        >
          {updating ? 'Saving...' : 'Save Changes'}
        </button>
      </div>
    </div>
  </div>
{/if}
