<script lang="ts">
  import { type Submission, formatLanguageName } from '@cpbridge/contracts';
  import MonacoEditor from '$lib/components/MonacoEditor.svelte';
  import { auth } from '$lib/stores/auth';
  import { X, Copy, Check, Code2, AlertCircle, ExternalLink } from 'lucide-svelte';

  export let submission: Submission | null = null;
  export let open: boolean = false;
  export let onClose: () => void = () => {};

  let copied = false;
  let copyTimeout: any = null;

  function judgingOutput(error: unknown): string {
    if (typeof error !== 'string') return '';
    const message = error.replace(/(?:&nbsp;?|&#160;?|&#x0*a0;?)(?=\s|$|<)/gi, ' ').replace(/\s+/g, ' ').trim();
    if (/^Codeforces:\s*$/i.test(message)) {
      return 'Codeforces submission failed before a submission ID was returned. Check the Codeforces submissions page.';
    }
    return message;
  }

  async function handleCopy() {
    if (!submission?.sourceCode) return;
    try {
      await navigator.clipboard.writeText(submission.sourceCode);
      copied = true;
      if (copyTimeout) clearTimeout(copyTimeout);
      copyTimeout = setTimeout(() => {
        copied = false;
      }, 2000);
    } catch (err) {
      console.error('Failed to copy code', err);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (open && e.key === 'Escape') {
      onClose();
    }
  }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if open && submission}
  <div
    class="fixed inset-0 bg-black/80 backdrop-blur-sm z-50 flex items-center justify-center p-3 sm:p-6"
    on:click|self={onClose}
    on:keydown={(e) => e.key === 'Escape' && onClose()}
    role="dialog"
    aria-modal="true"
    aria-labelledby="submission-modal-title"
    tabindex="-1"
  >
    <div class="w-full max-w-4xl max-h-[90vh] bg-zinc-950 border border-zinc-800 rounded-2xl flex flex-col shadow-2xl overflow-hidden">
      <!-- Modal Header -->
      <div class="p-4 sm:p-5 border-b border-zinc-800/80 bg-zinc-900/60 flex items-center justify-between gap-4">
        <div class="space-y-1 min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h3 id="submission-modal-title" class="font-bold text-white text-base flex items-center space-x-2">
              <Code2 class="w-4 h-4 text-zinc-400 shrink-0" />
              <span class="truncate">{submission.problemTitle || submission.problemId}</span>
            </h3>
            <span class="px-2 py-0.5 rounded text-[11px] font-bold font-mono {
              submission.platform === 'CODEFORCES' ? 'bg-red-500/10 text-red-400 border border-red-500/20' : 'bg-zinc-800 text-zinc-300 border border-zinc-700'
            }">
              {submission.platform}
            </span>
            <span class="font-bold font-mono px-2 py-0.5 rounded text-[11px] {
              submission.status === 'ACCEPTED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
              submission.status === 'WRONG_ANSWER' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30' :
              submission.status === 'JUDGING' || submission.status === 'PENDING' || submission.status === 'DISPATCHING' ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30 animate-pulse' :
              'bg-zinc-800 text-zinc-400 border border-zinc-700'
            }">
              {submission.status}
            </span>
          </div>

          <div class="flex flex-wrap items-center gap-3 text-xs text-zinc-400 font-mono">
            <span>Language: <strong class="text-zinc-200">{formatLanguageName(submission.language)}</strong></span>
            <span>•</span>
            <span>Submitted: <strong class="text-zinc-300 font-sans">{new Date(submission.submittedAt).toLocaleString()}</strong></span>
            {#if submission.username}
              <span>•</span>
              <span>User: <strong class="text-zinc-200 font-sans">{submission.username}</strong></span>
            {/if}
            {#if submission.externalSubmissionId}
              <span>•</span>
              <span>ID: <strong class="text-zinc-400">{submission.externalSubmissionId}</strong></span>
            {/if}
            {#if $auth.user?.role === 'ADMIN' && submission.sourceUrl}
              <span>•</span>
              <a
                href={submission.sourceUrl}
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-1 font-sans font-semibold text-sky-400 hover:text-sky-300 transition"
              >
                <span>External source</span>
                <ExternalLink class="w-3 h-3" />
              </a>
            {/if}
          </div>
        </div>

        <div class="flex items-center space-x-2 shrink-0">
          <button
            on:click={handleCopy}
            class="px-3 py-1.5 rounded-xl text-xs font-semibold border transition flex items-center space-x-1.5 {
              copied
                ? 'bg-emerald-500/20 border-emerald-500/40 text-emerald-300'
                : 'bg-zinc-900 border-zinc-800 hover:bg-zinc-800 text-zinc-200 hover:text-white'
            }"
            title="Copy source code to clipboard"
          >
            {#if copied}
              <Check class="w-3.5 h-3.5" />
              <span>Copied!</span>
            {:else}
              <Copy class="w-3.5 h-3.5" />
              <span>Copy Code</span>
            {/if}
          </button>

          <button
            on:click={onClose}
            class="p-1.5 rounded-xl border border-zinc-800 bg-zinc-900 text-zinc-400 hover:text-white hover:bg-zinc-800 transition"
            title="Close dialog (Esc)"
          >
            <X class="w-4 h-4" />
          </button>
        </div>
      </div>

      <!-- Error banner if any -->
      {#if submission.metadata && judgingOutput(submission.metadata.error)}
        <div class="p-3.5 bg-rose-500/10 border-b border-rose-500/20 text-rose-300 text-xs flex items-start space-x-2">
          <AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
          <div class="space-y-0.5">
            <div class="font-semibold">Execution / Judging Output:</div>
            <div class="font-mono whitespace-pre-wrap">{judgingOutput(submission.metadata.error)}</div>
          </div>
        </div>
      {/if}

      <!-- Code Viewer Body -->
      <div class="flex-1 p-4 bg-zinc-950 overflow-hidden min-h-[350px] h-[55vh]">
        <MonacoEditor
          value={submission.sourceCode}
          language={submission.language}
          readonly={true}
        />
      </div>

      <!-- Footer -->
      <div class="px-5 py-3 border-t border-zinc-800/80 bg-zinc-900/40 flex items-center justify-between text-xs text-zinc-500">
        <div>Press <kbd class="px-1.5 py-0.5 bg-zinc-800 border border-zinc-700 rounded text-zinc-300 font-mono text-[10px]">Esc</kbd> or click outside to close</div>
        <button
          on:click={onClose}
          class="px-4 py-1.5 rounded-xl text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition"
        >
          Close
        </button>
      </div>
    </div>
  </div>
{/if}
