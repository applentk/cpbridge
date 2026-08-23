<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { ProblemSet, Problem } from '@cp-hub/contracts';
  import ProblemCard from '$lib/components/ProblemCard.svelte';
  import { Layers, Plus, Trophy, Trash2, ArrowUp, ArrowDown, ExternalLink } from 'lucide-svelte';

  let setId = $page.params.id;
  let problemSet: ProblemSet | null = null;
  let loading = true;
  let error = '';

  let showAddModal = false;
  let allProblems: Problem[] = [];
  let selectedProblemId = '';

  async function loadSet() {
    loading = true;
    try {
      problemSet = await api.get<ProblemSet>(`/problem-sets/${setId}`);
    } catch (err: any) {
      error = err.message || 'Failed to load problem set';
    } finally {
      loading = false;
    }
  }

  async function openAddModal() {
    showAddModal = true;
    try {
      const res = await api.get<{ problems: Problem[] }>('/problems?limit=100');
      allProblems = res.problems || [];
      if (allProblems.length > 0) {
        selectedProblemId = allProblems[0].id;
      }
    } catch (err) {
      console.error(err);
    }
  }

  async function handleAddProblem() {
    if (!selectedProblemId) return;
    try {
      await api.post(`/problem-sets/${setId}/problems`, {
        problemId: selectedProblemId
      });
      showAddModal = false;
      await loadSet();
    } catch (err: any) {
      alert(err.message || 'Failed to add problem');
    }
  }

  async function handleRemoveProblem(problemId: string) {
    if (!confirm('Remove this problem from the set?')) return;
    try {
      await api.delete(`/problem-sets/${setId}/problems/${problemId}`);
      await loadSet();
    } catch (err: any) {
      alert(err.message || 'Failed to remove problem');
    }
  }

  async function handleMove(index: number, direction: 'up' | 'down') {
    if (!problemSet?.items) return;
    const targetIdx = direction === 'up' ? index - 1 : index + 1;
    if (targetIdx < 0 || targetIdx >= problemSet.items.length) return;

    const newItems = [...problemSet.items];
    const temp = newItems[index];
    newItems[index] = newItems[targetIdx];
    newItems[targetIdx] = temp;

    const problemIds = newItems.map((it) => it.problemId);
    try {
      await api.patch(`/problem-sets/${setId}/order`, { problemIds });
      await loadSet();
    } catch (err: any) {
      alert(err.message || 'Failed to reorder');
    }
  }

  async function handleDeleteSet() {
    if (!confirm('Are you sure you want to delete this Problem Set?')) return;
    try {
      await api.delete(`/problem-sets/${setId}`);
      goto('/problem-sets');
    } catch (err: any) {
      alert(err.message || 'Failed to delete');
    }
  }

  onMount(() => {
    loadSet();
  });
</script>

