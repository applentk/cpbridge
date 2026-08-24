import type {
  ExtensionMessage,
  ExtensionPingResponse,
  ExtensionRecoverSubmissionsResponse,
  ExtensionRecoveredSubmission,
  ExtensionSubmissionCreatedResponse,
  ExtensionSubmissionFailedResponse,
  ExtensionSubmitRequest,
  ExtensionStatusPollResponse
} from '@cpbridge/contracts';
import { checkCodeforcesSession, submitCodeforces, pollCodeforcesStatus } from './platforms/codeforces.js';
import { checkAtCoderSession, submitAtCoder, pollAtCoderStatus } from './platforms/atcoder.js';

const DISPATCH_STORAGE_PREFIX = 'cp_hub_dispatch:';

type StoredDispatch = ExtensionRecoveredSubmission;

const inFlightSubmissions = new Map<string, Promise<ExtensionSubmissionCreatedResponse | ExtensionSubmissionFailedResponse>>();

function dispatchStorageKey(submissionId: string): string {
  return `${DISPATCH_STORAGE_PREFIX}${submissionId}`;
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

async function dispatchSubmission(message: ExtensionSubmitRequest): Promise<ExtensionSubmissionCreatedResponse | ExtensionSubmissionFailedResponse> {
  await storeDispatch({ submissionId: message.submissionId, state: 'DISPATCHING' });

  try {
    let externalSubmissionId = '';

    if (message.platform === 'CODEFORCES') {
      const parts = message.problem.externalId.split('/');
      if (parts.length !== 2) throw new Error('Invalid Codeforces externalId');
      const res = await submitCodeforces(parts[0], parts[1], message.language, message.source);
      externalSubmissionId = res.externalSubmissionId;
    } else if (message.platform === 'ATCODER') {
      const parts = message.problem.externalId.split('/');
      if (parts.length !== 2) throw new Error('Invalid AtCoder externalId');
      const res = await submitAtCoder(parts[0], parts[1], message.language, message.source);
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
  } catch (err: any) {
    const messageText = err.message || 'Unknown extension error';
    const errResp: ExtensionSubmissionFailedResponse = {
      type: 'SUBMISSION_FAILED',
      submissionId: message.submissionId,
      error: err.message === 'NOT_LOGGED_IN' ? 'NOT_LOGGED_IN' : 'SUBMISSION_FAILED',
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

chrome.runtime.onMessage.addListener((message: ExtensionMessage, sender, sendResponse) => {
  handleMessage(message)
    .then(sendResponse)
    .catch((err) => {
      sendResponse({
        type: 'SUBMISSION_FAILED',
        submissionId: (message as any).submissionId || '',
        error: 'UNKNOWN',
        message: err.message || 'Unknown extension error'
      });
    });
  return true; // keep channel open for async response
});

async function handleMessage(message: ExtensionMessage): Promise<any> {
  if (message.type === 'PING') {
    const [cf, ac] = await Promise.all([
      checkCodeforcesSession(),
      checkAtCoderSession()
    ]);

    const res: ExtensionPingResponse = {
      type: 'PONG',
      version: '1.0.0',
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

    const dispatchPromise = dispatchSubmission(message);
    inFlightSubmissions.set(message.submissionId, dispatchPromise);
    try {
      return await dispatchPromise;
    } finally {
      inFlightSubmissions.delete(message.submissionId);
    }
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
        const parts = message.problem.externalId.split('/');
        const contestId = parts[0] || '';
        const res = await pollCodeforcesStatus(contestId, message.externalSubmissionId);
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
      status: resultStatus as any
    };
    return pollResp;
  }

  return { error: 'Unknown message type' };
}
