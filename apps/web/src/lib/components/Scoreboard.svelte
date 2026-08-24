<script lang="ts">
  import type { Standings } from '@cpbridge/contracts';
  import { Trophy } from 'lucide-svelte';

  export let standings: Standings;
</script>

<div class="overflow-x-auto rounded-xl border border-zinc-800 bg-zinc-900/60 shadow-lg">
  <table class="w-full text-left text-sm text-zinc-300">
    <thead class="bg-zinc-800/90 text-xs uppercase tracking-wider text-zinc-400 font-semibold border-b border-zinc-700">
      <tr>
        <th class="py-3.5 px-4 w-16 text-center">Rank</th>
        <th class="py-3.5 px-4">Participant</th>
        <th class="py-3.5 px-4 w-24 text-center">Solved</th>
        <th class="py-3.5 px-4 w-28 text-center">{standings.scoringType === 'ICPC' ? 'Penalty' : 'Time'}</th>
        {#each standings.problems as prob}
          <th class="py-3.5 px-4 text-center min-w-[70px]">
            <a href={`/problems/${prob.problemId}?contestId=${standings.contestId}`} class="hover:text-white font-bold text-base transition">
              {prob.label}
            </a>
          </th>
        {/each}
      </tr>
    </thead>
    <tbody class="divide-y divide-zinc-800/60">
      {#if standings.standings.length === 0}
        <tr>
          <td colspan={4 + standings.problems.length} class="py-8 text-center text-zinc-500">
            No participants or submissions recorded yet.
          </td>
        </tr>
      {/if}

      {#each standings.standings as participant}
        <tr class="hover:bg-zinc-800/40 transition {participant.rank === 1 ? 'bg-zinc-800/20' : ''}">
          <td class="py-3 px-4 text-center font-bold font-mono">
            {#if participant.rank === 1}
              <span class="inline-flex items-center justify-center text-white">
                <Trophy class="w-4 h-4 inline mr-1 text-white" /> 1
              </span>
            {:else if participant.rank === 2}
              <span class="text-zinc-300 font-bold">2</span>
            {:else if participant.rank === 3}
              <span class="text-zinc-400 font-bold">3</span>
            {:else}
              <span class="text-zinc-500">{participant.rank}</span>
            {/if}
          </td>

          <td class="py-3 px-4 font-semibold text-zinc-100 flex items-center space-x-2">
            <div class="w-6 h-6 rounded-full bg-zinc-800 border border-zinc-700 flex items-center justify-center text-xs text-white font-mono">
              {participant.username.slice(0, 1).toUpperCase()}
            </div>
            <span>{participant.username}</span>
          </td>

          <td class="py-3 px-4 text-center font-bold text-base text-white font-mono">
            {participant.solvedCount}
          </td>

          <td class="py-3 px-4 text-center font-mono text-zinc-400 text-sm">
            {participant.totalPenalty}
          </td>

          {#each standings.problems as prob}
            {@const pScore = participant.problemScores[prob.problemId]}
            <td class="py-3 px-4 text-center">
              {#if pScore && pScore.solved}
                <div class="p-1.5 rounded-lg bg-emerald-500/15 border border-emerald-500/30 text-emerald-300 font-mono text-xs font-bold">
                  <div>+{pScore.attempts > 1 ? pScore.attempts - 1 : ''}</div>
                  <div class="text-[10px] text-emerald-400/80 font-normal">{pScore.firstSolvedAtMinutes}m</div>
                </div>
              {:else if pScore && pScore.attempts > 0}
                <div class="p-1.5 rounded-lg bg-rose-500/15 border border-rose-500/30 text-rose-300 font-mono text-xs font-bold">
                  -{pScore.attempts}
                </div>
              {:else}
                <span class="text-zinc-600 font-mono">.</span>
              {/if}
            </td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>
