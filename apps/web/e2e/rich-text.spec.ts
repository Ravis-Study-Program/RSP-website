import { expect, test, type Locator, type Page } from '@playwright/test';

/**
 * Every rich-text toolbar control, on a week's resources page: each must make
 * its element in the editor, look formatted there, and survive saving into
 * the read-only view with the same look.
 */

type Style = Record<string, string>;
type Check = (style: Style, paragraph: Style) => void;

async function openEditor(page: Page) {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'coordinator'),
  );
  await page.goto('/seasons/summer-2025-26');
  await page.getByRole('link', { name: 'Week Resources' }).first().click();
  await expect(
    page.getByRole('heading', { name: 'Week 1 Resources', level: 1 }),
  ).toBeVisible();
  await edit(page);
}

/** Opens the editor, retrying a click that lands before the page is ready. */
async function edit(page: Page) {
  await expect(async () => {
    await page.getByRole('button', { name: 'Edit Resources' }).click();
    await expect(editor(page)).toBeVisible({ timeout: 1000 });
  }).toPass();
}

const editor = (page: Page) =>
  page.getByRole('textbox', { name: 'Resources', exact: true });
const view = (page: Page) => page.locator('.rich-text:not([contenteditable])');
const tool = (page: Page, name: string) =>
  page
    .getByRole('toolbar', { name: 'Resources formatting' })
    .getByRole('button', { name, exact: true });

/**
 * Replaces the editor's content with one line of text and selects that line.
 * (Select-all would also take in the empty line the editor keeps at the end.)
 */
async function startWith(page: Page, text: string) {
  await editor(page).click();
  await page.keyboard.press('ControlOrMeta+A');
  await page.keyboard.press('Delete');
  await page.keyboard.type(text);
  await expect(editor(page)).toHaveText(text);
  await editor(page).locator('p').selectText();
  // Wait for the browser to report the selection before a toolbar click acts
  // on it; selecting the paragraph avoids a timing-sensitive Shift+Home.
  await expect
    .poll(() => page.evaluate(() => window.getSelection()?.toString()))
    .toBe(text);
}

async function computed(root: Locator, selector: string) {
  const element = root.locator(selector).first();
  await expect(element).toBeVisible();
  return element.evaluate((node) => {
    const all = getComputedStyle(node);
    const paragraph = document.createElement('p');
    node.closest('.rich-text')!.append(paragraph);
    const base = getComputedStyle(paragraph);
    const pick = (style: CSSStyleDeclaration) => ({
      fontSize: style.fontSize,
      fontWeight: style.fontWeight,
      fontStyle: style.fontStyle,
      fontFamily: style.fontFamily,
      textDecorationLine: style.textDecorationLine,
      backgroundColor: style.backgroundColor,
      listStyleType: style.listStyleType,
      borderLeftWidth: style.borderLeftWidth,
      borderTopWidth: style.borderTopWidth,
      textAlign: style.textAlign,
      verticalAlign: style.verticalAlign,
      bottom: style.bottom,
      top: style.top,
    });
    const result = { style: pick(all), paragraph: pick(base) };
    paragraph.remove();
    return result;
  });
}

async function expectFormatted(
  page: Page,
  selector: string,
  check: Check,
  text: string,
  /** More checks on the saved page, before editing again. */
  viewChecks?: () => Promise<void>,
) {
  const inEditor = await computed(editor(page), selector);
  check(inEditor.style, inEditor.paragraph);
  await page.getByRole('button', { name: 'Save Resources' }).click();
  await expect(view(page)).toContainText(text);
  const inView = await computed(view(page), selector);
  check(inView.style, inView.paragraph);
  await viewChecks?.();
  await edit(page);
}

const px = (value: string) => Number.parseFloat(value);
const transparent = 'rgba(0, 0, 0, 0)';

const cases: Array<{
  tool: string;
  selector: string;
  check: Check;
}> = [
  {
    tool: 'Bold',
    selector: 'strong',
    check: (s) => expect(Number(s.fontWeight)).toBeGreaterThanOrEqual(600),
  },
  {
    tool: 'Italic',
    selector: 'em',
    check: (s) => expect(s.fontStyle).toBe('italic'),
  },
  {
    tool: 'Underline',
    selector: 'u',
    check: (s) => expect(s.textDecorationLine).toContain('underline'),
  },
  {
    tool: 'Strikethrough',
    selector: 's',
    check: (s) => expect(s.textDecorationLine).toContain('line-through'),
  },
  {
    tool: 'Highlight',
    selector: 'mark',
    check: (s) => expect(s.backgroundColor).not.toBe(transparent),
  },
  {
    tool: 'Inline code',
    selector: 'code',
    check: (s, p) => {
      expect(s.fontFamily).toContain('monospace');
      expect(s.backgroundColor).not.toBe(transparent);
      expect(px(s.fontSize)).toBeLessThan(px(p.fontSize));
    },
  },
  ...[
    ['Heading 1', 'h1', 1.5],
    ['Heading 2', 'h2', 1.25],
    ['Heading 3', 'h3', 1.125],
    ['Heading 4', 'h4', 1],
  ].map(([name, tag, scale]) => ({
    tool: name as string,
    selector: tag as string,
    check: ((s, p) => {
      expect(px(s.fontSize)).toBeCloseTo(px(p.fontSize) * Number(scale), 0);
      expect(Number(s.fontWeight)).toBeGreaterThanOrEqual(600);
    }) as Check,
  })),
  {
    tool: 'Block quote',
    selector: 'blockquote',
    check: (s) => expect(s.backgroundColor).not.toBe('rgba(0, 0, 0, 0)'),
  },
  {
    tool: 'Bulleted list',
    selector: 'ul',
    check: (s) => expect(s.listStyleType).toBe('disc'),
  },
  {
    tool: 'Numbered list',
    selector: 'ol',
    check: (s) => expect(s.listStyleType).toBe('decimal'),
  },
  {
    tool: 'Subscript',
    selector: 'sub',
    check: (s, p) => {
      expect(px(s.fontSize)).toBeLessThan(px(p.fontSize));
      expect(px(s.bottom)).toBeLessThan(0);
    },
  },
  {
    tool: 'Superscript',
    selector: 'sup',
    check: (s, p) => {
      expect(px(s.fontSize)).toBeLessThan(px(p.fontSize));
      expect(px(s.top)).toBeLessThan(0);
    },
  },
  ...['center', 'right', 'justify'].map((alignment) => ({
    tool: `Align ${alignment}`,
    selector: `[data-text-align="${alignment}"]`,
    check: ((s) => expect(s.textAlign).toBe(alignment)) as Check,
  })),
];

