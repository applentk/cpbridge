<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import type { Contest, Problem, ContestProblem } from '@cpbridge/contracts';
  import { ArrowLeft, Save, Plus, Trash2, ArrowUp, ArrowDown, Search, X, Check, Lock } from 'lucide-svelte';
  import { toDateTimeLocalValue, fromDateTimeLocalValue } from '$lib/utils/date';

  let contestId = $page.params.id;
  let contest: Contest | null = null;
  let problems: ContestProblem[] = [];
  let loading = true;
  let error = '';
  let successMsg = '';

  // Form Fields
  let name = '';
  let description = '';
  let startAt = '';
  let endAt = '';
  let visibility = 'PUBLIC';
  let scoringType: Contest['scoringType'] = 'ICPC';
  let publicationStatus: Contest['publicationStatus'] = 'PUBLISHED';
  let saving = false;

  // Add Problem Modal
  let showAddModal = false;
  let searchProblems: Problem[] = [];
  let searchQuery = '';
  let searching = false;

  async function loadContest() {
    loading = true;
    error = '';
    try {
      contest = await api.get<Contest>(`/admin/contests/${contestId}`);
      problems = contest.problems || [];
      name = contest.name;
      description = contest.description;
      startAt = toDateTimeLocalValue(contest.startAt);
      endAt = toDateTimeLocalValue(contest.endAt);
      visibility = contest.visibility;
      scoringType = contest.scoringType;
      publicationStatus = contest.publicationStatus;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load contest';
    } finally {
      loading = false;
    }
  }

  async function handleSaveDetails() {
    if (!name.trim()) return;
    if (!endAt) {
      alert('End time is required');
      return;
    }
    saving = true;
    try {
      const payload: Record<string, unknown> = {
        name: name.trim(),
        description: description.trim(),
        endAt: fromDateTimeLocalValue(endAt),
        visibility,
        scoringType,
        publicationStatus
      };

      if (contest?.state === 'UPCOMING') {
        if (!startAt) {
          alert('Start time is required');
          saving = false;
          return;
        }
        payload.startAt = fromDateTimeLocalValue(startAt);
      }

      await api.patch(`/admin/contests/${contestId}`, payload);
      successMsg = 'Contest details updated successfully!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadContest();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to save contest');
    } finally {
      saving = false;
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
      await api.post(`/admin/contests/${contestId}/problems`, {
        problemId: prob.id,
        position: problems.length
      });
      showAddModal = false;
      searchQuery = '';
      successMsg = `Added problem "${prob.title}"!`;
      setTimeout(() => (successMsg = ''), 4000);
      await loadContest();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to add problem');
    }
  }

  async function handleRemoveProblem(problemId: string) {
    if (!confirm('Remove this problem from contest?')) return;
    try {
      await api.delete(`/admin/contests/${contestId}/problems/${problemId}`);
      await loadContest();
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to remove problem');
    }
  }

  async function moveProblem(index: number, direction: 'up' | 'down') {
    const targetIndex = direction === 'up' ? index - 1 : index + 1;
    if (targetIndex < 0 || targetIndex >= problems.length) return;

    const newProblems = [...problems];
    const temp = newProblems[index];
    newProblems[index] = newProblems[targetIndex];
    newProblems[targetIndex] = temp;
    problems = newProblems;

    const pids = problems.map((cp) => cp.problemId);
    try {
      await api.patch(`/admin/contests/${contestId}/problem-order`, { problemIds: pids });
    } catch (err) {
      alert(err instanceof Error ? err.message : 'Failed to reorder problems');
      await loadContest();
    }
  }

  onMount(() => {
    loadContest();
  });
</script>

