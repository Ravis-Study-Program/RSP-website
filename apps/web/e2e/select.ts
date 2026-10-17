import { expect, type Locator, type Page } from '@playwright/test';

export async function chooseOption(
  scope: Page | Locator,
  label: string | RegExp,
  option: string | RegExp,
) {
  const page = 'page' in scope ? scope.page() : scope;
  await scope.getByRole('combobox', { name: label }).click();
  await page
    .getByRole('option', { name: option, exact: typeof option === 'string' })
    .click();
}

export async function expectSelected(
  scope: Page | Locator,
  label: string | RegExp,
  text: string | RegExp,
) {
  await expect(scope.getByRole('combobox', { name: label })).toHaveText(text);
}
