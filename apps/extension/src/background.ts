import type {
  ExtensionMessage,
  ExtensionPingResponse,
  ExtensionRecoverSubmissionsResponse,
  ExtensionRecoveredSubmission,
  ExtensionSubmissionActionRequiredResponse,
  ExtensionSubmissionCreatedResponse,
  ExtensionSubmissionFailedResponse,
  ExtensionSubmitRequest,
  ExtensionStatusPollResponse,
  LanguageId
} from '@cpbridge/contracts';
import {
  checkCodeforcesSession,
  detectManualCodeforcesSubmission,
  parseCodeforcesExternalId,
  snapshotCodeforcesSubmissionIds,
  pollCodeforcesStatus
} from './platforms/codeforces.js';
import { checkAtCoderSession, submitAtCoder, pollAtCoderStatus } from './platforms/atcoder.js';
import { activateTab } from './tab-utils.js';

const DISPATCH_STORAGE_PREFIX = 'cpbridge_dispatch:';
const MANUAL_STORAGE_PREFIX = 'cpbridge_manual:';
const MANUAL_SOURCE_STORAGE_PREFIX = 'cpbridge_manual_source:';
const PRODUCTION_WEB_ORIGIN = 'https://cpbridge.applentk.com';
declare const __CPBRIDGE_DEV__: boolean;

function isAllowedOrigin(origin: string): boolean {
  if (!origin) return false;
  try {
    const url = new URL(origin);
    const host = url.hostname;
    const protocol = url.protocol;

    if (protocol === 'https:' && url.origin === PRODUCTION_WEB_ORIGIN) {
      return true;
    }

    if (__CPBRIDGE_DEV__ && protocol === 'http:' && (host === 'localhost' || host === '127.0.0.1')) {
      return true;
    }
  } catch {
    return false;
  }
  return false;
}

function isTrustedSender(sender: chrome.runtime.MessageSender): boolean {
  const pageURL = sender.tab?.url;
  if (!pageURL) return false;
  try {
    return isAllowedOrigin(new URL(pageURL).origin);
  } catch {
    return false;
  }
}

function isCodeforcesSender(sender: chrome.runtime.MessageSender): boolean {
  const pageURL = sender.tab?.url;
  if (!pageURL) return false;
  try {
    return new URL(pageURL).origin === 'https://codeforces.com';
  } catch {
    return false;
  }
}

type StoredDispatch = ExtensionRecoveredSubmission;
type DispatchResponse = ExtensionSubmissionCreatedResponse | ExtensionSubmissionFailedResponse | ExtensionSubmissionActionRequiredResponse;

interface ManualCodeforcesSubmission {
  submissionId: string;
  contestId: string;
  problemIndex: string;
  language: LanguageId;
  knownSubmissionIds?: string[];
  submitUrl: string;
  message: string;
  createdAt: number;
  tabId?: number;
  sourceTabId?: number;
}

interface CodeforcesPrefillRequest {
  type: 'GET_CODEFORCES_PREFILL';
  submissionId: string;
}

interface CodeforcesSubmissionSubmittedRequest {
  type: 'CODEFORCES_SUBMISSION_SUBMITTED';
  submissionId: string;
}

const inFlightSubmissions = new Map<string, Promise<DispatchResponse>>();
const submittedCodeforcesSubmissions = new Set<string>();
const interactiveCodeforcesTabs = new Map<number, string>();

async function injectBridgeIntoOpenPages(): Promise<void> {
  try {
    const tabs = await chrome.tabs.query({});
    await Promise.all(tabs.map(async (tab) => {
      if (tab.id === undefined || !tab.url) return;

      let origin: string;
      try {
        origin = new URL(tab.url).origin;
      } catch {
        return;
      }
      if (!isAllowedOrigin(origin)) return;

      try {
        await chrome.scripting.executeScript({
          target: { tabId: tab.id },
          files: ['dist/bridge.js']
        });
      } catch {
        // The tab may navigate or become unavailable while the extension is
        // being installed. Normal page navigation will inject the script later.
      }
    }));
  } catch {
    // Chrome may suspend the service worker before the tab query completes.
  }
}

// Cover extension reloads as well as first installs. This makes an already
// open settings tab reconnect without requiring a full page reload.
void injectBridgeIntoOpenPages();

chrome.runtime.onInstalled.addListener(() => {
  void injectBridgeIntoOpenPages();
});

function dispatchStorageKey(submissionId: string): string {
  return `${DISPATCH_STORAGE_PREFIX}${submissionId}`;
}

