/**
 * Whether rich text has no visible words: only tags (such as the empty
 * paragraph a cleared editor leaves), spaces and non-breaking spaces. The API
 * stores such a page as empty; older saves may still hold the markup.
 */
export function isBlankRichText(html: string | null | undefined): boolean {
  if (!html) return true;
  const text = html
    .replace(/<[^>]*>/g, '')
    .replace(/&nbsp;|&#160;|&#xa0;/gi, ' ');
  // trim() also removes non-breaking spaces.
  return text.trim() === '';
}
