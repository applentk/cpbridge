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
    .replace(/&plusmn;/g, '\\pm ');
}

function renderKatexSafe(latex: string, displayMode: boolean): string {
  const decoded = decodeHtmlEntities(latex.trim());
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
 * Parses and replaces all math patterns from Codeforces ($$$...$$$, $$...$$),
 * AtCoder (\(...\), \[...\]), and LeetCode into rendered KaTeX HTML.
 */
export function renderMathInHtml(html: string): string {
  if (!html) return '';

  let output = html;

  // 1. Codeforces triple dollar: $$$...$$$ (inline math)
  output = output.replace(/\$\$\$(.+?)\$\$\$/gs, (_, math) => {
    return renderKatexSafe(math, false);
  });

  // 2. Display math: \[ ... \] or $$ ... $$
  output = output.replace(/\\\[(.+?)\\\]/gs, (_, math) => {
    return renderKatexSafe(math, true);
  });

  output = output.replace(/\$\$([^\$]+?)\$\$/gs, (_, math) => {
    return renderKatexSafe(math, true);
  });

  // 3. AtCoder inline math: \( ... \)
  output = output.replace(/\\\((.+?)\\\)/gs, (_, math) => {
    return renderKatexSafe(math, false);
  });

  // 4. Single dollar math: $...$ (ensure not double $$)
  // Match single $ that has non-space inside and is surrounded by spaces/punctuation/tags
  output = output.replace(/(^|[^\$])\$([^\$\n\r]+?)\$(?!\$)/g, (match, prefix, math) => {
    // Avoid matching single dollars in normal text like "$100" without math operators
    if (/^[0-9]+(\.[0-9]+)?$/.test(math.trim())) {
      return match;
    }
    return prefix + renderKatexSafe(math, false);
  });

  return output;
}