{#if loading}
  <div class="h-96 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
{:else if error || !contest}
  <div class="p-8 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300">
    <h2 class="font-bold text-lg">Error</h2>
    <p class="text-sm">{error || 'Contest not found'}</p>
  </div>
{:else}
  {@const isLocked = contest.state === 'ACTIVE' || contest.state === 'FINISHED'}
  <div class="space-y-6">
    <!-- Header -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="flex items-center space-x-3">
        <a
          href="/admin/contests"
          class="p-2 rounded-xl text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
        >
          <ArrowLeft class="w-4 h-4" />
        </a>
        <div>
          <h1 class="text-2xl font-bold text-white flex items-center space-x-2">
            <span>{contest.name}</span>
            <span class="text-xs px-2.5 py-0.5 rounded-full font-bold {
              contest.publicationStatus === 'PUBLISHED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' : 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
            }">
              {contest.publicationStatus}
            </span>
          </h1>
          <p class="text-xs text-zinc-400">Edit contest schedule, problems, and publication status.</p>
        </div>
      </div>

      <div class="flex items-center space-x-2">
        <a
          href={`/contests/${contest.id}`}
          class="px-4 py-2 rounded-xl text-xs font-semibold border border-zinc-700 hover:bg-zinc-800 text-zinc-200 transition"
        >
          View Public Page
        </a>
      </div>
    </div>

    {#if isLocked}
      <div class="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-300 text-sm flex items-center space-x-3">
        <Lock class="w-5 h-5 shrink-0 text-amber-400" />
        <div>
          <span class="font-bold">Contest is {contest.state}:</span>
          <span> Problem list and start time are locked to preserve competitive scoreboard integrity. You can still edit description, publication status, and extend end time.</span>
        </div>
      </div>
    {/if}

    {#if successMsg}
      <div class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center space-x-2">
        <Check class="w-4 h-4 text-emerald-400" />
        <span>{successMsg}</span>
      </div>
    {/if}

    <!-- Configuration Form -->
    <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 space-y-4">
      <h2 class="text-base font-bold text-white">Contest Settings</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-sm">
        <div class="sm:col-span-2">
          <label for="edit-contest-name" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Contest Name</label>
          <input
            id="edit-contest-name"
            type="text"
            bind:value={name}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div class="sm:col-span-2">
          <label for="edit-contest-desc" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Description</label>
          <textarea
            id="edit-contest-desc"
            bind:value={description}
            rows="2"
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          ></textarea>
        </div>

        <div>
          <label for="edit-contest-start" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">
            Start Time {#if isLocked}<span class="text-amber-400 font-normal">(Locked)</span>{/if}
          </label>
          <input
            id="edit-contest-start"
            type="datetime-local"
            bind:value={startAt}
            disabled={isLocked}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm disabled:opacity-50"
          />
        </div>

        <div>
          <label for="edit-contest-end" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">End Time</label>
          <input
            id="edit-contest-end"
            type="datetime-local"
            bind:value={endAt}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>

        <div>
          <label for="edit-contest-pub" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Publication Status</label>
          <select
            id="edit-contest-pub"
            bind:value={publicationStatus}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm font-semibold {
              publicationStatus === 'PUBLISHED' ? 'text-emerald-400' : 'text-amber-400'
            }"
          >
            <option value="PUBLISHED">PUBLISHED (Visible to all users)</option>
            <option value="DRAFT">DRAFT (Admin preview only)</option>
          </select>
        </div>

        <div>
          <label for="edit-contest-vis" class="block text-xs font-semibold uppercase text-zinc-400 mb-1">Visibility</label>
          <select
            id="edit-contest-vis"
            bind:value={visibility}
            class="w-full px-3.5 py-2 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            <option value="PUBLIC">PUBLIC</option>
            <option value="UNLISTED">UNLISTED</option>
            <option value="PRIVATE">PRIVATE</option>
          </select>
        </div>
      </div>

      <div class="flex justify-end pt-2">
        <button
          on:click={handleSaveDetails}
          disabled={saving || !name.trim()}
          class="px-5 py-2 rounded-xl text-xs font-bold bg-white hover:bg-zinc-200 text-black transition flex items-center space-x-1.5 shadow-sm"
        >
          <Save class="w-3.5 h-3.5" />
          <span>{saving ? 'Saving...' : 'Save Settings'}</span>
        </button>
      </div>
    </div>

    <!-- Contest Problems Management -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h2 class="text-lg font-bold text-white flex items-center space-x-2">
          <span>Contest Problems</span>
          <span class="text-xs font-mono text-zinc-400">({problems.length})</span>
        </h2>

        {#if !isLocked}
          <button
            on:click={() => { showAddModal = true; searchProblemLibrary(); }}
            class="px-3.5 py-1.5 rounded-xl text-xs font-bold bg-white text-black hover:bg-zinc-200 transition flex items-center space-x-1 shadow-sm"
          >
            <Plus class="w-3.5 h-3.5" />
            <span>Add Problem</span>
          </button>
        {/if}
      </div>

      {#if problems.length === 0}
        <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
          <p class="text-zinc-400 text-sm">No problems attached to this contest.</p>
          {#if !isLocked}
            <button
              on:click={() => { showAddModal = true; searchProblemLibrary(); }}
              class="px-4 py-2 rounded-xl text-xs font-bold bg-white text-black hover:bg-zinc-200 transition"
            >
              Add Problems from Library
            </button>
          {/if}
        </div>
      {:else}
        <div class="rounded-2xl border border-zinc-800 bg-zinc-900/40 divide-y divide-zinc-800/60 overflow-hidden">
          {#each problems as cp, index}
            <div class="p-4 flex items-center justify-between hover:bg-zinc-800/20 transition">
              <div class="flex items-center space-x-4 min-w-0">
                <span class="w-8 h-8 rounded-lg bg-zinc-800 border border-zinc-700 text-white text-sm font-bold flex items-center justify-center shrink-0">
                  {cp.label}
                </span>

                <div class="space-y-0.5 min-w-0">
                  <div class="flex items-center space-x-2">
                    {#if cp.problem?.platform}
                      <span class="text-[10px] px-1.5 py-0.2 rounded font-mono font-bold {
                        cp.problem.platform === 'CODEFORCES' ? 'bg-blue-500/15 text-blue-300 border border-blue-500/30' : 'bg-red-500/15 text-red-300 border border-red-500/30'
                      }">
                        {cp.problem.platform}
                      </span>
                    {/if}
                    <span class="text-sm font-semibold text-white truncate">{cp.problem?.title || cp.problemId}</span>
                  </div>
                  <div class="text-xs text-zinc-500 font-mono">
                    {cp.problem?.externalId}
                    {#if cp.problem?.difficulty}
                      • Difficulty: {cp.problem.difficulty}
                    {/if}
                  </div>
                </div>
              </div>

              <!-- Reorder and Delete (Enabled only when UPCOMING) -->
              {#if !isLocked}
                <div class="flex items-center space-x-1 shrink-0">
                  <button
                    on:click={() => moveProblem(index, 'up')}
                    disabled={index === 0}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 disabled:opacity-30 transition"
                    title="Move Up"
                  >
                    <ArrowUp class="w-4 h-4" />
                  </button>
                  <button
                    on:click={() => moveProblem(index, 'down')}
                    disabled={index === problems.length - 1}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 disabled:opacity-30 transition"
                    title="Move Down"
                  >
                    <ArrowDown class="w-4 h-4" />
                  </button>
                  <button
                    on:click={() => handleRemoveProblem(cp.problemId)}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-rose-400 hover:bg-rose-500/10 transition ml-2"
                    title="Remove Problem"
                  >
                    <Trash2 class="w-4 h-4" />
                  </button>
                </div>
              {:else}
                <span class="text-xs text-zinc-500 flex items-center space-x-1">
                  <Lock class="w-3.5 h-3.5" />
                  <span>Locked</span>
                </span>
              {/if}
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
        <h3 class="font-bold text-white text-lg">Add Problem to Contest</h3>
        <button on:click={() => (showAddModal = false)} class="text-zinc-500 hover:text-white">
          <X class="w-5 h-5" />
        </button>
      </div>

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

      <div class="flex-1 overflow-y-auto divide-y divide-zinc-800/60 pr-1">
        {#if searching}
          <div class="p-6 text-center text-zinc-500 text-xs animate-pulse">Searching problems...</div>
        {:else if searchProblems.length === 0}
          <div class="p-6 text-center text-zinc-500 text-xs">No problems found matching search.</div>
        {:else}
          {#each searchProblems as p}
            {@const isAlreadyIn = problems.some((cp) => cp.problemId === p.id)}
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
                <span class="text-xs text-zinc-500 font-medium px-2 py-1">In Contest</span>
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
