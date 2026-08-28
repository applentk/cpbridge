export async function activateTab(tabId: number | undefined): Promise<void> {
  if (tabId === undefined) return;

  try {
    await chrome.tabs.update(tabId, { active: true });
  } catch (err) {
    // The source tab may have been closed while the platform submission was
    // running. Restoring it is best effort and must not change the result.
    console.warn('[cpbridge Extension] Could not restore the submission tab:', err);
  }
}
