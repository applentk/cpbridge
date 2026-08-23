<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { browser } from '$app/environment';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import { pingExtension, submitViaExtension, pollStatusViaExtension } from '$lib/extension/bridge';
  import { renderMathInHtml } from '$lib/utils/math';
  import type { Problem, LanguageId, Submission, ProblemStatement, Contest, ContestProblem } from '@cp-hub/contracts';
  import MonacoEditor from '$lib/components/MonacoEditor.svelte';
  import ContestTimer from '$lib/components/ContestTimer.svelte';
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
    Columns,
    ArrowLeft,
    Trophy,
    ChevronLeft,
    ChevronRight,
    Layers,
    Lock
  } from 'lucide-svelte';

  let problemId: string = $page.params.id || '';
  let contestId: string | null = $page.url.searchParams.get('contestId');
  let contest: Contest | null = null;
  let contestProblems: ContestProblem[] = [];
  let contestSolvedProblemIds: Set<string> = new Set();
  let contestLoading = false;

  let currentLoadedProblemId: string = '';
  let currentLoadedContestId: string | null | undefined = undefined;

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
  let pollInterval: any = null;

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

  $: currentContestProblem = contestProblems.find(cp => cp.problemId === problemId);
  $: currentProblemIndex = contestProblems.findIndex(cp => cp.problemId === problemId);
  $: prevContestProblem = currentProblemIndex > 0 ? contestProblems[currentProblemIndex - 1] : null;
  $: nextContestProblem = currentProblemIndex >= 0 && currentProblemIndex < contestProblems.length - 1 ? contestProblems[currentProblemIndex + 1] : null;

  $: {
    if (browser) {
      const nextPId = $page.params.id || '';
      const nextCId = $page.url.searchParams.get('contestId');
      if (nextPId && (nextPId !== currentLoadedProblemId || nextCId !== currentLoadedContestId)) {
        currentLoadedProblemId = nextPId;
        currentLoadedContestId = nextCId;
        problemId = nextPId;
        contestId = nextCId;
        loadProblemAndContest();
      }
    }
  }

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
      case 'java21':
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

  async function loadProblemAndContest() {
    loading = true;
    error = '';
    stopSubmissionPolling();
    activeSubmission = null;
    submitStatus = '';

    try {
      const pId = problemId;
      const cId = contestId;
      if (!pId) return;

      const promises: Promise<any>[] = [
        api.get<Problem>(`/problems/${pId}`),
        loadStatement(pId),
        loadSubmissions(pId, cId)
      ];

      if (cId) {
        promises.push(loadContestData(cId));
      } else {
        contest = null;
        contestProblems = [];
        contestSolvedProblemIds = new Set();
      }

      const [probData] = await Promise.all(promises);
      problem = probData;

      // If there's an ongoing judging submission, start polling
      if (recentSubmissions.length > 0) {
        const latest = recentSubmissions[0];
        if (latest.status === 'JUDGING' || latest.status === 'PENDING' || latest.status === 'DISPATCHING') {
          activeSubmission = latest;
          submitStatus = `Judging in progress... Status: ${latest.status}`;
          startSubmissionPolling(latest.id);
        }
      }
    } catch (err: any) {
      error = err.message || 'Failed to load problem';
    } finally {
      loading = false;
    }
  }

  async function loadContestData(cId: string) {
    contestLoading = true;
    try {
      const [cRes, subsRes] = await Promise.all([
        api.get<Contest>(`/contests/${cId}`),
        api.get<Submission[]>(`/submissions?contestId=${cId}`).catch(() => [])
      ]);
      contest = cRes;
      contestProblems = cRes.problems || [];

      const solved = new Set<string>();
      if (Array.isArray(subsRes)) {
        for (const sub of subsRes) {
          if (sub.status === 'ACCEPTED') {
            solved.add(sub.problemId);
          }
        }
      }
      contestSolvedProblemIds = solved;
    } catch (err) {
      console.error('Failed to load contest context:', err);
    } finally {
      contestLoading = false;
    }
  }

  async function loadStatement(pId: string) {
    statementLoading = true;
    try {
      statement = await api.get<ProblemStatement>(`/problems/${pId}/statement`);
      if (statement && statement.html) {
        renderedHtml = renderMathInHtml(statement.html);
      } else {
        renderedHtml = '';
      }
    } catch (err) {
      console.error('Failed to load statement:', err);
      statement = null;
      renderedHtml = '';
    } finally {
      statementLoading = false;
    }
  }

  async function loadSubmissions(pId: string, cId: string | null) {
    try {
      const query = cId ? `/submissions?problemId=${pId}&contestId=${cId}` : `/submissions?problemId=${pId}`;
      recentSubmissions = await api.get<Submission[]>(query);
    } catch {
      recentSubmissions = [];
    }
  }

  function copyToClipboard(text: string, id: string) {
    navigator.clipboard.writeText(text);
    copiedCaseIndex = id;
    setTimeout(() => {
      if (copiedCaseIndex === id) copiedCaseIndex = null;
    }, 2000);
  }

  function stopSubmissionPolling() {
    if (pollInterval) {
      clearInterval(pollInterval);
      pollInterval = null;
    }
  }

  function startSubmissionPolling(submissionId: string) {
    stopSubmissionPolling();
    let attempts = 0;
    const maxAttempts = 60; // poll up to ~2.5 minutes

    pollInterval = setInterval(async () => {
      attempts++;
      if (attempts > maxAttempts) {
        stopSubmissionPolling();
        submitStatus = 'Judging timed out. Use the Sync button below or check the official site.';
        try {
          const synced = await api.post<Submission>(`/submissions/${submissionId}/sync`);
          if (synced) {
            activeSubmission = synced;
            submitStatus = `Status: ${synced.status}`;
          }
        } catch {}
        return;
      }

      try {
        // 1. Sync via Backend Platform Adapter
        const updated = await api.post<Submission>(`/submissions/${submissionId}/sync`);
        if (updated && updated.status !== 'JUDGING' && updated.status !== 'PENDING' && updated.status !== 'DISPATCHING') {
          activeSubmission = updated;
          submitStatus = `Verdict: ${updated.status}`;
          stopSubmissionPolling();
          if (updated.status === 'ACCEPTED' && problem) {
            contestSolvedProblemIds = new Set([...contestSolvedProblemIds, problem.id]);
          }
          if (problem) {
            await loadSubmissions(problem.id, contestId);
          }
          return;
        }

        // 2. Also check via Chrome Extension Bridge if available
        if (problem && updated && updated.externalSubmissionId && !updated.externalSubmissionId.startsWith('cf_') && !updated.externalSubmissionId.startsWith('ac_')) {
          const extRes = await pollStatusViaExtension(
            problem.platform,
            updated.externalSubmissionId,
            problem.externalId,
            problem.url
          );
          if (extRes && extRes.status && extRes.status !== 'JUDGING' && extRes.status !== 'PENDING') {
            // Trigger server-side synchronization with platform adapter
            const synced = await api.post<Submission>(`/submissions/${submissionId}/sync`);
            if (synced) {
              activeSubmission = synced;
              submitStatus = `Verdict: ${synced.status}`;
              stopSubmissionPolling();
              if (synced.status === 'ACCEPTED' && problem) {
                contestSolvedProblemIds = new Set([...contestSolvedProblemIds, problem.id]);
              }
              if (problem) {
                await loadSubmissions(problem.id, contestId);
              }
              return;
            }
          }
        }
      } catch (err) {
        console.error('Polling status error:', err);
      }
    }, 2500);
  }

  async function handleSubmit() {
    if (!$auth.user) {
      alert('Please log in to submit a solution');
      return;
    }
    if (!problem) return;

    submitting = true;
    submitStatus = 'Creating submission...';
    stopSubmissionPolling();

    let createdSub: Submission | null = null;
    try {
      const sub = await api.post<Submission>('/submissions', {
        problemId: problem.id,
        contestId: contestId || null,
        language,
        sourceCode
      });
      createdSub = sub;
      activeSubmission = sub;
      submitStatus = 'Dispatching via extension...';

      // Redirect to submissions tab
      activeTab = 'submissions';
      viewMode = 'tabbed';
      await loadSubmissions(problem.id, contestId);

      const extRes = await submitViaExtension(
        sub.id,
        problem.platform,
        problem.externalId,
        problem.url,
        language,
        sourceCode
      );

      if (extRes.type === 'SUBMISSION_CREATED' && extRes.externalSubmissionId) {
        submitStatus = 'Submitted! Status: JUDGING (Polling verdict...)';
        await api.post(`/submissions/${sub.id}/dispatched`, {
          externalSubmissionId: extRes.externalSubmissionId
        });
        activeSubmission.status = 'JUDGING';
        activeSubmission.externalSubmissionId = extRes.externalSubmissionId;
        startSubmissionPolling(sub.id);
      } else if (extRes.type === 'SUBMISSION_FAILED') {
        const errorMsg = extRes.message || extRes.error || 'Submission was not accepted on platform';
        submitStatus = `Submission failed: ${errorMsg}`;
        await api.post(`/submissions/${sub.id}/result`, {
          status: 'FAILED',
          metadata: { error: errorMsg }
        });
        activeSubmission.status = 'FAILED';
      } else {
        const errorMsg = 'Failed to obtain external submission ID';
        submitStatus = `Submission failed: ${errorMsg}`;
        await api.post(`/submissions/${sub.id}/result`, {
          status: 'FAILED',
          metadata: { error: errorMsg }
        });
        activeSubmission.status = 'FAILED';
      }

      await loadSubmissions(problem.id, contestId);
    } catch (err: any) {
      submitStatus = `Submission error: ${err.message}`;
      if (createdSub) {
        try {
          await api.post(`/submissions/${createdSub.id}/result`, {
            status: 'FAILED',
            metadata: { error: err.message }
          });
          if (activeSubmission) activeSubmission.status = 'FAILED';
        } catch {}
      }
    } finally {
      submitting = false;
    }
  }

  async function handleManualSync(subId: string) {
    try {
      submitStatus = 'Syncing status...';
      const updated = await api.post<Submission>(`/submissions/${subId}/sync`);
      if (updated) {
        activeSubmission = updated;
        submitStatus = `Status updated: ${updated.status}`;
        if (updated.status === 'ACCEPTED' && problem) {
          contestSolvedProblemIds = new Set([...contestSolvedProblemIds, problem.id]);
        }
        if (problem) {
          await loadSubmissions(problem.id, contestId);
        }
      }
    } catch (err: any) {
      submitStatus = `Sync failed: ${err.message}`;
    }
  }

  onDestroy(() => {
    stopSubmissionPolling();
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
    <!-- Contest Banner & Problem Switcher (if problem is opened within contest) -->
    {#if contest}
      <div class="p-4 sm:p-5 rounded-2xl border border-zinc-800 bg-zinc-900/90 shadow-xl space-y-4 backdrop-blur-md">
        <!-- Contest Title & Status Bar -->
        <div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
          <div class="space-y-1.5">
            <div class="flex flex-wrap items-center gap-2 text-xs">
              <a
                href={`/contests/${contest.id}`}
                class="font-semibold text-zinc-300 hover:text-white transition flex items-center space-x-1.5 px-2.5 py-1 rounded-lg bg-zinc-800/90 hover:bg-zinc-800 border border-zinc-700/60"
              >
                <ArrowLeft class="w-3.5 h-3.5" />
                <span>Contest Lobby</span>
              </a>

              <a
                href={`/contests/${contest.id}/standings`}
                class="font-semibold text-zinc-300 hover:text-white transition flex items-center space-x-1.5 px-2.5 py-1 rounded-lg bg-zinc-800/90 hover:bg-zinc-800 border border-zinc-700/60"
              >
                <Trophy class="w-3.5 h-3.5" />
                <span>Scoreboard</span>
              </a>

              <span class="px-2.5 py-0.5 rounded-full font-bold {
                contest.state === 'ACTIVE' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                contest.state === 'UPCOMING' ? 'bg-zinc-800 text-zinc-300 border border-zinc-700' :
                'bg-zinc-950 text-zinc-500 border border-zinc-800'
              }">
                {contest.state}
              </span>

              <span class="px-2 py-0.5 rounded-md font-mono bg-zinc-800 text-zinc-300 border border-zinc-700">
                {contest.scoringType} Scoring
              </span>
            </div>

            <div class="flex items-center space-x-2">
              <h2 class="text-xl sm:text-2xl font-extrabold text-white tracking-tight">
                {contest.name}
              </h2>
            </div>
          </div>

          <!-- Timer & Quick Prev/Next Problem Navigation -->
          <div class="flex items-center gap-3 shrink-0">
            <ContestTimer startAt={contest.startAt} endAt={contest.endAt} state={contest.state} />

            <!-- Prev / Next Problem Buttons -->
            <div class="flex items-center bg-zinc-950 p-1 rounded-xl border border-zinc-800 text-xs">
              {#if prevContestProblem}
                <a
                  href={`/problems/${prevContestProblem.problemId}?contestId=${contest.id}`}
                  class="px-3 py-1.5 rounded-lg font-semibold text-zinc-300 hover:text-white hover:bg-zinc-800 transition flex items-center space-x-1"
                  title={`Previous: Problem ${prevContestProblem.label}`}
                >
                  <ChevronLeft class="w-4 h-4" />
                  <span class="hidden sm:inline">Prev ({prevContestProblem.label})</span>
                </a>
              {:else}
                <span class="px-3 py-1.5 rounded-lg text-zinc-600 cursor-not-allowed flex items-center space-x-1">
                  <ChevronLeft class="w-4 h-4" />
                  <span class="hidden sm:inline">Prev</span>
                </span>
              {/if}

              {#if nextContestProblem}
                <a
                  href={`/problems/${nextContestProblem.problemId}?contestId=${contest.id}`}
                  class="px-3 py-1.5 rounded-lg font-semibold text-zinc-300 hover:text-white hover:bg-zinc-800 transition flex items-center space-x-1"
                  title={`Next: Problem ${nextContestProblem.label}`}
                >
                  <span class="hidden sm:inline">Next ({nextContestProblem.label})</span>
                  <ChevronRight class="w-4 h-4" />
                </a>
              {:else}
                <span class="px-3 py-1.5 rounded-lg text-zinc-600 cursor-not-allowed flex items-center space-x-1">
                  <span class="hidden sm:inline">Next</span>
                  <ChevronRight class="w-4 h-4" />
                </span>
              {/if}
            </div>
          </div>
        </div>

        <!-- Contest Problem Switcher Bar (Tabs) -->
        {#if contestProblems.length > 0}
          <div class="pt-3 border-t border-zinc-800/80 space-y-2">
            <div class="flex items-center justify-between text-xs text-zinc-400">
              <div class="flex items-center space-x-1.5 font-semibold uppercase tracking-wider text-[11px] text-zinc-400">
                <Layers class="w-3.5 h-3.5 text-zinc-400" />
                <span>Contest Problems ({contestProblems.length})</span>
              </div>
              {#if contestSolvedProblemIds.size > 0}
                <span class="text-emerald-400 font-medium text-xs">
                  {contestSolvedProblemIds.size} of {contestProblems.length} Solved
                </span>
              {/if}
            </div>

            <div class="flex items-center gap-2 overflow-x-auto pb-1">
              {#each contestProblems as cp}
                {@const isActive = cp.problemId === problemId}
                {@const isSolved = contestSolvedProblemIds.has(cp.problemId)}
                <a
                  href={`/problems/${cp.problemId}?contestId=${contest.id}`}
                  class="shrink-0 flex items-center space-x-2.5 px-3.5 py-2 rounded-xl text-xs font-semibold transition border {
                    isActive
                      ? 'bg-white text-black border-white shadow-md'
                      : isSolved
                      ? 'bg-emerald-500/10 text-emerald-300 border-emerald-500/30 hover:bg-emerald-500/20 hover:text-white'
                      : 'bg-zinc-950 text-zinc-300 border-zinc-800 hover:border-zinc-700 hover:bg-zinc-900 hover:text-white'
                  }"
                >
                  <span class="w-5 h-5 rounded-md flex items-center justify-center font-bold text-xs {
                    isActive
                      ? 'bg-black text-white'
                      : isSolved
                      ? 'bg-emerald-500/20 text-emerald-300'
                      : 'bg-zinc-800 text-zinc-300'
                  }">
                    {cp.label}
                  </span>

                  <span class="max-w-[150px] sm:max-w-[200px] truncate">
                    {cp.problem?.title || `Problem ${cp.label}`}
                  </span>

                  {#if isSolved}
                    <CheckCircle2 class="w-3.5 h-3.5 shrink-0 {isActive ? 'text-emerald-700' : 'text-emerald-400'}" />
                  {/if}
                </a>
              {/each}
            </div>
          </div>
        {/if}
      </div>
    {/if}

    {#if contest && contest.state === 'UPCOMING' && $auth.user?.role !== 'ADMIN'}
      <div class="p-12 rounded-2xl border border-zinc-800 bg-zinc-900/50 text-center space-y-3">
        <div class="w-12 h-12 rounded-full bg-zinc-800 border border-zinc-700 text-white flex items-center justify-center mx-auto">
          <Lock class="w-6 h-6" />
        </div>
        <h3 class="text-lg font-bold text-white">Problem is Locked</h3>
        <p class="text-xs text-zinc-400 max-w-md mx-auto">
          Problem statements and submissions for this contest will automatically unlock when the contest starts.
        </p>
        <a
          href={`/contests/${contest.id}`}
          class="inline-block mt-4 px-4 py-2 rounded-xl text-xs font-semibold bg-white text-black hover:bg-zinc-200 transition"
        >
          Return to Contest Lobby
        </a>
      </div>
    {:else}
      <!-- Header Navigation Card -->
      <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/70 space-y-4 shadow-xl">
        <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
          <div class="space-y-1.5">
            <div class="flex items-center space-x-2.5">
              {#if currentContestProblem}
                <span class="text-xs px-2.5 py-0.5 rounded-full font-bold font-mono bg-white text-black shadow-sm">
                  Problem {currentContestProblem.label}
                </span>
              {/if}
              {#if !contest}
                <span class="text-xs px-2.5 py-0.5 rounded-full font-semibold font-mono {
                  problem.platform === 'CODEFORCES' ? 'bg-red-500/15 text-red-300 border border-red-500/30' :
                  'bg-zinc-800 text-zinc-300 border border-zinc-700'
                }">
                  {problem.platform}
                </span>
                <span class="text-xs font-mono text-zinc-400">{problem.externalId}</span>
                {#if problem.difficulty}
                  <span class="text-xs px-2 py-0.5 rounded-full font-mono bg-zinc-950 text-zinc-400 border border-zinc-800">
                    ★ {problem.difficulty}
                  </span>
                {/if}
              {/if}
              {#if currentContestProblem?.points}
                <span class="text-xs px-2 py-0.5 rounded-full font-mono bg-indigo-500/20 text-indigo-300 border border-indigo-500/30">
                  {currentContestProblem.points} pts
                </span>
              {/if}
            </div>

            <h1 class="text-2xl font-extrabold text-white leading-tight">
              {#if currentContestProblem}
                <span class="text-zinc-400 font-mono mr-1.5">{currentContestProblem.label}.</span>
              {/if}
              {problem.title}
            </h1>
          </div>

        <div class="flex items-center space-x-3 shrink-0">
          <!-- View Layout Toggle (Tabbed vs Split) -->
          <div class="flex items-center bg-zinc-950 p-1 rounded-xl border border-zinc-800 text-xs">
            <button
              on:click={() => (viewMode = 'tabbed')}
              class="px-3 py-1 rounded-lg font-semibold transition {
                viewMode === 'tabbed' ? 'bg-white text-black shadow-sm' : 'text-zinc-400 hover:text-white'
              }"
            >
              Tabbed View
            </button>
            <button
              on:click={() => (viewMode = 'split')}
              class="px-3 py-1 rounded-lg font-semibold transition flex items-center space-x-1 {
                viewMode === 'split' ? 'bg-white text-black shadow-sm' : 'text-zinc-400 hover:text-white'
              }"
            >
              <Columns class="w-3.5 h-3.5" />
              <span>Split View</span>
            </button>
          </div>

          {#if !contest}
            <a
              href={problem.url}
              target="_blank"
              rel="noopener noreferrer"
              class="px-3 py-1.5 rounded-xl border border-zinc-700 bg-zinc-900 hover:bg-zinc-800 text-xs text-zinc-200 hover:text-white font-semibold transition flex items-center space-x-1.5"
              title="Open official statement on source website"
            >
              <span>Source</span>
              <ExternalLink class="w-3.5 h-3.5" />
            </a>
          {/if}
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
              activeTab === 'statement' ? 'bg-white text-black shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
            }"
          >
            <BookOpen class="w-4 h-4" />
            <span>Problem Statement</span>
          </button>

          <button
            on:click={() => (activeTab = 'editor')}
            class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-2 {
              activeTab === 'editor' ? 'bg-white text-black shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
            }"
          >
            <Code2 class="w-4 h-4" />
            <span>Code Editor & Submit</span>
          </button>

          <button
            on:click={() => (activeTab = 'submissions')}
            class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-2 {
              activeTab === 'submissions' ? 'bg-white text-black shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
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
                <h3 class="text-base font-bold text-white uppercase tracking-wider flex items-center space-x-2">
                  <Terminal class="w-5 h-5 text-white" />
                  <span>Sample Test Cases</span>
                </h3>

                {#each statement.sampleCases as sc, idx}
                  <div class="p-4 rounded-xl border border-zinc-800 bg-zinc-950/80 space-y-3">
                    <div class="text-sm font-bold text-zinc-300 uppercase">Example {idx + 1}</div>
                    <div class="space-y-1.5">
                      <div class="flex items-center justify-between text-sm font-mono text-zinc-300">
                        <span>Input:</span>
                        <button
                          on:click={() => copyToClipboard(sc.input, `in_split_${idx}`)}
                          class="px-2 py-0.5 rounded-md hover:bg-zinc-800 text-zinc-400 hover:text-white transition flex items-center space-x-1"
                        >
                          {#if copiedCaseIndex === `in_split_${idx}`}
                            <Check class="w-4 h-4 text-emerald-400" />
                            <span class="text-xs text-emerald-400">Copied!</span>
                          {:else}
                            <Copy class="w-4 h-4" />
                            <span class="text-xs">Copy</span>
                          {/if}
                        </button>
                      </div>
                      <pre class="p-3.5 rounded-xl bg-zinc-900 border border-zinc-800 text-sm md:text-base font-mono text-zinc-200 overflow-x-auto select-all leading-relaxed">{sc.input}</pre>
                    </div>

                    {#if sc.output}
                      <div class="space-y-1.5">
                        <div class="flex items-center justify-between text-sm font-mono text-zinc-300">
                          <span>Output:</span>
                          <button
                            on:click={() => copyToClipboard(sc.output, `out_split_${idx}`)}
                            class="px-2 py-0.5 rounded-md hover:bg-zinc-800 text-zinc-400 hover:text-white transition flex items-center space-x-1"
                          >
                            {#if copiedCaseIndex === `out_split_${idx}`}
                              <Check class="w-4 h-4 text-emerald-400" />
                              <span class="text-xs text-emerald-400">Copied!</span>
                            {:else}
                              <Copy class="w-4 h-4" />
                              <span class="text-xs">Copy</span>
                            {/if}
                          </button>
                        </div>
                        <pre class="p-3.5 rounded-xl bg-zinc-900 border border-zinc-800 text-sm md:text-base font-mono text-zinc-200 overflow-x-auto select-all leading-relaxed">{sc.output}</pre>
                      </div>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          {:else}
            <div class="p-8 text-center text-zinc-400 text-sm">
              Statement not loaded.
              {#if !contest}
                <a href={problem.url} target="_blank" class="text-white underline ml-1">Open source statement</a>
              {/if}
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
                class="px-3 py-1.5 rounded-lg bg-zinc-950 border border-zinc-800 text-zinc-200 text-xs font-mono focus:border-zinc-400 focus:outline-none"
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
                <Upload class="w-3.5 h-3.5 text-white" />
                <span>Upload File</span>
              </button>
            </div>

            <button
              on:click={handleSubmit}
              disabled={submitting}
              class="px-5 py-1.5 rounded-xl font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black shadow-sm transition flex items-center space-x-2 text-xs"
            >
              <Send class="w-3.5 h-3.5" />
              <span>{submitting ? 'Submitting...' : 'Submit Code'}</span>
            </button>
          </div>

          {#if uploadSuccessMessage}
            <div class="px-3 py-2 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs font-semibold flex items-center space-x-2">
              <CheckCircle2 class="w-3.5 h-3.5 shrink-0 text-emerald-400" />
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
                    activeSubmission.status === 'ACCEPTED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                    activeSubmission.status === 'WRONG_ANSWER' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30' :
                    'bg-zinc-800 text-zinc-200 border border-zinc-700'
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
                  <h3 class="text-base font-bold text-white uppercase tracking-wider flex items-center space-x-2">
                    <Terminal class="w-5 h-5 text-white" />
                    <span>Sample Test Cases</span>
                  </h3>

                  <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {#each statement.sampleCases as sc, idx}
                      <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-950/80 space-y-4">
                        <div class="text-sm font-bold text-zinc-300 uppercase">Example {idx + 1}</div>

                        <!-- Input -->
                        <div class="space-y-1.5">
                          <div class="flex items-center justify-between text-sm font-mono text-zinc-300">
                            <span>Input:</span>
                            <button
                              on:click={() => copyToClipboard(sc.input, `in_tab_${idx}`)}
                              class="px-2 py-0.5 rounded-md hover:bg-zinc-800 text-zinc-400 hover:text-white transition flex items-center space-x-1"
                            >
                              {#if copiedCaseIndex === `in_tab_${idx}`}
                                <Check class="w-4 h-4 text-emerald-400" />
                                <span class="text-xs text-emerald-400">Copied!</span>
                              {:else}
                                <Copy class="w-4 h-4" />
                                <span class="text-xs">Copy</span>
                              {/if}
                            </button>
                          </div>
                          <pre class="p-3.5 rounded-xl bg-zinc-900 border border-zinc-800 text-sm md:text-base font-mono text-zinc-200 overflow-x-auto select-all leading-relaxed">{sc.input}</pre>
                        </div>

                        <!-- Output -->
                        {#if sc.output}
                          <div class="space-y-1.5">
                            <div class="flex items-center justify-between text-sm font-mono text-zinc-300">
                              <span>Output:</span>
                              <button
                                on:click={() => copyToClipboard(sc.output, `out_tab_${idx}`)}
                                class="px-2 py-0.5 rounded-md hover:bg-zinc-800 text-zinc-400 hover:text-white transition flex items-center space-x-1"
                              >
                                {#if copiedCaseIndex === `out_tab_${idx}`}
                                  <Check class="w-4 h-4 text-emerald-400" />
                                  <span class="text-xs text-emerald-400">Copied!</span>
                                {:else}
                                  <Copy class="w-4 h-4" />
                                  <span class="text-xs">Copy</span>
                                {/if}
                              </button>
                            </div>
                            <pre class="p-3.5 rounded-xl bg-zinc-900 border border-zinc-800 text-sm md:text-base font-mono text-zinc-200 overflow-x-auto select-all leading-relaxed">{sc.output}</pre>
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
                  class="px-5 py-2.5 rounded-xl font-bold bg-white hover:bg-zinc-200 text-black shadow-sm transition flex items-center space-x-2 text-xs"
                >
                  <Code2 class="w-4 h-4" />
                  <span>Open Code Editor & Solve</span>
                </button>
              </div>
            {:else}
              <div class="p-12 text-center text-zinc-400 space-y-4">
                <p>Statement could not be loaded automatically.</p>
                {#if !contest}
                  <a
                    href={problem.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="px-4 py-2 rounded-xl text-xs font-bold bg-white hover:bg-zinc-200 text-black inline-flex items-center space-x-1.5"
                  >
                    <span>Open Source Statement</span>
                    <ExternalLink class="w-3.5 h-3.5" />
                  </a>
                {/if}
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
                    class="px-3 py-1.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-200 text-xs font-mono focus:border-zinc-400 focus:outline-none"
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
                  <Upload class="w-3.5 h-3.5 text-white" />
                  <span>Upload File (Auto-Detect)</span>
                </button>
              </div>

              <div class="flex items-center space-x-3">
                <button
                  on:click={handleSubmit}
                  disabled={submitting}
                  class="px-6 py-2 rounded-xl font-bold bg-white hover:bg-zinc-200 disabled:opacity-50 text-black shadow-sm transition flex items-center space-x-2 text-xs"
                >
                  <Send class="w-4 h-4" />
                  <span>{submitting ? 'Submitting Solution...' : 'Submit Solution'}</span>
                </button>
              </div>
            </div>

            <!-- Upload Feedback Notification -->
            {#if uploadSuccessMessage}
              <div class="px-4 py-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-300 text-xs font-semibold flex items-center space-x-2">
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
                      activeSubmission.status === 'ACCEPTED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                      activeSubmission.status === 'WRONG_ANSWER' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30' :
                      'bg-zinc-800 text-zinc-200 border border-zinc-700'
                    }">
                      {activeSubmission.status}
                    </span>
                  {/if}
                </div>

                <p class="text-xs text-zinc-400 font-mono">{submitStatus}</p>
              </div>
            {/if}
          </div>

        {:else}
          <!-- 3. Submissions History Tab -->
          <div class="space-y-4 max-w-4xl mx-auto">
            <div class="flex items-center justify-between">
              <h3 class="text-sm font-bold text-white uppercase tracking-wider">Your Submission History</h3>
              <div class="flex items-center space-x-2">
                <button
                  on:click={() => problem && loadSubmissions(problem.id, contestId)}
                  class="px-3 py-1 text-xs rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-300 transition"
                >
                  Refresh
                </button>
                <button
                  on:click={() => (activeTab = 'editor')}
                  class="px-3 py-1 text-xs rounded-lg bg-white text-black font-semibold hover:bg-zinc-200 transition flex items-center space-x-1"
                >
                  <Code2 class="w-3.5 h-3.5" />
                  <span>Editor</span>
                </button>
              </div>
            </div>

            <!-- Verdict & Dispatch Banner -->
            {#if activeSubmission || submitStatus}
              <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-950 space-y-3">
                <div class="flex items-center justify-between">
                  <span class="text-xs font-semibold uppercase tracking-wider text-zinc-400">Submission Verdict</span>
                  {#if activeSubmission}
                    <span class="text-xs font-bold font-mono px-3 py-1 rounded-lg {
                      activeSubmission.status === 'ACCEPTED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                      activeSubmission.status === 'WRONG_ANSWER' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30' :
                      activeSubmission.status === 'JUDGING' || activeSubmission.status === 'PENDING' || activeSubmission.status === 'DISPATCHING' ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30 animate-pulse' :
                      'bg-zinc-800 text-zinc-200 border border-zinc-700'
                    }">
                      {activeSubmission.status}
                    </span>
                  {/if}
                </div>

                <p class="text-xs text-zinc-400 font-mono">{submitStatus}</p>

                {#if activeSubmission && (activeSubmission.status === 'PENDING' || activeSubmission.status === 'JUDGING' || activeSubmission.status === 'DISPATCHING')}
                  <div class="flex flex-wrap items-center gap-2 pt-2 border-t border-zinc-800/80">
                    <button
                      on:click={() => handleManualSync(activeSubmission!.id)}
                      class="px-2.5 py-1 rounded-lg text-xs font-bold bg-blue-600/20 hover:bg-blue-600/30 text-blue-300 border border-blue-500/30 transition flex items-center space-x-1"
                    >
                      <Clock class="w-3 h-3" />
                      <span>Sync Status</span>
                    </button>
                  </div>
                {/if}
              </div>
            {/if}

            {#if recentSubmissions.length === 0}
              <p class="text-xs text-zinc-500 py-12 text-center">No submissions recorded yet for this problem.</p>
            {:else}
              <div class="space-y-2.5">
                {#each recentSubmissions as sub}
                  <div class="p-4 rounded-xl border border-zinc-800 bg-zinc-950 flex items-center justify-between text-xs">
                    <div class="space-y-1">
                      <div class="font-mono font-semibold text-zinc-200">{sub.language}</div>
                      <div class="text-zinc-500">{new Date(sub.submittedAt).toLocaleString()}</div>
                      {#if sub.metadata && sub.metadata.error}
                        <div class="text-rose-400 text-[11px] font-sans">{sub.metadata.error}</div>
                      {/if}
                    </div>
                    <div class="flex items-center space-x-2">
                      {#if sub.status === 'JUDGING' || sub.status === 'PENDING' || sub.status === 'DISPATCHING'}
                        <button
                          on:click={() => handleManualSync(sub.id)}
                          class="px-2 py-1 rounded-lg text-[11px] font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-300 border border-zinc-700 transition"
                          title="Re-check status on source platform"
                        >
                          Sync
                        </button>
                      {/if}
                      <span class="font-bold font-mono px-3 py-1.5 rounded-lg {
                        sub.status === 'ACCEPTED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                        sub.status === 'WRONG_ANSWER' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30' :
                        sub.status === 'JUDGING' || sub.status === 'PENDING' || sub.status === 'DISPATCHING' ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30 animate-pulse' :
                        'bg-zinc-800 text-zinc-400 border border-zinc-700'
                      }">
                        {sub.status}
                      </span>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        {/if}
      </div>
    {/if}
    {/if}
  </div>
{/if}

<style>
  :global(.statement-content p) {
    margin-bottom: 1.25rem;
    font-size: 1.075rem;
    line-height: 1.85;
    color: #e4e4e7;
  }
  :global(.statement-content ul) {
    list-style-type: disc;
    margin-left: 1.5rem;
    margin-bottom: 1.25rem;
    font-size: 1.075rem;
    line-height: 1.85;
    color: #e4e4e7;
  }
  :global(.statement-content ol) {
    list-style-type: decimal;
    margin-left: 1.5rem;
    margin-bottom: 1.25rem;
    font-size: 1.075rem;
    line-height: 1.85;
    color: #e4e4e7;
  }
  :global(.statement-content li) {
    margin-bottom: 0.5rem;
  }
  :global(.statement-content code) {
    background-color: #27272a;
    color: #a5b4fc;
    padding: 0.2rem 0.45rem;
    border-radius: 0.35rem;
    font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    font-size: 0.95em;
  }
  :global(.statement-content pre) {
    background-color: #18181b;
    border: 1px solid #27272a;
    padding: 1rem;
    border-radius: 0.75rem;
    overflow-x: auto;
    margin-bottom: 1.25rem;
    font-size: 1rem;
    line-height: 1.65;
  }
  :global(.statement-content h3),
  :global(.statement-content h4),
  :global(.statement-content .section-title) {
    font-weight: 800;
    font-size: 1.25rem;
    color: #ffffff;
    margin-top: 1.75rem;
    margin-bottom: 0.75rem;
    letter-spacing: -0.01em;
  }
  :global(.katex) {
    font-size: 1.15em;
    color: #f4f4f5;
  }
  :global(.katex-display) {
    margin: 1.25em 0;
    overflow-x: auto;
    overflow-y: hidden;
    padding: 0.75rem 0;
    font-size: 1.25em;
  }
</style>
