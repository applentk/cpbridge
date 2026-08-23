<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { Submission } from '@cp-hub/contracts';
  import { Cpu, RefreshCw, Filter, ExternalLink } from 'lucide-svelte';

  let submissions: Submission[] = [];
  let loading = true;
  let filterUser = '';

  async function loadSubmissions() {
    loading = true;
    try {
      let path = '/submissions?limit=50';
      if (filterUser) path += `&userId=${filterUser}`;
      submissions = await api.get<Submission[]>(path);
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadSubmissions();
  });
</script>

<div class="space-y-6">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-white flex items-center space-x-2">
        <Cpu class="w-7 h-7 text-indigo-400" />
        <span>Submissions Log</span>
      </h1>
      <p class="text-sm text-zinc-400">Track judging results and submission history across all platforms.</p>
    </div>

    <div class="flex items-center space-x-3">
      {#if $auth.user}
        <button
          on:click={() => {
            filterUser = filterUser ? '' : $auth.user!.id;
            loadSubmissions();
          }}
          class="px-3.5 py-2 rounded-xl text-xs font-semibold border border-zinc-800 transition {
            filterUser ? 'bg-indigo-600/20 text-indigo-300 border-indigo-500/40' : 'bg-zinc-900 text-zinc-300 hover:bg-zinc-800'
          }"
        >
          {filterUser ? 'Show All Submissions' : 'My Submissions Only'}
        </button>
      {/if}
      <button
        on:click={loadSubmissions}
        class="p-2.5 rounded-xl border border-zinc-800 bg-zinc-900 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition"
        title="Refresh"
      >
        <RefreshCw class="w-4 h-4" />
      </button>
    </div>
  </div>

  {#if loading}
    <div class="space-y-3">
      {#each Array(6) as _}
        <div class="h-16 rounded-xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      {/each}
    </div>
  {:else if submissions.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center text-zinc-500">
      No submissions recorded.
    </div>
  {:else}
    <div class="overflow-x-auto rounded-xl border border-zinc-800 bg-zinc-900/60 shadow-lg">
      <table class="w-full text-left text-sm text-zinc-300">
        <thead class="bg-zinc-800/80 text-xs uppercase tracking-wider text-zinc-400 font-semibold border-b border-zinc-700/80">
          <tr>
            <th class="py-3.5 px-4">Problem</th>
            <th class="py-3.5 px-4">User</th>
            <th class="py-3.5 px-4">Platform</th>
            <th class="py-3.5 px-4">Language</th>
            <th class="py-3.5 px-4">Verdict</th>
            <th class="py-3.5 px-4">External ID</th>
            <th class="py-3.5 px-4">Submitted At</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-zinc-800/60 font-mono text-xs">
          {#each submissions as s}
            <tr class="hover:bg-zinc-800/30 transition">
              <td class="py-3 px-4 font-sans font-semibold text-zinc-100">
                <a href={`/problems/${s.problemId}`} class="hover:text-indigo-400 transition">
                  {s.problemTitle || s.problemId}
                </a>
              </td>

              <td class="py-3 px-4 font-sans text-zinc-300">
                {s.username || 'user'}
              </td>

              <td class="py-3 px-4">
                <span class="px-2 py-0.5 rounded text-[11px] font-bold {
                  s.platform === 'CODEFORCES' ? 'text-red-400' :
                  'text-blue-400'
                }">
                  {s.platform}
                </span>
              </td>

              <td class="py-3 px-4 text-zinc-400">
                {s.language}
              </td>

              <td class="py-3 px-4">
                <span class="px-2.5 py-1 rounded-md font-bold {
                  s.status === 'ACCEPTED' ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
                  s.status === 'WRONG_ANSWER' ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' :
                  s.status === 'JUDGING' || s.status === 'PENDING' ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' :
                  'bg-zinc-800 text-zinc-400 border border-zinc-700'
                }">
                  {s.status}
                </span>
              </td>

              <td class="py-3 px-4 text-zinc-500">
                {s.externalSubmissionId || '-'}
              </td>

              <td class="py-3 px-4 text-zinc-500">
                {new Date(s.submittedAt).toLocaleString()}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>
