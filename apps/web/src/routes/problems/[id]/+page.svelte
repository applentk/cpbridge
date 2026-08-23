<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import { pingExtension, submitViaExtension } from '$lib/extension/bridge';
  import { renderMathInHtml } from '$lib/utils/math';
  import type { Problem, LanguageId, Submission, ProblemStatement } from '@cp-hub/contracts';
  import MonacoEditor from '$lib/components/MonacoEditor.svelte';
  import {
    ExternalLink,
    Tag,
    Send,
    AlertCircle,
    CheckCircle2,
    Clock,
    Cpu,
    Copy,
    Check,
    BookOpen,
    Terminal,
    Globe,
    RefreshCw,
    Maximize2
  } from 'lucide-svelte';

  let problemId = $page.params.id;
  let problem: Problem | null = null;
  let statement: ProblemStatement | null = null;
  let renderedHtml = '';
  let statementLoading = true;
  let loading = true;
  let error = '';

  let activeTab: 'statement' | 'iframe' | 'submissions' = 'statement';
  let copiedCaseIndex: string | null = null;

  let language: LanguageId = 'cpp23';
  let sourceCode = `#include <iostream>\nusing namespace std;\n\nint main() {\n    ios_base::sync_with_stdio(false);\n    cin.tie(NULL);\n    \n    // Solve problem\n    \n    return 0;\n}\n`;

  let submitting = false;
  let submitStatus = '';
  let activeSubmission: Submission | null = null;
  let recentSubmissions: Submission[] = [];

  const starterTemplates: Record<LanguageId, string> = {
    cpp23: `#include <iostream>\nusing namespace std;\n\nint main() {\n    ios_base::sync_with_stdio(false);\n    cin.tie(NULL);\n    \n    // Solve problem\n    \n    return 0;\n}\n`,
    python3: `import sys\n\ndef main():\n    input = sys.stdin.read\n    # Solve problem\n\nif __name__ == "__main__":\n    main()\n`,
    java21: `import java.util.*;\n\npublic class Main {\n    public static void main(String[] args) {\n        Scanner scanner = new Scanner(System.in);\n        // Solve problem\n    }\n}\n`,
    go: `package main\n\nimport "fmt"\n\nfunc main() {\n    // Solve problem\n    fmt.Println("Hello")\n}\n`,
    rust: `use std::io::{self, Read};\n\nfn main() {\n    // Solve problem\n}\n`
  };

  function handleLanguageChange() {
    sourceCode = starterTemplates[language] || '';
  }

  async function loadProblem() {
    loading = true;
    try {
      problem = await api.get<Problem>(`/problems/${problemId}`);
      await Promise.all([loadStatement(), loadSubmissions()]);
    } catch (err: any) {
      error = err.message || 'Failed to load problem';
    } finally {
      loading = false;
    }
  }

  async function loadStatement() {
    statementLoading = true;
    try {
      statement = await api.get<ProblemStatement>(`/problems/${problemId}/statement`);
      if (statement && statement.html) {
        renderedHtml = renderMathInHtml(statement.html);
      }
      // If statement extraction returned empty/fallback or failed, switch default tab to iframe
      if (!statement || !statement.html || statement.html.includes('Please refer to the official')) {
        activeTab = 'iframe';
      }
    } catch (err) {
      console.error('Failed to load statement, using iframe fallback:', err);
      activeTab = 'iframe';
    } finally {
      statementLoading = false;
    }
  }

  async function loadSubmissions() {
    try {
      recentSubmissions = await api.get<Submission[]>(`/submissions?problemId=${problemId}`);
    } catch {}
  }

  function copyToClipboard(text: string, id: string) {
    navigator.clipboard.writeText(text);
    copiedCaseIndex = id;
    setTimeout(() => {
      if (copiedCaseIndex === id) copiedCaseIndex = null;
    }, 2000);
  }

  async function handleSubmit() {
    if (!$auth.user) {
      alert('Please log in to submit a solution');
      return;
    }
    if (!problem) return;

    submitting = true;
    submitStatus = 'Creating submission...';

    try {
      const sub = await api.post<Submission>('/submissions', {
        problemId: problem.id,
        contestId: null,
        language,
        sourceCode
      });
      activeSubmission = sub;
      submitStatus = 'Dispatching via extension...';

      const extRes = await submitViaExtension(
        sub.id,
        problem.platform,
        problem.externalId,
        problem.url,
        language,
        sourceCode
      );

      if (extRes.type === 'SUBMISSION_CREATED') {
        submitStatus = 'Submitted! Status: JUDGING';
        await api.post(`/submissions/${sub.id}/dispatched`, {
          externalSubmissionId: extRes.externalSubmissionId
        });
        activeSubmission.status = 'JUDGING';
      } else {
        submitStatus = `Extension notice: ${extRes.error || 'Fallback to manual verification'}`;
      }

      await loadSubmissions();
    } catch (err: any) {
      submitStatus = `Submission recorded (${err.message})`;
    } finally {
      submitting = false;
    }
  }

  async function handleMockVerdict(status: 'ACCEPTED' | 'WRONG_ANSWER' | 'TIME_LIMIT') {
    if (!activeSubmission) return;
    try {
      await api.post(`/submissions/${activeSubmission.id}/result`, {
        status,
        metadata: { manualRecord: true }
      });
      activeSubmission.status = status;
      await loadSubmissions();
    } catch (err) {
      console.error(err);
    }
  }

  onMount(() => {
    loadProblem();
  });
