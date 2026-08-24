import type {
  ExtensionErrorCode,
  ExtensionMessage,
  ExtensionPingResponse,
  ExtensionRecoverSubmissionsResponse,
  ExtensionSubmissionCreatedResponse,
  ExtensionSubmissionFailedResponse,
  LanguageId,
  PlatformType,
} from '@cpbridge/contracts';

const EXTENSION_ORIGIN = 'CP_HUB_EXTENSION';
const WEB_APP_ORIGIN = 'CP_HUB_WEB';

export interface ExtensionBridgeStatus {
  isInstalled: boolean;
  version?: string;
  platforms: Record<PlatformType, { loggedIn: boolean; username?: string }>;
}

const pendingCallbacks = new Map<string, (response: any) => void>();

if (typeof window !== 'undefined') {
  window.addEventListener('message', (event) => {
    if (event.source !== window || !event.data || event.data.source !== EXTENSION_ORIGIN) {
      return;
    }

    const { id, payload } = event.data;
    if (id && pendingCallbacks.has(id)) {
      const cb = pendingCallbacks.get(id)!;
      pendingCallbacks.delete(id);
      cb(payload);
    }
  });
}

function sendToExtension<T>(payload: ExtensionMessage, timeoutMs = 10000): Promise<T> {
  return new Promise((resolve, reject) => {
    if (typeof window === 'undefined') {
      return reject(new Error('Browser environment required'));
    }

    const id = `req_${Date.now()}_${Math.random().toString(36).substring(2, 9)}`;

    const timer = setTimeout(() => {
      if (pendingCallbacks.has(id)) {
        pendingCallbacks.delete(id);
        reject(new Error('Extension response timed out'));
      }
    }, timeoutMs);

    pendingCallbacks.set(id, (response) => {
      clearTimeout(timer);
      resolve(response as T);
    });

    window.postMessage(
      {
        source: WEB_APP_ORIGIN,
        id,
        payload,
      },
      '*'
    );
  });
}

export async function pingExtension(): Promise<ExtensionPingResponse | null> {
  try {
    const res = await sendToExtension<ExtensionPingResponse>({ type: 'PING' });
    return res;
  } catch {
    return null;
  }
}

export async function submitViaExtension(
  submissionId: string,
  platform: PlatformType,
  externalId: string,
  url: string,
  language: LanguageId,
  source: string
): Promise<ExtensionSubmissionCreatedResponse | ExtensionSubmissionFailedResponse> {
  return sendToExtension<ExtensionSubmissionCreatedResponse | ExtensionSubmissionFailedResponse>({
    type: 'SUBMIT',
    submissionId,
    platform,
    problem: { externalId, url },
    language,
    source,
  }, 30000);
}

export async function pollStatusViaExtension(
  platform: PlatformType,
  externalSubmissionId: string,
  externalId: string,
  url: string
): Promise<{ status: string } | null> {
  try {
    const res = await sendToExtension<any>({
      type: 'POLL_STATUS',
      platform,
      externalSubmissionId,
      problem: { externalId, url }
    });
    if (res && res.type === 'POLL_STATUS_RESULT') {
      return { status: res.status };
    }
    return null;
  } catch {
    return null;
  }
}

export async function recoverPendingSubmissions(): Promise<ExtensionRecoverSubmissionsResponse['submissions']> {
  try {
    const res = await sendToExtension<ExtensionRecoverSubmissionsResponse>({ type: 'RECOVER_SUBMISSIONS' }, 20000);
    if (res && res.type === 'RECOVER_SUBMISSIONS_RESULT' && Array.isArray(res.submissions)) {
      return res.submissions;
    }
  } catch {}
  return [];
}

export async function acknowledgeRecoveredSubmission(submissionId: string): Promise<boolean> {
  try {
    const res = await sendToExtension<{ type: 'ACK_SUBMISSION_RESULT'; acknowledged: boolean }>({
      type: 'ACK_SUBMISSION',
      submissionId
    });
    return res?.type === 'ACK_SUBMISSION_RESULT' && res.acknowledged;
  } catch {
    return false;
  }
}
