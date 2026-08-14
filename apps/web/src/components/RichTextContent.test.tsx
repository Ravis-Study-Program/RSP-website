import { describe, expect, it } from 'vitest';

import { sanitizeRichTextForDisplay } from '@/components/RichTextContent';

describe('sanitizeRichTextForDisplay', () => {
  it('preserves every supported formatting element and bounded alignment', () => {
    const source =
      '<h1 style="text-align: center">Title</h1><p data-type="paragraph"><strong>b</strong><em>i</em><u>u</u><s>s</s><strike>strike</strike><mark data-color="#ffcc00" style="background-color:#ffcc00">h</mark><span>span</span><sub>2</sub><sup>3</sup><code>c</code></p><blockquote>q</blockquote><ul><li>one</li></ul><ol><li>two</li></ol><pre>pre</pre><hr><a href="https://example.test" title="safe">link</a>';
    const result = sanitizeRichTextForDisplay(source);
    expect(result).toContain('<h1 data-text-align="center">Title</h1>');
    for (const tag of [
      'strong',
      'em',
      'u',
      's',
      'strike',
      'mark',
      'span',
      'sub',
      'sup',
      'code',
      'blockquote',
      'ul',
      'ol',
      'pre',
      'hr',
      'a',
    ])
      expect(result).toContain(`<${tag}`);
    expect(result).toContain('data-color="#ffcc00"');
    expect(result).toContain('background-color: #ffcc00');
    expect(result).toContain('data-type="paragraph"');
    expect(result).toContain('rel="noopener noreferrer"');
  });

  it('removes active content, event handlers, unsafe URLs and arbitrary attributes', () => {
    const result = sanitizeRichTextForDisplay(
      '<script>alert(1)</script><img src=x onerror=alert(2)><p style="background:url(javascript:evil)" onclick="evil()" data-text-align="wat">Safe</p><a href="javascript:evil()">bad</a>',
    );
    expect(result).not.toMatch(
      /script|img|onerror|onclick|javascript:|style=/i,
    );
    expect(result).not.toContain('data-text-align');
    expect(result).toContain('<p>Safe</p>');
    expect(result).toContain('bad');
  });
});
