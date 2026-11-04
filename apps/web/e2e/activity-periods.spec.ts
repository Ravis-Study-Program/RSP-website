import { chooseOption, expectSelected } from './select';
import { expect, test, type Page } from '@playwright/test';

async function toggleWeek(page: Page, week: number, checked: boolean) {
  const checkbox = page.getByRole('checkbox', {
    name: `Week ${week}`,
    exact: true,
  });
  await expect(checkbox).toBeChecked({ checked: !checked });
  await checkbox.click();
  // URL changes use a React navigation transition; wait for that state to settle.
  await expect(checkbox).toBeChecked({ checked });
}

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
  // Season weeks live in the attempt table's Filters popover.
  const openTableFilters = () =>
    page.getByRole('button', { name: /^Filters/ }).click();
  const weeksFilter = page.getByRole('combobox', { name: 'Season weeks' });
  await page.goto('/seasons/summer-2025-26/leetcode');
  await expectSelected(page, 'View activity', 'Summer 2025/26');
  await expect(page.getByLabel('Year (Adelaide time)')).toHaveCount(0);
  await openTableFilters();
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
  await expect(page.getByRole('button', { name: 'Filters (2)' })).toBeVisible();
  await page.keyboard.press('Escape');
  await chooseOption(page, 'View activity', 'Summer 2024/25');
  await expect(page).toHaveURL(/\/seasons\/summer-2024-25\/leetcode$/);
  await expect(page.getByRole('button', { name: 'Filters' })).toBeVisible();
  await expect(tree).toHaveCount(0);
  await page.goBack();
  await expectSelected(page, 'View activity', 'Summer 2025/26');
  await openTableFilters();
  await expect(weeksFilter).toHaveText('Week 2, Week 3');
  await page.keyboard.press('Escape');
  await expect(tree).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('season-practice-weeks.png'),
    fullPage: true,
  });
  await chooseOption(page, 'View activity', 'All time');
  await expect(page).toHaveURL(/\/leetcode$/);
  await openTableFilters();
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
  await expectSelected(page, 'View activity', 'All time');
  await expect(anagram).toBeVisible();
  await openTableFilters();
  await expect(weeksFilter).toHaveCount(0);
});

test('mocks and both profile tabs support weeks inside a season', async ({
  page,
}, testInfo) => {
  await page.goto('/mock-interviews');
  await expectSelected(page, 'View activity', 'All time');
  await chooseOption(page, 'View activity', 'Summer 2025/26');
  await page.getByRole('tab', { name: 'All', exact: true }).click();
  await page.getByText('Season weeks: All', { exact: true }).click();
  await toggleWeek(page, 3, true);
  const amelia = page
    .getByRole('link', { name: 'Amelia Chen', exact: true })
    .filter({ visible: true });
  const zara = page
    .getByRole('link', { name: 'Zara Ahmed', exact: true })
    .filter({ visible: true });
  await expect(amelia).toBeVisible();
  await expect(zara).toHaveCount(0);
  await toggleWeek(page, 2, true);
  await expect(zara).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('season-mock-weeks.png'),
    fullPage: true,
  });
  await chooseOption(page, 'View activity', 'All time');
  await expect(page).toHaveURL(/\/mock-interviews$/);
  await expect(page.getByText(/^Season weeks/)).toHaveCount(0);

  await page.goto('/people/amelia-chen?seasonId=season_2026_s2');
  await page.getByText('Season weeks: All', { exact: true }).click();
  await toggleWeek(page, 3, true);
  await expect(
    page
      .getByText('Binary Tree Level Order Traversal', { exact: true })
      .filter({ visible: true }),
  ).toBeVisible();
  await expect(
    page.getByText('Valid Anagram', { exact: true }).filter({ visible: true }),
  ).toHaveCount(0);
  await page.getByRole('tab', { name: 'Mock Interviews', exact: true }).click();
  await expect(
    page.getByText('Season weeks (1)', { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText('View feedback', { exact: true }).filter({ visible: true }),
  ).toBeVisible();
  await page.getByText('Season weeks (1)', { exact: true }).click();
  await toggleWeek(page, 3, false);
  await toggleWeek(page, 2, true);
  await expect(
    page.getByText('No mocks in this period', { exact: true }),
  ).toBeVisible();
  await chooseOption(page, 'View activity', 'All time');
  await expect(page).not.toHaveURL(/seasonId=|weekId=/);
  await expect(
    page.getByRole('tab', { name: 'Mock Interviews', exact: true }),
  ).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText(/^Season weeks/)).toHaveCount(0);
  await expect(
    page.getByText('View feedback', { exact: true }).filter({ visible: true }),
  ).toBeVisible();
});
