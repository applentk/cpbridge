// Bridge script injected into cpbridge web application pages.
// Facilitates message passing between the cpbridge Web App and the Chrome Extension background worker.

const EXTENSION_ORIGIN = 'CPBRIDGE_EXTENSION';
const WEB_APP_ORIGIN = 'CPBRIDGE_WEB';
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

window.addEventListener('message', async (event) => {
  // Only accept messages from same origin window
  if (event.source !== window || !event.data || event.data.source !== WEB_APP_ORIGIN) {
    return;
  }

  const currentOrigin = window.location.origin;
  // Enforce allowed origin validation
  if (!isAllowedOrigin(event.origin || currentOrigin) || !isAllowedOrigin(currentOrigin)) {
    console.warn('[cpbridge Extension] Message ignored from unauthorized origin:', event.origin, currentOrigin);
    return;
  }

  const { id, payload } = event.data;

  try {
    const response = await chrome.runtime.sendMessage(payload);
    window.postMessage(
      {
        source: EXTENSION_ORIGIN,
        id,
        payload: response
      },
      currentOrigin
    );
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Extension communication failed';
    window.postMessage(
      {
        source: EXTENSION_ORIGIN,
        id,
        payload: {
          type: 'SUBMISSION_FAILED',
          submissionId: payload?.submissionId || '',
          error: 'PLATFORM_UNAVAILABLE',
          message
        }
      },
      currentOrigin
    );
  }
});

// Broadcast extension presence to trusted origins only
if (isAllowedOrigin(window.location.origin)) {
  window.postMessage(
    {
      source: EXTENSION_ORIGIN,
      type: 'EXTENSION_READY',
      version: chrome.runtime.getManifest().version
    },
    window.location.origin
  );
}
