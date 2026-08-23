import katex from 'katex';

function decodeHtmlEntities(str: string): string {
  return str
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&le;/g, '\\le ')
    .replace(/&ge;/g, '\\ge ')
    .replace(/&ne;/g, '\\ne ')
    .replace(/&times;/g, '\\times ')
    .replace(/&middot;/g, '\\cdot ')
    .replace(/&plusmn;/g, '\\pm ')
    .replace(/&hellip;/g, '\\dots ')
    .replace(/&minus;/g, '-');
}

function renderKatexSafe(latex: string, displayMode: boolean): string {
  const trimmed = latex.trim();
  if (!trimmed) return '';
  const decoded = decodeHtmlEntities(trimmed);
  try {
    return katex.renderToString(decoded, {
      displayMode,
      throwOnError: false,
      output: 'htmlAndMathml'
    });
  } catch (err) {
    return `<span class="katex-error" title="${err}">${latex}</span>`;
  }
}

/**
 * Parses and renders LaTeX formulas from:
 * 1. AtCoder (<var>...</var>, \(...\), \[...\])
 * 2. Codeforces ($$$...$$$, $$...$$, .tex-span)
 * 3. Standard LaTeX ($...$, $$...$$)
 */
export function renderMathInHtml(html: string): string {
  if (!html) return '';

  let output = html;

  // 1. AtCoder <var>...</var> math and variable tags
  output = output.replace(/<var\b[^>]*>(.*?)<\/var>/gis, (_, math) => {
    // Strip inner tags if any (e.g. <i>N</i>)
    const cleanMath = math.replace(/<[^>]+>/g, '').trim();
    return renderKatexSafe(cleanMath, false);
  });

  // 2. Codeforces triple dollar: $$$...$$$ (inline math)
  output = output.replace(/\$\$\$(.+?)\$\$\$/gs, (_, math) => {
    return renderKatexSafe(math, false);
  });

  // 3. Display math: \[ ... \] or $$ ... $$
  output = output.replace(/\\\[(.+?)\\\]/gs, (_, math) => {
    return renderKatexSafe(math, true);
  });

  output = output.replace(/\$\$([^\$]+?)\$\$/gs, (_, math) => {
    return renderKatexSafe(math, true);
  });

  // 4. Standard inline math: \( ... \)
  output = output.replace(/\\\((.+?)\\\)/gs, (_, math) => {
    return renderKatexSafe(math, false);
  });

  // 5. Single dollar math: $...$ (when not empty or simple price)
  output = output.replace(/(^|[^\$])\$([^\$\n\r]+?)\$(?!\$)/g, (match, prefix, math) => {
    const trimmed = math.trim();
    // Avoid single numbers like "$100" or empty
    if (/^[0-9]+(\.[0-9]+)?$/.test(trimmed) || !trimmed) {
      return match;
    }
    return prefix + renderKatexSafe(trimmed, false);
  });

  return output;
}
