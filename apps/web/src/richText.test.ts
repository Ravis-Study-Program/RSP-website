import { isBlankRichText } from './richText';

it('treats markup without words as blank', () => {
  for (const blank of [
    '',
    null,
    undefined,
    '<p></p>',
    '<p> </p><p>&nbsp;</p>',
    '<h1></h1><hr><ul><li><p></p></li></ul>',
    '<p><br class="ProseMirror-trailingBreak"></p>',
  ])
    expect(isBlankRichText(blank)).toBe(true);
  for (const content of [
    '<p>Read chapter 3</p>',
    '<h2>x</h2>',
    '<p><a href="https://example.com">link</a></p>',
  ])
    expect(isBlankRichText(content)).toBe(false);
});
