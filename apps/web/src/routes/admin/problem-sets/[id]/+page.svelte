<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import type { ProblemSet, Problem, ProblemSetItem } from '@cpbridge/contracts';
  import { Plus, Trash2, ArrowUp, ArrowDown, ArrowLeft, Trophy, Search, X, Save, Check } from 'lucide-svelte';

  let setId = $page.params.id;
  let set: ProblemSet | null = null;
  let items: ProblemSetItem[] = [];
  let loading = true;
  let error = '';
  let successMsg = '';

  // Metadata form
  let editName = '';
  let editDescription = '';
  let editVisibility: any = 'PUBLIC';
  let savingMeta = false;

  // Add Problem Modal
  let showAddModal = false;
  let searchProblems: Problem[] = [];
  let searchQuery = '';
  let searching = false;

  async function loadSet() {
    loading = true;
    error = '';
    try {
      set = await api.get<ProblemSet>(`/admin/problem-sets/${setId}`);
      items = set.items || [];
      editName = set.name;
      editDescription = set.description;
      editVisibility = set.visibility;
    } catch (err: any) {
      error = err.message || 'Failed to load problem set';
    } finally {
      loading = false;
    }
  }

  async function saveMetadata() {
    if (!editName.trim()) return;
    savingMeta = true;
    try {
      await api.patch(`/admin/problem-sets/${setId}`, {
        name: editName.trim(),
        description: editDescription.trim(),
        visibility: editVisibility
      });
      successMsg = 'Problem set updated successfully!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadSet();
    } catch (err: any) {
      alert(err.message || 'Failed to update metadata');
    } finally {
      savingMeta = false;
    }
  }

  async function searchProblemLibrary() {
    searching = true;
    try {
      const res = await api.get<{ problems: Problem[] }>(`/admin/problems?query=${encodeURIComponent(searchQuery)}&limit=20`);
      searchProblems = res.problems;
    } catch (err) {
      console.error(err);
    } finally {
      searching = false;
    }
  }

  async function handleAddProblem(prob: Problem) {
    try {
      await api.post(`/admin/problem-sets/${setId}/problems`, {
        problemId: prob.id,
        position: items.length
      });
      showAddModal = false;
      searchQuery = '';
      successMsg = `Added problem "${prob.title}"!`;
      setTimeout(() => (successMsg = ''), 4000);
      await loadSet();
    } catch (err: any) {
      alert(err.message || 'Failed to add problem');
    }
  }

  async function handleRemoveProblem(problemId: string) {
    try {
      await api.delete(`/admin/problem-sets/${setId}/problems/${problemId}`);
      await loadSet();
    } catch (err: any) {
      alert(err.message || 'Failed to remove problem');
    }
  }

  async function moveItem(index: number, direction: 'up' | 'down') {
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= items.length) return;

    const newItems = [...items];
    const temp = newItems[index];
    newItems[index] = newItems[targetIndex];
    newItems[targetIndex] = temp;
    items = newItems;

    const pids = items.map((it) => it.problemId);
    try {
      await api.patch(`/admin/problem-sets/${setId}/order`, { problemIds: pids });
    } catch (err: any) {
      alert(err.message || 'Failed to reorder problems');
      await loadSet();
    }
  }

  onMount(() => {
    loadSet();
  });
</script>

