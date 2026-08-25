// Bridge script injected into cpbridge web application pages.
// Facilitates message passing between the cpbridge Web App and the Chrome Extension background worker.

const EXTENSION_ORIGIN = 'CP_HUB_EXTENSION';
const WEB_APP_ORIGIN = 'CP_HUB_WEB';

const ALLOWED_ORIGINS = new Set([
  'http://localhost:3000',
  'http://localhost:5173',
  'http://localhost:8080',
  'http://127.0.0.1:3000',
  'http://127.0.0.1:5173',
  'http://127.0.0.1:8080',
  'https://cphub.dev',
  'https://app.cphub.dev'
]);

function isAllowedOrigin(origin: string): boolean {
  if (!origin) return false;
  if (ALLOWED_ORIGINS.has(origin)) return true;
  // Local development ports
  if (/^http:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/.test(origin)) {
    return true;
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
      version: '1.0.0'
    },
    window.location.origin
  );
}
