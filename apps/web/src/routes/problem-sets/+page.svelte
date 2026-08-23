<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { ProblemSet, Visibility } from '@cp-hub/contracts';
  import { Layers, Plus, Lock, Globe, Eye, Trash2, ArrowRight } from 'lucide-svelte';

  let problemSets: ProblemSet[] = [];
  let loading = true;
  let showCreateModal = false;

  let name = '';
  let description = '';
  let visibility: Visibility = 'PUBLIC';
  let createLoading = false;
  let createError = '';

  async function loadProblemSets() {
    loading = true;
    try {
      problemSets = await api.get<ProblemSet[]>('/problem-sets');
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  }

  async function handleCreate() {
    if (!name.trim()) return;
    createLoading = true;
    createError = '';
    try {
      const set = await api.post<ProblemSet>('/problem-sets', {
        name,
        description,
        visibility
      });
      showCreateModal = false;
      name = '';
      description = '';
      await loadProblemSets();
    } catch (err: any) {
      createError = err.message || 'Failed to create problem set';
    } finally {
      createLoading = false;
    }
  }

  onMount(() => {
    loadProblemSets();
  });
</script>

<div class="space-y-6">
  <div class="flex items-center justify-between">
    <div>
      <h1 class="text-3xl font-bold text-white">Problem Sets</h1>
      <p class="text-sm text-zinc-400">Curate and share collections of problems for training and virtual contests.</p>
    </div>

    {#if $auth.user}
      <button
        on:click={() => (showCreateModal = true)}
        class="px-4 py-2.5 rounded-xl font-bold bg-white hover:bg-zinc-200 text-black shadow-sm transition flex items-center space-x-2"
      >
        <Plus class="w-4 h-4" />
        <span>New Problem Set</span>
      </button>
    {/if}
  </div>

  {#if loading}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      {#each Array(6) as _}
        <div class="h-44 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      {/each}
    </div>
  {:else if problemSets.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-4">
      <p class="text-zinc-400 text-base">No problem sets found.</p>
      {#if $auth.user}
        <button
          on:click={() => (showCreateModal = true)}
          class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black transition inline-flex items-center space-x-1.5"
        >
          <Plus class="w-4 h-4" />
          <span>Create your first set</span>
        </button>
      {/if}
    </div>
  {:else}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-5">
      {#each problemSets as set}
        <a
          href={`/problem-sets/${set.id}`}
          class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-800/40 hover:border-zinc-700 transition flex flex-col justify-between space-y-4 group"
        >
          <div class="space-y-2">
            <div class="flex items-center justify-between">
              <span class="text-xs px-2.5 py-0.5 rounded-full font-semibold flex items-center space-x-1 {
                set.visibility === 'PUBLIC' ? 'bg-zinc-800 text-zinc-200 border border-zinc-700' :
                set.visibility === 'PRIVATE' ? 'bg-zinc-950 text-zinc-400 border border-zinc-800' :
                'bg-zinc-900 text-zinc-300 border border-zinc-700'
              }">
                {#if set.visibility === 'PUBLIC'}
                  <Globe class="w-3 h-3 inline mr-1" />
                {:else if set.visibility === 'PRIVATE'}
                  <Lock class="w-3 h-3 inline mr-1" />
                {:else}
                  <Eye class="w-3 h-3 inline mr-1" />
                {/if}
                <span>{set.visibility}</span>
              </span>

              <span class="text-xs text-zinc-500 font-mono">by {set.ownerUsername || 'User'}</span>
            </div>

            <h3 class="text-lg font-bold text-white group-hover:text-zinc-300 transition">{set.name}</h3>
            <p class="text-xs text-zinc-400 line-clamp-2">{set.description || 'No description provided.'}</p>
          </div>

          <div class="flex items-center justify-between pt-3 border-t border-zinc-800/80 text-xs">
            <span class="font-semibold text-zinc-300">{set.problemCount} problem{set.problemCount === 1 ? '' : 's'}</span>
            <span class="text-zinc-500 flex items-center space-x-1 group-hover:text-zinc-300 transition">
              <span>View Set</span>
              <ArrowRight class="w-3.5 h-3.5" />
            </span>
          </div>
        </a>
      {/each}
    </div>
  {/if}

  <!-- Create Modal -->
  {#if showCreateModal}
    <div class="fixed inset-0 z-50 bg-black/75 backdrop-blur-sm flex items-center justify-center p-4">
      <div class="max-w-md w-full rounded-2xl border border-zinc-800 bg-zinc-900 p-6 shadow-2xl space-y-5">
        <div class="space-y-1">
          <h3 class="text-xl font-bold text-white">Create Problem Set</h3>
          <p class="text-xs text-zinc-400">Organize problems into a reusable set.</p>
        </div>

        {#if createError}
          <div class="p-3 rounded-xl bg-zinc-900 border border-zinc-700 text-zinc-200 text-sm">
            {createError}
          </div>
        {/if}

        <div class="space-y-4">
          <div>
            <label for="set-name" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Set Name</label>
            <input
              id="set-name"
              type="text"
              bind:value={name}
              placeholder="e.g. Dynamic Programming Practice"
              class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm placeholder-zinc-600 transition"
            />
          </div>

          <div>
            <label for="set-desc" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Description</label>
            <textarea
              id="set-desc"
              bind:value={description}
              rows="3"
              placeholder="Curated classic DP problems from Codeforces and AtCoder..."
              class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm placeholder-zinc-600 transition"
            ></textarea>
          </div>

          <div>
            <label for="set-vis" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Visibility</label>
            <select
              id="set-vis"
              bind:value={visibility}
              class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
            >
              <option value="PUBLIC">PUBLIC</option>
              <option value="UNLISTED">UNLISTED</option>
              <option value="PRIVATE">PRIVATE</option>
            </select>
          </div>
        </div>

        <div class="flex items-center justify-end space-x-3 pt-2">
          <button
            on:click={() => (showCreateModal = false)}
            class="px-4 py-2 rounded-xl text-sm font-semibold text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
          >
            Cancel
          </button>
          <button
            on:click={handleCreate}
            disabled={createLoading || !name.trim()}
            class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black transition"
          >
            {createLoading ? 'Creating...' : 'Create'}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>
