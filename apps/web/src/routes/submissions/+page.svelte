<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import { reconcileExtensionSubmissions } from '$lib/extension/reconcile';
  import { type Submission, formatLanguageName } from '@cp-hub/contracts';
  import SubmissionModal from '$lib/components/SubmissionModal.svelte';
  import { Cpu, RefreshCw, Filter, ExternalLink, Code2 } from 'lucide-svelte';

  let submissions: Submission[] = [];
  let loading = true;
  let filterUser = '';
  let interval: any = null;
  let viewingSubmission: Submission | null = null;

  async function loadSubmissions(silent = false) {
    if (!silent) loading = true;
    try {
      let path = '/submissions?limit=50';
      if (filterUser) path += `&userId=${filterUser}`;
      submissions = await api.get<Submission[]>(path);
    } catch (err) {
      console.error(err);
    } finally {
      if (!silent) loading = false;
    }
  }

  onMount(() => {
    reconcileExtensionSubmissions().finally(() => loadSubmissions());
    // Poll every 3 seconds to auto-update judging submissions
    interval = setInterval(() => {
      const hasPending = submissions.some(s => s.status === 'JUDGING' || s.status === 'PENDING' || s.status === 'DISPATCHING');
      if (hasPending) {
        loadSubmissions(true);
      }
    }, 3000);
  });

  onDestroy(() => {
    if (interval) clearInterval(interval);
  });
</script>

<div class="space-y-6">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-white flex items-center space-x-2">
        <Cpu class="w-7 h-7 text-white" />
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
            filterUser ? 'bg-white text-black shadow-sm' : 'bg-zinc-900 text-zinc-300 hover:text-white hover:bg-zinc-800'
          }"
        >
          {filterUser ? 'Show All Submissions' : 'My Submissions Only'}
        </button>
      {/if}
      <button
        on:click={() => loadSubmissions()}
        class="p-2.5 rounded-xl border border-zinc-800 bg-zinc-900 text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
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
        <thead class="bg-zinc-800/90 text-xs uppercase tracking-wider text-zinc-400 font-semibold border-b border-zinc-700">
          <tr>
            <th class="py-3.5 px-4">Problem</th>
            <th class="py-3.5 px-4">User</th>
            <th class="py-3.5 px-4">Platform</th>
            <th class="py-3.5 px-4">Language & ID</th>
            <th class="py-3.5 px-4">Verdict</th>
            <th class="py-3.5 px-4">External ID</th>
            <th class="py-3.5 px-4">Submitted At</th>
            <th class="py-3.5 px-4 text-right">Source Code</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-zinc-800/60 font-mono text-xs">
          {#each submissions as s}
            <tr
              on:click={() => (viewingSubmission = s)}
              class="hover:bg-zinc-800/40 cursor-pointer transition group"
            >
              <td class="py-3 px-4 font-sans font-semibold text-zinc-100">
                <a
                  href={`/problems/${s.problemId}`}
                  on:click|stopPropagation
                  class="hover:text-white underline decoration-zinc-700 hover:decoration-white transition"
                >
                  {s.problemTitle || s.problemId}
                </a>
              </td>

              <td class="py-3 px-4 font-sans text-zinc-300">
                {s.username || 'user'}
              </td>

              <td class="py-3 px-4">
                <span class="px-2 py-0.5 rounded text-[11px] font-bold font-mono {
                  s.platform === 'CODEFORCES' ? 'text-red-400' : 'text-zinc-300'
                }">
                  {s.platform}
                </span>
              </td>

              <td class="py-3 px-4">
                <div class="space-y-0.5">
                  <div class="text-zinc-200 font-semibold">{formatLanguageName(s.language)}</div>
                  <div class="text-zinc-500 text-[11px] font-mono">{s.id}</div>
                </div>
              </td>

              <td class="py-3 px-4">
                <div class="space-y-1">
                  <span class="px-2.5 py-1 rounded-md font-bold inline-block {
                    s.status === 'ACCEPTED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                    s.status === 'WRONG_ANSWER' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30' :
                    s.status === 'JUDGING' || s.status === 'PENDING' || s.status === 'DISPATCHING' ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30 animate-pulse' :
                    'bg-zinc-950 text-zinc-400 border border-zinc-800'
                  }">
                    {s.status}
                  </span>
                  {#if s.metadata && s.metadata.error}
                    <div class="text-[11px] font-sans text-rose-400 max-w-xs truncate" title={s.metadata.error}>
                      {s.metadata.error}
                    </div>
                  {/if}
                </div>
              </td>

              <td class="py-3 px-4 text-zinc-500">
                {#if $auth.user?.role === 'ADMIN' && s.sourceUrl}
                  <a
                    href={s.sourceUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    on:click|stopPropagation
                    class="inline-flex items-center gap-1 text-sky-400 hover:text-sky-300 transition"
                    title="Open external submission source"
                  >
                    <span>{s.externalSubmissionId}</span>
                    <ExternalLink class="w-3 h-3" />
                  </a>
                {:else}
                  {s.externalSubmissionId || '-'}
                {/if}
              </td>

              <td class="py-3 px-4 text-zinc-500">
                {new Date(s.submittedAt).toLocaleString()}
              </td>

              <td class="py-3 px-4 text-right">
                <button
                  on:click|stopPropagation={() => (viewingSubmission = s)}
                  class="px-2.5 py-1.5 rounded-lg text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-300 hover:text-white border border-zinc-700 transition inline-flex items-center space-x-1.5"
                >
                  <Code2 class="w-3.5 h-3.5 text-zinc-400" />
                  <span>View Code</span>
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<SubmissionModal
  submission={viewingSubmission}
  open={!!viewingSubmission}
  onClose={() => (viewingSubmission = null)}
/>
