<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import type { ProblemSet, Problem, ScoringType, ContestPublicationStatus } from '@cpbridge/contracts';
  import { Trophy, ArrowLeft, Layers, AlertCircle } from 'lucide-svelte';

  let problemSets: ProblemSet[] = [];
  let allProblems: Problem[] = [];
  let selectedSetId = $page.url.searchParams.get('setId') || '';
  let mode: 'set' | 'manual' = selectedSetId ? 'set' : 'set';

  let selectedProblemIds: string[] = [];

  let name = '';
  let description = '';
  let startOption = 'now';
  let customStart = '';
  let durationMinutes = 120;
  let scoringType: ScoringType = 'ICPC';
  let visibility = 'PUBLIC';
  let publicationStatus: ContestPublicationStatus = 'PUBLISHED';

  let _loading = true;
  let submitting = false;
  let error = '';

  onMount(async () => {
    try {
      const [psRes, pRes] = await Promise.all([
        api.get<ProblemSet[]>('/admin/problem-sets'),
        api.get<{ problems: Problem[] }>('/admin/problems?limit=100')
      ]);
      problemSets = psRes;
      allProblems = pRes.problems;

      if (problemSets.length > 0 && !selectedSetId) {
        selectedSetId = problemSets[0].id;
      }
    } catch (err: any) {
      error = err.message || 'Failed to load problem sets';
    } finally {
      _loading = false;
    }
  });

  function toggleProblemSelection(id: string) {
    if (selectedProblemIds.includes(id)) {
      selectedProblemIds = selectedProblemIds.filter((pid) => pid !== id);
    } else {
      selectedProblemIds = [...selectedProblemIds, id];
    }
  }

  async function handleCreate() {
    if (!name.trim()) {
      error = 'Contest name is required';
      return;
    }

    if (mode === 'set' && !selectedSetId) {
      error = 'Please select a Problem Set';
      return;
    }

    submitting = true;
    error = '';

    try {
      const now = new Date();
      let startTime = new Date();
      if (startOption === 'now') {
        startTime = new Date(now.getTime() + 10 * 1000); // 10s countdown
      } else if (startOption === '5m') {
        startTime = new Date(now.getTime() + 5 * 60 * 1000);
      } else if (startOption === 'custom' && customStart) {
        startTime = new Date(customStart);
      }

      const endTime = new Date(startTime.getTime() + durationMinutes * 60 * 1000);

      const payload: any = {
        name: name.trim(),
        description: description.trim(),
        startAt: startTime.toISOString(),
        endAt: endTime.toISOString(),
        visibility,
        scoringType,
        publicationStatus
      };

      if (mode === 'set') {
        payload.problemSetId = selectedSetId;
      } else {
        payload.problemIds = selectedProblemIds;
      }

      await api.post('/admin/contests', payload);
      goto('/admin/contests');
    } catch (err: any) {
      error = err.message || 'Failed to create contest';
    } finally {
      submitting = false;
    }
  }
</script>

