import { chooseOption, expectSelected } from './select';
import { expect, test, type Page } from '@playwright/test';

/** Toggles a week in the LeetCode table's Season weeks dropdown. */
async function toggleWeekOption(page: Page, week: number, selected: boolean) {
  const option = page.getByRole('option', {
    name: `Week ${week}`,
    exact: true,
  });
  if (!(await option.isVisible()))
    await page.getByRole('combobox', { name: 'Season weeks' }).click();
  await expect(option).toHaveAttribute('aria-selected', String(!selected));
  await option.click();
  // URL changes use a React navigation transition; wait for that state to settle.
  await expect(option).toHaveAttribute('aria-selected', String(selected));
}

test('practice uses season or all time and clears weeks when changing periods @compat', async ({
  page,
}, testInfo) => {
  // Season and weeks live in the attempt table's Filters popover.
  const openTableFilters = () =>
    page.getByRole('button', { name: /^Filters/ }).click();
  const weeksFilter = page.getByRole('combobox', { name: 'Season weeks' });
  await page.goto('/seasons/summer-2025-26/leetcode');
  await expect(page.getByRole('combobox', { name: /^Season$/ })).toHaveCount(0);
  await openTableFilters();
  await expectSelected(page, /^Season$/, 'Summer 2025/26');
  await expect(page.getByLabel('Year (Adelaide time)')).toHaveCount(0);
  await toggleWeekOption(page, 3, true);
  await expect(page).toHaveURL(/weekId=week_season_2026_s2_3/);
  const tree = page
    .getByText('Binary Tree Level Order Traversal', { exact: true })
    .filter({ visible: true });
  const anagram = page
    .getByText('Valid Anagram', { exact: true })
    .filter({ visible: true });
  await expect(tree).toBeVisible();
  await expect(anagram).toHaveCount(0);
  await toggleWeekOption(page, 2, true);
  await expect(anagram).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(weeksFilter).toHaveText('Week 2, Week 3');
  await expect(page.getByRole('button', { name: 'Filters (3)' })).toBeVisible();
  await chooseOption(page, /^Season$/, 'Summer 2024/25');
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/seasons\/summer-2024-25\/leetcode$/);
  await expect(page.getByRole('button', { name: 'Filters (1)' })).toBeVisible();
  await expect(tree).toHaveCount(0);
  await page.goBack();
  await openTableFilters();
  await expectSelected(page, /^Season$/, 'Summer 2025/26');
  await expect(weeksFilter).toHaveText('Week 2, Week 3');
  await page.screenshot({
    path: testInfo.outputPath('practice-filters.png'),
    fullPage: true,
  });
  await page.keyboard.press('Escape');
  await expect(page.getByRole('combobox', { name: /^Season$/ })).toHaveCount(0);
  await expect(tree).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('season-practice-weeks.png'),
    fullPage: true,
  });
  await openTableFilters();
  await chooseOption(page, /^Season$/, 'All time');
  await expect(page).toHaveURL(/\/leetcode$/);
  await expect(weeksFilter).toHaveCount(0);
  await page.keyboard.press('Escape');
  await expect(
    page
      .getByText('Climbing Stairs', { exact: true })
      .filter({ visible: true }),
  ).toBeVisible();

  // Old saved URLs must not silently narrow the All time view, and the
  // suggestion's difficulty never filters the table.
  await page.goto(
    '/leetcode?year=1900&weekId=week_season_2026_s2_3&difficulty=hard',
  );
  await expect(anagram).toBeVisible();
  await openTableFilters();
  await expectSelected(page, /^Season$/, 'All time');
  await expect(weeksFilter).toHaveCount(0);
});

test('mock interviews support weeks inside a season', async ({
  page,
}, testInfo) => {
  await page.goto('/mock-interviews');
  await expect(page.getByRole('tab', { name: 'All', exact: true })).toHaveCount(
    0,
  );
  await expect(
    page.getByRole('combobox', { name: 'Season', exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole('link', { name: 'Avery Example', exact: true }),
  ).toHaveCount(0);
  await expect(
    page
      .getByRole('link', { name: 'Noah Williams', exact: true })
      .filter({ visible: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: /^Filters/ }).click();
  await expectSelected(page, /^Season$/, 'All time');
  await chooseOption(page, /^Season$/, 'Summer 2025/26');
  await page.keyboard.press('Escape');
  await page.getByRole('tab', { name: 'Given', exact: true }).click();
  await page.getByRole('button', { name: /^Filters/ }).click();
  await page.screenshot({
    path: testInfo.outputPath('mock-filters.png'),
    fullPage: true,
  });
  await toggleWeekOption(page, 3, true);
  const amelia = page
    .getByRole('link', { name: 'Amelia Chen', exact: true })
    .filter({ visible: true });
  const zara = page
    .getByRole('link', { name: 'Zara Ahmed', exact: true })
    .filter({ visible: true });
  await expect(amelia).toBeVisible();
  await expect(zara).toHaveCount(0);
  await toggleWeekOption(page, 2, true);
  await expect(zara).toHaveCount(0);
  await toggleWeekOption(page, 3, false);
  await expect(amelia).toHaveCount(0);
  await toggleWeekOption(page, 3, true);
  await expect(amelia).toBeVisible();
  await page.keyboard.press('Escape');
  await page.keyboard.press('Escape');
  await expect(
    page.getByRole('link', { name: 'Avery Example', exact: true }),
  ).toHaveCount(0);
  await page.screenshot({
    path: testInfo.outputPath('season-mock-weeks.png'),
    fullPage: true,
  });
  await page.getByRole('button', { name: /^Filters/ }).click();
  await chooseOption(page, /^Season$/, 'All time');
  await expect(page).toHaveURL(/\/mock-interviews$/);
  await expect(page.getByText(/^Season weeks/)).toHaveCount(0);
  await page.keyboard.press('Escape');
});

test('season pickers only offer seasons where the member was a student', async ({
  page,
}) => {
  // The demo account was a student in both demo seasons.
  await page.goto('/leetcode');
  await page.getByRole('button', { name: /^Filters/ }).click();
  await page.getByRole('combobox', { name: 'Season', exact: true }).click();
  await expect(page.getByRole('option')).toHaveText([
    'All time',
    'Summer 2025/26',
    'Summer 2024/25',
  ]);
});

test('the season chosen in one table is remembered in the others', async ({
  page,
}) => {
  await page.goto('/mock-interviews');
  await page.getByRole('button', { name: /^Filters/ }).click();
  await chooseOption(page, /^Season$/, 'Summer 2024/25');
  await page.keyboard.press('Escape');

  // A plain link to another table opens on the same season.
  await page.goto('/leetcode');
  await expect(page).toHaveURL(/seasonId=season_2026_s1/);
  await page.getByRole('button', { name: /^Filters/ }).click();
  await expectSelected(page, /^Season$/, 'Summer 2024/25');

  // All time is remembered too, even over a season link.
  await chooseOption(page, /^Season$/, 'All time');
  await page.keyboard.press('Escape');
  await page.goto('/seasons/summer-2025-26/mock-interviews');
  await expect(page).toHaveURL(/\/mock-interviews$/);
});
