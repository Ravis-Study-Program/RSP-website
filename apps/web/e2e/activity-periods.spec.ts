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

test('practice uses season or all time and clears weeks when changing periods @compat', async ({
  page,
}, testInfo) => {
  await page.goto('/seasons/2026-semester-2/practice');
  await expect(page.getByLabel('View activity')).toHaveValue('season_2026_s2');
  await expect(page.getByLabel('Year (Adelaide time)')).toHaveCount(0);
  await page.getByText('Season weeks: All', { exact: true }).click();
  await toggleWeek(page, 3, true);
  await expect(page).toHaveURL(/weekId=week_season_2026_s2_3/);
  const tree = page
    .getByText('Binary Tree Level Order Traversal', { exact: true })
    .filter({ visible: true });
  const anagram = page
    .getByText('Valid Anagram', { exact: true })
    .filter({ visible: true });
  await expect(tree).toBeVisible();
  await expect(anagram).toHaveCount(0);
  await toggleWeek(page, 2, true);
  await expect(anagram).toBeVisible();
  await page.getByLabel('View activity').selectOption('season_2026_s1');
  await expect(page).toHaveURL(/\/seasons\/2026-semester-1\/practice$/);
  await expect(
    page.getByText('Season weeks: All', { exact: true }),
  ).toBeVisible();
  await expect(tree).toHaveCount(0);
  await page.goBack();
  await expect(page.getByLabel('View activity')).toHaveValue('season_2026_s2');
  await expect(
    page.getByRole('checkbox', { name: 'Week 3', exact: true }),
  ).toBeChecked();
  await expect(tree).toBeVisible();
  await page.screenshot({
    path: testInfo.outputPath('season-practice-weeks.png'),
    fullPage: true,
  });
  await page.getByLabel('View activity').selectOption('');
  await expect(page).toHaveURL(/\/practice$/);
  await expect(page.getByText(/^Season weeks/)).toHaveCount(0);
  await expect(
    page
      .getByText('Climbing Stairs', { exact: true })
      .filter({ visible: true }),
  ).toBeVisible();

  // Old saved URLs must not silently narrow the All time view.
  await page.goto('/practice?year=1900&weekId=week_season_2026_s2_3');
  await expect(page.getByLabel('View activity')).toHaveValue('');
  await expect(anagram).toBeVisible();
  await expect(page.getByText(/^Season weeks/)).toHaveCount(0);
});

test('mocks and both profile tabs support weeks inside a season', async ({
  page,
}, testInfo) => {
  await page.goto('/mock-interviews');
  await expect(page.getByLabel('View activity')).toHaveValue('');
  await page.getByLabel('View activity').selectOption('season_2026_s2');
  await page.getByRole('tab', { name: 'All available', exact: true }).click();
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
  await page.getByLabel('View activity').selectOption('');
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
  await page.getByRole('tab', { name: 'Mock interviews', exact: true }).click();
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
  await page.getByLabel('View activity').selectOption('');
  await expect(page).not.toHaveURL(/seasonId=|weekId=/);
  await expect(
    page.getByRole('tab', { name: 'Mock interviews', exact: true }),
  ).toHaveAttribute('aria-selected', 'true');
  await expect(page.getByText(/^Season weeks/)).toHaveCount(0);
  await expect(
    page.getByText('View feedback', { exact: true }).filter({ visible: true }),
  ).toBeVisible();
});
