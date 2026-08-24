import { browser } from '$app/environment';
import { api } from '$lib/api/client';
import {
  acknowledgeRecoveredSubmission,
  recoverPendingSubmissions
} from '$lib/extension/bridge';

let reconciliation: Promise<void> | null = null;

function recoveredFailureMessage(error: unknown): string {
  const message = typeof error === 'string'
    ? error.replace(/(?:&nbsp;?|&#160;?|&#x0*a0;?)(?=\s|$|<)/gi, ' ').replace(/\s+/g, ' ').trim()
    : '';
  // Older extension builds could persist only `Codeforces:`, which would be
  // replayed after every page reload by the recovery flow. Builds that parsed
  // Codeforces' empty error element as `&nbsp;` need the same fallback.
  if (!message || /^Codeforces:\s*$/i.test(message)) {
    return 'Codeforces submission failed before a submission ID was returned. Check the Codeforces submissions page.';
  }
  return message;
}

/**
 * Completes the API handoff if a page reload interrupted the original
 * extension response. The extension keeps the result until this succeeds.
 */
export function reconcileExtensionSubmissions(): Promise<void> {
  if (!browser) return Promise.resolve();
  if (reconciliation) return reconciliation;

  reconciliation = (async () => {
    const recovered = await recoverPendingSubmissions();

    await Promise.all(recovered.map(async (dispatch) => {
      try {
        if (dispatch.state === 'CREATED' && dispatch.externalSubmissionId) {
          await api.post(`/submissions/${dispatch.submissionId}/dispatched`, {
            externalSubmissionId: dispatch.externalSubmissionId
          });
        } else if (dispatch.state === 'FAILED') {
          await api.post(`/submissions/${dispatch.submissionId}/result`, {
            status: 'FAILED',
            metadata: { error: recoveredFailureMessage(dispatch.error) }
          });
        } else {
          return;
        }

        await acknowledgeRecoveredSubmission(dispatch.submissionId);
      } catch {
        // Keep the extension record when the API is unavailable or the user
        // is not authenticated yet; a later page load can retry it.
      }
    }));
  })().catch((err) => {
    console.warn('[CP Hub Web] Submission recovery failed:', err);
  });

  return reconciliation;
}
