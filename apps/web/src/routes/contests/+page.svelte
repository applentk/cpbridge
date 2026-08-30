<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import type { Contest } from '@cpbridge/contracts';
  import { Clock, Users, ArrowRight, Plus } from 'lucide-svelte';

  let contests: Contest[] = [];
  let loading = true;

  async function loadContests() {
    loading = true;
    try {
      contests = await api.get<Contest[]>('/contests');
    } catch (err) {
      console.error(err);
    } finally {
      loading = false;
    }
  }

  $: activeContests = contests.filter((c) => c.state === 'ACTIVE');
  $: upcomingContests = contests.filter((c) => c.state === 'UPCOMING');
  $: allContests = contests.filter((c) => c.state === 'FINISHED');

  $: contestSections = [
    {
      title: 'Active Contests',
      contests: activeContests,
      cardClass:
        'border-emerald-500/35 bg-emerald-500/10 hover:bg-emerald-500/15 hover:border-emerald-400/50',
      badgeClass: 'bg-emerald-500/20 text-emerald-200 border border-emerald-400/40'
    },
    {
      title: 'Upcoming Contests',
      contests: upcomingContests,
      cardClass: 'border-amber-500/35 bg-amber-500/10 hover:bg-amber-500/15 hover:border-amber-400/50',
      badgeClass: 'bg-amber-500/20 text-amber-200 border border-amber-400/40'
    },
    {
      title: 'All Events',
      contests: allContests,
      cardClass: 'border-zinc-700 bg-zinc-800/45 hover:bg-zinc-800/70 hover:border-zinc-600',
      badgeClass: 'bg-zinc-700/70 text-zinc-200 border border-zinc-600'
    }
  ].filter((section) => section.contests.length > 0);

  onMount(() => {
    loadContests();
  });
</script>

<div class="space-y-6">
  <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-white">Contests</h1>
      <p class="text-sm text-zinc-400">Participate in server-timed competitive programming contests and test your problem solving skills.</p>
    </div>

    {#if $auth.user?.role === 'ADMIN'}
      <a
        href="/admin/contests/new"
        class="px-4 py-2.5 rounded-xl font-bold bg-white hover:bg-zinc-200 text-black shadow-sm transition flex items-center space-x-2 shrink-0 self-start sm:self-auto"
      >
        <Plus class="w-4 h-4" />
        <span>Create Contest</span>
      </a>
    {/if}
  </div>

  {#if loading}
    <div class="grid grid-cols-1 md:grid-cols-3 gap-4">
      {#each Array(6) as _}
        <div class="h-48 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
      {/each}
    </div>
  {:else if contestSections.length === 0}
    <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/20 text-center space-y-4">
      <p class="text-zinc-400 text-base">No contests currently available.</p>
      {#if $auth.user?.role === 'ADMIN'}
        <a
          href="/admin/contests/new"
          class="px-4 py-2 rounded-xl text-sm font-bold bg-white hover:bg-zinc-200 text-black transition inline-flex items-center space-x-1.5"
        >
          <Plus class="w-4 h-4" />
          <span>Create a Contest</span>
        </a>
      {/if}
    </div>
  {:else}
    <div class="space-y-8">
      {#each contestSections as section}
        <section aria-labelledby={section.title.toLowerCase().replaceAll(' ', '-')} class="space-y-3">
          <div class="flex items-center justify-between border-b border-zinc-800 pb-3">
            <h2 id={section.title.toLowerCase().replaceAll(' ', '-')} class="text-xl font-bold text-white">
              {section.title}
            </h2>
            <span class="text-xs font-semibold text-zinc-500">{section.contests.length} event{section.contests.length === 1 ? '' : 's'}</span>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-3 gap-5">
            {#each section.contests as c}
              <a
                href={`/contests/${c.id}`}
                class="p-6 rounded-2xl border transition flex flex-col justify-between space-y-4 group {section.cardClass}"
              >
                <div class="space-y-3">
                  <div class="flex items-center justify-between">
                    <span class="text-xs px-2.5 py-0.5 rounded-full font-bold {section.badgeClass}">
                      {c.state}
                    </span>

                    <span class="text-xs font-mono font-semibold text-zinc-400">{c.scoringType}</span>
                  </div>

                  <div>
                    <h3 class="text-lg font-bold text-white group-hover:text-zinc-200 transition">{c.name}</h3>
                    <p class="text-xs text-zinc-400 line-clamp-2 mt-1">{c.description || 'No description.'}</p>
                  </div>

                  <div class="space-y-1.5 text-xs text-zinc-400 pt-1">
                    <div class="flex items-center space-x-1.5">
                      <Clock class="w-3.5 h-3.5 text-zinc-500" />
                      <span>Starts: {new Date(c.startAt).toLocaleString()}</span>
                    </div>
                    <div class="flex items-center space-x-1.5">
                      <Users class="w-3.5 h-3.5 text-zinc-500" />
                      <span>{c.participantCount} participant{c.participantCount === 1 ? '' : 's'}</span>
                    </div>
                  </div>
                </div>

                <div class="flex items-center justify-between pt-3 border-t border-white/10 text-xs">
                  <span class="text-zinc-500">by {c.ownerUsername}</span>
                  <span class="text-zinc-300 font-semibold flex items-center space-x-1 group-hover:text-white group-hover:translate-x-0.5 transition">
                    <span>View Contest</span>
                    <ArrowRight class="w-3.5 h-3.5" />
                  </span>
                </div>
              </a>
            {/each}
          </div>
        </section>
      {/each}
    </div>
  {/if}
</div>
