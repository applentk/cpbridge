<script lang="ts">
  import type { Problem } from '@cpbridge/contracts';
  import { CheckCircle2, XCircle, ChevronRight } from 'lucide-svelte';

  export let problem: Problem;
  export let label: string | null = null;
  export let contestId: string | null = null;
  export let inContest: boolean = false;
  export let isSolved: boolean = false;
  export let isWrong: boolean = false;

  $: isInContest = inContest || Boolean(contestId);
  $: problemHref = contestId ? `/problems/${problem.id}?contestId=${contestId}` : `/problems/${problem.id}`;
</script>

<a
  href={problemHref}
  class="group h-full flex items-center justify-between p-4 sm:p-5 rounded-2xl border transition shadow-sm hover:shadow-md {
    isSolved
      ? 'bg-emerald-500/[0.04] border-emerald-500/30 hover:border-emerald-500/50 hover:bg-emerald-500/[0.08]'
      : isWrong
      ? 'bg-rose-500/[0.04] border-rose-500/30 hover:border-rose-500/50 hover:bg-rose-500/[0.08]'
      : 'bg-zinc-900/50 border-zinc-800 hover:border-zinc-700 hover:bg-zinc-900/80'
  }"
>
  <div class="flex items-center space-x-3 min-w-0 flex-1 mr-3">
    {#if label}
      <span class="w-8 h-8 rounded-xl font-mono font-bold text-sm flex items-center justify-center shrink-0 {
        isSolved
          ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/40'
          : isWrong
          ? 'bg-rose-500/20 text-rose-300 border border-rose-500/40'
          : 'bg-zinc-800 text-white border border-zinc-700'
      }">
        {label}
      </span>
    {/if}

    <div class="min-w-0 flex-1">
      <span class="font-bold text-base text-white group-hover:text-zinc-100 transition line-clamp-1">
        {problem.title}
      </span>

      {#if !isInContest}
        <div class="flex items-center space-x-1.5 mt-1">
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
        </div>
      {/if}
    </div>
  </div>

  <div class="flex items-center space-x-2.5 shrink-0">
    {#if isInContest && (isSolved || isWrong)}
      <div>
        {#if isSolved}
          <div class="flex items-center space-x-1 px-2.5 py-0.5 rounded-lg text-xs font-semibold bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
            <CheckCircle2 class="w-3.5 h-3.5 text-emerald-400" />
            <span>Solved</span>
          </div>
        {:else if isWrong}
          <div class="flex items-center space-x-1 px-2.5 py-0.5 rounded-lg text-xs font-semibold bg-rose-500/15 text-rose-300 border border-rose-500/30">
            <XCircle class="w-3.5 h-3.5 text-rose-400" />
            <span>Attempted</span>
          </div>
        {/if}
      </div>
    {/if}
    <ChevronRight class="w-4 h-4 text-zinc-600 group-hover:text-zinc-400 group-hover:translate-x-0.5 transition-transform" />
  </div>
</a>
