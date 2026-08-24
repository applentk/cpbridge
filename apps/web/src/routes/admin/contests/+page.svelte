<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import type { Contest } from '@cpbridge/contracts';
  import { Trophy, Plus, Trash2, Edit3, ExternalLink, Clock, Users, Eye, EyeOff, Check } from 'lucide-svelte';

  let contests: Contest[] = [];
  let loading = true;
  let error = '';
  let successMsg = '';

  async function loadContests() {
    loading = true;
    error = '';
    try {
      contests = await api.get<Contest[]>('/admin/contests');
    } catch (err: any) {
      error = err.message || 'Failed to load contests';
    } finally {
      loading = false;
    }
  }

  async function togglePublication(c: Contest) {
    const nextStatus = c.publicationStatus === 'PUBLISHED' ? 'DRAFT' : 'PUBLISHED';
    try {
      await api.patch(`/admin/contests/${c.id}`, { publicationStatus: nextStatus });
      successMsg = `Contest is now ${nextStatus}!`;
      setTimeout(() => (successMsg = ''), 4000);
      await loadContests();
    } catch (err: any) {
      alert(err.message || 'Failed to update publication status');
    }
  }

  async function handleDelete(c: Contest) {
    if (!confirm(`Are you sure you want to delete contest "${c.name}"?`)) return;
    try {
      await api.delete(`/admin/contests/${c.id}`);
      successMsg = 'Contest deleted successfully!';
      setTimeout(() => (successMsg = ''), 4000);
      await loadContests();
    } catch (err: any) {
      alert(err.message || 'Failed to delete contest');
    }
  }

  onMount(() => {
    loadContests();
  });
</script>

<div class="space-y-6">
  <!-- Header -->
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-2xl font-bold text-white">Contests Management</h1>
      <p class="text-sm text-zinc-400">Manage competition timing, problems, draft/published status, and rule sets.</p>
    </div>

    <a
      href="/admin/contests/new"
      class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black transition flex items-center space-x-1.5 shadow-sm self-start sm:self-auto"
    >
      <Plus class="w-4 h-4" />
      <span>Create Contest</span>
    </a>
  </div>

  {#if successMsg}
    <div class="p-3.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-sm flex items-center space-x-2">
      <Check class="w-4 h-4 text-emerald-400" />
      <span>{successMsg}</span>
    </div>
  {/if}

  {#if loading}
    <div class="h-64 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
  {:else if error}
    <div class="p-6 rounded-2xl bg-red-500/10 border border-red-500/30 text-red-300 text-sm">
      {error}
    </div>
  {:else if contests.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-3">
      <p class="text-zinc-400 text-sm">No contests created yet.</p>
      <a
        href="/admin/contests/new"
        class="px-4 py-2 rounded-xl text-xs font-bold bg-white text-black hover:bg-zinc-200 transition inline-flex items-center space-x-1.5"
      >
        <Plus class="w-4 h-4" />
        <span>Create your first Contest</span>
      </a>
    </div>
  {:else}
    <div class="rounded-2xl border border-zinc-800 bg-zinc-900/40 overflow-hidden">
      <table class="w-full text-left text-sm text-zinc-300">
        <thead class="bg-zinc-900/80 border-b border-zinc-800 text-xs text-zinc-400 uppercase font-semibold">
          <tr>
            <th class="px-5 py-3.5">Status</th>
            <th class="px-5 py-3.5">Contest</th>
            <th class="px-5 py-3.5">State</th>
            <th class="px-5 py-3.5">Schedule</th>
            <th class="px-5 py-3.5">Participants</th>
            <th class="px-5 py-3.5 text-right">Actions</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-zinc-800/60 font-medium">
          {#each contests as c}
            <tr class="hover:bg-zinc-800/30 transition">
              <!-- Publication Status -->
              <td class="px-5 py-3.5 whitespace-nowrap">
                <button
                  on:click={() => togglePublication(c)}
                  class="text-xs px-2.5 py-1 rounded-full font-bold transition flex items-center space-x-1.5 {
                    c.publicationStatus === 'PUBLISHED'
                      ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30 hover:bg-emerald-500/25'
                      : 'bg-amber-500/15 text-amber-300 border border-amber-500/30 hover:bg-amber-500/25'
                  }"
                  title="Click to toggle publication status"
                >
                  {#if c.publicationStatus === 'PUBLISHED'}
                    <Eye class="w-3 h-3" />
                  {:else}
                    <EyeOff class="w-3 h-3" />
                  {/if}
                  <span>{c.publicationStatus}</span>
                </button>
              </td>

              <!-- Contest Name -->
              <td class="px-5 py-3.5">
                <div class="space-y-0.5">
                  <div class="text-white font-semibold flex items-center space-x-1.5">
                    <span>{c.name}</span>
                    <span class="text-xs font-mono text-zinc-500">({c.scoringType})</span>
                  </div>
                  <div class="text-xs text-zinc-400 line-clamp-1">{c.description || 'No description.'}</div>
                </div>
              </td>

              <!-- State -->
              <td class="px-5 py-3.5 whitespace-nowrap">
                <span class="text-xs px-2.5 py-0.5 rounded-full font-bold {
                  c.state === 'ACTIVE' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                  c.state === 'UPCOMING' ? 'bg-zinc-800 text-zinc-300 border border-zinc-700' :
                  'bg-zinc-950 text-zinc-500 border border-zinc-800'
                }">
                  {c.state}
                </span>
              </td>

              <!-- Schedule -->
              <td class="px-5 py-3.5 whitespace-nowrap text-xs text-zinc-400">
                <div class="space-y-0.5">
                  <div>{new Date(c.startAt).toLocaleString()}</div>
                  <div class="text-zinc-500">to {new Date(c.endAt).toLocaleString()}</div>
                </div>
              </td>

              <!-- Participants -->
              <td class="px-5 py-3.5 whitespace-nowrap text-xs text-zinc-400">
                <div class="flex items-center space-x-1">
                  <Users class="w-3.5 h-3.5 text-zinc-500" />
                  <span>{c.participantCount}</span>
                </div>
              </td>

              <!-- Actions -->
              <td class="px-5 py-3.5 text-right whitespace-nowrap">
                <div class="flex items-center justify-end space-x-2">
                  <a
                    href={`/admin/contests/${c.id}/edit`}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
                    title="Edit Contest"
                  >
                    <Edit3 class="w-4 h-4" />
                  </a>

                  <a
                    href={`/contests/${c.id}`}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition"
                    title="View Public Page"
                  >
                    <ExternalLink class="w-4 h-4" />
                  </a>

                  <button
                    on:click={() => handleDelete(c)}
                    class="p-1.5 rounded-lg text-zinc-400 hover:text-rose-400 hover:bg-rose-500/10 transition"
                    title="Delete Contest"
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
