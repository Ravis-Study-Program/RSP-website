import { expect, test, type Page } from '@playwright/test';
import { chooseOption } from './select';

/**
 * Rows start collapsed; open one interview's details. The chevron names the
 * interviewee, and on mobile each card has a View details button instead.
 */
async function openDetails(page: Page, interviewee = 'Avery Example') {
  const other = interviewee === 'Avery Example' ? 'Noah Williams' : interviewee;
  await page
    .getByRole('button', { name: `Show details for ${interviewee}` })
    .or(
      page
        .getByRole('article')
        .filter({ hasText: other })
        .getByRole('button', { name: 'View details' }),
    )
    .filter({ visible: true })
    .click();
}

test('a saved review shows straight away without reloading', async ({
  page,
}) => {
  await page.goto('/mock-interviews');
  await openDetails(page);
  const review = page
    .getByLabel('Interviewee notes on Behavioural round')
    .filter({ visible: true });
  await expect(review.getByText('Great session.')).toBeVisible();

  // The review opens from the row's menu, without expanding it.
  await page
    .getByRole('button', {
      name: 'More actions for the interview with Noah Williams',
    })
    .filter({ visible: true })
    .click();
  await page.getByRole('menuitem', { name: 'Edit notes' }).click();
  const dialog = page.getByRole('dialog', { name: 'Edit Notes' });
  await dialog
    .locator('#review-comment-round_4')
    .fill('Clearer on trade-offs now.');
  await dialog.getByRole('button', { name: 'Save notes' }).click();
  await expect(dialog).toBeHidden();

  await expect(review.getByText('Clearer on trade-offs now.')).toBeVisible();
  // Every round's notes are edited in the same dialog.
  await page
    .getByRole('button', {
      name: 'More actions for the interview with Noah Williams',
    })
    .filter({ visible: true })
    .click();
  await page.getByRole('menuitem', { name: 'Edit notes' }).click();
  await expect(
    page.getByRole('dialog', { name: 'Edit Notes' }).getByText(/^Round \d:/),
  ).toHaveText([
    'Round 1: Behavioural round',
    'Round 2: 3. Merge Intervals',
    'Round 3: 4. Two Sum',
  ]);
  await expect(review.getByText('Great session.')).toHaveCount(0);
});

test('LeetCode rounds show the score for each part', async ({ page }) => {
  await page.goto('/mock-interviews');
  await openDetails(page);
  const breakdown = page
    .getByLabel('Merge Intervals score breakdown')
    .filter({ visible: true });
  await expect(breakdown.getByRole('term')).toHaveText([
    'Confirming the Question',
    'Algorithm Design',
    'Complexity Analysis',
    'Coding',
    'Testing',
  ]);
  await expect(breakdown.getByRole('definition')).toHaveText([
    '8/10',
    '7/10',
    '6/10',
    '8/10',
    '9/10',
  ]);
  // The question is numbered and links to the problem, as on LeetCode.
  await expect(
    page
      .getByRole('article', { name: 'Merge Intervals' })
      .filter({ visible: true })
      .getByRole('link', { name: '3. Merge Intervals' }),
  ).toHaveAttribute('href', /search=Merge%20Intervals$/);
  // Each part has a reminder of what it scores, opened by clicking its (i).
  await breakdown
    .getByRole('button', { name: /^Clarifying the problem/ })
    .click();
  await expect(
    page.getByRole('dialog').filter({ visible: true }),
  ).toContainText(/before solving it\.$/);
  await page.keyboard.press('Escape');
  // Behavioural rounds explain their single score beside the title.
  await expect(
    page
      .getByRole('article', { name: 'Behavioural round' })
      .filter({ visible: true })
      .getByRole('button', { name: /past experience/ }),
  ).toBeVisible();
  await expect(
    page
      .getByRole('article', { name: 'Overall' })
      .filter({ visible: true })
      .getByRole('button'),
  ).toHaveCount(0);
});

test('the interviewee marks rounds reviewed from the row or the details', async ({
  page,
}) => {
  await page.goto('/mock-interviews');
  await openDetails(page);
  // The row's checkbox is dashed while only some rounds are reviewed, and
  // the details' Overall row mirrors it.
  const reviewed = page
    .getByRole('checkbox', { name: /every round/ })
    .filter({ visible: true });
  const details = page.getByRole('region', { name: 'Rounds and review' });
  const whole = reviewed.first();
  const overall = details.getByRole('checkbox', { name: /every round/ });
  await expect(reviewed).toHaveCount(2);
  await expect(whole).toHaveAccessibleName(
    'Mark every round reviewed (1 of 3 rounds reviewed)',
  );
  await expect(whole).not.toBeChecked();

  // Each round has its own checkbox beside the notes on it.
  for (const round of ['Merge Intervals', 'Two Sum'])
    await page
      .getByRole('checkbox', { name: `${round} reviewed` })
      .filter({ visible: true })
      .click();
  await expect(whole).toBeChecked();
  await expect(overall).toBeChecked();
  await expect(whole).toHaveAccessibleName(
    'Mark every round unreviewed (3 of 3 rounds reviewed)',
  );

  // The row's checkbox marks the whole interview at once.
  await whole.click();
  await expect(whole).toHaveAccessibleName(
    'Mark every round reviewed (0 of 3 rounds reviewed)',
  );
  await expect(
    page
      .getByRole('checkbox', { name: 'Behavioural round reviewed' })
      .filter({ visible: true }),
  ).not.toBeChecked();
  // The Overall checkbox does the same from the details.
  await overall.click();
  await expect(whole).toBeChecked();
  await expect(overall).toBeChecked();
});

