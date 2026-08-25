import type { LanguageId } from '@cpbridge/contracts';

interface PrefillPayload {
  submissionId: string;
  contestId: string;
  problemIndex: string;
  language: LanguageId;
}

interface PrefillResponse {
  type: 'CODEFORCES_PREFILL_RESULT';
  pending?: PrefillPayload;
  source?: string;
  error?: string;
}

interface CodeMirrorElement extends HTMLElement {
  CodeMirror?: { setValue(value: string): void };
}

function submissionIdFromHash(): string | undefined {
  const params = new URLSearchParams(window.location.hash.replace(/^#/, ''));
  return params.get('cpbridge') || undefined;
}

function setFormValue(element: HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement, value: string): void {
  element.value = value;
  element.dispatchEvent(new Event('input', { bubbles: true }));
  element.dispatchEvent(new Event('change', { bubbles: true }));
}

function selectLanguage(select: HTMLSelectElement, language: LanguageId): boolean {
  const patterns: Record<LanguageId, RegExp[]> = {
    cpp23: [/c\+\+\s*23/i, /gnu c\+\+\s*23/i, /c\+\+\s*20/i],
    python3: [/python.*3/i, /pypy.*3/i],
    java21: [/^java\s*21\b/i, /^java\s*17\b/i, /^java(?:\s*\d+|\s*\()/i]
  };
  for (const pattern of patterns[language]) {
    const option = [...select.options].find((candidate) => pattern.test(candidate.textContent?.trim() || ''));
    if (option) {
      setFormValue(select, option.value);
      return true;
    }
  }
  return false;
}

function showBanner(message: string, ready: boolean): void {
  const id = 'cpbridge-interactive-submit-banner';
  let banner = document.getElementById(id);
  if (!banner) {
    banner = document.createElement('div');
    banner.id = id;
    banner.style.cssText = [
      'position:fixed', 'top:12px', 'left:50%', 'transform:translateX(-50%)',
      'z-index:2147483647', 'max-width:680px', 'padding:12px 16px',
      'border-radius:10px', 'font:600 13px/1.4 Arial,sans-serif',
      'box-shadow:0 8px 30px rgba(0,0,0,.3)'
    ].join(';');
    document.documentElement.appendChild(banner);
  }
  banner.style.background = ready ? '#dcfce7' : '#fef3c7';
  banner.style.color = ready ? '#166534' : '#92400e';
  banner.style.border = ready ? '1px solid #86efac' : '1px solid #fcd34d';
  banner.textContent = message;
}

function fillSubmissionForm(pending: PrefillPayload, source: string | undefined): boolean {
  const problemCode = document.querySelector<HTMLInputElement>('input[name="submittedProblemCode"]');
  const problemIndex = document.querySelector<HTMLSelectElement>('select[name="submittedProblemIndex"]');
  const language = document.querySelector<HTMLSelectElement>('select[name="programTypeId"]');
  const sourceArea = document.querySelector<HTMLTextAreaElement>('textarea[name="source"]');

  if ((!problemCode && !problemIndex) || !language || !sourceArea) return false;

  if (problemCode) setFormValue(problemCode, `${pending.contestId}${pending.problemIndex}`);
  if (problemIndex) setFormValue(problemIndex, pending.problemIndex);
  selectLanguage(language, pending.language);

  if (source) {
    setFormValue(sourceArea, source);
    const codeMirror = document.querySelector<CodeMirrorElement>('.CodeMirror');
    codeMirror?.CodeMirror?.setValue(source);
    showBanner('cpbridge pre-filled this solution. Complete Codeforces verification, review the form, and click Submit.', true);
  } else {
    showBanner('cpbridge restored this handoff, but the browser session no longer has the source. Copy it from cpbridge, paste it here, complete verification, and click Submit.', false);
  }
  return true;
}

function watchSubmissionForm(submissionId: string): void {
  const form = document.querySelector<HTMLFormElement>('form[action*="submit"]');
  if (!form || form.dataset.cpbridgeSubmitObserver === 'true') return;

  form.dataset.cpbridgeSubmitObserver = 'true';
  form.addEventListener('submit', () => {
    showBanner('Codeforces received the submission. cpbridge is detecting the verdict and will close this tab.', true);
    void chrome.runtime.sendMessage({
      type: 'CODEFORCES_SUBMISSION_SUBMITTED',
      submissionId
    });
  }, { capture: true });
}

async function initialize(): Promise<void> {
  const submissionId = submissionIdFromHash();
  if (!submissionId) return;

  showBanner('cpbridge is preparing the Codeforces submission form…', false);
  let response: PrefillResponse | undefined;
  for (let attempt = 0; attempt < 600; attempt += 1) {
    response = await chrome.runtime.sendMessage({
      type: 'GET_CODEFORCES_PREFILL',
      submissionId
    }) as PrefillResponse;
    if (response?.pending) break;
    if (attempt < 599) await new Promise((resolve) => window.setTimeout(resolve, 1000));
  }
  if (!response?.pending) {
    showBanner(response?.error || 'The safe Codeforces submission check could not be prepared. Return to cpbridge and reopen this handoff.', false);
    return;
  }

  let attempts = 0;
  const tryFill = () => {
    attempts += 1;
    const filled = fillSubmissionForm(response.pending!, response.source);
    if (filled) watchSubmissionForm(submissionId);
    if (filled || attempts >= 600) {
      window.clearInterval(timer);
      if (attempts >= 600) showBanner('Codeforces did not expose its submission form. Return to cpbridge and use the official submission link manually.', false);
    }
  };
  const timer = window.setInterval(tryFill, 1000);
  tryFill();
}

void initialize();
