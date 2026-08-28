<script lang="ts">
  import type { Standings } from '@cpbridge/contracts';
  import ScoreboardRow from '$lib/components/ScoreboardRow.svelte';
  import Pagination from '$lib/components/Pagination.svelte';

  export let standings: Standings;
  export let currentUserId: string | null = null;
  export let contestFinished = false;

  type ScoreboardTab = 'contest' | 'after';

  let activeTab: ScoreboardTab = 'contest';
  let currentPage = 1;
  let pageSize = 20;

  $: activeStandings = activeTab === 'contest' ? standings.standings : standings.upsolveStandings ?? [];
  $: orderedStandings = currentUserId
    ? [...activeStandings].sort((a, b) => Number(b.userId === currentUserId) - Number(a.userId === currentUserId))
    : activeStandings;
  $: paginatedStandings = orderedStandings.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  $: if (currentPage > Math.max(1, Math.ceil(orderedStandings.length / pageSize))) {
    currentPage = Math.max(1, Math.ceil(orderedStandings.length / pageSize));
  }

  function selectTab(tab: ScoreboardTab) {
    activeTab = tab;
    currentPage = 1;
  }
</script>

{#if contestFinished}
  <div class="flex items-center gap-6 px-1" role="tablist" aria-label="Scoreboard period">
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'contest'}
      on:click={() => selectTab('contest')}
      class="border-b-2 px-1 py-2 text-sm font-semibold transition {activeTab === 'contest' ? 'border-white text-white' : 'border-transparent text-zinc-500 hover:text-zinc-200'}"
    >
      In contest <span class="ml-1 text-xs opacity-70">({standings.standings.length})</span>
    </button>
    <button
      type="button"
      role="tab"
      aria-selected={activeTab === 'after'}
      on:click={() => selectTab('after')}
      class="border-b-2 px-1 py-2 text-sm font-semibold transition {activeTab === 'after' ? 'border-white text-white' : 'border-transparent text-zinc-500 hover:text-zinc-200'}"
    >
      After contest <span class="ml-1 text-xs opacity-70">({standings.upsolveStandings?.length ?? 0})</span>
    </button>
  </div>
{/if}

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
      {#if activeStandings.length === 0}
        <tr>
          <td colspan={4 + standings.problems.length} class="py-8 text-center text-zinc-500">
            {activeTab === 'contest' ? 'No participants or submissions recorded yet.' : 'No post-contest solves recorded yet.'}
          </td>
        </tr>
      {:else}
        {#each paginatedStandings as participant}
          <ScoreboardRow participant={participant} problems={standings.problems} />
        {/each}
      {/if}
    </tbody>
  </table>
</div>

<Pagination
  {currentPage}
  {pageSize}
  totalItems={orderedStandings.length}
  pageSizeOptions={[10, 20, 50, 100]}
  itemName={activeTab === 'contest' ? 'participants' : 'upsolvers'}
  on:pageChange={(e) => (currentPage = e.detail)}
  on:pageSizeChange={(e) => {
    pageSize = e.detail;
    currentPage = 1;
  }}
/>