<div class="max-w-3xl mx-auto py-4 space-y-6">
  <!-- Back Button & Header -->
  <div class="flex items-center space-x-3">
    <a
      href="/admin/contests"
      class="p-2 rounded-xl text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
    >
      <ArrowLeft class="w-4 h-4" />
    </a>
    <div>
      <h1 class="text-2xl font-bold text-white flex items-center space-x-2">
        <Trophy class="w-5 h-5 text-white" />
        <span>Create Contest</span>
      </h1>
      <p class="text-xs text-zinc-400">Schedule and configure a competitive programming contest.</p>
    </div>
  </div>

  <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/60 shadow-xl space-y-6">
    {#if error}
      <div class="p-3.5 rounded-xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm flex items-center space-x-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-red-400" />
        <span>{error}</span>
      </div>
    {/if}

    <div class="space-y-5 text-sm">
      <!-- Name -->
      <div>
        <label for="contest-name" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Contest Name</label>
        <input
          id="contest-name"
          type="text"
          bind:value={name}
          placeholder="e.g. Weekly Contest #1"
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm placeholder-zinc-600"
        />
      </div>

      <!-- Description -->
      <div>
        <label for="contest-description" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Description (Optional)</label>
        <textarea
          id="contest-description"
          bind:value={description}
          rows="2"
          placeholder="Rules, invited participants, scoring details..."
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm placeholder-zinc-600"
        ></textarea>
      </div>

      <!-- Source Selection Mode -->
      <div class="space-y-2">
        <span class="block text-xs font-semibold uppercase text-zinc-400">Problem Source</span>
        <div class="flex items-center space-x-3">
          <button
            type="button"
            on:click={() => (mode = 'set')}
            class="px-4 py-2 rounded-xl text-xs font-bold transition flex items-center space-x-1.5 {
              mode === 'set' ? 'bg-white text-black' : 'bg-zinc-950 text-zinc-400 border border-zinc-800'
            }"
          >
            <Layers class="w-3.5 h-3.5" />
            <span>From Problem Set</span>
          </button>
          <button
            type="button"
            on:click={() => (mode = 'manual')}
            class="px-4 py-2 rounded-xl text-xs font-bold transition flex items-center space-x-1.5 {
              mode === 'manual' ? 'bg-white text-black' : 'bg-zinc-950 text-zinc-400 border border-zinc-800'
            }"
          >
            <Trophy class="w-3.5 h-3.5" />
            <span>Select Problems Manually</span>
          </button>
        </div>
      </div>

      <!-- Problem Set Dropdown -->
      {#if mode === 'set'}
        <div>
          <label for="source-set" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Source Problem Set (Snapshot)</label>
          <select
            id="source-set"
            bind:value={selectedSetId}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            {#each problemSets as ps}
              <option value={ps.id}>{ps.name} ({ps.problemCount} problems)</option>
            {/each}
          </select>
          <p class="text-[11px] text-zinc-500 mt-1">
            Problems will be snapshotted directly into the contest.
          </p>
        </div>
      {:else}
        <!-- Manual Problem Multiselect -->
        <div class="space-y-2">
          <span class="block text-xs font-semibold uppercase text-zinc-400">Select Problems ({selectedProblemIds.length} chosen)</span>
          <div class="max-h-48 overflow-y-auto rounded-xl border border-zinc-800 bg-zinc-950 divide-y divide-zinc-800/60 p-2 space-y-1">
            {#each allProblems as p}
              {@const isSelected = selectedProblemIds.includes(p.id)}
              <button
                type="button"
                class="w-full text-left flex items-center justify-between p-2 rounded-lg cursor-pointer hover:bg-zinc-900 transition {
                  isSelected ? 'bg-zinc-900 border border-zinc-700' : ''
                }"
                on:click={() => toggleProblemSelection(p.id)}
              >
                <div class="flex items-center space-x-2 min-w-0">
                  <input
                    type="checkbox"
                    checked={isSelected}
                    class="rounded bg-zinc-800 border-zinc-700 text-white focus:ring-0 pointer-events-none"
                  />
                  <span class="text-xs font-semibold text-white truncate">{p.title}</span>
                  <span class="text-[10px] font-mono text-zinc-500">{p.platform}</span>
                </div>
                {#if p.difficulty}
                  <span class="text-xs font-mono text-zinc-400">{p.difficulty}</span>
                {/if}
              </button>
            {/each}
          </div>
        </div>
      {/if}

      <!-- Timing Configuration -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label for="start-timing" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Start Timing</label>
          <select
            id="start-timing"
            bind:value={startOption}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            <option value="now">Immediately (10s countdown)</option>
            <option value="5m">In 5 minutes</option>
            <option value="custom">Custom Date & Time</option>
          </select>
        </div>

        <div>
          <label for="contest-duration" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Duration</label>
          <select
            id="contest-duration"
            bind:value={durationMinutes}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            <option value={30}>30 minutes</option>
            <option value={60}>1 hour</option>
            <option value={120}>2 hours (Standard)</option>
            <option value={180}>3 hours</option>
            <option value={300}>5 hours (ICPC Regional)</option>
          </select>
        </div>
      </div>

      {#if startOption === 'custom'}
        <div>
          <label for="custom-start" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Custom Start Time (Local / UTC)</label>
          <input
            id="custom-start"
            type="datetime-local"
            bind:value={customStart}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          />
        </div>
      {/if}

      <!-- Scoring, Visibility, Publication Status -->
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div>
          <label for="scoring-engine" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Scoring Engine</label>
          <select
            id="scoring-engine"
            bind:value={scoringType}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            <option value="ICPC">ICPC (Solved + Penalties)</option>
            <option value="SIMPLE">SIMPLE (Solved Count)</option>
          </select>
        </div>

        <div>
          <label for="contest-visibility" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Visibility</label>
          <select
            id="contest-visibility"
            bind:value={visibility}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm"
          >
            <option value="PUBLIC">PUBLIC</option>
            <option value="UNLISTED">UNLISTED</option>
            <option value="PRIVATE">PRIVATE</option>
          </select>
        </div>

        <div>
          <label for="contest-pub-status" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Publication Status</label>
          <select
            id="contest-pub-status"
            bind:value={publicationStatus}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-100 text-sm font-semibold {
              publicationStatus === 'PUBLISHED' ? 'text-emerald-400' : 'text-amber-400'
            }"
          >
            <option value="PUBLISHED">PUBLISHED (Visible to users)</option>
            <option value="DRAFT">DRAFT (Admin only)</option>
          </select>
        </div>
      </div>
    </div>

    <!-- Actions -->
    <div class="flex items-center justify-end space-x-3 pt-4 border-t border-zinc-800">
      <a
        href="/admin/contests"
        class="px-4 py-2 rounded-xl text-sm font-semibold text-zinc-400 hover:text-white"
      >
        Cancel
      </a>
      <button
        on:click={handleCreate}
        disabled={submitting || !name.trim()}
        class="px-6 py-2.5 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black shadow-sm transition flex items-center space-x-2"
      >
        <span>{submitting ? 'Creating Contest...' : 'Create Contest'}</span>
      </button>
    </div>
  </div>
</div>
