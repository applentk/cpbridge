<script lang="ts">
  import { Code2, Trophy, Layers, ShieldCheck, ArrowRight, Sparkles, CheckCircle2 } from 'lucide-svelte';
  import { auth } from '$lib/stores/auth';
</script>

<div class="space-y-20 py-8">
  <!-- Hero Section -->
  <div class="text-center max-w-3xl mx-auto space-y-6">
    <div class="inline-flex items-center space-x-2 px-3.5 py-1.5 rounded-full bg-indigo-500/10 border border-indigo-500/30 text-indigo-400 text-xs font-semibold">
      <Sparkles class="w-3.5 h-3.5" />
      <span>One Account. Codeforces + AtCoder.</span>
    </div>

    <h1 class="text-5xl sm:text-6xl font-extrabold tracking-tight text-white leading-tight">
      Competitive Programming, <br />
      <span class="text-transparent bg-clip-text bg-gradient-to-r from-indigo-400 via-purple-400 to-pink-400">
        Unified & Simplified.
      </span>
    </h1>

    <p class="text-lg text-zinc-400 max-w-2xl mx-auto">
      Browse and import problems from Codeforces and AtCoder, organize reusable problem sets, host ICPC-style virtual contests, and write code in an integrated Monaco editor without leaking credentials.
    </p>

    <div class="flex items-center justify-center space-x-4 pt-2">
      {#if $auth.user}
        <a href="/dashboard" class="px-6 py-3 rounded-xl font-semibold bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/30 transition flex items-center space-x-2">
          <span>Go to Dashboard</span>
          <ArrowRight class="w-4 h-4" />
        </a>
      {:else}
        <a href="/register" class="px-6 py-3 rounded-xl font-semibold bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/30 transition flex items-center space-x-2">
          <span>Get Started Free</span>
          <ArrowRight class="w-4 h-4" />
        </a>
        <a href="/problems" class="px-6 py-3 rounded-xl font-semibold border border-zinc-800 bg-zinc-900/60 hover:bg-zinc-800/60 text-zinc-200 transition">
          Browse Problems
        </a>
      {/if}
    </div>
  </div>

  <!-- Platform Cards -->
  <div class="grid grid-cols-1 md:grid-cols-2 gap-6 max-w-4xl mx-auto">
    <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:border-zinc-700 transition space-y-4">
      <div class="w-12 h-12 rounded-xl bg-red-500/10 border border-red-500/30 flex items-center justify-center text-red-400 font-bold text-lg">
        CF
      </div>
      <h3 class="text-xl font-bold text-white">Codeforces</h3>
      <p class="text-sm text-zinc-400">
        Import problems directly from Codeforces contests, problem sets, and gym. Submit solutions via your local session.
      </p>
    </div>

    <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 hover:border-zinc-700 transition space-y-4">
      <div class="w-12 h-12 rounded-xl bg-blue-500/10 border border-blue-500/30 flex items-center justify-center text-blue-400 font-bold text-lg">
        AC
      </div>
      <h3 class="text-xl font-bold text-white">AtCoder</h3>
      <p class="text-sm text-zinc-400">
        Full support for AtCoder Beginner, Regular, and Grand Contests tasks with official URL import, statement rendering, and judging feedback.
      </p>
    </div>
  </div>

  <!-- Features Grid -->
  <div class="rounded-3xl border border-zinc-800 bg-zinc-900/20 p-8 sm:p-12 space-y-8">
    <div class="text-center max-w-2xl mx-auto space-y-2">
      <h2 class="text-3xl font-bold text-white">Engineered for Competitive Programmers</h2>
      <p class="text-zinc-400 text-sm">Everything you need to practice, compete with friends, and track progress.</p>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
      <div class="flex items-start space-x-4">
        <div class="p-3 rounded-xl bg-indigo-500/10 border border-indigo-500/30 text-indigo-400 shrink-0">
          <Layers class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h4 class="font-semibold text-white">Reusable Problem Sets</h4>
          <p class="text-sm text-zinc-400">Curate training sets across Codeforces and AtCoder, reorder problems, and share sets with teammates.</p>
        </div>
      </div>

      <div class="flex items-start space-x-4">
        <div class="p-3 rounded-xl bg-indigo-500/10 border border-indigo-500/30 text-indigo-400 shrink-0">
          <Trophy class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h4 class="font-semibold text-white">Virtual Contests & ICPC Scoring</h4>
          <p class="text-sm text-zinc-400">Snapshot problem sets into live contests with server-enforced countdowns, problem reveals, and penalty calculations.</p>
        </div>
      </div>

      <div class="flex items-start space-x-4">
        <div class="p-3 rounded-xl bg-indigo-500/10 border border-indigo-500/30 text-indigo-400 shrink-0">
          <Code2 class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h4 class="font-semibold text-white">Monaco Editor & KaTeX Reader</h4>
          <p class="text-sm text-zinc-400">Integrated Monaco code editor with file upload & auto language detection alongside math-rendered problem statements.</p>
        </div>
      </div>

      <div class="flex items-start space-x-4">
        <div class="p-3 rounded-xl bg-indigo-500/10 border border-indigo-500/30 text-indigo-400 shrink-0">
          <ShieldCheck class="w-6 h-6" />
        </div>
        <div class="space-y-1">
          <h4 class="font-semibold text-white">Zero-Cookie Browser Bridge</h4>
          <p class="text-sm text-zinc-400">Submit solutions securely using your active browser session via our Manifest V3 extension without sharing passwords.</p>
        </div>
      </div>
    </div>
  </div>
</div>
