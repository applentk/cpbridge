<script lang="ts">
  import type { Problem } from '@cp-hub/contracts';
  import { ExternalLink, Tag } from 'lucide-svelte';

  export let problem: Problem;
  export let label: string | null = null;
</script>

<div class="p-4 rounded-xl border border-zinc-800 bg-zinc-900/40 hover:bg-zinc-800/30 transition flex items-center justify-between">
  <div class="space-y-1">
    <div class="flex items-center space-x-2.5">
      {#if label}
        <span class="w-7 h-7 rounded-lg bg-indigo-500/20 border border-indigo-500/40 text-indigo-300 font-bold text-sm flex items-center justify-center">
          {label}
        </span>
      {/if}
      <a href={`/problems/${problem.id}`} class="font-semibold text-zinc-100 hover:text-indigo-400 transition text-base">
        {problem.title}
      </a>
      <span class="text-xs px-2 py-0.5 rounded-full font-medium {
        problem.platform === 'CODEFORCES' ? 'bg-red-500/20 text-red-300 border border-red-500/30' :
        'bg-blue-500/20 text-blue-300 border border-blue-500/30'
      }">
        {problem.platform}
      </span>
      {#if problem.difficulty}
        <span class="text-xs px-2 py-0.5 rounded-full font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
          ★ {problem.difficulty}
        </span>
      {/if}
    </div>

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
  </div>

  <div class="flex items-center space-x-2">
    <a
      href={problem.url}
      target="_blank"
      rel="noopener noreferrer"
      class="p-2 rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 transition"
      title="Open official statement"
    >
      <ExternalLink class="w-4 h-4" />
    </a>
    <a
      href={`/problems/${problem.id}`}
      class="px-3 py-1.5 rounded-lg text-xs font-semibold bg-indigo-600 hover:bg-indigo-500 text-white transition"
    >
      Solve
    </a>
  </div>
</div>