test.describe('rich text editor', () => {
  test.skip(
    ({ isMobile }) => isMobile,
    'Toolbar formatting is exercised on desktop',
  );

  for (const item of cases)
    test(`${item.tool} formats text in the editor and the saved page`, async ({
      page,
    }) => {
      await openEditor(page);
      const text = `${item.tool} sample`;
      await startWith(page, text);
      await tool(page, item.tool).click();
      await expect(tool(page, item.tool)).toHaveAttribute(
        'aria-pressed',
        'true',
      );
      await expectFormatted(page, item.selector, item.check, text);
    });

  test('Link adds a safe link that Remove link takes away', async ({
    page,
  }) => {
    await openEditor(page);
    await startWith(page, 'Linked sample');
    await tool(page, 'Link').click();
    const dialog = page.getByRole('dialog', { name: 'Add or edit link' });
    await dialog.getByLabel('Link Address').fill('javascript:alert(1)');
    await dialog.getByRole('button', { name: 'Save Link' }).click();
    await expect(dialog.getByRole('alert')).toContainText('HTTP, HTTPS');
    await dialog.getByLabel('Link Address').fill('example.com/notes');
    await dialog.getByRole('button', { name: 'Save Link' }).click();
    await expect(editor(page).locator('a')).toHaveAttribute(
      'href',
      'https://example.com/notes',
    );
    await expectFormatted(
      page,
      'a',
      (s) => expect(s.textDecorationLine).toContain('underline'),
      'Linked sample',
      () =>
        expect(view(page).locator('a')).toHaveAttribute(
          'href',
          'https://example.com/notes',
        ),
    );
    await editor(page).getByText('Linked sample').click();
    await tool(page, 'Remove link').click();
    await expect(editor(page).locator('a')).toHaveCount(0);
  });

  test('Horizontal rule draws a divider', async ({ page }) => {
    await openEditor(page);
    await startWith(page, 'Above the rule');
    await page.keyboard.press('End');
    await tool(page, 'Horizontal rule').click();
    await expectFormatted(
      page,
      'hr',
      (s) => expect(px(s.borderTopWidth)).toBeGreaterThan(0),
      'Above the rule',
    );
  });

  test('Clear formatting removes marks and headings', async ({ page }) => {
    await openEditor(page);
    await startWith(page, 'Plain again');
    await tool(page, 'Heading 2').click();
    await tool(page, 'Bold').click();
    await expect(editor(page).locator('h2 strong')).toBeVisible();
    await tool(page, 'Clear formatting').click();
    await expect(editor(page).locator('h2, strong')).toHaveCount(0);
    await expect(editor(page).locator('p').first()).toHaveText('Plain again');
  });

  test('a format turned on with nothing selected shows at once and carries on while typing', async ({
    page,
  }) => {
    await openEditor(page);
    await startWith(page, 'Plain ');
    await page.keyboard.press('End');
    for (const name of ['Bold', 'Italic', 'Underline']) {
      await tool(page, name).click();
      // Pressed straight away, before anything is typed.
      await expect(tool(page, name)).toHaveAttribute('aria-pressed', 'true');
    }
    await page.keyboard.type('running');
    await expect(editor(page).locator('strong em u')).toHaveText('running');
    await tool(page, 'Bold').click();
    await expect(tool(page, 'Bold')).toHaveAttribute('aria-pressed', 'false');
    await page.keyboard.type(' on');
    await expect(editor(page).locator('strong')).toHaveText('running');
  });

  test('Undo and Redo step through changes', async ({ page }) => {
    await openEditor(page);
    await startWith(page, 'Undoable');
    await tool(page, 'Bold').click();
    await expect(editor(page).locator('strong')).toBeVisible();
    await tool(page, 'Undo').click();
    await expect(editor(page).locator('strong')).toHaveCount(0);
    await tool(page, 'Redo').click();
    await expect(editor(page).locator('strong')).toBeVisible();
  });
});
