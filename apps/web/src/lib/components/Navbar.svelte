<script lang="ts">
  import { auth } from '$lib/stores/auth';
  import { Trophy, Code2, Layers, Cpu, LogOut, LogIn, UserPlus, Puzzle } from 'lucide-svelte';
</script>

<nav class="border-b border-zinc-800 bg-zinc-900/60 backdrop-blur-md sticky top-0 z-50">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
    <!-- Brand / Logo -->
    <div class="flex items-center space-x-8">
      <a href="/" class="flex items-center space-x-2 text-indigo-400 font-bold text-xl tracking-tight hover:opacity-90">
        <div class="w-8 h-8 rounded-lg bg-indigo-600/30 border border-indigo-500/40 flex items-center justify-center">
          <Code2 class="w-5 h-5 text-indigo-400" />
        </div>
        <span class="text-white">CP<span class="text-indigo-400">Hub</span></span>
      </a>

      <div class="hidden md:flex items-center space-x-1">
        <a href="/problems" class="px-3 py-1.5 rounded-md text-sm font-medium text-zinc-300 hover:text-white hover:bg-zinc-800/60 flex items-center space-x-1.5 transition">
          <Code2 class="w-4 h-4 text-zinc-400" />
          <span>Problems</span>
        </a>
        <a href="/problem-sets" class="px-3 py-1.5 rounded-md text-sm font-medium text-zinc-300 hover:text-white hover:bg-zinc-800/60 flex items-center space-x-1.5 transition">
          <Layers class="w-4 h-4 text-zinc-400" />
          <span>Problem Sets</span>
        </a>
        <a href="/contests" class="px-3 py-1.5 rounded-md text-sm font-medium text-zinc-300 hover:text-white hover:bg-zinc-800/60 flex items-center space-x-1.5 transition">
          <Trophy class="w-4 h-4 text-zinc-400" />
          <span>Virtual Contests</span>
        </a>
        <a href="/submissions" class="px-3 py-1.5 rounded-md text-sm font-medium text-zinc-300 hover:text-white hover:bg-zinc-800/60 flex items-center space-x-1.5 transition">
          <Cpu class="w-4 h-4 text-zinc-400" />
          <span>Submissions</span>
        </a>
      </div>
    </div>

    <!-- Right Side Actions -->
    <div class="flex items-center space-x-3">
      <a href="/settings/integrations" class="p-2 rounded-lg text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/60 transition" title="Platform Integrations">
        <Puzzle class="w-5 h-5" />
      </a>

      {#if $auth.loading}
        <div class="w-20 h-8 bg-zinc-800/50 rounded animate-pulse"></div>
      {:else if $auth.user}
        <div class="flex items-center space-x-3 pl-2 border-l border-zinc-800">
          <a href="/dashboard" class="flex items-center space-x-2 text-sm font-medium text-zinc-200 hover:text-white">
            <div class="w-7 h-7 rounded-full bg-indigo-600/30 border border-indigo-500/50 flex items-center justify-center text-xs text-indigo-300 font-semibold uppercase">
              {$auth.user.username.slice(0, 2)}
            </div>
            <span>{$auth.user.username}</span>
          </a>
          <button
            on:click={() => auth.logout()}
            class="p-1.5 text-zinc-400 hover:text-red-400 rounded-md hover:bg-zinc-800/60 transition"
            title="Log Out"
          >
            <LogOut class="w-4 h-4" />
          </button>
        </div>
      {:else}
        <div class="flex items-center space-x-2">
          <a href="/login" class="px-3.5 py-1.5 rounded-lg text-sm font-medium text-zinc-300 hover:text-white hover:bg-zinc-800 transition flex items-center space-x-1.5">
            <LogIn class="w-4 h-4" />
            <span>Sign In</span>
          </a>
          <a href="/register" class="px-3.5 py-1.5 rounded-lg text-sm font-medium bg-indigo-600 hover:bg-indigo-500 text-white shadow-sm transition flex items-center space-x-1.5">
            <UserPlus class="w-4 h-4" />
            <span>Sign Up</span>
          </a>
        </div>
      {/if}
    </div>
  </div>
</nav>
