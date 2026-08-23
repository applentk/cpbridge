<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api/client';
  import type { ProblemSet, ScoringType, Contest } from '@cp-hub/contracts';
  import { Trophy, Clock, AlertCircle } from 'lucide-svelte';

  let problemSets: ProblemSet[] = [];
  let selectedSetId = $page.url.searchParams.get('setId') || '';

  let name = '';
  let description = '';
  let startOption = 'now';
  let customStart = '';
  let durationMinutes = 120;
  let scoringType: ScoringType = 'ICPC';
  let visibility = 'PUBLIC';

  let loading = true;
  let submitting = false;
  let error = '';

  onMount(async () => {
    try {
      problemSets = await api.get<ProblemSet[]>('/problem-sets');
      if (problemSets.length > 0 && !selectedSetId) {
        selectedSetId = problemSets[0].id;
      }
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  });

  async function handleCreate() {
    if (!name.trim() || !selectedSetId) {
      error = 'Contest name and Problem Set are required';
      return;
    }

    submitting = true;
    error = '';

    try {
      const now = new Date();
      let startTime = new Date();
      if (startOption === 'now') {
        startTime = new Date(now.getTime() + 10 * 1000); // 10s from now
      } else if (startOption === '5m') {
        startTime = new Date(now.getTime() + 5 * 60 * 1000);
      } else if (startOption === 'custom' && customStart) {
        startTime = new Date(customStart);
      }

      const endTime = new Date(startTime.getTime() + durationMinutes * 60 * 1000);

      const contest = await api.post<Contest>('/contests', {
        problemSetId: selectedSetId,
        name,
        description,
        startAt: startTime.toISOString(),
        endAt: endTime.toISOString(),
        visibility,
        scoringType
      });

      goto(`/contests/${contest.id}`);
    } catch (err: any) {
      error = err.message || 'Failed to create contest';
    } finally {
      submitting = false;
    }
  }
</script>

<div class="max-w-2xl mx-auto py-8">
  <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/60 shadow-xl space-y-6">
    <div class="space-y-1.5 border-b border-zinc-800 pb-4">
      <h1 class="text-2xl font-bold text-white flex items-center space-x-2">
        <Trophy class="w-6 h-6 text-white" />
        <span>Create Virtual Contest</span>
      </h1>
      <p class="text-sm text-zinc-400">Snapshot problems from a problem set into a timed competitive contest.</p>
    </div>

    {#if error}
      <div class="p-3.5 rounded-xl bg-zinc-900 border border-zinc-700 text-zinc-200 text-sm flex items-center space-x-2">
        <AlertCircle class="w-4 h-4 shrink-0 text-white" />
        <span>{error}</span>
      </div>
    {/if}

    <div class="space-y-4">
      <div>
        <label for="contest-name" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Contest Name</label>
        <input
          id="contest-name"
          type="text"
          bind:value={name}
          placeholder="e.g. Weekly Algorithm Practice Round #1"
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm placeholder-zinc-600 transition"
        />
      </div>

      <div>
        <label for="source-set" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Source Problem Set (Snapshot)</label>
        <select
          id="source-set"
          bind:value={selectedSetId}
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
        >
          {#each problemSets as ps}
            <option value={ps.id}>{ps.name} ({ps.problemCount} problems)</option>
          {/each}
        </select>
        <p class="text-[11px] text-zinc-500 mt-1">
          Problems will be snapshotted directly into the contest. Later edits to the Problem Set will not affect this contest.
        </p>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label for="start-timing" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Start Timing</label>
          <select
            id="start-timing"
            bind:value={startOption}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
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
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
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
          <label for="custom-start-time" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Custom Start Time (UTC)</label>
          <input
            id="custom-start-time"
            type="datetime-local"
            bind:value={customStart}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
          />
        </div>
      {/if}

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label for="scoring-engine" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Scoring Engine</label>
          <select
            id="scoring-engine"
            bind:value={scoringType}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
          >
            <option value="ICPC">ICPC (Solved + 20m Penalties)</option>
            <option value="SIMPLE">SIMPLE (Solved Count Only)</option>
          </select>
        </div>

        <div>
          <label for="contest-visibility" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Visibility</label>
          <select
            id="contest-visibility"
            bind:value={visibility}
            class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm transition"
          >
            <option value="PUBLIC">PUBLIC</option>
            <option value="UNLISTED">UNLISTED</option>
            <option value="PRIVATE">PRIVATE</option>
          </select>
        </div>
      </div>

      <div>
        <label for="contest-description" class="block text-xs font-semibold uppercase text-zinc-400 mb-1.5">Description (Optional)</label>
        <textarea
          id="contest-description"
          bind:value={description}
          rows="2"
          placeholder="Contest rules, invited participants..."
          class="w-full px-4 py-2.5 rounded-xl bg-zinc-950 border border-zinc-800 focus:border-zinc-400 focus:outline-none text-zinc-100 text-sm placeholder-zinc-600 transition"
        ></textarea>
      </div>
    </div>

    <div class="flex items-center justify-end space-x-3 pt-4 border-t border-zinc-800">
      <a
        href="/contests"
        class="px-4 py-2 rounded-xl text-sm font-semibold text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
      >
        Cancel
      </a>
      <button
        on:click={handleCreate}
        disabled={submitting || !name.trim() || !selectedSetId}
        class="px-6 py-2.5 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black shadow-sm transition flex items-center space-x-2"
      >
        <span>{submitting ? 'Creating...' : 'Start Contest'}</span>
      </button>
    </div>
  </div>
</div>
