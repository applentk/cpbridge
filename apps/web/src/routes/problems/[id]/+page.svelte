<script lang="ts">
  import { onDestroy } from 'svelte';
  import { browser } from '$app/environment';
  import { goto } from '$app/navigation';
  import { page } from '$app/stores';
  import { api } from '$lib/api/client';
  import { auth } from '$lib/stores/auth';
  import { isExtensionVersionCompatible, pingExtension, submitViaExtension, completeManualSubmission, pollStatusViaExtension, recoverPendingSubmissions, acknowledgeRecoveredSubmission } from '$lib/extension/bridge';
  import { reconcileExtensionSubmissions } from '$lib/extension/reconcile';
  import { renderMathInHtml } from '$lib/utils/math';
  import {
    type Problem,
    type LanguageId,
    type Submission,
    type ExtensionPingResponse,
    type ExtensionSubmissionActionRequiredResponse,
    type ProblemStatement,
    type Contest,
    type ContestProblem,
    LATEST_EXTENSION_VERSION,
    formatLanguageName
  } from '@cpbridge/contracts';
  import MonacoEditor from '$lib/components/MonacoEditor.svelte';
  import SubmissionModal from '$lib/components/SubmissionModal.svelte';
  import Pagination from '$lib/components/Pagination.svelte';
  import ContestTimer from '$lib/components/ContestTimer.svelte';
  import ManualSubmissionActions from '$lib/components/ManualSubmissionActions.svelte';
  import {
    ExternalLink,
    Send,
    CheckCircle2,
    Clock,
    Cpu,
    Copy,
    Check,
    BookOpen,
    Code2,
    Terminal,
    Upload,
    ArrowLeft,
    Trophy,
    Layers,
    Lock,
    XCircle,
    AlertTriangle
  } from 'lucide-svelte';

  let problemId: string = $page.params.id || '';
  let contestId: string | null = $page.url.searchParams.get('contestId');
  let contest: Contest | null = null;
  let contestProblems: ContestProblem[] = [];
  let contestSolvedProblemIds: Set<string> = new Set();
  let contestWrongProblemIds: Set<string> = new Set();
  let _contestLoading = false;

  let currentLoadedProblemId: string = '';
  let currentLoadedContestId: string | null | undefined = undefined;

  let problem: Problem | null = null;
  let extensionInfo: ExtensionPingResponse | null = null;
  let extensionCheckLoading = true;
  let extensionCheckProblemId = '';
  let statement: ProblemStatement | null = null;
  let renderedHtml = '';
  let renderedNote = '';
  let statementLoading = true;
  let loading = true;
  let error = '';

  type ProblemTab = 'statement' | 'editor' | 'submissions';

  let activeTab: ProblemTab = 'statement';
  let copiedCaseIndex: string | null = null;

  let language: LanguageId = 'cpp23';
  let sourceCode = `#include <iostream>\nusing namespace std;\n\nint main() {\n    ios_base::sync_with_stdio(false);\n    cin.tie(NULL);\n    \n    // Solve problem\n    \n    return 0;\n}\n`;

  let submitting = false;
  let submitStatus = '';
  let duplicateSubmission = false;
  let activeSubmission: Submission | null = null;
  let manualSubmissionAction: ExtensionSubmissionActionRequiredResponse | null = null;
  let manualSourceCode = '';
  let checkingManualSubmission = false;
  let manualCodeCopied = false;
  let recentSubmissions: Submission[] = [];
  let submissionsLoading = false;
  let submissionsInitialized = false;
  let viewingSubmission: Submission | null = null;
  let submissionsPage = 1;
  let submissionsPageSize = 10;
  let pollInterval: ReturnType<typeof setInterval> | null = null;
  let manualSubmissionPollInterval: ReturnType<typeof setInterval> | null = null;
  let manualSubmissionCheckInFlight = false;

  let uploadSuccessMessage = '';
  let fileInputElement: HTMLInputElement;

  const starterTemplates: Record<LanguageId, string> = {
    cpp23: `#include <iostream>\nusing namespace std;\n\nint main() {\n    ios_base::sync_with_stdio(false);\n    cin.tie(NULL);\n    \n    // Solve problem\n    \n    return 0;\n}\n`,
    python3: `import sys\n\ndef main():\n    input = sys.stdin.read\n    # Solve problem\n\nif __name__ == "__main__":\n    main()\n`,
    java21: `import java.util.*;\n\npublic class Main {\n    public static void main(String[] args) {\n        Scanner scanner = new Scanner(System.in);\n        // Solve problem\n    }\n}\n`
  };

  const languageLabels: Record<LanguageId, string> = {
    cpp23: 'C++23 (GCC)',
    python3: 'Python 3',
    java21: 'Java 21'
  };

  $: currentContestProblem = contestProblems.find(cp => cp.problemId === problemId);
  $: paginatedRecentSubmissions = recentSubmissions.slice((submissionsPage - 1) * submissionsPageSize, submissionsPage * submissionsPageSize);
  $: if (submissionsPage > Math.max(1, Math.ceil(recentSubmissions.length / submissionsPageSize))) {
    submissionsPage = Math.max(1, Math.ceil(recentSubmissions.length / submissionsPageSize));
  }
  $: extensionCompatible = !!extensionInfo && isExtensionVersionCompatible(extensionInfo.version);
  $: externalPlatformConnected = !!problem && !!extensionInfo?.platforms[problem.platform]?.loggedIn && !!extensionInfo.platforms[problem.platform]?.username?.trim();
  $: canUseSubmissionWorkspace = !$auth.loading && !!$auth.user && !extensionCheckLoading && extensionCompatible && externalPlatformConnected;

  $: {
    if (browser && !$auth.loading) {
      const nextPId = $page.params.id || '';
      const nextCId = $page.url.searchParams.get('contestId');
      const nextTab = tabFromURL();

      if (!nextCId && $auth.user?.role !== 'ADMIN') {
        void goto('/contests');
      } else if (nextPId && (nextPId !== currentLoadedProblemId || nextCId !== currentLoadedContestId)) {
        currentLoadedProblemId = nextPId;
        currentLoadedContestId = nextCId;
        problemId = nextPId;
        contestId = nextCId;
        activeTab = nextTab;
        loadProblemAndContest();
      } else if (activeTab !== nextTab) {
        activeTab = nextTab;
      }
    }
  }

  function isProblemTab(value: string | null): value is ProblemTab {
    return value === 'statement' || value === 'editor' || value === 'submissions';
  }

  function tabFromURL(): ProblemTab {
    const tab = $page.url.searchParams.get('tab');
    return isProblemTab(tab) ? tab : 'statement';
  }

  function setActiveTab(tab: ProblemTab) {
    activeTab = tab;
    if (!browser) return;

    const url = new URL($page.url);
    url.searchParams.set('tab', tab);
    void goto(`${url.pathname}${url.search}${url.hash}`, {
      replaceState: true,
      keepFocus: true,
      noScroll: true
    });
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
    stopManualSubmissionPolling();
    activeSubmission = null;
    manualSubmissionAction = null;
    manualSourceCode = '';
    checkingManualSubmission = false;
    manualCodeCopied = false;
    submitStatus = '';
    duplicateSubmission = false;
    recentSubmissions = [];
    submissionsInitialized = false;
    submissionsLoading = false;

    try {
      const pId = problemId;
      const cId = contestId;
      if (!pId) return;

      if (!cId && $auth.user?.role !== 'ADMIN') {
        error = 'Problems can only be accessed within a contest. Please select a contest to view this problem.';
        loading = false;
        return;
      }

      const qParam = cId ? `?contestId=${encodeURIComponent(cId)}` : '';
      const primaryPromises: [Promise<Problem>, Promise<void>, Promise<void>?] = [
        api.get<Problem>(`/problems/${pId}${qParam}`),
        loadStatement(pId, cId)
      ];

      if (cId) {
        if (!contest || contest.id !== cId) {
          primaryPromises.push(loadContestData(cId));
        } else {
          // Contest standings are auxiliary to the problem and statement.
          void loadContestData(cId);
        }
      } else {
        contest = null;
      contestProblems = [];
      contestSolvedProblemIds = new Set();
      contestWrongProblemIds = new Set();
      }

      const [probData] = await Promise.all(primaryPromises);
      problem = probData;
      extensionInfo = null;
      extensionCheckProblemId = pId;
      extensionCheckLoading = true;
      void refreshExtensionGuard(pId);

      // Submission history is intentionally independent: it can be slow due
      // to verdict synchronization, but must never delay the statement.
      void loadInitialSubmissions(pId, cId);
    } catch (err) {
      error = err instanceof Error ? err.message : 'Failed to load problem';
    } finally {
      loading = false;
    }
  }

  async function refreshExtensionGuard(pId: string) {
    const info = await pingExtension();
    if (extensionCheckProblemId !== pId || pId !== problemId) return;
    extensionInfo = info;
    extensionCheckLoading = false;
  }

  async function loadContestData(cId: string) {
    _contestLoading = true;
    try {
      const cRes = await api.get<Contest>(`/contests/${cId}`);
      if (cId !== contestId) return;
      contest = cRes;
      contestProblems = cRes.problems || [];

      // Fetch standings context separately; it is not required to render the
      // problem and can wait behind a slow submissions response.
      void loadContestSolvedProblems(cId);
    } catch (err) {
      console.error('Failed to load contest context:', err);
    } finally {
      _contestLoading = false;
    }
  }

  async function loadContestSolvedProblems(cId: string) {
    try {
      const subsRes = await api.get<Submission[]>(`/submissions?contestId=${cId}`);
      if (cId !== contestId) return;
      const solved = new Set<string>();
      const wrong = new Set<string>();
      if (Array.isArray(subsRes)) {
        for (const sub of subsRes) {
          if (sub.status === 'ACCEPTED') {
            solved.add(sub.problemId);
          } else if (sub.status === 'WRONG_ANSWER') {
            wrong.add(sub.problemId);
          }
        }
      }
      contestSolvedProblemIds = solved;
      contestWrongProblemIds = new Set([...wrong].filter((id) => !solved.has(id)));
    } catch {}
  }

  async function loadStatement(pId: string, cId: string | null = null) {
    statementLoading = true;
    try {
      const qParam = cId ? `?contestId=${encodeURIComponent(cId)}` : '';
      statement = await api.get<ProblemStatement>(`/problems/${pId}/statement${qParam}`);
      if (statement && statement.html) {
        renderedHtml = renderMathInHtml(statement.html);
        renderedNote = statement.note ? renderMathInHtml(statement.note) : '';
      } else {
        renderedHtml = '';
        renderedNote = '';
      }
    } catch (err) {
      console.error('Failed to load statement:', err);
      statement = null;
      renderedHtml = '';
      renderedNote = '';
    } finally {
      statementLoading = false;
    }
  }

  async function loadSubmissions(pId: string, cId: string | null) {
    // The skeleton is only for the first history load. Submission dispatch
    // and verdict polling refresh this list in the background, so preserve the
    // already rendered history while those requests are in flight.
    const showSkeleton = !submissionsInitialized;
    if (showSkeleton) submissionsLoading = true;
    try {
      await reconcileExtensionSubmissions();
      const query = cId ? `/submissions?problemId=${pId}&contestId=${cId}` : `/submissions?problemId=${pId}`;
      const submissions = await api.get<Submission[]>(query);
      if (pId === problemId && cId === contestId) {
        recentSubmissions = submissions;
        const recovered = await recoverPendingSubmissions();
        const awaiting = recovered.find((dispatch) =>
          dispatch.state === 'AWAITING_USER_ACTION' &&
          Boolean(dispatch.actionUrl) &&
          submissions.some((submission) => submission.id === dispatch.submissionId)
        );
        if (awaiting?.actionUrl) {
          const pendingSubmission = submissions.find((submission) => submission.id === awaiting.submissionId);
          if (pendingSubmission) {
            activeSubmission = pendingSubmission;
            manualSourceCode = pendingSubmission.sourceCode;
            manualSubmissionAction = {
              type: 'SUBMISSION_ACTION_REQUIRED',
              submissionId: awaiting.submissionId,
              platform: 'CODEFORCES',
              action: 'COMPLETE_ANTIBOT',
              submitUrl: awaiting.actionUrl,
              message: awaiting.actionMessage || 'Complete the Codeforces verification and submit the prefilled solution.'
            };
            submitStatus = manualSubmissionAction.message;
            startManualSubmissionPolling();
          }
        }
      }
      return submissions;
    } catch {
      if (pId === problemId && cId === contestId) {
        recentSubmissions = [];
      }
      return [];
    } finally {
      if (pId === problemId && cId === contestId) {
        submissionsInitialized = true;
        if (showSkeleton) submissionsLoading = false;
      }
    }
  }

  async function loadInitialSubmissions(pId: string, cId: string | null) {
    const submissions = await loadSubmissions(pId, cId);
    if (pId !== problemId || cId !== contestId || submissions.length === 0) return;

    const latest = submissions[0];
    if (manualSubmissionAction?.submissionId === latest.id) return;
    if (latest.status === 'JUDGING' || latest.status === 'PENDING' || latest.status === 'DISPATCHING') {
      activeSubmission = latest;
      submitStatus = `Judging in progress... Status: ${latest.status}`;
      startSubmissionPolling(latest.id);
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

  function stopManualSubmissionPolling() {
    if (manualSubmissionPollInterval) {
      clearInterval(manualSubmissionPollInterval);
      manualSubmissionPollInterval = null;
    }
  }

  function startManualSubmissionPolling() {
    stopManualSubmissionPolling();
    let attempts = 0;
    const maxAttempts = 180; // keep checking for up to ~3 minutes

    manualSubmissionPollInterval = setInterval(() => {
      attempts += 1;
      if (attempts > maxAttempts || !manualSubmissionAction) {
        stopManualSubmissionPolling();
        return;
      }
      void reconcileManualSubmission(false);
    }, 1000);
  }

  function startSubmissionPolling(submissionId: string) {
    stopSubmissionPolling();
    let attempts = 0;
    const maxAttempts = 60; // poll up to ~2.5 minutes

    pollInterval = setInterval(async () => {
      attempts++;
      if (attempts > maxAttempts) {
        stopSubmissionPolling();
        submitStatus = 'Judging timed out. Please check the official site.';
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
        if (updated) {
          const idx = recentSubmissions.findIndex((s) => s.id === submissionId);
          if (idx !== -1) {
            recentSubmissions[idx] = updated;
            recentSubmissions = [...recentSubmissions];
          }
        }
        if (updated && updated.status !== 'JUDGING' && updated.status !== 'PENDING' && updated.status !== 'DISPATCHING') {
          activeSubmission = updated;
          submitStatus = `Verdict: ${updated.status}`;
          stopSubmissionPolling();
          const currentProblemId = problem?.id;
          if (updated.status === 'ACCEPTED' && currentProblemId) {
            contestSolvedProblemIds = new Set([...contestSolvedProblemIds, currentProblemId]);
            contestWrongProblemIds = new Set([...contestWrongProblemIds].filter((id) => id !== currentProblemId));
          } else if (updated.status === 'WRONG_ANSWER' && currentProblemId && !contestSolvedProblemIds.has(currentProblemId)) {
            contestWrongProblemIds = new Set([...contestWrongProblemIds, currentProblemId]);
          }
          if (problem) {
            await loadSubmissions(problem.id, contestId);
          }
          return;
        }

        // 2. Also check via Chrome Extension Bridge if available
        if (problem && updated && updated.externalSubmissionId && !updated.externalSubmissionId.startsWith('cf_') && !updated.externalSubmissionId.startsWith('ac_')) {
          try {
            const extRes = await pollStatusViaExtension(
              problem.platform,
              updated.externalSubmissionId,
              problem.externalId,
              problem.url
            );
            if (extRes && extRes.status && extRes.status !== 'JUDGING' && extRes.status !== 'PENDING' && extRes.status !== 'DISPATCHING') {
              const synced = await api.post<Submission>(`/submissions/${submissionId}/sync`);
              if (synced) {
                activeSubmission = synced;
                submitStatus = `Verdict: ${synced.status}`;
                stopSubmissionPolling();
                const currentProblemId = problem?.id;
                if (synced.status === 'ACCEPTED' && currentProblemId) {
                  contestSolvedProblemIds = new Set([...contestSolvedProblemIds, currentProblemId]);
                  contestWrongProblemIds = new Set([...contestWrongProblemIds].filter((id) => id !== currentProblemId));
                } else if (synced.status === 'WRONG_ANSWER' && currentProblemId && !contestSolvedProblemIds.has(currentProblemId)) {
                  contestWrongProblemIds = new Set([...contestWrongProblemIds, currentProblemId]);
                }
                if (problem) {
                  await loadSubmissions(problem.id, contestId);
                }
                return;
              }
            }
          } catch {}
        }
      } catch (err) {
        console.error('Polling error:', err);
      }
    }, 2500);
  }

  function isDuplicateSubmissionError(error: unknown): boolean {
    return error instanceof Error && error.message.includes('an identical solution was already submitted for this problem');
  }

  function openSubmissionsTab() {
    setActiveTab('submissions');
  }

  function navigateToSubmissionsTab() {
    if (!browser) return;

    const url = new URL($page.url);
    url.searchParams.set('tab', 'submissions');
    void goto(`${url.pathname}${url.search}${url.hash}`, {
      replaceState: true,
      keepFocus: true,
      noScroll: true
    });
  }

  function openManualSubmitPage() {
    if (!manualSubmissionAction) return;
    window.open(manualSubmissionAction.submitUrl, '_blank', 'noopener,noreferrer');
  }

  async function copyManualSource() {
    const code = manualSourceCode || activeSubmission?.sourceCode || sourceCode;
    await navigator.clipboard.writeText(code);
    manualCodeCopied = true;
    setTimeout(() => {
      manualCodeCopied = false;
    }, 2000);
  }

  async function markSubmissionDispatched(submission: Submission, externalSubmissionId: string) {
    let lastError: unknown;
    // Codeforces can expose the new row in the browser before its public API
    // exposes the same row. Retrying this bookkeeping call is safe: the
    // extension has already persisted the external ID and the API update is
    // idempotent for this cpbridge submission.
    for (let attempt = 0; attempt < 6; attempt += 1) {
      try {
        await api.post(`/submissions/${submission.id}/dispatched`, { externalSubmissionId });
        lastError = undefined;
        break;
      } catch (error) {
        lastError = error;
        if (attempt < 5) await new Promise((resolve) => setTimeout(resolve, 1000));
      }
    }
    if (lastError) throw lastError;
    stopManualSubmissionPolling();
    activeSubmission = { ...submission, status: 'JUDGING', externalSubmissionId };
    const idx = recentSubmissions.findIndex((item) => item.id === submission.id);
    if (idx !== -1) {
      recentSubmissions[idx] = { ...recentSubmissions[idx], status: 'JUDGING', externalSubmissionId };
      recentSubmissions = [...recentSubmissions];
    }
    manualSubmissionAction = null;
    manualSourceCode = '';
    submitStatus = 'Submitted! Status: JUDGING (Polling verdict...)';
    navigateToSubmissionsTab();
    startSubmissionPolling(submission.id);
  }

  async function reconcileManualSubmission(showCheckingState: boolean) {
    if (!manualSubmissionAction || !activeSubmission || manualSubmissionCheckInFlight) return;
    const action = manualSubmissionAction;
    const submission = activeSubmission;
    manualSubmissionCheckInFlight = true;
    if (showCheckingState) checkingManualSubmission = true;
    try {
      const response = await completeManualSubmission(submission.id);
      if (manualSubmissionAction?.submissionId !== action.submissionId || activeSubmission?.id !== submission.id) return;

      if (response.type === 'SUBMISSION_CREATED' && response.externalSubmissionId) {
        await markSubmissionDispatched(submission, response.externalSubmissionId);
        if (problem) await loadSubmissions(problem.id, contestId);
      } else if (response.type === 'SUBMISSION_ACTION_REQUIRED') {
        manualSubmissionAction = response;
        submitStatus = response.message;
      } else if (response.type === 'SUBMISSION_FAILED') {
        stopManualSubmissionPolling();
        const errorMessage = response.message || response.error;
        submitStatus = `Submission failed: ${errorMessage}`;
        await api.post(`/submissions/${submission.id}/result`, {
          status: 'FAILED',
          metadata: { error: errorMessage }
        });
        activeSubmission = { ...submission, status: 'FAILED', metadata: { error: errorMessage } };
        const idx = recentSubmissions.findIndex((item) => item.id === submission.id);
        if (idx !== -1) {
          recentSubmissions[idx] = { ...recentSubmissions[idx], status: 'FAILED', metadata: { error: errorMessage } };
          recentSubmissions = [...recentSubmissions];
        }
        manualSubmissionAction = null;
        manualSourceCode = '';
      } else {
        submitStatus = 'Codeforces did not return a submission ID yet. Keep this page open and try again after submitting.';
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'The extension did not respond';
      submitStatus = `Could not verify the Codeforces submission: ${message}. You can safely try again.`;
    } finally {
      manualSubmissionCheckInFlight = false;
      if (showCheckingState) checkingManualSubmission = false;
    }
  }

  async function checkManualSubmission() {
    await reconcileManualSubmission(true);
  }

  async function handleSubmit() {
    if (!$auth.user) {
      return;
    }
    if (!problem) return;

    if (!canUseSubmissionWorkspace) return;

    const extension = await pingExtension();
    if (extension && !isExtensionVersionCompatible(extension.version)) {
      submitStatus = 'Extension update required. Install v' + LATEST_EXTENSION_VERSION + ' before submitting.';
      return;
    }
    if (extension) {
      const platformSession = extension.platforms[problem.platform];
      if (!platformSession?.loggedIn || !platformSession.username?.trim()) {
        const platformName = problem.platform === 'CODEFORCES' ? 'Codeforces' : 'AtCoder';
        submitStatus = `Log in to ${platformName} in this browser before submitting.`;
        return;
      }
    }

    submitting = true;
    submitStatus = 'Creating submission...';
    duplicateSubmission = false;
    stopSubmissionPolling();

    let createdSub: Submission | null = null;
    let extensionDispatchStarted = false;
    let extensionDispatchCompleted = false;
    try {
      const sub = await api.post<Submission>('/submissions', {
        problemId: problem.id,
        contestId: contestId || null,
        language,
        sourceCode
      });
      createdSub = sub;
      activeSubmission = sub;
      const dispatchSource = sub.sourceCode || sourceCode;
      submitStatus = 'Dispatching via extension...';

      // The API only returns a submission for non-duplicate source. Open the
      // tab at that point, before any external dispatch is attempted.
      openSubmissionsTab();
      await loadSubmissions(problem.id, contestId);

      extensionDispatchStarted = true;
      let extRes;
      let lastError: unknown;
      // The extension persists dispatch state and deduplicates by submission
      // ID, so retries are safe even when the first request reached the
      // external platform but its response was lost.
      for (let attempt = 0; attempt < 3; attempt += 1) {
        try {
          extRes = await submitViaExtension(sub.id, problem.platform, problem.externalId, problem.url, language, dispatchSource);
          break;
        } catch (err) {
          lastError = err;
          if (attempt < 2) await new Promise((resolve) => setTimeout(resolve, 1500));
        }
      }
      if (!extRes) {
        const recovered = (await recoverPendingSubmissions()).find((dispatch) => dispatch.submissionId === sub.id);
        if (recovered?.state === 'CREATED' && recovered.externalSubmissionId) {
          extRes = {
            type: 'SUBMISSION_CREATED' as const,
            submissionId: sub.id,
            externalSubmissionId: recovered.externalSubmissionId
          };
          await acknowledgeRecoveredSubmission(sub.id);
        } else {
          throw lastError || new Error('Extension did not confirm the submission; please retry while the dispatch is recoverable.');
        }
      }

      if (extRes.type === 'SUBMISSION_CREATED' && extRes.externalSubmissionId) {
        extensionDispatchCompleted = true;
        await markSubmissionDispatched(sub, extRes.externalSubmissionId);
      } else if (extRes.type === 'SUBMISSION_ACTION_REQUIRED') {
        extensionDispatchCompleted = true;
        manualSubmissionAction = extRes;
        manualSourceCode = dispatchSource;
        manualCodeCopied = false;
        submitStatus = extRes.message;
        startManualSubmissionPolling();
      } else if (extRes.type === 'SUBMISSION_FAILED') {
        stopManualSubmissionPolling();
        manualSubmissionAction = null;
        const errorMsg = extRes.message || extRes.error || 'Submission was not accepted on platform';
        submitStatus = `Submission failed: ${errorMsg}`;
        await api.post(`/submissions/${sub.id}/result`, {
          status: 'FAILED',
          metadata: { error: errorMsg }
        });
        activeSubmission.status = 'FAILED';
        const idx = recentSubmissions.findIndex((s) => s.id === sub.id);
        if (idx !== -1) {
          recentSubmissions[idx] = { ...recentSubmissions[idx], status: 'FAILED', metadata: { error: errorMsg } };
          recentSubmissions = [...recentSubmissions];
        }
      } else {
        const errorMsg = 'Failed to obtain external submission ID';
        submitStatus = `Submission failed: ${errorMsg}`;
        await api.post(`/submissions/${sub.id}/result`, {
          status: 'FAILED',
          metadata: { error: errorMsg }
        });
        activeSubmission.status = 'FAILED';
        const idx = recentSubmissions.findIndex((s) => s.id === sub.id);
        if (idx !== -1) {
          recentSubmissions[idx] = { ...recentSubmissions[idx], status: 'FAILED', metadata: { error: errorMsg } };
          recentSubmissions = [...recentSubmissions];
        }
      }

      await loadSubmissions(problem.id, contestId);
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : String(err || 'Unknown error');
      const dispatchMayStillBeRunning = extensionDispatchStarted && !extensionDispatchCompleted;
      if (!dispatchMayStillBeRunning && isDuplicateSubmissionError(err)) {
        duplicateSubmission = true;
        submitStatus = 'This exact solution was already submitted. It was not sent to the external judge.';
      } else if (extensionDispatchCompleted) {
        const platformName = problem.platform === 'CODEFORCES' ? 'Codeforces' : 'AtCoder';
        submitStatus = `Submission reached ${platformName}, but cpbridge could not verify the handoff: ${errMsg}. Reload to retry safely.`;
      } else {
        submitStatus = dispatchMayStillBeRunning
          ? 'Dispatch is still being completed by the extension. You can safely reload this page.'
          : `Submission error: ${errMsg}`;
      }
      if (createdSub && !extensionDispatchStarted) {
        try {
          await api.post(`/submissions/${createdSub.id}/result`, {
            status: 'FAILED',
            metadata: { error: errMsg }
          });
          if (activeSubmission) activeSubmission.status = 'FAILED';
        } catch {}
      }
    } finally {
      submitting = false;
    }
  }

  onDestroy(() => {
    stopSubmissionPolling();
    stopManualSubmissionPolling();
  });
</script>

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

        <!-- Timer -->
        <div class="flex items-center gap-3 shrink-0">
          <ContestTimer startAt={contest.startAt} endAt={contest.endAt} state={contest.state} />
        </div>
      </div>
    </div>
  {/if}

  <div class={contest ? 'grid grid-cols-1 lg:grid-cols-[280px_minmax(0,1fr)] gap-4 items-start' : 'space-y-4'}>
    <main class={contest ? 'min-w-0 space-y-4 lg:col-start-2 lg:row-start-1' : 'space-y-4'}>
      {#if loading}
        <!-- Problem Section Skeleton -->
        <div class="space-y-4">
          <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/70 space-y-4 shadow-xl animate-pulse">
            <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
              <div class="space-y-2.5 flex-1">
                <div class="flex items-center space-x-2">
                  <div class="h-5 w-24 bg-zinc-800 rounded-full"></div>
                  <div class="h-5 w-16 bg-zinc-800 rounded-full"></div>
                </div>
                <div class="h-7 w-3/4 max-w-md bg-zinc-800 rounded-lg"></div>
              </div>
              <div class="h-9 w-44 bg-zinc-800 rounded-xl"></div>
            </div>
          </div>

          <div class="h-130 rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6 animate-pulse space-y-4">
            <div class="h-6 w-1/4 bg-zinc-800 rounded-lg"></div>
            <div class="space-y-2.5 pt-3">
              <div class="h-4 w-full bg-zinc-800/60 rounded"></div>
              <div class="h-4 w-11/12 bg-zinc-800/60 rounded"></div>
              <div class="h-4 w-4/5 bg-zinc-800/60 rounded"></div>
              <div class="h-4 w-2/3 bg-zinc-800/60 rounded"></div>
            </div>
            <div class="h-28 w-full bg-zinc-800/30 rounded-xl mt-6"></div>
          </div>
        </div>
      {:else if error || !problem}
        <div class="p-8 rounded-2xl border border-red-500/30 bg-red-500/10 text-red-300 space-y-2">
          <h2 class="text-xl font-bold">Error loading problem</h2>
          <p class="text-sm">{error || 'Problem not found.'}</p>
        </div>
      {:else if contest && contest.state === 'UPCOMING' && $auth.user?.role !== 'ADMIN'}
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
        <div class="sticky top-16 z-20 pt-4 bg-zinc-950 -mt-4 rounded-lg">
          <div class="p-5 rounded-2xl border border-zinc-800 bg-zinc-900/95 backdrop-blur-md space-y-4 shadow-xl">
            <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
              <div class="space-y-1.5">
                <div class="flex items-center space-x-2.5">
                  {#if currentContestProblem}
                    <span class="text-xs px-2.5 py-0.5 rounded-full font-bold font-mono bg-zinc-800 text-zinc-200 border border-zinc-700 shadow-sm">
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
                  {problem.title}
                </h1>
              </div>

              <div class="flex items-center space-x-3 shrink-0">
                <!-- Limits (Time Limit & Memory Limit text) -->
                {#if statement?.timeLimit || statement?.memoryLimit}
                  <div class="flex flex-col gap-1 text-xs font-mono text-zinc-400">
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

            <!-- Main Navigation Tabs -->
            <div class="flex items-center space-x-2 pt-2 border-t border-zinc-800/80">
              <button
                on:click={() => setActiveTab('statement')}
                class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-2 {
                  activeTab === 'statement' ? 'bg-white text-black shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
                }"
              >
                <BookOpen class="w-4 h-4" />
                <span>Problem Statement</span>
              </button>

              <button
                on:click={() => setActiveTab('editor')}
                class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-2 {
                  activeTab === 'editor' ? 'bg-white text-black shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
                }"
              >
                <Code2 class="w-4 h-4" />
                <span>Code Editor & Submit</span>
              </button>

              <button
                on:click={() => setActiveTab('submissions')}
                class="px-4 py-2 rounded-xl text-xs font-semibold transition flex items-center space-x-2 {
                  activeTab === 'submissions' ? 'bg-white text-black shadow-sm' : 'bg-zinc-800 text-zinc-400 hover:text-white'
                }"
              >
                <Cpu class="w-4 h-4" />
                <span>Submissions ({recentSubmissions.length})</span>
              </button>
            </div>
          </div>
        </div>

    <!-- Hidden File Input for Auto-Detect Upload -->
    <input
      type="file"
      bind:this={fileInputElement}
      on:change={handleFileUpload}
      accept=".cpp,.cc,.cxx,.c++,.cp,.py,.py3,.python,.java,.txt"
      class="hidden"
    />

    <!-- Layout Container -->
    <div class="rounded-2xl border border-zinc-800 bg-zinc-900/40 p-6 min-h-137.5">
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

              {#if renderedNote}
                <div class="statement-content text-sm text-zinc-300 leading-relaxed space-y-4 pt-6 border-t border-zinc-800">
                  <h3 class="text-base font-bold text-white uppercase tracking-wider">Note</h3>
                  {@html renderedNote}
                </div>
              {/if}

              <!-- Quick Jump to Code Editor -->
              <div class="pt-6 border-t border-zinc-800 flex justify-end">
                <button
                  on:click={() => setActiveTab('editor')}
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
            {#if !$auth.loading && !extensionCheckLoading && !canUseSubmissionWorkspace}
              <div class="p-5 rounded-2xl border border-amber-500/30 bg-amber-500/5 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                <div class="flex items-start gap-3 text-xs">
                  <AlertTriangle class="w-5 h-5 shrink-0 mt-0.5 text-amber-400" />
                  <div class="space-y-1.5">
                    {#if !$auth.user}
                      <p class="font-semibold text-amber-200">Sign in before using the Code Editor & Submit workspace.</p>
                      <p class="text-amber-200/70">Authentication is required before extension setup or external submissions.</p>
                    {:else if !extensionInfo}
                      <p class="font-semibold text-amber-200">Browser extension is not active.</p>
                      <p class="text-amber-200/70">Install or activate the cpbridge extension to enable editing and submissions.</p>
                    {:else if !extensionCompatible}
                      <p class="font-semibold text-amber-200">Browser extension update required.</p>
                      <p class="text-amber-200/70">Install the latest cpbridge extension before using external submissions.</p>
                    {:else}
                      <p class="font-semibold text-amber-200">{problem.platform === 'CODEFORCES' ? 'Codeforces' : 'AtCoder'} is not connected.</p>
                      <p class="text-amber-200/70">Log in to the external platform in this browser to enable editing and submissions.</p>
                    {/if}
                  </div>
                </div>
                <a
                  href="/settings/integrations"
                  class="shrink-0 px-4 py-2.5 rounded-xl text-xs font-semibold bg-amber-400 text-zinc-950 hover:bg-amber-300 transition inline-flex items-center justify-center gap-1.5"
                >
                  {#if !$auth.user}Sign in & open setup{:else}Open Extension Setup{/if}
                  <ExternalLink class="w-3.5 h-3.5" />
                </a>
              </div>
            {/if}

            <!-- Toolbar -->
            <div class="flex flex-wrap items-center justify-between gap-3 bg-zinc-950 p-4 rounded-2xl border border-zinc-800">
              <div class="flex items-center space-x-3">
                <div class="flex items-center space-x-2">
                  <label for="lang-select-tab" class="text-xs font-semibold uppercase text-zinc-400">Language:</label>
                  <select
                    id="lang-select-tab"
                    bind:value={language}
                    on:change={handleLanguageChange}
                    disabled={!canUseSubmissionWorkspace}
                    class="px-3 py-1.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-200 text-xs font-mono focus:border-zinc-400 focus:outline-none disabled:pointer-events-none disabled:opacity-50"
                  >
                    <option value="cpp23">C++23 (GCC)</option>
                    <option value="python3">Python 3</option>
                    <option value="java21">Java 21</option>
                  </select>
                </div>

                <!-- Upload File with Auto Detect -->
                <button
                  type="button"
                  on:click={() => fileInputElement.click()}
                  disabled={!canUseSubmissionWorkspace}
                  class="px-3.5 py-1.5 rounded-lg text-xs font-semibold bg-zinc-800 hover:bg-zinc-700 text-zinc-200 border border-zinc-700 transition flex items-center space-x-1.5 shadow-sm disabled:pointer-events-none disabled:opacity-50"
                  title="Upload code file (.cpp, .py, .java) to automatically set source code and detect language"
                >
                  <Upload class="w-3.5 h-3.5 text-white" />
                  <span>Upload File (Auto-Detect)</span>
                </button>
              </div>

              <div class="flex items-center space-x-3">
                <button
                  on:click={handleSubmit}
                  disabled={submitting || !canUseSubmissionWorkspace}
                  title={!canUseSubmissionWorkspace ? 'Activate the extension and connect the external platform first' : 'Submit solution'}
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

            {#if duplicateSubmission}
              <div class="p-4 rounded-2xl border border-red-500/40 bg-red-500/10 text-red-200 flex items-start space-x-2.5">
                <XCircle class="w-4 h-4 shrink-0 mt-0.5 text-red-400" />
                <p class="text-xs font-semibold">{submitStatus}</p>
              </div>
            {/if}

            <!-- Monaco Editor -->
            <div class="h-137.5 rounded-2xl overflow-hidden border border-zinc-800">
              <MonacoEditor bind:value={sourceCode} {language} readonly={!canUseSubmissionWorkspace} disabled={!canUseSubmissionWorkspace} />
            </div>

            <!-- Verdict & Dispatch Banner -->
            {#if (activeSubmission && !duplicateSubmission) || (submitStatus && !duplicateSubmission)}
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
                {#if manualSubmissionAction}
                  <ManualSubmissionActions
                    action={manualSubmissionAction}
                    checking={checkingManualSubmission}
                    copied={manualCodeCopied}
                    onCheck={checkManualSubmission}
                    onOpen={openManualSubmitPage}
                    onCopy={copyManualSource}
                  />
                {/if}
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
                  on:click={() => setActiveTab('editor')}
                  class="px-3 py-1 text-xs rounded-lg bg-white text-black font-semibold hover:bg-zinc-200 transition flex items-center space-x-1"
                >
                  <Code2 class="w-3.5 h-3.5" />
                  <span>Editor</span>
                </button>
              </div>
            </div>

            {#if submissionsLoading}
              <div class="space-y-2.5 py-2 animate-pulse">
                <div class="h-20 rounded-xl border border-zinc-800 bg-zinc-950/70"></div>
                <div class="h-20 rounded-xl border border-zinc-800 bg-zinc-950/70"></div>
              </div>
            {:else if recentSubmissions.length === 0}
              <p class="text-xs text-zinc-500 py-12 text-center">No submissions recorded yet for this problem.</p>
            {:else}
              <div class="space-y-2.5">
                {#each paginatedRecentSubmissions as sub}
                  <button
                    type="button"
                    on:click={() => (viewingSubmission = sub)}
                    class="w-full text-left p-4 rounded-xl border border-zinc-800 bg-zinc-950 hover:bg-zinc-900/90 hover:border-zinc-700 focus:outline-none focus:ring-1 focus:ring-zinc-600 transition cursor-pointer group space-y-2"
                  >
                    <div class="flex items-center justify-between text-xs gap-3">
                      <div class="space-y-1.5 min-w-0">
                        <div class="flex flex-wrap items-center gap-2">
                          <span class="font-mono font-bold text-zinc-100 group-hover:text-white transition">
                            {formatLanguageName(sub.language)}
                          </span>
                          <span class="text-zinc-600">•</span>
                          <span class="font-mono text-[11px] text-zinc-400 bg-zinc-900 px-2 py-0.5 rounded border border-zinc-800/80 group-hover:border-zinc-700 transition">
                            {sub.id}
                          </span>
                          {#if sub.externalSubmissionId && sub.externalSubmissionId !== sub.id}
                            <span class="font-mono text-[11px] text-zinc-500">
                              (ext: {sub.externalSubmissionId})
                            </span>
                          {/if}
                        </div>
                        <div class="text-zinc-500 text-[11px] font-sans">{new Date(sub.submittedAt).toLocaleString()}</div>
                        {#if sub.metadata && sub.metadata.error}
                          <div class="text-rose-400 text-[11px] font-sans truncate max-w-md">{sub.metadata.error}</div>
                        {/if}
                      </div>

                      <div class="flex items-center space-x-2.5 shrink-0">
                        <span class="font-bold font-mono px-3 py-1.5 rounded-lg text-xs {
                          sub.status === 'ACCEPTED' ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30' :
                          sub.status === 'WRONG_ANSWER' ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30' :
                          sub.status === 'JUDGING' || sub.status === 'PENDING' || sub.status === 'DISPATCHING' ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30 animate-pulse' :
                          'bg-zinc-800 text-zinc-400 border border-zinc-700'
                        }">
                          {sub.status}
                        </span>
                        <Code2 class="w-4 h-4 text-zinc-600 group-hover:text-zinc-300 transition" />
                      </div>
                    </div>
                  </button>
                {/each}
              </div>

              {#if recentSubmissions.length > submissionsPageSize}
                <Pagination
                  currentPage={submissionsPage}
                  pageSize={submissionsPageSize}
                  totalItems={recentSubmissions.length}
                  pageSizeOptions={[5, 10, 20]}
                  itemName="submissions"
                  on:pageChange={(e) => (submissionsPage = e.detail)}
                  on:pageSizeChange={(e) => {
                    submissionsPageSize = e.detail;
                    submissionsPage = 1;
                  }}
                />
              {/if}
            {/if}
          </div>
        {/if}
      </div>

    {/if}
      </main>

      {#if contest && contestProblems.length > 0}
        <aside class="lg:col-start-1 lg:row-start-1 lg:sticky lg:top-20 self-start p-4 rounded-2xl border border-zinc-800 bg-zinc-900/70 shadow-xl space-y-3">
          <div class="flex items-center justify-between text-xs text-zinc-400">
            <div class="flex items-center space-x-1.5 font-semibold uppercase tracking-wider text-[11px]">
              <Layers class="w-3.5 h-3.5" />
              <span>Contest Problems</span>
            </div>
            {#if contestSolvedProblemIds.size > 0}
              <span class="text-emerald-400 font-medium">
                {contestSolvedProblemIds.size}/{contestProblems.length}
              </span>
            {/if}
          </div>

          <div class="max-h-[calc(100vh-140px)] overflow-y-auto space-y-2 pr-1">
            {#each contestProblems as cp}
              {@const isActive = cp.problemId === problemId}
              {@const isSolved = contestSolvedProblemIds.has(cp.problemId)}
              {@const isWrong = !isSolved && contestWrongProblemIds.has(cp.problemId)}
              <a
                href={`/problems/${cp.problemId}?contestId=${contest.id}`}
                class="w-full flex items-center space-x-2.5 px-3 py-2.5 rounded-xl text-xs font-semibold transition border {
                  isActive
                    ? isSolved
                      ? 'bg-emerald-500/15 text-emerald-200 border-emerald-400 ring-2 ring-emerald-500/30 shadow-sm'
                      : isWrong
                      ? 'bg-rose-500/15 text-rose-200 border-rose-400 ring-2 ring-rose-500/30 shadow-sm'
                      : 'bg-zinc-800/80 text-white border-zinc-400 ring-2 ring-zinc-500/30 shadow-sm'
                    : isSolved
                    ? 'bg-emerald-500/10 text-emerald-300 border-emerald-500/30 hover:bg-emerald-500/20 hover:text-white'
                    : isWrong
                    ? 'bg-rose-500/10 text-rose-300 border-rose-500/30 hover:bg-rose-500/20 hover:text-white'
                    : 'bg-zinc-950 text-zinc-300 border-zinc-800 hover:border-zinc-700 hover:bg-zinc-900 hover:text-white'
                }"
              >
                <span class="font-mono font-bold w-6 h-6 rounded-lg flex items-center justify-center text-xs shrink-0 {
                  isSolved
                    ? isActive
                      ? 'bg-emerald-500/30 text-emerald-200 border border-emerald-400'
                      : 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                    : isWrong
                    ? isActive
                      ? 'bg-rose-500/30 text-rose-200 border border-rose-400'
                      : 'bg-rose-500/20 text-rose-300 border border-rose-500/30'
                    : isActive
                    ? 'bg-zinc-700 text-white border border-zinc-500'
                    : 'bg-zinc-900 text-zinc-400 border border-zinc-800'
                }">
                  {cp.label}
                </span>

                <span class="min-w-0 flex-1 truncate">
                  {cp.problem?.title || `Problem ${cp.label}`}
                </span>

                {#if isSolved}
                  <CheckCircle2 class="w-3.5 h-3.5 shrink-0 text-emerald-400" />
                {:else if isWrong}
                  <XCircle class="w-3.5 h-3.5 shrink-0 text-rose-400" />
                {/if}
              </a>
            {/each}
          </div>
        </aside>
      {/if}
    </div>
  </div>

<SubmissionModal
  submission={viewingSubmission}
  open={!!viewingSubmission}
  onClose={() => (viewingSubmission = null)}
/>

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