{#if loading}
  <div class="h-96 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
{:else if error || !problemSet}
  <div class="p-8 rounded-2xl border border-zinc-700 bg-zinc-900 text-zinc-200">
    <h2 class="text-xl font-bold">Error</h2>
    <p class="text-sm">{error || 'Problem set not found.'}</p>
  </div>
{:else}
  <div class="space-y-8">
    <!-- Header -->
    <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 space-y-4">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div class="space-y-1">
          <div class="flex items-center space-x-2 text-xs font-semibold text-zinc-400">
            <span class="px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-300 border border-zinc-700">{problemSet.visibility}</span>
            <span>Created by {problemSet.ownerUsername}</span>
          </div>
          <h1 class="text-3xl font-extrabold text-white">{problemSet.name}</h1>
          <p class="text-sm text-zinc-400">{problemSet.description || 'No description provided.'}</p>
        </div>

        <div class="flex items-center space-x-3 shrink-0">
          <a
            href={`/contests/new?setId=${problemSet.id}`}
            class="px-4 py-2.5 rounded-xl font-bold bg-white hover:bg-zinc-200 text-black shadow-sm transition flex items-center space-x-2 text-sm"
          >
            <Trophy class="w-4 h-4" />
            <span>Host Virtual Contest</span>
          </a>

          {#if $auth.user && $auth.user.id === problemSet.ownerId}
            <button
              on:click={handleDeleteSet}
              class="p-2.5 rounded-xl border border-zinc-800 hover:border-zinc-700 hover:bg-zinc-800 text-zinc-400 hover:text-white transition"
              title="Delete Problem Set"
            >
              <Trash2 class="w-4 h-4" />
            </button>
          {/if}
        </div>
      </div>
    </div>

    <!-- Problems In Set -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-xl font-bold text-white flex items-center space-x-2">
          <Layers class="w-5 h-5 text-white" />
          <span>Problems ({problemSet.items?.length || 0})</span>
        </h2>

        {#if $auth.user && $auth.user.id === problemSet.ownerId}
          <button
            on:click={openAddModal}
            class="px-3.5 py-1.5 rounded-xl text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-white border border-zinc-700 transition flex items-center space-x-1.5"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Problem</span>
          </button>
        {/if}
      </div>

      {#if !problemSet.items || problemSet.items.length === 0}
        <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
          <p class="text-sm text-zinc-500">This problem set has no problems yet.</p>
          {#if $auth.user && $auth.user.id === problemSet.ownerId}
            <button
              on:click={openAddModal}
              class="px-4 py-2 rounded-xl text-xs font-bold bg-white hover:bg-zinc-200 text-black transition inline-flex items-center space-x-1.5"
            >
              <Plus class="w-4 h-4" />
              <span>Add your first problem</span>
            </button>
          {/if}
        </div>
      {:else}
        <div class="space-y-3">
          {#each problemSet.items as item, index}
            {#if item.problem}
              <div class="flex items-center space-x-2">
                <div class="flex-1">
                  <ProblemCard problem={item.problem} label={String(index + 1)} />
                </div>

                {#if $auth.user && $auth.user.id === problemSet.ownerId}
                  <div class="flex flex-col space-y-1">
                    <button
                      on:click={() => handleMove(index, 'up')}
                      disabled={index === 0}
                      class="p-1.5 rounded-lg border border-zinc-800 hover:bg-zinc-800 disabled:opacity-30 text-zinc-400 hover:text-white transition"
                      title="Move Up"
                    >
                      <ArrowUp class="w-3.5 h-3.5" />
                    </button>
                    <button
                      on:click={() => handleMove(index, 'down')}
                      disabled={index === problemSet.items.length - 1}
                      class="p-1.5 rounded-lg border border-zinc-800 hover:bg-zinc-800 disabled:opacity-30 text-zinc-400 hover:text-white transition"
                      title="Move Down"
                    >
                      <ArrowDown class="w-3.5 h-3.5" />
                    </button>
                    <button
                      on:click={() => handleRemoveProblem(item.problemId)}
                      class="p-1.5 rounded-lg border border-zinc-800 hover:bg-zinc-800 hover:border-zinc-700 text-zinc-500 hover:text-white transition"
                      title="Remove"
                    >
                      <Trash2 class="w-3.5 h-3.5" />
                    </button>
                  </div>
                {/if}
              </div>
            {/if}
          {/each}
        </div>
      {/if}
    </div>
  </div>

  <!-- Add Problem Modal -->
  {#if showAddModal}
    <div class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-4">
      <div class="max-w-lg w-full rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl space-y-5">
        <div class="space-y-1">
          <h3 class="text-xl font-bold text-white">Add Problem to Set</h3>
          <p class="text-xs text-zinc-400">Select an existing problem from the library to add.</p>
        </div>

        <div>
          <label for="select-prob" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Problem</label>
          <select
            id="select-prob"
            bind:value={selectedProblemId}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
          >
            {#each allProblems as p}
              <option value={p.id}>[{p.platform}] {p.title}</option>
            {/each}
          </select>
        </div>

        <div class="flex items-center justify-end space-x-3 pt-2">
          <button
            on:click={() => (showAddModal = false)}
            class="px-4 py-2 rounded-xl text-sm font-semibold text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
          >
            Cancel
          </button>
          <button
            on:click={handleAddProblem}
            class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black transition"
          >
            Add to Set
          </button>
        </div>
      </div>
    </div>
  {/if}
{/if}