function manualStorageKey(submissionId: string): string {
  return `${MANUAL_STORAGE_PREFIX}${submissionId}`;
}

function manualSourceStorageKey(submissionId: string): string {
  return `${MANUAL_SOURCE_STORAGE_PREFIX}${submissionId}`;
}

async function storeDispatch(dispatch: StoredDispatch): Promise<void> {
  try {
    await chrome.storage.local.set({ [dispatchStorageKey(dispatch.submissionId)]: dispatch });
  } catch (err) {
    console.warn('[cpbridge Extension] Could not persist submission dispatch:', err);
  }
}

async function readStoredDispatches(): Promise<StoredDispatch[]> {
  try {
    const stored = await chrome.storage.local.get(null);
    return Object.entries(stored)
      .filter(([key]) => key.startsWith(DISPATCH_STORAGE_PREFIX))
      .map(([, value]) => value as StoredDispatch)
      .filter((value) => !!value?.submissionId && !!value?.state);
  } catch (err) {
    console.warn('[cpbridge Extension] Could not read stored submission dispatches:', err);
    return [];
  }
}

async function acknowledgeDispatch(submissionId: string): Promise<boolean> {
  try {
    await chrome.storage.local.remove(dispatchStorageKey(submissionId));
    return true;
  } catch (err) {
    console.warn('[cpbridge Extension] Could not acknowledge submission dispatch:', err);
    return false;
  }
}

async function storeManualSubmission(pending: ManualCodeforcesSubmission, source: string): Promise<void> {
  await chrome.storage.local.set({ [manualStorageKey(pending.submissionId)]: pending });
  await chrome.storage.session.set({ [manualSourceStorageKey(pending.submissionId)]: source });
}

async function storeManualMetadata(pending: ManualCodeforcesSubmission): Promise<void> {
  await chrome.storage.local.set({ [manualStorageKey(pending.submissionId)]: pending });
}

async function readManualSubmission(submissionId: string): Promise<ManualCodeforcesSubmission | undefined> {
  const stored = await chrome.storage.local.get(manualStorageKey(submissionId));
  return stored[manualStorageKey(submissionId)] as ManualCodeforcesSubmission | undefined;
}

async function clearManualSubmission(submissionId: string): Promise<void> {
  await Promise.all([
    chrome.storage.local.remove(manualStorageKey(submissionId)),
    chrome.storage.session.remove(manualSourceStorageKey(submissionId))
  ]);
}

function actionRequiredResponse(pending: ManualCodeforcesSubmission): ExtensionSubmissionActionRequiredResponse {
  return {
    type: 'SUBMISSION_ACTION_REQUIRED',
    submissionId: pending.submissionId,
    platform: 'CODEFORCES',
    action: 'CONFIRM_SUBMISSION',
    submitUrl: pending.submitUrl,
    message: pending.message
  };
}

async function beginInteractiveCodeforcesSubmission(
  message: ExtensionSubmitRequest,
  contestId: string,
  problemIndex: string,
  knownSubmissionIds: string[] | undefined,
  sourceTabId?: number
): Promise<ExtensionSubmissionActionRequiredResponse> {
  const submitPath = contestId.startsWith('gym/') ? `${contestId}/submit` : 'problemset/submit';
  const submitUrl = `https://codeforces.com/${submitPath}#cpbridge=${encodeURIComponent(message.submissionId)}`;
  const actionMessage = 'Review the prefilled Codeforces form, complete verification if prompted, and click Submit. cpbridge will close the tab after it detects your submission.';
  const pending: ManualCodeforcesSubmission = {
    submissionId: message.submissionId,
    contestId,
    problemIndex,
    language: message.language,
    knownSubmissionIds,
    submitUrl,
    message: actionMessage,
    createdAt: Date.now(),
    sourceTabId
  };
  await storeManualSubmission(pending, message.source);
  await storeDispatch({
    submissionId: message.submissionId,
    state: 'AWAITING_USER_ACTION',
    actionUrl: submitUrl,
    actionMessage
  });
  try {
    const tab = await chrome.tabs.create({ url: submitUrl, active: true });
    if (tab.id !== undefined) {
      pending.tabId = tab.id;
      interactiveCodeforcesTabs.set(tab.id, message.submissionId);
      await storeManualMetadata(pending);
    }
  } catch (err) {
    console.warn('[cpbridge Extension] Could not open interactive Codeforces tab:', err);
  }
  return actionRequiredResponse(pending);
}

