/**
 * Codeforces uses a dynamic query-string action for the problemset submission
 * form, so its URL does not reliably contain the word "submit".
 */
export function isCodeforcesSubmissionForm(form: HTMLFormElement): boolean {
  if (form.classList.contains('submit-form')) return true;

  const action = form.querySelector<HTMLInputElement>('input[name="action"]');
  return action?.value === 'submitSolutionFormSubmitted';
}

export function findCodeforcesSubmissionForm(document: Document): HTMLFormElement | undefined {
  return [...document.forms].find(isCodeforcesSubmissionForm);
}
