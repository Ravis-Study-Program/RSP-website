import { createHmac } from 'node:crypto';

import { expect, test, type Page } from '@playwright/test';

type AccountKind = 'STUDENT' | 'COORDINATOR' | 'DIRECTOR';

function requiredEnvironment(name: string) {
  const value = process.env[name];
  if (!value)
    throw new Error(`${name} is required for the real-stack Playwright suite.`);
  return value;
}

function account(kind: AccountKind) {
  return {
    email: requiredEnvironment(`RSP_E2E_${kind}_EMAIL`),
    password: requiredEnvironment(`RSP_E2E_${kind}_PASSWORD`),
    totpSecret:
      kind === 'STUDENT'
        ? undefined
        : requiredEnvironment(`RSP_E2E_${kind}_TOTP_SECRET`),
  };
}

function decodeBase32(value: string) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  const bits = value
    .toUpperCase()
    .replace(/[^A-Z2-7]/g, '')
    .split('')
    .map((character) =>
      alphabet.indexOf(character).toString(2).padStart(5, '0'),
    )
    .join('');
  const bytes: number[] = [];
  for (let offset = 0; offset + 8 <= bits.length; offset += 8)
    bytes.push(Number.parseInt(bits.slice(offset, offset + 8), 2));
  return Buffer.from(bytes);
}

function totp(secret: string) {
  const counter = BigInt(Math.floor(Date.now() / 30_000));
  const message = Buffer.alloc(8);
  message.writeBigUInt64BE(counter);
  const digest = createHmac('sha1', decodeBase32(secret))
    .update(message)
    .digest();
  const offset = digest[digest.length - 1] & 0x0f;
  return ((digest.readUInt32BE(offset) & 0x7fff_ffff) % 1_000_000)
    .toString()
    .padStart(6, '0');
}

async function signIn(page: Page, kind: AccountKind) {
  const credentials = account(kind);
  await page.goto('/sign-in');
  await page
    .getByRole('textbox', { name: 'Email', exact: true })
    .fill(credentials.email);
  await page.getByLabel('Password', { exact: true }).fill(credentials.password);
  // Provisioning intentionally uses the public ingress too. Pace each fresh
  // browser sign-in to the production-equivalent 5 requests/minute IP bucket
  // instead of weakening or bypassing authentication protections in E2E.
  await page.waitForTimeout(13_000);
  await page.getByRole('button', { name: 'Sign in with email' }).click();
  if (credentials.totpSecret) {
    await expect(page).toHaveURL(/\/two-factor$/);
    const secondsInWindow = Math.floor(Date.now() / 1_000) % 30;
    if (secondsInWindow >= 27)
      await page.waitForTimeout((31 - secondsInWindow) * 1_000);
    await page
      .getByLabel('Authenticator code')
      .fill(totp(credentials.totpSecret));
    await page.getByRole('button', { name: 'Verify' }).click();
  }
  await expect(page).toHaveURL(/\/dashboard$/);
  await expect(page.getByRole('heading', { name: /Welcome/i })).toBeVisible();
}

