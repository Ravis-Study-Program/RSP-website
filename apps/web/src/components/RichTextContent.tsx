import { useMemo } from 'react';

import styles from '@/styles/App.module.css';

const allowedElements = new Set([
  'P',
  'BR',
  'STRONG',
  'B',
  'EM',
  'I',
  'U',
  'S',
  'STRIKE',
  'MARK',
  'SPAN',
  'SUB',
  'SUP',
  'UL',
  'OL',
  'LI',
  'BLOCKQUOTE',
  'CODE',
  'PRE',
  'H1',
  'H2',
  'H3',
  'H4',
  'HR',
  'A',
]);
const alignedElements = new Set(['P', 'BLOCKQUOTE', 'H1', 'H2', 'H3', 'H4']);
const hexColor = /^#[0-9a-f]{3,8}$/i;

export function sanitizeRichTextForDisplay(html: string) {
  if (!html || typeof DOMParser === 'undefined') return '';
  const document = new DOMParser().parseFromString(
    `<div>${html}</div>`,
    'text/html',
  );
  const root = document.body.firstElementChild;
  if (!root) return '';

  const clean = (node: Element) => {
    [...node.children].forEach(clean);
    if (!allowedElements.has(node.tagName)) {
      node.replaceWith(...node.childNodes);
      return;
    }
    const sourceHref =
      node.tagName === 'A' ? (node.getAttribute('href') ?? '') : '';
    const sourceTitle =
      node.tagName === 'A' ? (node.getAttribute('title') ?? '') : '';
    const sourceTarget =
      node.tagName === 'A' ? (node.getAttribute('target') ?? '') : '';
    const styles = Object.fromEntries(
      (node.getAttribute('style') ?? '')
        .split(';')
        .map((declaration) =>
          declaration.split(':').map((part) => part.trim().toLowerCase()),
        )
        .filter((parts) => parts.length === 2),
    );
    const sourceAlignment = alignedElements.has(node.tagName)
      ? (node.getAttribute('data-text-align') ?? styles['text-align'] ?? '')
      : '';
    const sourceDataType =
      node.tagName === 'P' || node.tagName === 'BLOCKQUOTE'
        ? (node.getAttribute('data-type') ?? '')
        : '';
    const sourceColor =
      node.tagName === 'MARK'
        ? (node.getAttribute('data-color') ?? styles['background-color'] ?? '')
        : '';
    [...node.attributes].forEach((attribute) =>
      node.removeAttribute(attribute.name),
    );
    if (node.tagName === 'A') {
      let safeHref = '';
      try {
        const url = new URL(sourceHref, window.location.origin);
        if (
          url.protocol === 'http:' ||
          url.protocol === 'https:' ||
          url.protocol === 'mailto:'
        )
          safeHref = url.href;
      } catch {
        // Invalid URLs are intentionally stripped from imported rich text.
      }
      if (safeHref) {
        node.setAttribute('href', safeHref);
        node.setAttribute('rel', 'noopener noreferrer');
        if (sourceTitle) node.setAttribute('title', sourceTitle);
        if (safeHref.startsWith('http') || sourceTarget === '_blank')
          node.setAttribute('target', '_blank');
      } else node.replaceWith(...node.childNodes);
    }
    if (
      alignedElements.has(node.tagName) &&
      ['left', 'center', 'right', 'justify'].includes(sourceAlignment)
    )
      node.setAttribute('data-text-align', sourceAlignment);
    if (
      (node.tagName === 'P' || node.tagName === 'BLOCKQUOTE') &&
      /^[\w-]+(?:\s+[\w-]+)*$/.test(sourceDataType)
    )
      node.setAttribute('data-type', sourceDataType);
    if (node.tagName === 'MARK' && hexColor.test(sourceColor)) {
      node.setAttribute('data-color', sourceColor);
      node.setAttribute('style', `background-color: ${sourceColor}`);
    }
  };
  [...root.children].forEach(clean);
  return root.innerHTML;
}

export function RichTextContent({
  html,
  empty = 'No notes recorded.',
}: {
  html?: string | null;
  empty?: string;
}) {
  const sanitized = useMemo(
    () => sanitizeRichTextForDisplay(html ?? ''),
    [html],
  );
  if (!sanitized || sanitized === '<p></p>')
    return <p className={styles.muted}>{empty}</p>;
  return (
    <div
      className={styles.richTextContent}
      dangerouslySetInnerHTML={{ __html: sanitized }}
    />
  );
}