test('each round shows the interviewer and interviewee notes on it', async ({
  page,
}) => {
  await page.goto('/mock-interviews');
  await openDetails(page);
  await expect(
    page
      .getByLabel('Interviewer notes on Merge Intervals')
      .filter({ visible: true }),
  ).toHaveText(/Talked through sorting by start time before coding\./);
  await expect(
    page
      .getByLabel('Interviewee notes on Merge Intervals')
      .filter({ visible: true }),
  ).toHaveText(/No notes from the interviewee\./);
  // The review is edited from the row menu, so there is no separate button.
  await expect(
    page.getByRole('button', { name: /my review/ }).filter({ visible: true }),
  ).toHaveCount(0);
});

test('the row menu shows details and holds the review', async ({ page }) => {
  await page.goto('/mock-interviews');
  const details = page
    .getByLabel('Rounds and review')
    .filter({ visible: true });
  await expect(details).toHaveCount(0);
  const menu = page
    .getByRole('button', {
      name: 'More actions for the interview with Noah Williams',
    })
    .filter({ visible: true });
  await menu.click();
  await page.getByRole('menuitem', { name: 'Show details' }).click();
  await expect(details).toBeVisible();
  await menu.click();
  await page.getByRole('menuitem', { name: 'Hide details' }).click();
  await expect(details).toHaveCount(0);
});

test('the interviewer edits and deletes from the row menu', async ({
  page,
}) => {
  await page.goto('/mock-interviews');
  await openDetails(page);
  await page.getByRole('tab', { name: 'Given' }).click();
  await openDetails(page, 'Amelia Chen');
  // Only the interviewee can change review state.
  await expect(
    page
      .getByRole('checkbox', { name: 'Array and graph problem reviewed' })
      .filter({ visible: true }),
  ).toBeDisabled();

  const menu = page
    .getByRole('button', {
      name: 'More actions for the interview with Amelia Chen',
    })
    .filter({ visible: true });
  await menu.click();
  await expect(page.getByRole('menuitem')).toHaveText([
    'Hide details',
    'Edit notes and scores',
    'Correct identities',
    'Delete interview',
  ]);
  // Only the interviewee reviews, and only they can tick the row's checkbox
  // or its mirror in the details.
  const reviewed = page
    .getByRole('checkbox', { name: '3 of 3 rounds reviewed' })
    .filter({ visible: true });
  await expect(reviewed).toHaveCount(2);
  for (const checkbox of await reviewed.all())
    await expect(checkbox).toBeDisabled();
  await page.getByRole('menuitem', { name: 'Edit notes and scores' }).click();
  await expect(
    page.getByRole('dialog', { name: 'Edit Notes and Scores' }),
  ).toBeVisible();
});

test('All shows given and received interviews, marked by type', async ({
  page,
}) => {
  await page.goto('/mock-interviews');
  await expect(page.getByRole('tab', { name: 'All' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
  const types = page
    .locator('[data-slot=badge]')
    .filter({ hasText: /^(Given|Received)$/, visible: true });
  await expect(types).toHaveText(['Given', 'Received']);

  // Type in the Filters popover is the same choice, second after Season.
  await page.getByRole('button', { name: /^Filters/ }).click();
  const popover = page.locator('[data-slot=popover-content]');
  await expect(popover.getByRole('combobox').nth(1)).toHaveText('All');
  await chooseOption(popover, 'Type', 'Given');
  await expect(page.getByRole('tab', { name: 'Given' })).toHaveAttribute(
    'aria-selected',
    'true',
  );
});

test('the interviewer cannot see the interviewee notes', async ({ page }) => {
  await page.goto('/mock-interviews');
  await openDetails(page, 'Amelia Chen');
  // Avery interviewed Amelia, so Amelia's notes are not shown to Avery.
  await expect(
    page
      .getByLabel('Interviewer notes on Array and graph problem')
      .filter({ visible: true }),
  ).toBeVisible();
  await expect(
    page.getByLabel(/^Interviewee notes on/).filter({ visible: true }),
  ).toHaveCount(0);
  // Custom rounds show their description and reference link.
  await expect(
    page
      .getByRole('link', { name: 'example.test/questions/rate-limiter' })
      .filter({ visible: true }),
  ).toBeVisible();
});
