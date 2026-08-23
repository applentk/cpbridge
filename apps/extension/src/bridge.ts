// Bridge script injected into CP Hub web application pages.
// Facilitates message passing between the CP Hub Web App and the Chrome Extension background worker.

const EXTENSION_ORIGIN = 'CP_HUB_EXTENSION';
const WEB_APP_ORIGIN = 'CP_HUB_WEB';

window.addEventListener('message', async (event) => {
  // Only accept messages from same origin web app
  if (event.source !== window || !event.data || event.data.source !== WEB_APP_ORIGIN) {
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
      '*'
    );
  } catch (err: any) {
    window.postMessage(
      {
        source: EXTENSION_ORIGIN,
        id,
        payload: {
          type: 'SUBMISSION_FAILED',
          submissionId: payload.submissionId || '',
          error: 'PLATFORM_UNAVAILABLE',
          message: err.message || 'Extension communication failed'
        }
      },
      '*'
    );
  }
});

// Broadcast extension presence
window.postMessage(
  {
    source: EXTENSION_ORIGIN,
    type: 'EXTENSION_READY',
    version: '1.0.0'
  },
  '*'
);
