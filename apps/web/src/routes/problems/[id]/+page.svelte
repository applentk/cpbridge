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
    Send,
    AlertCircle,
    CheckCircle2,
    Clock,
    Cpu,
    Copy,
    Check,
    BookOpen,
    Code2,
    Terminal,
    Upload,
    FileCode,
    Columns
  } from 'lucide-svelte';

  let problemId = $page.params.id;
  let problem: Problem | null = null;
  let statement: ProblemStatement | null = null;
  let renderedHtml = '';
  let statementLoading = true;
  let loading = true;
  let error = '';

  let viewMode: 'tabbed' | 'split' = 'tabbed';
  let activeTab: 'statement' | 'editor' | 'submissions' = 'statement';
  let copiedCaseIndex: string | null = null;

  let language: LanguageId = 'cpp23';
  let sourceCode = `#include <iostream>\nusing namespace std;\n\nint main() {\n    ios_base::sync_with_stdio(false);\n    cin.tie(NULL);\n    \n    // Solve problem\n    \n    return 0;\n}\n`;

  let submitting = false;
  let submitStatus = '';
  let activeSubmission: Submission | null = null;
  let recentSubmissions: Submission[] = [];

  let uploadSuccessMessage = '';
  let fileInputElement: HTMLInputElement;

  const starterTemplates: Record<LanguageId, string> = {
    cpp23: `#include <iostream>\nusing namespace std;\n\nint main() {\n    ios_base::sync_with_stdio(false);\n    cin.tie(NULL);\n    \n    // Solve problem\n    \n    return 0;\n}\n`,
    python3: `import sys\n\ndef main():\n    input = sys.stdin.read\n    # Solve problem\n\nif __name__ == "__main__":\n    main()\n`,
    java21: `import java.util.*;\n\npublic class Main {\n    public static void main(String[] args) {\n        Scanner scanner = new Scanner(System.in);\n        // Solve problem\n    }\n}\n`,
    go: `package main\n\nimport "fmt"\n\nfunc main() {\n    // Solve problem\n    fmt.Println("Hello")\n}\n`,
    rust: `use std::io::{self, Read};\n\nfn main() {\n    // Solve problem\n}\n`
  };

  const languageLabels: Record<LanguageId, string> = {
    cpp23: 'C++23 (GCC)',
    python3: 'Python 3',
    java21: 'Java 21',
    go: 'Go',
    rust: 'Rust'
  };

  function detectLanguageFromFilename(filename: string): LanguageId | null {
    const ext = filename.split('.').pop()?.toLowerCase();
    switch (ext) {
      case 'cpp':
      case 'cc':
      case 'cxx':
      case 'c++':
      case 'cp':
      case 'hpp':
      case 'h':
        return 'cpp23';
      case 'py':
      case 'py3':
      case 'python':
        return 'python3';
      case 'java':
        return 'java21';
      case 'go':
        return 'go';
      case 'rs':
      case 'rust':
        return 'rust';
      default:
        return null;
    }
  }

  function handleFileUpload(event: Event) {
    const target = event.target as HTMLInputElement;
    const file = target.files?.[0];
    if (!file) return;

    const detected = detectLanguageFromFilename(file.name);
    const reader = new FileReader();

    reader.onload = (e) => {
      const content = e.target?.result as string;
      if (content !== undefined) {
        sourceCode = content;
        if (detected) {
          language = detected;
          uploadSuccessMessage = `Loaded "${file.name}" (Auto-detected: ${languageLabels[detected]})`;
        } else {
          uploadSuccessMessage = `Loaded "${file.name}"`;
        }
        setTimeout(() => {
          uploadSuccessMessage = '';
        }, 4000);
      }
    };

    reader.readAsText(file);
    // Reset file input so user can re-upload the same file if desired
    target.value = '';
  }

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
    } catch (err) {
      console.error('Failed to load statement:', err);
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
  <div class="space-y-4">
    <!-- Header Navigation Card -->
    <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/70 space-y-4 shadow-xl">
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
        <div class="space-y-1.5">
          <div class="flex items-center space-x-2.5">
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

          <h1 class="text-2xl font-extrabold text-white leading-tight">
            {problem.title}
          </h1>
        </div>

        <div class="flex items-center space-x-3 shrink-0">
          <!-- View Layout Toggle (Tabbed vs Split) -->
          <div class="flex items-center bg-zinc-950 p-1 rounded-xl border border-zinc-800 text-xs">
            <button
              on:click={() => (viewMode = 'tabbed')}
              class="px-3 py-1 rounded-lg font-semibold transition {
                viewMode === 'tabbed' ? 'bg-zinc-800 text-white shadow-sm' : 'text-zinc-400 hover:text-white'
              }"
            >
              Tabbed View
            </button>
            <button
              on:click={() => (viewMode = 'split')}
              class="px-3 py-1 rounded-lg font-semibold transition flex items-center space-x-1 {
                viewMode === 'split' ? 'bg-zinc-800 text-white shadow-sm' : 'text-zinc-400 hover:text-white'
              }"
            >
              <Columns class="w-3.5 h-3.5" />
              <span>Split View</span>
            </button>
          </div>

          <a
            href={problem.url}
            target="_blank"
            rel="noopener noreferrer"
            class="px-3 py-1.5 rounded-xl border border-zinc-800 hover:bg-zinc-800 text-xs text-indigo-400 hover:text-indigo-300 font-semibold transition flex items-center space-x-1.5"
            title="Open official statement on source website"
          >
            <span>Source</span>
            <ExternalLink class="w-3.5 h-3.5" />
          </a>
        </div>
      </div>

      <!-- Limits & Metadata Bar -->
      {#if statement?.timeLimit || statement?.memoryLimit}
        <div class="flex items-center space-x-5 text-xs font-mono text-zinc-400 pt-2 border-t border-zinc-800/80">
          {#if statement.timeLimit}
            <div class="flex items-center space-x-1.5">
              <Clock class="w-3.5 h-3.5 text-zinc-500" />
              <span>Time Limit: {statement.timeLimit}</span>
            </div>
          {/if}
          {#if statement.memoryLimit}
            <div class="flex items-center space-x-1.5">
              <Cpu class="w-3.5 h-3.5 text-zinc-500" />
              <span>Memory Limit: {statement.memoryLimit}</span>
            </div>
          {/if}
        </div>
      {/if}

      <!-- Main Navigation Tabs (Visible in Tabbed Mode) -->
      {#if viewMode === 'tabbed'}
        <div class="flex items-center space-x-2 pt-2 border-t border-zinc-800/80">
          <button
            on:click={() => (activeTab = 'statement')}
            class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-2 {
              activeTab === 'statement' ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/20' : 'bg-zinc-800/80 text-zinc-400 hover:text-white'
            }"
          >
            <BookOpen class="w-4 h-4" />
            <span>Problem Statement</span>
          </button>

          <button
            on:click={() => (activeTab = 'editor')}
            class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-2 {
              activeTab === 'editor' ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/20' : 'bg-zinc-800/80 text-zinc-400 hover:text-white'
            }"
          >
            <Code2 class="w-4 h-4" />
            <span>Code Editor & Submit</span>
          </button>

          <button
            on:click={() => (activeTab = 'submissions')}
            class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-2 {
              activeTab === 'submissions' ? 'bg-indigo-600 text-white shadow-md shadow-indigo-600/20' : 'bg-zinc-800/80 text-zinc-400 hover:text-white'
            }"
          >
            <Cpu class="w-4 h-4" />
            <span>Submissions ({recentSubmissions.length})</span>
          </button>
        </div>
      {/if}
    </div>

    <!-- Hidden File Input for Auto-Detect Upload -->
    <input
      type="file"
      bind:this={fileInputElement}
      on:change={handleFileUpload}
      accept=".cpp,.cc,.cxx,.c++,.cp,.py,.py3,.python,.java,.go,.rs,.rust,.txt"
      class="hidden"
    />

    <!-- Layout Container -->
    {#if viewMode === 'split'}
      <!-- Split View Layout -->
      <div class="grid grid-cols-1 lg:grid-cols-12 gap-6 h-[calc(100vh-220px)] min-h-[650px]">
        <!-- Left: Statement -->
        <div class="lg:col-span-6 overflow-y-auto rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6 space-y-6">
          {#if statementLoading}
            <div class="space-y-3 py-6">
              <div class="h-4 bg-zinc-800/60 rounded w-3/4 animate-pulse"></div>
              <div class="h-4 bg-zinc-800/60 rounded w-5/6 animate-pulse"></div>
              <div class="h-4 bg-zinc-800/60 rounded w-2/3 animate-pulse"></div>
            </div>
          {:else if renderedHtml}
            <div class="statement-content text-sm text-zinc-300 leading-relaxed space-y-4">
              {@html renderedHtml}
            </div>

            <!-- Sample Cases -->
            {#if statement && statement.sampleCases && statement.sampleCases.length > 0}
              <div class="space-y-4 pt-4 border-t border-zinc-800">
                <h3 class="text-sm font-bold text-white uppercase tracking-wider flex items-center space-x-2">
                  <Terminal class="w-4 h-4 text-indigo-400" />
                  <span>Sample Test Cases</span>
                </h3>

                {#each statement.sampleCases as sc, idx}
                  <div class="p-3.5 rounded-xl border border-zinc-800 bg-zinc-950/80 space-y-3">
                    <div class="text-xs font-bold text-zinc-400 uppercase">Example {idx + 1}</div>
                    <div class="space-y-1">
                      <div class="flex items-center justify-between text-xs font-mono text-zinc-400">
                        <span>Input:</span>
                        <button
                          on:click={() => copyToClipboard(sc.input, `in_split_${idx}`)}
                          class="p-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition flex items-center space-x-1"
                        >
                          {#if copiedCaseIndex === `in_split_${idx}`}
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

                    {#if sc.output}
                      <div class="space-y-1">
                        <div class="flex items-center justify-between text-xs font-mono text-zinc-400">
                          <span>Output:</span>
                          <button
                            on:click={() => copyToClipboard(sc.output, `out_split_${idx}`)}
                            class="p-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition flex items-center space-x-1"
                          >
                            {#if copiedCaseIndex === `out_split_${idx}`}
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
            <div class="p-8 text-center text-zinc-400 text-sm">
              Statement not loaded.
              <a href={problem.url} target="_blank" class="text-indigo-400 underline ml-1">Open source statement</a>
            </div>
          {/if}
        </div>

        <!-- Right: Code Editor & Upload Toolbar -->
        <div class="lg:col-span-6 flex flex-col space-y-3 h-full">
          <div class="flex flex-wrap items-center justify-between gap-2 bg-zinc-900/60 p-3 rounded-2xl border border-zinc-800">
            <div class="flex items-center space-x-2">
              <select
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

              <!-- Upload File Button with Auto-Detect -->
              <button
                type="button"
                on:click={() => fileInputElement.click()}
                class="px-3 py-1.5 rounded-lg text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-200 border border-zinc-700 transition flex items-center space-x-1.5"
                title="Upload code file (.cpp, .py, .java, .go, .rs) with auto-detected language"
              >
                <Upload class="w-3.5 h-3.5" />
                <span>Upload File</span>
              </button>
            </div>

            <button
              on:click={handleSubmit}
              disabled={submitting}
              class="px-5 py-1.5 rounded-xl font-bold bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white shadow-lg shadow-indigo-600/30 transition flex items-center space-x-2 text-xs"
            >
              <Send class="w-3.5 h-3.5" />
              <span>{submitting ? 'Submitting...' : 'Submit Code'}</span>
            </button>
          </div>

          {#if uploadSuccessMessage}
            <div class="px-3 py-2 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex items-center space-x-2">
              <CheckCircle2 class="w-3.5 h-3.5 shrink-0" />
              <span>{uploadSuccessMessage}</span>
            </div>
          {/if}

          <!-- Editor -->
          <div class="flex-1 min-h-[350px]">
            <MonacoEditor bind:value={sourceCode} {language} />
          </div>

          <!-- Verdict Banner -->
          {#if activeSubmission || submitStatus}
            <div class="p-3.5 rounded-2xl border border-zinc-800 bg-zinc-900/80 space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-xs font-semibold uppercase tracking-wider text-zinc-400">Verdict</span>
                {#if activeSubmission}
                  <span class="text-xs font-bold font-mono px-2 py-0.5 rounded-lg {
                    activeSubmission.status === 'ACCEPTED' ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
                    activeSubmission.status === 'WRONG_ANSWER' ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' :
                    'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                  }">
                    {activeSubmission.status}
                  </span>
                {/if}
              </div>
              <p class="text-xs text-zinc-400">{submitStatus}</p>
            </div>
          {/if}
        </div>
      </div>

    {:else}
      <!-- Tabbed View Layout -->
      <div class="rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6 min-h-[550px]">
        {#if activeTab === 'statement'}
          <!-- 1. Full-Width Statement Tab -->
          <div class="max-w-4xl mx-auto space-y-6">
            {#if statementLoading}
              <div class="space-y-3 py-6">
                <div class="h-4 bg-zinc-800/60 rounded w-3/4 animate-pulse"></div>
                <div class="h-4 bg-zinc-800/60 rounded w-5/6 animate-pulse"></div>
                <div class="h-4 bg-zinc-800/60 rounded w-2/3 animate-pulse"></div>
              </div>
            {:else if renderedHtml}
              <div class="statement-content text-sm text-zinc-300 leading-relaxed space-y-4">
                {@html renderedHtml}
              </div>

              <!-- Sample Test Cases -->
              {#if statement && statement.sampleCases && statement.sampleCases.length > 0}
                <div class="space-y-4 pt-6 border-t border-zinc-800">
                  <h3 class="text-sm font-bold text-white uppercase tracking-wider flex items-center space-x-2">
                    <Terminal class="w-4 h-4 text-indigo-400" />
                    <span>Sample Test Cases</span>
                  </h3>

                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {#each statement.sampleCases as sc, idx}
                      <div class="p-4 rounded-xl border border-zinc-800 bg-zinc-950/80 space-y-3">
                        <div class="text-xs font-bold text-zinc-400 uppercase">Example {idx + 1}</div>

                        <!-- Input -->
                        <div class="space-y-1">
                          <div class="flex items-center justify-between text-xs font-mono text-zinc-400">
                            <span>Input:</span>
                            <button
                              on:click={() => copyToClipboard(sc.input, `in_tab_${idx}`)}
                              class="p-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition flex items-center space-x-1"
                            >
                              {#if copiedCaseIndex === `in_tab_${idx}`}
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
                                on:click={() => copyToClipboard(sc.output, `out_tab_${idx}`)}
                                class="p-1 rounded hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition flex items-center space-x-1"
                              >
                                {#if copiedCaseIndex === `out_tab_${idx}`}
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
                </div>
              {/if}

              <!-- Quick Jump to Code Editor -->
              <div class="pt-6 border-t border-zinc-800 flex justify-end">
                <button
                  on:click={() => (activeTab = 'editor')}
                  class="px-5 py-2.5 rounded-xl font-bold bg-indigo-600 hover:bg-indigo-500 text-white shadow-lg shadow-indigo-600/30 transition flex items-center space-x-2 text-xs"
                >
                  <Code2 class="w-4 h-4" />
                  <span>Open Code Editor & Solve</span>
                </button>
              </div>
            {:else}
              <div class="p-12 text-center text-zinc-400 space-y-4">
                <p>Statement could not be loaded automatically.</p>
                <a
                  href={problem.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  class="px-4 py-2 rounded-xl text-xs font-semibold bg-indigo-600 hover:bg-indigo-500 text-white inline-flex items-center space-x-1.5"
                >
                  <span>Open Source Statement</span>
                  <ExternalLink class="w-3.5 h-3.5" />
                </a>
              </div>
            {/if}
          </div>

        {:else if activeTab === 'editor'}
          <!-- 2. Full-Width Code Editor Tab -->
          <div class="space-y-4">
            <!-- Toolbar -->
            <div class="flex flex-wrap items-center justify-between gap-3 bg-zinc-950 p-4 rounded-2xl border border-zinc-800">
              <div class="flex items-center space-x-3">
                <div class="flex items-center space-x-2">
                  <label for="lang-select-tab" class="text-xs font-semibold uppercase text-zinc-400">Language:</label>
                  <select
                    id="lang-select-tab"
                    bind:value={language}
                    on:change={handleLanguageChange}
                    class="px-3 py-1.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-200 text-xs font-mono focus:border-indigo-500 focus:outline-none"
                  >
                    <option value="cpp23">C++23 (GCC)</option>
                    <option value="python3">Python 3</option>
                    <option value="java21">Java 21</option>
                    <option value="go">Go</option>
                    <option value="rust">Rust</option>
                  </select>
                </div>

                <!-- Upload File with Auto Detect -->
                <button
                  type="button"
                  on:click={() => fileInputElement.click()}
                  class="px-3.5 py-1.5 rounded-lg text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-200 border border-zinc-700 transition flex items-center space-x-1.5 shadow-sm"
                  title="Upload code file (.cpp, .py, .java, .go, .rs) to automatically set source code and detect language"
                >
                  <Upload class="w-3.5 h-3.5 text-indigo-400" />
                  <span>Upload File (Auto-Detect)</span>
                </button>
              </div>

              <div class="flex items-center space-x-3">
                <button
                  on:click={handleSubmit}
                  disabled={submitting}
                  class="px-6 py-2 rounded-xl font-bold bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white shadow-lg shadow-indigo-600/30 transition flex items-center space-x-2 text-xs"
                >
                  <Send class="w-4 h-4" />
                  <span>{submitting ? 'Submitting Solution...' : 'Submit Solution'}</span>
                </button>
              </div>
            </div>

            <!-- Upload Feedback Notification -->
            {#if uploadSuccessMessage}
              <div class="px-4 py-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs flex items-center space-x-2">
                <CheckCircle2 class="w-4 h-4 shrink-0 text-emerald-400" />
                <span class="font-medium">{uploadSuccessMessage}</span>
              </div>
            {/if}

            <!-- Monaco Editor -->
            <div class="h-[550px] rounded-2xl overflow-hidden border border-zinc-800">
              <MonacoEditor bind:value={sourceCode} {language} />
            </div>

            <!-- Verdict & Dispatch Banner -->
            {#if activeSubmission || submitStatus}
              <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-950 space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-semibold uppercase tracking-wider text-zinc-400">Submission Verdict</span>
                  {#if activeSubmission}
                    <span class="text-xs font-bold font-mono px-3 py-1 rounded-lg {
                      activeSubmission.status === 'ACCEPTED' ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30' :
                      activeSubmission.status === 'WRONG_ANSWER' ? 'bg-rose-500/20 text-rose-300 border border-rose-500/30' :
                      'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                    }">
                      {activeSubmission.status}
                    </span>
                  {/if}
                </div>

                <p class="text-xs text-zinc-400 font-mono">{submitStatus}</p>

                {#if activeSubmission && (activeSubmission.status === 'PENDING' || activeSubmission.status === 'JUDGING')}
                  <div class="flex items-center space-x-2 pt-2 border-t border-zinc-800/80">
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

        {:else}
          <!-- 3. Submissions History Tab -->
          <div class="space-y-4 max-w-4xl mx-auto">
            <h3 class="text-sm font-bold text-white uppercase tracking-wider">Your Submission History</h3>
            {#if recentSubmissions.length === 0}
              <p class="text-xs text-zinc-500 py-12 text-center">No submissions recorded yet for this problem.</p>
            {:else}
              <div class="space-y-2.5">
                {#each recentSubmissions as sub}
                  <div class="p-4 rounded-xl border border-zinc-800 bg-zinc-950 flex items-center justify-between text-xs">
                    <div class="space-y-1">
                      <div class="font-mono font-semibold text-zinc-200">{sub.language}</div>
                      <div class="text-zinc-500">{new Date(sub.submittedAt).toLocaleString()}</div>
                    </div>
                    <span class="font-bold font-mono px-3 py-1.5 rounded-lg {
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
          </div>
        {/if}
      </div>
    {/if}
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
