import type {
  ExtensionMessage,
  ExtensionPingResponse,
  ExtensionSubmissionCreatedResponse,
  ExtensionSubmissionFailedResponse,
  ExtensionStatusPollResponse
} from '@cp-hub/contracts';
import { checkCodeforcesSession, submitCodeforces, pollCodeforcesStatus } from './platforms/codeforces.js';
import { checkAtCoderSession, submitAtCoder, pollAtCoderStatus } from './platforms/atcoder.js';

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
      return successResp;
    } catch (err: any) {
      const errResp: ExtensionSubmissionFailedResponse = {
        type: 'SUBMISSION_FAILED',
        submissionId: message.submissionId,
        error: err.message === 'NOT_LOGGED_IN' ? 'NOT_LOGGED_IN' : 'SUBMISSION_FAILED',
        message: err.message
      };
      return errResp;
    }
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
