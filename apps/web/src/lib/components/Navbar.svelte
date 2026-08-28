<script lang="ts">
  import { auth } from '$lib/stores/auth';
  import { Trophy, Code2, Cpu, LogOut, LogIn, UserPlus, Puzzle, LayoutDashboard, ChevronDown } from 'lucide-svelte';
</script>

<nav class="border-b border-zinc-800 bg-zinc-900/80 backdrop-blur-md sticky top-0 z-50">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
    <!-- Brand / Logo -->
    <div class="flex items-center space-x-8">
      <a href={$auth.user?.role === 'ADMIN' ? '/admin' : '/contests'} class="flex items-center space-x-2 text-white font-bold text-xl tracking-tight hover:opacity-90">
        <div class="w-8 h-8 rounded-lg bg-zinc-900 border border-zinc-700 flex items-center justify-center">
          <Code2 class="w-5 h-5 text-white" />
        </div>
        <span class="text-white">cp<span class="text-zinc-400">bridge</span></span>
      </a>

      <!-- Navigation Links -->
      <div class="hidden md:flex items-center space-x-1">
        <a href="/contests" class="px-3 py-1.5 rounded-md text-sm font-medium text-zinc-300 hover:text-white hover:bg-zinc-800/60 flex items-center space-x-1.5 transition">
          <Trophy class="w-4 h-4 text-zinc-400" />
          <span>Contests</span>
        </a>
        {#if $auth.user}
          <a href="/submissions" class="px-3 py-1.5 rounded-md text-sm font-medium text-zinc-300 hover:text-white hover:bg-zinc-800/60 flex items-center space-x-1.5 transition">
            <Cpu class="w-4 h-4 text-zinc-400" />
            <span>Submissions</span>
          </a>
        {/if}
        {#if $auth.user?.role === 'ADMIN'}
          <a href="/admin" class="px-3 py-1.5 rounded-md text-sm font-medium text-amber-400 hover:text-amber-300 hover:bg-zinc-800/60 flex items-center space-x-1.5 transition">
            <LayoutDashboard class="w-4 h-4 text-amber-400" />
            <span>Admin Dashboard</span>
          </a>
        {/if}
      </div>
    </div>

    <!-- Right Side Actions -->
    <div class="flex items-center space-x-3">
      <a href="/settings/integrations" class="p-2 rounded-lg text-zinc-400 hover:text-white hover:bg-zinc-800 transition" title="Platform Integrations">
        <Puzzle class="w-5 h-5" />
      </a>

      {#if $auth.loading}
        <div class="w-20 h-8 bg-zinc-800/50 rounded animate-pulse"></div>
      {:else if $auth.user}
        <div class="flex items-center space-x-3 pl-2 border-l border-zinc-800">
          <a href={$auth.user.role === 'ADMIN' ? '/admin' : '/contests'} class="flex items-center space-x-2 text-sm font-medium text-zinc-200 hover:text-white">
            <div class="w-7 h-7 rounded-full bg-zinc-800 border border-zinc-600 flex items-center justify-center text-xs text-white font-semibold uppercase">
              {$auth.user.username.slice(0, 2)}
            </div>
            <div class="flex items-center space-x-1.5">
              <span>{$auth.user.username}</span>
              {#if $auth.user.role === 'ADMIN'}
                <span class="text-[10px] px-1.5 py-0.2 rounded bg-amber-500/20 text-amber-300 font-bold border border-amber-500/30">
                  ADMIN
                </span>
              {/if}
            </div>
          </a>
          <div class="flex items-center">
            <button
                type="button"
                on:click={() => auth.logout()}
                class="p-1.5 text-zinc-400 hover:text-red-400 rounded-l-md hover:bg-red-500/10 transition"
                title="Log Out"
              >
              <LogOut class="w-4 h-4" />
            </button>
            <details class="relative group">
              <summary
                class="list-none cursor-pointer py-1.5 text-zinc-600 hover:text-white rounded-r-md hover:bg-zinc-800 transition [&::-webkit-details-marker]:hidden"
                title="More account actions"
                aria-label="More account actions"
              >
                <ChevronDown class="w-4 h-4 transition-transform group-open:rotate-180" />
              </summary>
              <div class="absolute right-0 top-full mt-2 w-48 rounded-lg border border-zinc-700 bg-zinc-900 p-1.5 shadow-xl shadow-black/30">
                <button
                  type="button"
                  on:click={() => void auth.logoutAll().catch(() => undefined)}
                  class="w-full px-3 py-2 rounded-md text-left text-xs text-zinc-300 hover:text-red-400 hover:bg-red-500/10 transition flex items-center space-x-2"
                  title="Log Out All Devices"
                >
                  <LogOut class="w-3.5 h-3.5" />
                  <span>Log Out All Devices</span>
                </button>
              </div>
            </details>
          </div>
        </div>
      {:else}
        <div class="flex items-center space-x-2">
          <a href="/login" class="px-3.5 py-1.5 rounded-lg text-sm font-medium text-zinc-300 hover:text-white hover:bg-zinc-800 transition flex items-center space-x-1.5">
            <LogIn class="w-4 h-4" />
            <span>Sign In</span>
          </a>
          <a href="/register" class="px-3.5 py-1.5 rounded-lg text-sm font-semibold bg-white hover:bg-zinc-200 text-black shadow-sm transition flex items-center space-x-1.5">
            <UserPlus class="w-4 h-4" />
            <span>Sign Up</span>
          </a>
        </div>
      {/if}
    </div>
  </div>
</nav>
