<script lang="ts">
  import { browser } from '$app/environment';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { auth } from '$lib/stores/auth';
  import { LayoutDashboard, Code2, Layers, Trophy, Users, ExternalLink } from 'lucide-svelte';

  $: {
    if (browser && !$auth.loading && (!$auth.user || $auth.user.role !== 'ADMIN')) {
      goto('/contests');
    }
  }

  const navItems = [
    { label: 'Dashboard', href: '/admin', icon: LayoutDashboard },
    { label: 'Problems', href: '/admin/problems', icon: Code2 },
    { label: 'Problem Sets', href: '/admin/problem-sets', icon: Layers },
    { label: 'Contests', href: '/admin/contests', icon: Trophy },
    { label: 'Users', href: '/admin/users', icon: Users },
  ];
</script>

{#if $auth.loading}
  <div class="p-12 text-center text-zinc-500">Checking permissions...</div>
{:else if $auth.user?.role === 'ADMIN'}
  <div class="flex flex-col md:flex-row gap-8">
    <!-- Admin Sidebar -->
    <aside class="w-full md:w-64 shrink-0 space-y-6">
      <div class="p-4 rounded-2xl border border-zinc-800 bg-zinc-900/60 space-y-4">
        <nav class="space-y-1">
          {#each navItems as item}
            {@const isActive = $page.url.pathname === item.href || ($page.url.pathname.startsWith(item.href) && item.href !== '/admin')}
            <a
              href={item.href}
              class="flex items-center space-x-2.5 px-3 py-2 rounded-xl text-sm font-semibold transition {
                isActive
                  ? 'bg-white text-black shadow-sm'
                  : 'text-zinc-400 hover:text-white hover:bg-zinc-800/60'
              }"
            >
              <svelte:component this={item.icon} class="w-4 h-4" />
              <span>{item.label}</span>
            </a>
          {/each}
        </nav>

        <div class="pt-3 border-t border-zinc-800">
          <a
            href="/contests"
            class="flex items-center justify-between px-3 py-2 rounded-xl text-xs font-medium text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/40 transition"
          >
            <span>View User Site</span>
            <ExternalLink class="w-3.5 h-3.5" />
          </a>
        </div>
      </div>
    </aside>

    <!-- Admin Page Content -->
    <div class="flex-1 min-w-0">
      <slot />
    </div>
  </div>
{:else}
  <div class="p-12 text-center text-zinc-500">Redirecting to contests...</div>
{/if}
