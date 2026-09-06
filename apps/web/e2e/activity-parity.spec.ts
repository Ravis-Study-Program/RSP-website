import { expect, test } from '@playwright/test';

test('practice charts follow filters and the picker accepts keyboard search', async ({
  page,
}, testInfo) => {
  await page.goto('/practice');
  await expect(
    page.getByRole('heading', { name: 'Problem practice' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Difficulty breakdown' }),
  ).toBeVisible();
  await expect(
    page.getByRole('heading', { name: 'Time by difficulty' }),
  ).toBeVisible();
  await page.getByLabel('Show practice goals (20 / 35 / 50 minutes)').uncheck();
  await page
    .getByLabel('Search problem attempts')
    .fill('no matching question xyz');
  await expect(
    page.getByText('No attempts match these filters.'),
  ).toBeVisible();
  await page.getByLabel('Search problem attempts').clear();
  await page.getByRole('button', { name: 'Log attempt' }).first().click();
  await page.getByRole('combobox', { name: 'Problem' }).fill('Two Sum');
  await page.getByRole('combobox', { name: 'Problem' }).press('ArrowDown');
  await page.getByRole('combobox', { name: 'Problem' }).press('Enter');
  await expect(page.getByRole('combobox', { name: 'Problem' })).toHaveValue(
    /Two Sum/,
  );
  await page.screenshot({
    path: testInfo.outputPath('practice-question-picker.png'),
    fullPage: true,
  });
});

test('suggesting a question leaves practice counts unchanged', async ({
  page,
}, testInfo) => {
  await page.goto('/practice');
  const count = page.getByText(/\d+ recorded$/);
  await expect(count).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Suggest a question' }),
  ).toBeEnabled();
  const before = await count.textContent();
  await page.getByRole('button', { name: 'Suggest a question' }).click();
  await expect(
    page.getByRole('button', { name: 'Suggest a question' }),
  ).toBeEnabled();
  await expect(count).toHaveText(before!);
  await page.screenshot({
    path: testInfo.outputPath('practice-charts.png'),
    fullPage: true,
  });
});