test('student creates, edits, and deletes a Go-backed practice attempt', async ({
  page,
}) => {
  await signIn(page, 'STUDENT');
  await page.goto('/practice');
  await expect(
    page.getByRole('heading', { name: 'Problem practice' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Log attempt' }).click();
  const create = page.getByRole('dialog', { name: 'Log problem attempt' });
  await create.getByLabel('Problem').selectOption({ index: 1 });
  await create.getByLabel('Time taken (minutes)').fill('24');
  await create
    .locator('#attempt-notes')
    .fill('Real-stack note with structured details.');
  await create.getByRole('button', { name: 'Save attempt' }).click();
  await expect(
    page.getByRole('status').filter({ hasText: 'Two Sum was added.' }),
  ).toBeVisible();

  const row = page.getByRole('row').filter({ hasText: 'Two Sum' }).first();
  await expect(row).toBeVisible();
  await expect(row).toContainText('24 min');
  await row.getByRole('button', { name: 'View notes' }).click();
  await expect(
    page.getByRole('region', { name: 'Notes for Two Sum' }),
  ).toContainText('Real-stack note with structured details.');
  await row.getByRole('button', { name: 'Edit' }).click();
  const edit = page.getByRole('dialog', { name: 'Edit Two Sum' });
  await expect(edit.getByLabel('Time taken (minutes)')).toHaveValue('24');
  await expect(edit.locator('#attempt-notes')).toContainText(
    'Real-stack note with structured details.',
  );
  await edit.getByLabel('Confidence (optional, 1–5)').fill('5');
  await edit
    .locator('#attempt-notes')
    .fill('Updated real-stack note with rich-text persistence.');
  await edit.getByRole('button', { name: 'Update attempt' }).click();
  await expect(page.getByText('Two Sum was saved.')).toBeVisible();
  await expect(row).toContainText('5/5');
  await expect(
    page.getByRole('region', { name: 'Notes for Two Sum' }),
  ).toContainText('Updated real-stack note with rich-text persistence.');

  await row.getByRole('button', { name: 'Delete attempt' }).click();
  const confirmation = page.getByRole('alertdialog', {
    name: 'Delete attempt Two Sum?',
  });
  await confirmation.getByLabel(/Type Two Sum/).fill('Two Sum');
  await confirmation.getByRole('button', { name: 'Delete attempt' }).click();
  await expect(page.getByText('Two Sum was deleted.')).toBeVisible();
});

test('coordinator MFA session creates, edits, and deletes an open-season week', async ({
  page,
}) => {
  const weekNumber = 10_000 + (Date.now() % 80_000);
  const updatedWeekNumber = weekNumber + 1;
  const resourceUrl = `https://example.test/e2e/week-${weekNumber}`;
  await signIn(page, 'COORDINATOR');
  await page.goto('/admin/weeks');
  await expect(
    page.getByRole('heading', { name: 'Weeks', exact: true }),
  ).toBeVisible();
  await expect(page.getByLabel('Season')).toHaveValue('dev-season');
  await expect(page.getByLabel('Season')).toContainText(
    'Development Season · open',
  );

  await page.getByRole('button', { name: 'Create week' }).click();
  const create = page.getByRole('dialog', { name: 'Create week' });
  await create.getByLabel('Week number').fill(String(weekNumber));
  await create.getByLabel('Resource URL (optional)').fill(resourceUrl);
  await create.getByRole('button', { name: 'Create week' }).click();
  const createdWeek = page
    .getByRole('row')
    .filter({ hasText: `Week ${weekNumber}` });
  await expect(createdWeek).toBeVisible();
  await expect(createdWeek).toContainText(resourceUrl);
  await expect(createdWeek).toContainText(/2026.*2026/);

  await createdWeek.getByRole('button', { name: 'Edit' }).click();
  const edit = page.getByRole('dialog', { name: `Edit week ${weekNumber}` });
  await edit.getByLabel('Week number').fill(String(updatedWeekNumber));
  await edit.getByRole('button', { name: 'Save week' }).click();
  const updatedWeek = page
    .getByRole('row')
    .filter({ hasText: `Week ${updatedWeekNumber}` });
  await expect(updatedWeek).toBeVisible();
  await expect(updatedWeek).toContainText(resourceUrl);
  await expect(updatedWeek).toContainText(/2026.*2026/);

  await updatedWeek.getByRole('button', { name: 'Delete week' }).click();
  const confirmation = page.getByRole('alertdialog', {
    name: `Delete week Week ${updatedWeekNumber}?`,
  });
  await confirmation
    .getByLabel(`Type Week ${updatedWeekNumber} to confirm`)
    .fill(`Week ${updatedWeekNumber}`);
  await confirmation.getByRole('button', { name: 'Delete week' }).click();
  await expect(updatedWeek).toHaveCount(0);

  await page.goto('/admin/enrollments');
  await expect(
    page.getByRole('heading', { name: 'Enrollments' }),
  ).toBeVisible();
  await expect(page.getByRole('button', { name: 'Add member' })).toBeEnabled();

  await page.goto('/seasons/unknown-real-stack-season');
  await expect(page.getByText('Season not found')).toBeVisible();
});

test('director MFA session creates, edits, closes, and reopens a throwaway season', async ({
  page,
}) => {
  const suffix = Date.now().toString(36);
  const seasonName = `E2E Release Season ${suffix}`;
  const seasonSlug = `e2e-release-season-${suffix}`;
  await signIn(page, 'DIRECTOR');
  await page.goto('/seasons');
  await expect(page.getByRole('heading', { name: 'Seasons' })).toBeVisible();
  await page.getByRole('button', { name: 'Create season' }).click();
  const create = page.getByRole('dialog', { name: 'Create season' });
  await create.getByLabel('Name').fill(seasonName);
  await create.getByLabel('URL slug').fill(seasonSlug);
  await create.getByLabel('Location').fill('E2E Test Campus');
  await create.getByRole('button', { name: 'Create season' }).click();
  const row = page.getByRole('row').filter({ hasText: seasonName });
  await expect(row).toBeVisible();

  await row.getByRole('button', { name: 'Edit season' }).click();
  const edit = page.getByRole('dialog', { name: `Edit ${seasonName}` });
  await edit.getByLabel('Location').fill('E2E Test Campus Updated');
  await edit.getByRole('button', { name: 'Save season' }).click();
  await expect(row).toContainText('E2E Test Campus Updated');

  await row.getByRole('link', { name: seasonName }).click();
  await expect(page.getByRole('heading', { name: seasonName })).toBeVisible();
  await page.getByRole('button', { name: 'Close season' }).click();
  const close = page.getByRole('dialog', {
    name: `Close season: ${seasonName}`,
  });
  await close
    .getByLabel('Reason')
    .fill('Disposable real-stack acceptance workflow');
  await close.getByLabel(/Type E2E Release Season/).fill(seasonName);
  await close.getByRole('button', { name: 'Close season' }).click();
  await expect(page.getByText('This season is read-only.')).toBeVisible();

  await page.getByRole('button', { name: 'Reopen season' }).click();
  const reopen = page.getByRole('dialog', {
    name: `Reopen season: ${seasonName}`,
  });
  await reopen
    .getByLabel('Reason')
    .fill('Reopen lifecycle acceptance workflow');
  await reopen.getByLabel(/Type E2E Release Season/).fill(seasonName);
  await reopen.getByRole('button', { name: 'Reopen season' }).click();
  await expect(page.getByText('This season is read-only.')).toHaveCount(0);
  await expect(
    page.getByRole('button', { name: 'Close season' }),
  ).toBeVisible();

  await page.goto('/admin/users');
  await expect(
    page.getByRole('heading', { name: 'You do not have access' }),
  ).toBeVisible();
});
