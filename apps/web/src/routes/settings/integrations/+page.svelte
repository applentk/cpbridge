<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api/client';
  import { pingExtension } from '$lib/extension/bridge';
  import type { ExtensionPingResponse, PlatformType } from '@cpbridge/contracts';
  import {
    Puzzle,
    ShieldCheck,
    CheckCircle2,
    ExternalLink,
    RefreshCw,
    AlertTriangle,
    Download,
    Copy,
    Check,
    Info
  } from 'lucide-svelte';

  let checking = true;
  let extInfo: ExtensionPingResponse | null = null;
  let copiedUrl = false;
  let identitySyncError = '';

  interface PlatformIntegration {
    platform: PlatformType;
    externalUsername: string;
    connectionStatus: string;
  }

  async function syncVerifiedIdentities(info: ExtensionPingResponse) {
    const existing = await api.get<PlatformIntegration[]>('/integrations');
    const platforms: PlatformType[] = ['CODEFORCES', 'ATCODER'];

    await Promise.all(platforms.map(async (platform) => {
      const session = info.platforms[platform];
      const username = session?.username?.trim();
      if (!session?.loggedIn || !username) return;

      const linked = existing.find((integration) => integration.platform === platform);
      if (
        linked?.connectionStatus === 'CONNECTED'
        && linked.externalUsername.toLowerCase() === username.toLowerCase()
      ) {
        return;
      }

      await api.put(`/integrations/${platform}`, {
        externalUsername: username,
        connectionStatus: 'CONNECTED'
      });
    }));
  }

  async function checkConnections() {
    checking = true;
    identitySyncError = '';
    try {
      extInfo = await pingExtension();
      if (extInfo) {
        try {
          await syncVerifiedIdentities(extInfo);
        } catch (err) {
          identitySyncError = err instanceof Error ? err.message : 'Could not synchronize platform identities';
        }
      }
    } catch (err) {
      console.error(err);
    } finally {
      checking = false;
    }
  }

  function copyExtensionsUrl() {
    navigator.clipboard.writeText('chrome://extensions');
    copiedUrl = true;
    setTimeout(() => {
      copiedUrl = false;
    }, 2000);
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
      cpbridge never stores or uploads your external platform passwords, access tokens, or session cookies.
      The cpbridge Chrome Extension connects directly to your existing browser tabs to automate submissions safely.
    </p>
  </div>

  <!-- Extension Status & Download Banner -->
  {#if checking && !extInfo}
    <div class="p-6 rounded-2xl border border-zinc-800 bg-zinc-900/40 space-y-6 animate-pulse">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div class="flex items-center space-x-3.5">
          <div class="w-10 h-10 rounded-xl bg-zinc-800 shrink-0"></div>
          <div class="space-y-2">
            <div class="h-5 w-48 bg-zinc-800 rounded-md"></div>
            <div class="h-3.5 w-64 max-w-full bg-zinc-800/60 rounded-md"></div>
          </div>
        </div>
        <div class="flex items-center space-x-3 shrink-0">
          <div class="h-8 w-36 bg-zinc-800 rounded-xl"></div>
          <div class="h-8 w-28 bg-zinc-800 rounded-xl"></div>
        </div>
      </div>
    </div>
  {:else}
    <div class="p-6 rounded-2xl border {extInfo ? 'border-emerald-500/30 bg-emerald-500/5' : 'border-amber-500/30 bg-amber-500/5'} space-y-6 transition">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div class="flex items-center space-x-3.5">
          {#if extInfo}
            <div class="w-10 h-10 rounded-xl bg-emerald-500/20 border border-emerald-500/40 flex items-center justify-center shrink-0">
              <CheckCircle2 class="w-5 h-5 text-emerald-400" />
            </div>
            <div>
              <div class="text-base font-bold text-white flex items-center space-x-2">
                <span>Browser Extension Active</span>
                <span class="text-xs px-2 py-0.5 rounded-md bg-emerald-500/20 text-emerald-300 font-semibold border border-emerald-500/30">
                  v{extInfo.version}
                </span>
              </div>
              <div class="text-xs text-zinc-400">Connected and ready to submit to Codeforces and AtCoder.</div>
            </div>
          {:else}
            <div class="w-10 h-10 rounded-xl bg-amber-500/20 border border-amber-500/40 flex items-center justify-center shrink-0">
              <AlertTriangle class="w-5 h-5 text-amber-400" />
            </div>
            <div>
              <div class="text-base font-bold text-white">Extension Not Detected</div>
              <div class="text-xs text-zinc-400">Download and load the cpbridge Extension to enable automatic submissions.</div>
            </div>
          {/if}
        </div>

        <div class="flex items-center space-x-3 shrink-0">
          <a
            href="/downloads/cpbridge-extension.zip"
            download="cpbridge-extension.zip"
            class="px-4 py-2 rounded-xl text-xs font-semibold bg-indigo-600 hover:bg-indigo-500 text-white transition flex items-center space-x-2 shadow-lg shadow-indigo-600/20"
          >
            <Download class="w-4 h-4" />
            <span>{extInfo ? 'Re-download (.zip)' : 'Download Extension (.zip)'}</span>
          </a>

          <button
            on:click={checkConnections}
            disabled={checking}
            class="px-3.5 py-2 rounded-xl text-xs font-semibold border border-zinc-700 bg-zinc-900 hover:bg-zinc-800 text-white transition flex items-center space-x-1.5"
          >
            <RefreshCw class="w-3.5 h-3.5 {checking ? 'animate-spin' : ''}" />
            <span>{checking ? 'Checking...' : 'Check Status'}</span>
          </button>
        </div>
      </div>

      <!-- Installation Step-by-Step Guide (Shown prominently if not connected, or compact if connected) -->
      {#if !extInfo}
        <div class="border-t border-amber-500/20 pt-5 space-y-4">
          <div class="flex items-center space-x-2 text-xs font-bold text-amber-300 uppercase tracking-wider">
            <Info class="w-4 h-4" />
            <span>Quick 4-Step Installation Guide</span>
          </div>

          <div class="grid grid-cols-1 md:grid-cols-4 gap-3 text-xs">
            <!-- Step 1 -->
            <div class="p-3.5 rounded-xl border border-zinc-800 bg-zinc-900/80 space-y-1.5">
              <div class="flex items-center space-x-2 font-bold text-white">
                <span class="w-5 h-5 rounded-full bg-indigo-600/30 text-indigo-300 flex items-center justify-center text-[10px]">1</span>
                <span>Download & Unzip</span>
              </div>
              <p class="text-zinc-400 text-[11px] leading-relaxed">
                Click the download button above and extract the <code class="text-indigo-300">cpbridge-extension.zip</code> file to a folder.
              </p>
            </div>

            <!-- Step 2 -->
            <div class="p-3.5 rounded-xl border border-zinc-800 bg-zinc-900/80 space-y-1.5">
              <div class="flex items-center space-x-2 font-bold text-white">
                <span class="w-5 h-5 rounded-full bg-indigo-600/30 text-indigo-300 flex items-center justify-center text-[10px]">2</span>
                <span>Open Extensions</span>
              </div>
              <p class="text-zinc-400 text-[11px] leading-relaxed">
                Navigate to <code class="text-indigo-300">chrome://extensions</code> in Chrome, Brave, or Edge.
              </p>
              <button
                on:click={copyExtensionsUrl}
                class="mt-1 w-full px-2 py-1 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-300 hover:text-white flex items-center justify-center space-x-1 text-[10px] transition"
              >
                {#if copiedUrl}
                  <Check class="w-3 h-3 text-emerald-400" />
                  <span class="text-emerald-400">Copied!</span>
                {:else}
                  <Copy class="w-3 h-3" />
                  <span>Copy URL</span>
                {/if}
              </button>
            </div>

            <!-- Step 3 -->
            <div class="p-3.5 rounded-xl border border-zinc-800 bg-zinc-900/80 space-y-1.5">
              <div class="flex items-center space-x-2 font-bold text-white">
                <span class="w-5 h-5 rounded-full bg-indigo-600/30 text-indigo-300 flex items-center justify-center text-[10px]">3</span>
                <span>Developer Mode</span>
              </div>
              <p class="text-zinc-400 text-[11px] leading-relaxed">
                Turn on the <strong>Developer mode</strong> toggle switch located in the top-right corner.
              </p>
            </div>

            <!-- Step 4 -->
            <div class="p-3.5 rounded-xl border border-zinc-800 bg-zinc-900/80 space-y-1.5">
              <div class="flex items-center space-x-2 font-bold text-white">
                <span class="w-5 h-5 rounded-full bg-indigo-600/30 text-indigo-300 flex items-center justify-center text-[10px]">4</span>
                <span>Load Unpacked</span>
              </div>
              <p class="text-zinc-400 text-[11px] leading-relaxed">
                Click <strong>Load unpacked</strong> and select the extracted extension folder, then click <strong>Check Status</strong>.
              </p>
            </div>
          </div>
        </div>
      {/if}
    </div>
  {/if}

  {#if identitySyncError}
    <div class="p-4 rounded-xl border border-amber-500/30 bg-amber-500/5 text-xs text-amber-200 flex items-start gap-2">
      <AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
      <span>Browser sessions are active, but cpbridge could not synchronize the platform identity: {identitySyncError}</span>
    </div>
  {/if}

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
            {#if checking && !extInfo}
              <div class="w-20 h-5 rounded-full bg-zinc-800 animate-pulse"></div>
            {:else if extInfo?.platforms?.CODEFORCES?.loggedIn}
              <span class="text-[11px] px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 font-semibold border border-emerald-500/30 flex items-center space-x-1">
                <CheckCircle2 class="w-3 h-3" />
                <span>Connected ({extInfo.platforms.CODEFORCES.username || 'Session Active'})</span>
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
            {#if checking && !extInfo}
              <div class="w-20 h-5 rounded-full bg-zinc-800 animate-pulse"></div>
            {:else if extInfo?.platforms?.ATCODER?.loggedIn}
              <span class="text-[11px] px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 font-semibold border border-emerald-500/30 flex items-center space-x-1">
                <CheckCircle2 class="w-3 h-3" />
                <span>Connected ({extInfo.platforms.ATCODER.username || 'Session Active'})</span>
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
