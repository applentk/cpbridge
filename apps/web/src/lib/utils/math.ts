import katex from 'katex';
import DOMPurify from 'dompurify';

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

function prepareStatementHtml(html: string, sourceUrl?: string): string {
  if (typeof DOMParser === 'undefined') return html;

  const document = new DOMParser().parseFromString(`<body>${html}</body>`, 'text/html');

  // Statements copied from a browser after MathJax has started may contain its
  // preview nodes plus the original TeX in inert script tags. Keep the TeX and
  // discard the generated preview so formulas are not duplicated or lost.
  for (const script of document.body.querySelectorAll('script')) {
    const type = script.getAttribute('type')?.toLowerCase().replace(/\s+/g, ' ') ?? '';
    if (type !== 'math/tex' && type !== 'math/tex; mode=display') continue;

    const template = document.createElement('template');
    template.innerHTML = renderKatexSafe(script.textContent ?? '', type.includes('mode=display'));
    script.replaceWith(template.content);
  }

  for (const generatedMath of document.body.querySelectorAll(
    '.MathJax_Preview, .MathJax_Display, .MathJax_Processing, .MathJax'
  )) {
    generatedMath.remove();
  }

  if (sourceUrl) {
    for (const image of document.body.querySelectorAll<HTMLImageElement>('img[src]')) {
      const src = image.getAttribute('src');
      if (!src) continue;

      try {
        const resolvedUrl = new URL(src, sourceUrl);
        image.src = resolvedUrl.href;
        if (resolvedUrl.pathname.toLowerCase().endsWith('.png')) {
          image.classList.add('statement-image-png');
        }
        image.setAttribute('loading', 'lazy');
        image.setAttribute('decoding', 'async');
        image.setAttribute('referrerpolicy', 'no-referrer');
      } catch {
        image.removeAttribute('src');
      }
    }
  }

  return document.body.innerHTML;
}

/**
 * Sanitizes HTML content with a strict allowlist to prevent XSS while preserving KaTeX math and layout.
 */
export function sanitizeHtml(html: string): string {
  if (!html) return '';
  if (typeof window === 'undefined') {
    return html;
  }
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'p', 'br', 'b', 'i', 'strong', 'em', 'u', 's', 'sub', 'sup', 'ul', 'ol', 'li',
      'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'blockquote', 'pre', 'code', 'table',
      'thead', 'tbody', 'tr', 'th', 'td', 'div', 'span', 'img', 'a', 'section', 'article', 'hr',
      'math', 'semantics', 'mrow', 'mi', 'mo', 'mn', 'annotation', 'mspace', 'msup', 'msub',
      'mfrac', 'msqrt', 'mroot', 'mtable', 'mtr', 'mtd', 'munder', 'mover', 'munderover'
    ],
    ALLOWED_ATTR: [
      'href', 'src', 'alt', 'title', 'class', 'style', 'target', 'rel', 'id',
      'aria-hidden', 'aria-label', 'role', 'tabindex', 'xmlns', 'display', 'width', 'height',
      'loading', 'decoding', 'referrerpolicy'
    ],
    ALLOWED_URI_REGEXP: /^(?:(?:(?:f|ht)tps?|mailto|data):|[^a-z]|[a-z+.-]+(?:[^a-z+.:]|$))/i,
    ALLOW_DATA_ATTR: false
  });
}

/**
 * Strips redundant Codeforces/AtCoder header divs, duplicate sample testcase tables,
 * copyright footers, server time, mobile version links, and terms.
 */
export function cleanBoilerplate(html: string): string {
  if (!html) return '';

  let cleaned = html;

  // 1. Codeforces header div with title, time limit, memory limit, input/output
  cleaned = cleaned.replace(/<div class="header">.*?<\/div>\s*<\/div>/gis, '');

  // 2. Codeforces duplicate sample tests div (since rendered as interactive cards)
  cleaned = cleaned.replace(/<div class="sample-tests?">.*?<\/div>\s*<\/div>/gis, '');

  // 3. Plain text headers (e.g. C. Rabbits\ntime limit per test\n2 seconds\n...)
  cleaned = cleaned.replace(/^[A-Z0-9.\s-]+\n(?:time limit(?:\s+per test)?\n[^\n]+\n)?(?:memory limit(?:\s+per test)?\n[^\n]+\n)?(?:input\n[^\n]+\n)?(?:output\n[^\n]+\n)?/gis, '');

  // 4. Codeforces footer (copyright, server time, mobile version, terms)
  cleaned = cleaned.replace(/(?:\[?Codeforces\]?|\(c\)\s*Copyright).*?(?:Mike Mirzayanov|Server time:|Desktop version|Privacy Policy|Supported by).*/gis, '');
  cleaned = cleaned.replace(/Server time:.*$/gims, '');
  cleaned = cleaned.replace(/Desktop version, switch to.*$/gims, '');
  cleaned = cleaned.replace(/\[?Privacy Policy\]?\s*\|?\s*\[?Terms and Conditions\]?.*/gis, '');

  // 5. AtCoder footer & Japanese section
  cleaned = cleaned.replace(/<p[^>]*>\s*Score\s*:.*?<\/p>/gis, '');
  cleaned = cleaned.replace(/(?:Copyright\s*\d+-\d+\s*AtCoder Inc\.|AtCoder is a trademark).*/gis, '');
  cleaned = cleaned.replace(/<span class="lang-ja">.*?<\/span>/gis, '');

  return cleaned.trim();
}

/**
 * Parses and renders LaTeX formulas from:
 * 1. AtCoder (<var>...</var>, \(...\), \[...\])
 * 2. Codeforces ($$$...$$$, $$...$$, .tex-span)
 * 3. Standard LaTeX ($...$, $$...$$)
 */
export function renderMathInHtml(html: string, sourceUrl?: string): string {
  if (!html) return '';

  let output = cleanBoilerplate(prepareStatementHtml(html, sourceUrl));

  // 1. AtCoder <var>...</var> math and variable tags
  output = output.replace(/<var\b[^>]*>(.*?)<\/var>/gis, (_, math) => {
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

  output = output.replace(/\$\$([^$]+?)\$\$/gs, (_, math) => {
    return renderKatexSafe(math, true);
  });

  // 4. Standard inline math: \( ... \)
  output = output.replace(/\\\((.+?)\\\)/gs, (_, math) => {
    return renderKatexSafe(math, false);
  });

  // 5. Single dollar math: $...$
  output = output.replace(/(^|[^$])\$([^$\n\r]+?)\$(?!\$)/g, (match, prefix, math) => {
    const trimmed = math.trim();
    if (/^[0-9]+(\.[0-9]+)?$/.test(trimmed) || !trimmed) {
      return match;
    }
    return prefix + renderKatexSafe(trimmed, false);
  });

  return sanitizeHtml(output);
}