async function completeManualCodeforcesSubmission(submissionId: string): Promise<DispatchResponse> {
  const stored = (await readStoredDispatches()).find((dispatch) => dispatch.submissionId === submissionId);
  if (stored?.state === 'CREATED' && stored.externalSubmissionId) {
    return { type: 'SUBMISSION_CREATED', submissionId, externalSubmissionId: stored.externalSubmissionId };
  }
  if (stored?.state === 'FAILED') {
    return {
      type: 'SUBMISSION_FAILED',
      submissionId,
      error: 'SUBMISSION_FAILED',
      message: stored.error || 'The Codeforces submission tab was closed before the solution was submitted.'
    };
  }

  const pending = await readManualSubmission(submissionId);
  if (!pending) {
    return {
      type: 'SUBMISSION_FAILED',
      submissionId,
      error: 'SUBMISSION_FAILED',
      message: 'The interactive Codeforces handoff expired. Start the submission again from cpbridge.'
    };
  }

  if (!pending.knownSubmissionIds) {
    pending.message = 'Codeforces verification is still preparing the safe submission check. Finish the browser verification and wait for the form to be prefilled before submitting.';
    await storeManualMetadata(pending);
    await storeDispatch({
      submissionId,
      state: 'AWAITING_USER_ACTION',
      actionUrl: pending.submitUrl,
      actionMessage: pending.message
    });
    return actionRequiredResponse(pending);
  }

  const externalSubmissionId = await detectManualCodeforcesSubmission(
    pending.contestId,
    pending.problemIndex,
    pending.knownSubmissionIds
  );
  if (!externalSubmissionId) {
    pending.message = 'No new Codeforces submission was found yet. Submit in the opened Codeforces tab, then click “I submitted — check now” again.';
    await storeDispatch({
      submissionId,
      state: 'AWAITING_USER_ACTION',
      actionUrl: pending.submitUrl,
      actionMessage: pending.message
    });
    return actionRequiredResponse(pending);
  }

  await storeDispatch({ submissionId, state: 'CREATED', externalSubmissionId });
  await clearManualSubmission(submissionId);
  return { type: 'SUBMISSION_CREATED', submissionId, externalSubmissionId };
}

async function completeSubmittedCodeforcesTab(submissionId: string, tabId?: number): Promise<DispatchResponse> {
  // The submit form can navigate before Codeforces publishes the new row in
  // /my. Keep polling for a short window so the tab closes only after the
  // external ID is safely attached to this cpbridge submission.
  const sourceTabId = (await readManualSubmission(submissionId))?.sourceTabId;
  let result: DispatchResponse = await completeManualCodeforcesSubmission(submissionId);
  for (let attempt = 0; attempt < 12 && result.type === 'SUBMISSION_ACTION_REQUIRED'; attempt += 1) {
    await new Promise((resolve) => setTimeout(resolve, 1000));
    result = await completeManualCodeforcesSubmission(submissionId);
  }

  if (result.type === 'SUBMISSION_CREATED' && tabId !== undefined) {
    await chrome.tabs.remove(tabId).catch(() => undefined);
    await activateTab(sourceTabId);
  }
  return result;
}

async function prepareCodeforcesPrefill(submissionId: string): Promise<ManualCodeforcesSubmission | undefined> {
  const pending = await readManualSubmission(submissionId);
  if (!pending || pending.knownSubmissionIds) return pending;

  const knownSubmissionIds = await snapshotCodeforcesSubmissionIds(pending.contestId, pending.problemIndex);
  if (!knownSubmissionIds) return undefined;

  pending.knownSubmissionIds = knownSubmissionIds;
  await storeManualMetadata(pending);
  return pending;
}

