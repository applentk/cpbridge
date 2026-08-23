<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import { pingExtension } from '$lib/extension/bridge';
  import type { ExtensionPingResponse, PlatformType } from '@cp-hub/contracts';
  import { Puzzle, ShieldCheck, CheckCircle2, XCircle, ExternalLink, RefreshCw, AlertTriangle } from 'lucide-svelte';

  let checking = true;
  let extInfo: ExtensionPingResponse | null = null;

  async function checkConnections() {
    checking = true;
    try {
      extInfo = await pingExtension();
    } catch (err) {
      console.error(err);
    } finally {
      checking = false;
    }
  }

  onMount(() => {
    checkConnections();
  });
</script>

<div class="max-w-4xl mx-auto space-y-8 py-4">
  <div class="space-y-1.5 border-b border-zinc-800 pb-4">
    <h1 class="text-3xl font-bold text-white flex items-center space-x-2.5">
      <Puzzle class="w-8 h-8 text-indigo-400" />
      <span>Platform Integrations</span>
    </h1>
    <p class="text-sm text-zinc-400">
      Manage browser session connections for Codeforces and AtCoder.
    </p>
  </div>

  <!-- Security Notice Card -->
  <div class="p-6 rounded-2xl border border-indigo-500/30 bg-indigo-500/5 space-y-3">
    <div class="flex items-center space-x-2 text-indigo-300 font-bold text-sm">
      <ShieldCheck class="w-5 h-5 text-indigo-400 shrink-0" />
      <span>Zero-Cookie Privacy Guarantee</span>
    </div>
    <p class="text-xs text-zinc-300 leading-relaxed">
      CP Hub never stores or uploads your external platform passwords, access tokens, or session cookies.
      The CP Hub Chrome Extension connects directly to your existing browser tabs to automate submissions safely.
    </p>
  </div>

  <!-- Extension Status Banner -->
  <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/60 flex items-center justify-between">
    <div class="flex items-center space-x-3">
      {#if extInfo}
        <div class="w-3 h-3 rounded-full bg-emerald-500 shadow-sm shadow-emerald-500/50 animate-pulse"></div>
        <div>
          <div class="text-sm font-bold text-white">Browser Extension Active</div>
          <div class="text-xs text-zinc-400">Version {extInfo.version} connected</div>
        </div>
      {:else}
        <div class="w-3 h-3 rounded-full bg-zinc-600"></div>
        <div>
          <div class="text-sm font-bold text-white">Extension Not Detected</div>
          <div class="text-xs text-zinc-400">Install or enable the CP Hub Chrome Extension to automate submissions.</div>
        </div>
      {/if}
    </div>

    <button
      on:click={checkConnections}
      disabled={checking}
      class="px-3.5 py-1.5 rounded-xl text-xs font-semibold border border-zinc-700 bg-zinc-900 hover:bg-zinc-800 text-white transition flex items-center space-x-1.5"
    >
      <RefreshCw class="w-3.5 h-3.5 {checking ? 'animate-spin' : ''}" />
      <span>{checking ? 'Checking...' : 'Check Status'}</span>
    </button>
  </div>

  <!-- Platforms Grid -->
  <div class="space-y-4">
    <h2 class="text-lg font-bold text-white">Supported Platforms</h2>

    <!-- Codeforces Card -->
    <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="flex items-start space-x-4">
        <div class="w-12 h-12 rounded-xl bg-red-500/10 border border-red-500/30 flex items-center justify-center text-red-400 font-bold text-lg shrink-0">
          CF
        </div>
        <div class="space-y-1">
          <div class="flex items-center space-x-2">
            <h3 class="text-base font-bold text-white">Codeforces</h3>
            {#if extInfo?.platforms?.CODEFORCES?.loggedIn}
              <span class="text-[11px] px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 font-semibold border border-emerald-500/30">
                Connected ({extInfo.platforms.CODEFORCES.username || 'Session Active'})
              </span>
            {:else}
              <span class="text-[11px] px-2 py-0.5 rounded-full bg-zinc-950 text-zinc-500 border border-zinc-800">
                Not Connected
              </span>
            {/if}
          </div>
          <p class="text-xs text-zinc-400">
            Submit solutions to Codeforces contests and problem set problems.
          </p>
        </div>
      </div>

      <div class="flex items-center space-x-2 shrink-0">
        <a
          href="https://codeforces.com/enter"
          target="_blank"
          rel="noopener noreferrer"
          class="px-4 py-2 rounded-xl text-xs font-semibold border border-zinc-700 bg-zinc-950 hover:bg-zinc-800 text-zinc-200 hover:text-white transition flex items-center space-x-1.5"
        >
          <span>Open Codeforces</span>
          <ExternalLink class="w-3.5 h-3.5" />
        </a>
      </div>
    </div>

    <!-- AtCoder Card -->
    <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
      <div class="flex items-start space-x-4">
        <div class="w-12 h-12 rounded-xl bg-zinc-800 border border-zinc-700 flex items-center justify-center text-white font-bold text-lg shrink-0">
          AC
        </div>
        <div class="space-y-1">
          <div class="flex items-center space-x-2">
            <h3 class="text-base font-bold text-white">AtCoder</h3>
            {#if extInfo?.platforms?.ATCODER?.loggedIn}
              <span class="text-[11px] px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 font-semibold border border-emerald-500/30">
                Connected ({extInfo.platforms.ATCODER.username || 'Session Active'})
              </span>
            {:else}
              <span class="text-[11px] px-2 py-0.5 rounded-full bg-zinc-950 text-zinc-500 border border-zinc-800">
                Not Connected
              </span>
            {/if}
          </div>
          <p class="text-xs text-zinc-400">
            Submit solutions to AtCoder Beginner, Regular, and Grand contests.
          </p>
        </div>
      </div>

      <div class="flex items-center space-x-2 shrink-0">
        <a
          href="https://atcoder.jp/login"
          target="_blank"
          rel="noopener noreferrer"
          class="px-4 py-2 rounded-xl text-xs font-semibold border border-zinc-800 bg-zinc-950 hover:bg-zinc-800 text-zinc-200 transition flex items-center space-x-1.5"
        >
          <span>Open AtCoder</span>
          <ExternalLink class="w-3.5 h-3.5" />
        </a>
      </div>
    </div>
  </div>
</div>
