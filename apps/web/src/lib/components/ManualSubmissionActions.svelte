<script lang="ts">
  import type { ExtensionSubmissionActionRequiredResponse } from '@cpbridge/contracts';
  import { CheckCircle2, Clipboard, ExternalLink, LoaderCircle } from 'lucide-svelte';

  export let action: ExtensionSubmissionActionRequiredResponse;
  export let checking = false;
  export let copied = false;
  export let onCheck: () => void;
  export let onOpen: () => void;
  export let onCopy: () => void;
</script>

<div class="rounded-xl border border-amber-500/30 bg-amber-500/10 p-3.5 space-y-3" data-testid="manual-submission-actions">
  <div class="space-y-1">
    <p class="text-sm font-semibold text-amber-200">Finish this submission on Codeforces</p>
    <p class="text-xs leading-relaxed text-amber-100/75">
      {action.message} If Codeforces shows a verification challenge, complete it yourself.
    </p>
  </div>

  <div class="flex flex-wrap gap-2">
    <button
      type="button"
      on:click={onOpen}
      class="inline-flex items-center gap-1.5 rounded-lg border border-amber-400/30 bg-amber-300 px-3 py-1.5 text-xs font-bold text-amber-950 transition hover:bg-amber-200"
    >
      <ExternalLink class="h-3.5 w-3.5" />
      Open Codeforces
    </button>
    <button
      type="button"
      on:click={onCopy}
      class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-700 bg-zinc-900 px-3 py-1.5 text-xs font-semibold text-zinc-200 transition hover:bg-zinc-800"
    >
      {#if copied}
        <CheckCircle2 class="h-3.5 w-3.5 text-emerald-400" />
        Copied
      {:else}
        <Clipboard class="h-3.5 w-3.5" />
        Copy code
      {/if}
    </button>
    <button
      type="button"
      on:click={onCheck}
      disabled={checking}
      class="inline-flex items-center gap-1.5 rounded-lg border border-zinc-600 bg-white px-3 py-1.5 text-xs font-bold text-black transition hover:bg-zinc-200 disabled:cursor-wait disabled:opacity-60"
    >
      {#if checking}
        <LoaderCircle class="h-3.5 w-3.5 animate-spin" />
        Checking…
      {:else}
        <CheckCircle2 class="h-3.5 w-3.5" />
        I submitted — check now
      {/if}
    </button>
  </div>
</div>