</script>

{#if loading}
  <div class="h-96 rounded-2xl bg-zinc-900/40 border border-zinc-800 animate-pulse"></div>
{:else if error || !problem}
  <div class="p-8 rounded-2xl border border-red-500/30 bg-red-500/10 text-red-300 space-y-2">
    <h2 class="text-xl font-bold">Error loading problem</h2>
    <p class="text-sm">{error || 'Problem not found.'}</p>
  </div>
{:else}
  <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 h-[calc(100vh-140px)] min-h-[650px]">
    <!-- Left Column: Rich Statement Reader, Iframe Embed & Submissions -->
    <div class="lg:col-span-6 flex flex-col space-y-3 h-full overflow-hidden">
      <!-- Problem Header Card -->
      <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/60 shrink-0 space-y-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center space-x-2">
            <span class="text-xs px-2.5 py-0.5 rounded-full font-semibold {
              problem.platform === 'CODEFORCES' ? 'bg-red-500/20 text-red-300 border border-red-500/30' :
              problem.platform === 'LEETCODE' ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30' :
              'bg-blue-500/20 text-blue-300 border border-blue-500/30'
            }">
              {problem.platform}
            </span>
            <span class="text-xs font-mono text-zinc-400">{problem.externalId}</span>
            {#if problem.difficulty}
              <span class="text-xs px-2 py-0.5 rounded-full font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                ★ {problem.difficulty}
              </span>
            {/if}
          </div>

          <a
            href={problem.url}
            target="_blank"
            rel="noopener noreferrer"
            class="text-xs text-indigo-400 hover:text-indigo-300 flex items-center space-x-1 font-semibold"
            title="Open official statement in new tab"
          >
            <span>Open Source</span>
            <ExternalLink class="w-3.5 h-3.5" />
          </a>
        </div>

        <h1 class="text-2xl font-extrabold text-white leading-tight">
          {problem.title}
        </h1>

        <!-- Limits Bar -->
        {#if statement?.timeLimit || statement?.memoryLimit}
          <div class="flex items-center space-x-4 text-xs font-mono text-zinc-400 pt-1 border-t border-zinc-800/80">
            {#if statement.timeLimit}
              <div class="flex items-center space-x-1">
                <Clock class="w-3.5 h-3.5 text-zinc-500" />
                <span>Time: {statement.timeLimit}</span>
              </div>
            {/if}
            {#if statement.memoryLimit}
              <div class="flex items-center space-x-1">
                <Cpu class="w-3.5 h-3.5 text-zinc-500" />
                <span>Memory: {statement.memoryLimit}</span>
              </div>
            {/if}
          </div>
        {/if}

        <!-- View Mode Tabs -->
        <div class="flex items-center space-x-2 pt-2">
          <button
            on:click={() => (activeTab = 'statement')}
            class="px-3 py-1.5 rounded-lg text-xs font-semibold transition flex items-center space-x-1.5 {
              activeTab === 'statement' ? 'bg-indigo-600 text-white shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
            }"
          >
            <BookOpen class="w-3.5 h-3.5" />
            <span>Extracted Reader (LaTeX)</span>
          </button>

          <button
            on:click={() => (activeTab = 'iframe')}
            class="px-3 py-1.5 rounded-lg text-xs font-semibold transition flex items-center space-x-1.5 {
              activeTab === 'iframe' ? 'bg-indigo-600 text-white shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
            }"
          >
            <Globe class="w-3.5 h-3.5" />
            <span>Live Embed (iframe)</span>
          </button>

          <button
            on:click={() => (activeTab = 'submissions')}
            class="px-3 py-1.5 rounded-lg text-xs font-semibold transition flex items-center space-x-1.5 {
              activeTab === 'submissions' ? 'bg-indigo-600 text-white shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
            }"
          >
            <Cpu class="w-3.5 h-3.5" />
            <span>Submissions ({recentSubmissions.length})</span>
          </button>
        </div>
      </div>

      <!-- Tab Content Container -->
      <div class="flex-1 overflow-y-auto rounded-2xl border border-zinc-800 bg-zinc-900/40 {activeTab === 'iframe' ? 'p-2 flex flex-col' : 'p-5 space-y-6'}">
        {#if activeTab === 'statement'}
          <!-- 1. Extracted Reader View -->
          {#if statementLoading}
            <div class="space-y-3 py-6">
              <div class="h-4 bg-zinc-800/60 rounded w-3/4 animate-pulse"></div>
              <div class="h-4 bg-zinc-800/60 rounded w-5/6 animate-pulse"></div>
              <div class="h-4 bg-zinc-800/60 rounded w-2/3 animate-pulse"></div>
            </div>
          {:else if renderedHtml && !renderedHtml.includes('Please refer to the official')}
            <!-- Statement Body HTML with KaTeX formulas -->
            <div class="statement-content text-sm text-zinc-300 leading-relaxed space-y-4">
              {@html renderedHtml}
            </div>

            <!-- Sample Test Cases -->
            {#if statement && statement.sampleCases && statement.sampleCases.length > 0}
              <div class="space-y-4 pt-4 border-t border-zinc-800">
                <h3 class="text-sm font-bold text-white uppercase tracking-wider flex items-center space-x-2">
                  <Terminal class="w-4 h-4 text-indigo-400" />
                  <span>Sample Test Cases</span>
                </h3>

                {#each statement.sampleCases as sc, idx}
                  <div class="p-3.5 rounded-xl border border-zinc-800 bg-zinc-950/80 space-y-3">
                    <div class="text-xs font-bold text-zinc-400 uppercase">Example {idx + 1}</div>

                    <!-- Input -->
                    <div class="space-y-1">
                      <div class="flex items-center justify-between text-xs font-mono text-zinc-400">
                        <span>Input:</span>
                        <button
                          on:click={() => copyToClipboard(sc.input, `in_${idx}`)}
                          class="p-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition flex items-center space-x-1"
                          title="Copy input"
                        >
                          {#if copiedCaseIndex === `in_${idx}`}
                            <Check class="w-3.5 h-3.5 text-emerald-400" />
                            <span class="text-[10px] text-emerald-400">Copied!</span>
                          {:else}
                            <Copy class="w-3.5 h-3.5" />
                            <span class="text-[10px]">Copy</span>
                          {/if}
                        </button>
                      </div>
                      <pre class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800 text-xs font-mono text-zinc-200 overflow-x-auto select-all">{sc.input}</pre>
                    </div>

                    <!-- Output -->
                    {#if sc.output}
                      <div class="space-y-1">
                        <div class="flex items-center justify-between text-xs font-mono text-zinc-400">
                          <span>Output:</span>
                          <button
                            on:click={() => copyToClipboard(sc.output, `out_${idx}`)}
                            class="p-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition flex items-center space-x-1"
                            title="Copy output"
                          >
                            {#if copiedCaseIndex === `out_${idx}`}
                              <Check class="w-3.5 h-3.5 text-emerald-400" />
                              <span class="text-[10px] text-emerald-400">Copied!</span>
                            {:else}
                              <Copy class="w-3.5 h-3.5" />
                              <span class="text-[10px]">Copy</span>
                            {/if}
                          </button>
                        </div>
                        <pre class="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800 text-xs font-mono text-zinc-200 overflow-x-auto select-all">{sc.output}</pre>
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          {:else}
            <!-- Fallback prompt to iframe -->
            <div class="p-8 rounded-2xl border border-zinc-800 bg-zinc-900/60 text-center space-y-4">
              <div class="w-12 h-12 rounded-full bg-indigo-500/10 border border-indigo-500/30 text-indigo-400 flex items-center justify-center mx-auto">
                <Globe class="w-6 h-6" />
              </div>
              <div class="space-y-1">
                <h3 class="font-bold text-white text-base">Direct Reader Unavailable</h3>
                <p class="text-xs text-zinc-400 max-w-sm mx-auto">
                  {problem.platform} blocks direct background scraping. Switch to the live iframe embed or open in a new tab.
                </p>
              </div>
              <div class="flex items-center justify-center space-x-3 pt-2">
                <button
                  on:click={() => (activeTab = 'iframe')}
                  class="px-4 py-2 rounded-xl text-xs font-semibold bg-indigo-600 hover:bg-indigo-500 text-white transition flex items-center space-x-1.5"
                >
                  <Globe class="w-3.5 h-3.5" />
                  <span>View in Live Embed (iframe)</span>
                </button>
                <a
                  href={problem.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="px-4 py-2 rounded-xl text-xs font-semibold border border-zinc-800 hover:bg-zinc-800 text-zinc-300 transition flex items-center space-x-1.5"
                >
                  <span>Open Tab</span>
                  <ExternalLink class="w-3.5 h-3.5" />
                </a>
              </div>
            </div>
          {/if}

        {:else if activeTab === 'iframe'}
          <!-- 2. Iframe Embed View -->
          <div class="flex flex-col h-full space-y-2">
            <!-- Iframe Toolbar -->
            <div class="flex items-center justify-between px-3 py-2 rounded-xl bg-zinc-950/80 border border-zinc-800 text-xs">
              <div class="flex items-center space-x-2 text-zinc-400 truncate">
                <span class="font-mono text-zinc-500 truncate">{problem.url}</span>
              </div>
              <div class="flex items-center space-x-2 shrink-0">
                <a
                  href={problem.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="px-2.5 py-1 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition flex items-center space-x-1 text-[11px] font-semibold"
                  title="Open in new window"
                >
                  <span>Pop out</span>
                  <ExternalLink class="w-3 h-3" />
                </a>
              </div>
            </div>

            <!-- Iframe Frame -->
            <div class="flex-1 relative rounded-xl overflow-hidden border border-zinc-800 bg-white">
              <iframe
                src={problem.url}
                title={problem.title}
                class="w-full h-full"
                sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
                loading="lazy"
              ></iframe>
            </div>

            <p class="text-[11px] text-zinc-500 text-center py-0.5">
              If the platform blocks inline embedding, click
              <a href={problem.url} target="_blank" class="text-indigo-400 underline ml-0.5">Pop out</a>
              to view in a separate tab side-by-side.
            </p>
          </div>

        {:else}
          <!-- 3. Submissions Tab -->
          {#if recentSubmissions.length === 0}
            <p class="text-xs text-zinc-500 py-8 text-center">No submissions yet for this problem.</p>
          {:else}
            <div class="space-y-2">
              {#each recentSubmissions as sub}
                <div class="p-3 rounded-xl border border-zinc-800 bg-zinc-950 flex items-center justify-between text-xs">
                  <div class="space-y-0.5">
                    <div class="font-mono text-zinc-300">{sub.language}</div>
                    <div class="text-zinc-500">{new Date(sub.submittedAt).toLocaleTimeString()}</div>
                  </div>
                  <span class="font-bold font-mono px-2.5 py-1 rounded-md {
                    sub.status === 'ACCEPTED' ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
                    sub.status === 'WRONG_ANSWER' ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' :
                    'bg-zinc-800 text-zinc-400'
                  }">
                    {sub.status}
                  </span>
                </div>
              {/each}
            </div>
          {/if}
        {/if}
      </div>
    </div>

    <!-- Right Column: Monaco Code Editor & Submission Controls -->
    <div class="lg:col-span-6 flex flex-col space-y-3 h-full">
      <div class="flex items-center justify-between bg-zinc-900/60 p-3 rounded-2xl border border-zinc-800">
        <div class="flex items-center space-x-3">
          <label for="lang-select" class="text-xs font-semibold uppercase text-zinc-400">Language:</label>
          <select
            id="lang-select"
            bind:value={language}
            on:change={handleLanguageChange}
            class="px-3 py-1.5 rounded-lg bg-zinc-950 border border-zinc-800 text-zinc-200 text-xs font-mono focus:border-indigo-500 focus:outline-none"
          >
            <option value="cpp23">C++23 (GCC)</option>
            <option value="python3">Python 3</option>
            <option value="java21">Java 21</option>
            <option value="go">Go</option>
            <option value="rust">Rust</option>
          </select>
        </div>

        <div class="flex items-center space-x-2">
          <button
            on:click={handleSubmit}
            disabled={submitting}
            class="px-5 py-2 rounded-xl font-bold bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white shadow-lg shadow-indigo-600/30 transition flex items-center space-x-2 text-sm"
          >
            <Send class="w-4 h-4" />
            <span>{submitting ? 'Submitting...' : 'Submit Code'}</span>
          </button>
        </div>
      </div>

      <!-- Editor Container -->
      <div class="flex-1 min-h-[350px]">
        <MonacoEditor bind:value={sourceCode} {language} />
      </div>

      <!-- Verdict / Submission Banner -->
      {#if activeSubmission || submitStatus}
        <div class="p-4 rounded-2xl border border-zinc-800 bg-zinc-900/80 space-y-2">
          <div class="flex items-center justify-between">
            <span class="text-xs font-semibold uppercase tracking-wider text-zinc-400">Verdict</span>
            {#if activeSubmission}
              <span class="text-xs font-bold font-mono px-2.5 py-1 rounded-lg {
                activeSubmission.status === 'ACCEPTED' ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
                activeSubmission.status === 'WRONG_ANSWER' ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' :
                'bg-amber-500/20 text-amber-300 border border-amber-500/30'
              }">
                {activeSubmission.status}
              </span>
            {/if}
          </div>

          <p class="text-xs text-zinc-400">{submitStatus}</p>

          {#if activeSubmission && (activeSubmission.status === 'PENDING' || activeSubmission.status === 'JUDGING')}
            <div class="flex items-center space-x-2 pt-2 border-t border-zinc-800">
              <span class="text-xs text-zinc-500">Record dev mock verdict:</span>
              <button
                on:click={() => handleMockVerdict('ACCEPTED')}
                class="px-2.5 py-1 rounded-lg text-xs font-semibold bg-emerald-600/20 hover:bg-emerald-600/30 text-emerald-300 border border-emerald-500/30 transition"
              >
                Mark AC
              </button>
              <button
                on:click={() => handleMockVerdict('WRONG_ANSWER')}
                class="px-2.5 py-1 rounded-lg text-xs font-semibold bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 border border-rose-500/30 transition"
              >
                Mark WA
              </button>
            </div>
          {/if}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  :global(.statement-content p) {
    margin-bottom: 0.75rem;
  }
  :global(.statement-content ul) {
    list-style-type: disc;
    margin-left: 1.25rem;
    margin-bottom: 0.75rem;
  }
  :global(.statement-content ol) {
    list-style-type: decimal;
    margin-left: 1.25rem;
    margin-bottom: 0.75rem;
  }
  :global(.statement-content code) {
    background-color: #27272a;
    padding: 0.15rem 0.35rem;
    border-radius: 0.25rem;
    font-family: monospace;
    font-size: 0.85em;
  }
  :global(.statement-content pre) {
    background-color: #18181b;
    border: 1px solid #27272a;
    padding: 0.75rem;
    border-radius: 0.5rem;
    overflow-x: auto;
    margin-bottom: 0.75rem;
  }
  :global(.statement-content .section-title) {
    font-weight: 700;
    font-size: 1rem;
    color: #f4f4f5;
    margin-top: 1rem;
    margin-bottom: 0.5rem;
  }
  :global(.katex) {
    font-size: 1.05em;
    color: #e4e4e7;
  }
  :global(.katex-display) {
    margin: 1em 0;
    overflow-x: auto;
    overflow-y: hidden;
    padding: 0.5rem 0;
  }
</style>