async function dispatchSubmission(message: ExtensionSubmitRequest, sourceTabId?: number): Promise<DispatchResponse> {
  await storeDispatch({ submissionId: message.submissionId, state: 'DISPATCHING' });

  try {
    let externalSubmissionId = '';

    if (message.platform === 'CODEFORCES') {
      const problemRef = parseCodeforcesExternalId(message.problem.externalId);
      if (!problemRef) throw new Error('Invalid Codeforces externalId');
      // Always hand Codeforces submissions to the visible official form. A
      // background POST can create the submission before an anti-bot response
      // is rendered, which makes cpbridge report JUDGING before the user has
      // reviewed or submitted the form.
      const knownSubmissionIds = await snapshotCodeforcesSubmissionIds(
        problemRef.contestId,
        problemRef.problemIndex
      );
      return beginInteractiveCodeforcesSubmission(
        message,
        problemRef.contestId,
        problemRef.problemIndex,
        knownSubmissionIds,
        sourceTabId
      );
    } else if (message.platform === 'ATCODER') {
      const parts = message.problem.externalId.split('/');
      if (parts.length !== 2) throw new Error('Invalid AtCoder externalId');
      const res = await submitAtCoder(parts[0], parts[1], message.language, message.source, sourceTabId);
      externalSubmissionId = res.externalSubmissionId;
    } else {
      throw new Error('Unsupported platform');
    }

    const successResp: ExtensionSubmissionCreatedResponse = {
      type: 'SUBMISSION_CREATED',
      submissionId: message.submissionId,
      externalSubmissionId
    };
    await storeDispatch({
      submissionId: message.submissionId,
      state: 'CREATED',
      externalSubmissionId
    });
    return successResp;
  } catch (err) {
    const messageText = err instanceof Error ? err.message : 'Unknown extension error';
    const errResp: ExtensionSubmissionFailedResponse = {
      type: 'SUBMISSION_FAILED',
      submissionId: message.submissionId,
      error: messageText === 'NOT_LOGGED_IN' ? 'NOT_LOGGED_IN' : 'SUBMISSION_FAILED',
      message: messageText
    };
    await storeDispatch({
      submissionId: message.submissionId,
      state: 'FAILED',
      error: messageText
    });
    return errResp;
  }
}

chrome.runtime.onMessage.addListener((message: ExtensionMessage | CodeforcesPrefillRequest | CodeforcesSubmissionSubmittedRequest, sender, sendResponse) => {
  if (message.type === 'CODEFORCES_SUBMISSION_SUBMITTED') {
    if (!isCodeforcesSender(sender)) {
      sendResponse({ type: 'SUBMISSION_FAILED', submissionId: message.submissionId, error: 'PLATFORM_UNAVAILABLE', message: 'Untrusted Codeforces page' });
      return false;
    }
    submittedCodeforcesSubmissions.add(message.submissionId);
    if (sender.tab?.id !== undefined) interactiveCodeforcesTabs.delete(sender.tab.id);
    void completeSubmittedCodeforcesTab(message.submissionId, sender.tab?.id)
      .then(sendResponse)
      .catch((err) => sendResponse({
        type: 'SUBMISSION_FAILED',
        submissionId: message.submissionId,
        error: 'SUBMISSION_FAILED',
        message: err instanceof Error ? err.message : 'Could not detect the Codeforces submission'
      }))
      .finally(() => submittedCodeforcesSubmissions.delete(message.submissionId));
    return true;
  }

  if (message.type === 'GET_CODEFORCES_PREFILL') {
    if (!isCodeforcesSender(sender)) {
      sendResponse({ type: 'CODEFORCES_PREFILL_RESULT', error: 'Untrusted Codeforces page' });
      return false;
    }
    Promise.all([
      prepareCodeforcesPrefill(message.submissionId),
      chrome.storage.session.get(manualSourceStorageKey(message.submissionId))
    ]).then(([pending, sourceStore]) => {
      sendResponse({
        type: 'CODEFORCES_PREFILL_RESULT',
        pending,
        source: sourceStore[manualSourceStorageKey(message.submissionId)] as string | undefined
      });
    }).catch((err) => {
      sendResponse({
        type: 'CODEFORCES_PREFILL_RESULT',
        error: err instanceof Error ? err.message : 'Could not prepare the pending submission'
      });
    });
    return true;
  }

  if (!isTrustedSender(sender)) {
    sendResponse({
      type: 'SUBMISSION_FAILED',
      submissionId: 'submissionId' in message ? String(message.submissionId || '') : '',
      error: 'PLATFORM_UNAVAILABLE',
      message: 'Untrusted page origin'
    });
    return false;
  }
  handleMessage(message, sender.tab?.id)
    .then(sendResponse)
    .catch((err) => {
      const submissionId = 'submissionId' in message ? String(message.submissionId || '') : '';
      sendResponse({
        type: 'SUBMISSION_FAILED',
        submissionId,
        error: 'UNKNOWN',
        message: err instanceof Error ? err.message : 'Unknown extension error'
      });
    });
  return true; // keep channel open for async response
});