{#if loading}
  <div class="h-96 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
{:else if error || !set}
  <div class="p-8 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300">
    <h2 class="font-bold text-lg">Error</h2>
    <p class="text-sm">{error || 'Problem set not found'}</p>
  </div>
{:else}
  <div class="space-y-6">
    <!-- Top breadcrumb & Actions -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="flex items-center space-x-3">
        <a
          href="/admin/problem-sets"
          class="p-2 rounded-xl text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
        >
          <ArrowLeft class="w-4 h-4" />
        </a>
        <div>
          <h1 class="text-2xl font-bold text-white">{set.name}</h1>
          <p class="text-xs text-zinc-400">Edit problem set metadata and problem ordering.</p>
        </div>
      </div>

      <div class="flex items-center space-x-2">
        <a
          href={`/admin/contests/new?setId=${set.id}`}
          class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black transition flex items-center space-x-1.5 shadow-sm"
        >
          <Trophy class="w-4 h-4" />
          <span>Create Contest from Set</span>
        </a>
      </div>
    </div>

    {#if successMsg}
      <div class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center space-x-2">
        <Check class="w-4 h-4 text-emerald-400" />
        <span>{successMsg}</span>
      </div>
    {/if}

    <!-- Metadata Form Card -->
    <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 space-y-4">
      <h2 class="text-base font-bold text-white">Problem Set Details</h2>
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div class="sm:col-span-2">
          <label for="edit-set-name" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Set Name</label>
          <input
            id="edit-set-name"
            type="text"
            bind:value={editName}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="edit-set-visibility" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Visibility</label>
          <select
            id="edit-set-visibility"
            bind:value={editVisibility}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            <option value="PUBLIC">PUBLIC</option>
            <option value="UNLISTED">UNLISTED</option>
            <option value="PRIVATE">PRIVATE</option>
          </select>
        </div>

        <div class="sm:col-span-3">
          <label for="edit-set-desc" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Description</label>
          <textarea
            id="edit-set-desc"
            bind:value={editDescription}
            rows="2"
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          ></textarea>
        </div>
      </div>

      <div class="flex justify-end pt-2">
        <button
          on:click={saveMetadata}
          disabled={savingMeta || !editName.trim()}
          class="px-4 py-1.5 rounded-xl text-xs font-bold bg-zinc-800 hover:bg-zinc-700 text-white transition flex items-center space-x-1.5"
        >
          <Save class="w-3.5 h-3.5" />
          <span>{savingMeta ? 'Saving...' : 'Save Details'}</span>
        </button>
      </div>
    </div>

    <!-- Problems List Card -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-bold text-white flex items-center space-x-2">
          <span>Problems in Set</span>
          <span class="text-xs font-mono text-zinc-400">({items.length})</span>
        </h2>

        <button
          on:click={() => { showAddModal = true; searchProblemLibrary(); }}
          class="px-3.5 py-1.5 rounded-xl text-xs font-bold bg-white text-black hover:bg-zinc-200 transition flex items-center space-x-1 shadow-sm"
        >
          <Plus class="w-3.5 h-3.5" />
          <span>Add Problem</span>
        </button>
      </div>

      {#if items.length === 0}
        <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
          <p class="text-zinc-400 text-sm">No problems added to this set yet.</p>
          <button
            on:click={() => { showAddModal = true; searchProblemLibrary(); }}
            class="px-4 py-2 rounded-xl text-xs font-bold bg-white text-black hover:bg-zinc-200 transition"
          >
            Add Problems from Library
          </button>
        </div>
      {:else}
        <div class="rounded-2xl border border-zinc-800 bg-zinc-900/40 divide-y divide-zinc-800/60 overflow-hidden">
          {#each items as item, index}
            <div class="p-4 flex items-center justify-between hover:bg-zinc-800/20 transition">
              <div class="flex items-center space-x-4 min-w-0">
                <span class="w-7 h-7 rounded-lg bg-zinc-800 border border-zinc-700 text-zinc-300 text-xs font-mono font-bold flex items-center justify-center shrink-0">
                  {index + 1}
                </span>

                <div class="space-y-0.5 min-w-0">
                  <div class="flex items-center space-x-2">
                    {#if item.problem?.platform}
                      <span class="text-[10px] px-1.5 py-0.2 rounded font-mono font-bold {
                        item.problem.platform === 'CODEFORCES' ? 'bg-blue-500/15 text-blue-300 border border-blue-500/30' : 'bg-red-500/15 text-red-300 border border-red-500/30'
                      }">
                        {item.problem.platform}
                      </span>
                    {/if}
                    <span class="text-sm font-semibold text-white truncate">{item.problem?.title || item.problemId}</span>
                  </div>
                  <div class="text-xs text-zinc-500 font-mono">
                    {item.problem?.externalId}
                    {#if item.problem?.difficulty}
                      • Difficulty: {item.problem.difficulty}
                    {/if}
                  </div>
                </div>
              </div>

              <!-- Reorder and Delete -->
              <div class="flex items-center space-x-1 shrink-0">
                <button
                  on:click={() => moveItem(index, 'up')}
                  disabled={index === 0}
                  class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 disabled:opacity-30 transition"
                  title="Move Up"
                >
                  <ArrowUp class="w-4 h-4" />
                </button>
                <button
                  on:click={() => moveItem(index, 'down')}
                  disabled={index === items.length - 1}
                  class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 disabled:opacity-30 transition"
                  title="Move Down"
                >
                  <ArrowDown class="w-4 h-4" />
                </button>
                <button
                  on:click={() => handleRemoveProblem(item.problemId)}
                  class="p-1.5 rounded-lg text-zinc-400 hover:text-rose-400 hover:bg-rose-500/10 transition ml-2"
                  title="Remove from Set"
                >
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  </div>
{/if}

<!-- Add Problem Modal -->
{#if showAddModal}
  <div class="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
    <div class="w-full max-w-lg bg-zinc-900 border border-zinc-800 rounded-2xl p-6 shadow-2xl space-y-4 max-h-[85vh] flex flex-col">
      <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
        <h3 class="font-bold text-white text-lg">Add Problem to Set</h3>
        <button on:click={() => (showAddModal = false)} class="text-zinc-500 hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Search input -->
      <div class="relative">
        <Search class="w-4 h-4 absolute left-3 top-3 text-zinc-500" />
        <input
          type="text"
          bind:value={searchQuery}
          on:input={searchProblemLibrary}
          placeholder="Search problems library..."
          class="w-full pl-9 pr-4 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
        />
      </div>

      <!-- Problem list -->
      <div class="flex-1 overflow-y-auto divide-y divide-zinc-800/60 pr-1">
        {#if searching}
          <div class="p-6 text-center text-zinc-500 text-xs animate-pulse">Searching problems...</div>
        {:else if searchProblems.length === 0}
          <div class="p-6 text-center text-zinc-500 text-xs">No problems found matching search.</div>
        {:else}
          {#each searchProblems as p}
            {@const isAlreadyIn = items.some((it) => it.problemId === p.id)}
            <div class="py-3 flex items-center justify-between gap-2">
              <div class="space-y-0.5 min-w-0">
                <div class="flex items-center space-x-2">
                  <span class="text-[10px] px-1.5 py-0.2 rounded font-mono font-bold {
                    p.platform === 'CODEFORCES' ? 'bg-blue-500/15 text-blue-300 border border-blue-500/30' : 'bg-red-500/15 text-red-300 border border-red-500/30'
                  }">
                    {p.platform}
                  </span>
                  <span class="font-semibold text-white text-xs truncate">{p.title}</span>
                </div>
                <div class="text-[11px] text-zinc-500 font-mono">
                  {p.externalId} {#if p.difficulty}• {p.difficulty}{/if}
                </div>
              </div>

              {#if isAlreadyIn}
                <span class="text-xs text-zinc-500 font-medium px-2 py-1">Added</span>
              {:else}
                <button
                  on:click={() => handleAddProblem(p)}
                  class="px-3 py-1 rounded-lg text-xs font-bold bg-white text-black hover:bg-zinc-200 transition shrink-0"
                >
                  Add
                </button>
              {/if}
            </div>
          {/each}
        {/if}
      </div>

      <div class="pt-2 border-t border-zinc-800 flex justify-end">
        <button
          on:click={() => (showAddModal = false)}
          class="px-4 py-1.5 rounded-xl text-xs font-semibold text-zinc-400 hover:text-white"
        >
          Close
        </button>
      </div>
    </div>
  </div>
{/if}
