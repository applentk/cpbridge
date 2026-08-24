<script lang="ts">
  import type { Problem } from '@cpbridge/contracts';
  import { ExternalLink, Tag } from 'lucide-svelte';

  export let problem: Problem;
  export let label: string | null = null;
  export let contestId: string | null = null;
  export let inContest: boolean = false;

  $: isInContest = inContest || Boolean(contestId);
  $: problemHref = contestId ? `/problems/${problem.id}?contestId=${contestId}` : `/problems/${problem.id}`;
</script>

<div class="p-4 rounded-xl border border-zinc-800 bg-zinc-900/50 hover:border-zinc-700 hover:bg-zinc-900/80 transition flex items-center justify-between">
  <div class="space-y-1">
    <div class="flex items-center space-x-2.5">
      {#if label}
        <span class="w-7 h-7 rounded-lg bg-zinc-800 border border-zinc-700 text-white font-bold text-sm flex items-center justify-center">
          {label}
        </span>
      {/if}
      <a href={problemHref} class="font-semibold text-white hover:text-zinc-300 transition text-base">
        {problem.title}
      </a>
      {#if !isInContest}
        <span class="text-xs px-2 py-0.5 rounded-full font-mono font-semibold {
          problem.platform === 'CODEFORCES' ? 'bg-red-500/15 text-red-300 border border-red-500/30' :
          'bg-zinc-800 text-zinc-300 border border-zinc-700'
        }">
          {problem.platform}
        </span>
        {#if problem.difficulty}
          <span class="text-xs px-2 py-0.5 rounded-full font-mono bg-zinc-950 text-zinc-400 border border-zinc-800">
            ★ {problem.difficulty}
          </span>
        {/if}
      {/if}
    </div>

    {#if !isInContest}
      <div class="flex items-center space-x-2 text-xs text-zinc-400">
        <span class="font-mono text-zinc-500">{problem.externalId}</span>
        {#if problem.tags && problem.tags.length > 0}
          <span>•</span>
          <div class="flex items-center space-x-1.5 overflow-hidden">
            <Tag class="w-3 h-3 text-zinc-500" />
            <span>{problem.tags.slice(0, 3).join(', ')}</span>
          </div>
        {/if}
      </div>
    {/if}
  </div>

  <div class="flex items-center space-x-2">
    {#if !isInContest}
      <a
        href={problem.url}
        target="_blank"
        rel="noopener noreferrer"
        class="p-2 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
        title="Open official statement"
      >
        <ExternalLink class="w-4 h-4" />
      </a>
    {/if}
    <a
      href={problemHref}
      class="px-3.5 py-1.5 rounded-lg text-xs font-semibold bg-white hover:bg-zinc-200 text-black shadow-sm transition"
    >
      Solve
    </a>
  </div>
</div>