chrome.tabs.onRemoved.addListener((tabId) => {
  const submissionId = interactiveCodeforcesTabs.get(tabId);
  interactiveCodeforcesTabs.delete(tabId);
  if (!submissionId || submittedCodeforcesSubmissions.has(submissionId)) return;

  void (async () => {
    const stored = (await readStoredDispatches()).find((dispatch) => dispatch.submissionId === submissionId);
    if (stored?.state !== 'AWAITING_USER_ACTION') return;

    const message = 'The Codeforces tab was closed before the solution was submitted.';
    await storeDispatch({ submissionId, state: 'FAILED', error: message });
    await clearManualSubmission(submissionId);
  })().catch((err) => {
    console.warn('[cpbridge Extension] Could not mark the closed Codeforces handoff as failed:', err);
  });
});

async function handleMessage(message: ExtensionMessage, sourceTabId?: number): Promise<unknown> {
  if (message.type === 'PING') {
    const [cf, ac] = await Promise.all([
      checkCodeforcesSession(),
      checkAtCoderSession()
    ]);

    const res: ExtensionPingResponse = {
      type: 'PONG',
      version: chrome.runtime.getManifest().version,
      platforms: {
        CODEFORCES: cf,
        ATCODER: ac
      }
    };
    return res;
  }

  if (message.type === 'SUBMIT') {
    const existing = inFlightSubmissions.get(message.submissionId);
    if (existing) return existing;

    // A page can time out while the platform request is still completing.
    // Replaying the same request must return the persisted result instead of
    // submitting the source a second time.
    const stored = (await readStoredDispatches()).find(
      (dispatch) => dispatch.submissionId === message.submissionId
    );
    if (stored?.state === 'CREATED' && stored.externalSubmissionId) {
      return {
        type: 'SUBMISSION_CREATED',
        submissionId: message.submissionId,
        externalSubmissionId: stored.externalSubmissionId
      } satisfies ExtensionSubmissionCreatedResponse;
    }
    if (stored?.state === 'AWAITING_USER_ACTION') {
      const pending = await readManualSubmission(message.submissionId);
      if (pending) return actionRequiredResponse(pending);
    }

    const dispatchPromise = dispatchSubmission(message, sourceTabId);
    inFlightSubmissions.set(message.submissionId, dispatchPromise);
    try {
      return await dispatchPromise;
    } finally {
      inFlightSubmissions.delete(message.submissionId);
    }
  }

  if (message.type === 'COMPLETE_MANUAL_SUBMISSION') {
    return completeManualCodeforcesSubmission(message.submissionId);
  }

  if (message.type === 'RECOVER_SUBMISSIONS') {
    // If the original page was reloaded while the platform request was still
    // running, wait briefly for that same operation to finish. This avoids
    // asking the platform to submit the source a second time.
    const stored = await readStoredDispatches();
    await Promise.all(stored
      .filter((dispatch) => dispatch.state === 'DISPATCHING')
      .map(async (dispatch) => {
        const inFlight = inFlightSubmissions.get(dispatch.submissionId);
        if (!inFlight) return;
        await Promise.race([
          inFlight,
          new Promise((resolve) => setTimeout(resolve, 15000))
        ]);
      }));

    const response: ExtensionRecoverSubmissionsResponse = {
      type: 'RECOVER_SUBMISSIONS_RESULT',
      submissions: await readStoredDispatches()
    };
    return response;
  }

  if (message.type === 'ACK_SUBMISSION') {
    const acknowledged = await acknowledgeDispatch(message.submissionId);
    return {
      type: 'ACK_SUBMISSION_RESULT',
      submissionId: message.submissionId,
      acknowledged
    };
  }

  if (message.type === 'POLL_STATUS') {
    let resultStatus = 'JUDGING';
    try {
      if (message.platform === 'CODEFORCES') {
        const problemRef = parseCodeforcesExternalId(message.problem.externalId);
        if (!problemRef) throw new Error('Invalid Codeforces externalId');
        const res = await pollCodeforcesStatus(problemRef.contestId, message.externalSubmissionId);
        resultStatus = res.status;
      } else if (message.platform === 'ATCODER') {
        const parts = message.problem.externalId.split('/');
        const contestId = parts[0] || '';
        const res = await pollAtCoderStatus(contestId, message.externalSubmissionId);
        resultStatus = res.status;
      }
    } catch (err) {
      console.error('Error polling status in extension:', err);
    }

    const pollResp: ExtensionStatusPollResponse = {
      type: 'POLL_STATUS_RESULT',
      externalSubmissionId: message.externalSubmissionId,
      status: resultStatus as ExtensionStatusPollResponse['status']
    };
    return pollResp;
  }

  return { error: 'Unknown message type' };
}
