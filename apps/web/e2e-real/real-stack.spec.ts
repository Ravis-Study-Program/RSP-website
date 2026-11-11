import { chooseOption, expectSelected } from '../e2e/select';
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
  await page.getByRole('button', { name: 'Sign In with Email' }).click();
  if (credentials.totpSecret) {
    await expect(page).toHaveURL(/\/two-factor$/);
    const secondsInWindow = Math.floor(Date.now() / 1_000) % 30;
    if (secondsInWindow >= 27)
      await page.waitForTimeout((31 - secondsInWindow) * 1_000);
    await page
      .getByLabel('Authenticator Code')
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
  await page.goto('/leetcode');
  await expect(
    page.getByRole('heading', { name: 'LeetCode Practice' }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Log Attempt' }).click();
  const create = page.getByRole('dialog', { name: 'Log Problem Attempt' });
  await create.getByLabel('Problem').fill('Two Sum');
  await page.getByRole('option').filter({ hasText: 'Two Sum' }).first().click();
  await create.getByLabel('Time taken (minutes)').fill('24');
  await create
    .locator('#attempt-notes')
    .fill('Real-stack note with structured details.');
  await create.getByRole('button', { name: 'Save Attempt' }).click();
  await expect(
    page.getByRole('status').filter({ hasText: '1. Two Sum was added.' }),
  ).toBeVisible();

  const row = page.getByRole('row').filter({ hasText: 'Two Sum' }).first();
  await expect(row).toBeVisible();
  await expect(row).toContainText('24 min');
  await row.getByRole('button', { name: 'Show notes for 1. Two Sum' }).click();
  await expect(
    page.getByRole('region', { name: 'Notes for 1. Two Sum' }),
  ).toContainText('Real-stack note with structured details.');
  // Edit and delete sit in the row's kebab menu.
  const rowMenu = row.getByRole('button', {
    name: 'More actions for 1. Two Sum',
  });
  await rowMenu.click();
  await page.getByRole('menuitem', { name: 'Edit' }).click();
  const edit = page.getByRole('dialog', { name: 'Edit 1. Two Sum' });
  await expect(edit.getByLabel('Time taken (minutes)')).toHaveValue('24');
  await expect(edit.locator('#attempt-notes')).toContainText(
    'Real-stack note with structured details.',
  );
  await chooseOption(edit, 'Outcome', 'Unsolved');
  await edit
    .locator('#attempt-notes')
    .fill('Updated real-stack note with rich-text persistence.');
  await edit.getByRole('button', { name: 'Update Attempt' }).click();
  await expect(page.getByText('1. Two Sum was saved.')).toBeVisible();
  await expect(row).toContainText('Unsolved');
  const reviewed = row.getByRole('checkbox', { name: '1. Two Sum reviewed' });
  await reviewed.click();
  await expect(reviewed).toBeChecked();
  await expect(
    page.getByRole('region', { name: 'Notes for 1. Two Sum' }),
  ).toContainText('Updated real-stack note with rich-text persistence.');

  await rowMenu.click();
  await page.getByRole('menuitem', { name: 'Delete Attempt' }).click();
  const confirmation = page.getByRole('alertdialog', {
    name: 'Delete attempt 1. Two Sum?',
  });
  await confirmation.getByLabel(/Type 1\. Two Sum/).fill('1. Two Sum');
  await confirmation.getByRole('button', { name: 'Delete Attempt' }).click();
  await expect(page.getByText('1. Two Sum was deleted.')).toBeVisible();
});

test('coordinator MFA session creates, edits, and deletes an open-season week', async ({
  page,
}) => {
  const weekNumber = 10_000 + (Date.now() % 80_000);
  const updatedWeekNumber = weekNumber + 1;
  await signIn(page, 'COORDINATOR');
  await page.goto('/admin/weeks');
  await expect(
    page.getByRole('heading', { name: 'Weeks (Admin)', exact: true }),
  ).toBeVisible();
  await expectSelected(page, 'Season', 'Development Season · Open');

  await page.getByRole('button', { name: 'Create Week' }).click();
  const create = page.getByRole('dialog', { name: 'Create Week' });
  await create.getByLabel('Week Number').fill(String(weekNumber));
  await create.getByRole('button', { name: 'Create Week' }).click();
  // The table is paged and sorted by number, so find the new week by search.
  const search = page.getByPlaceholder('Search all columns');
  await search.fill(`Week ${weekNumber}`);
  const createdWeek = page
    .getByRole('row')
    .filter({ hasText: `Week ${weekNumber}` });
  await expect(createdWeek).toBeVisible();
  await expect(createdWeek).toContainText(
    /\d{2}\/\d{2}\/\d{2} – \d{2}\/\d{2}\/\d{2}/,
  );

  await createdWeek
    .getByRole('button', { name: `More actions for Week ${weekNumber}` })
    .click();
  await page.getByRole('menuitem', { name: 'Edit', exact: true }).click();
  const edit = page.getByRole('dialog', { name: `Edit Week ${weekNumber}` });
  await edit.getByLabel('Week Number').fill(String(updatedWeekNumber));
  await edit.getByRole('button', { name: 'Save Week' }).click();
  await search.fill(`Week ${updatedWeekNumber}`);
  const updatedWeek = page
    .getByRole('row')
    .filter({ hasText: `Week ${updatedWeekNumber}` });
  await expect(updatedWeek).toBeVisible();
  await expect(updatedWeek).toContainText(
    /\d{2}\/\d{2}\/\d{2} – \d{2}\/\d{2}\/\d{2}/,
  );

  await updatedWeek
    .getByRole('button', {
      name: `More actions for Week ${updatedWeekNumber}`,
    })
    .click();
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  const confirmation = page.getByRole('alertdialog', {
    name: `Delete Week ${updatedWeekNumber}?`,
  });
  await confirmation
    .getByLabel(`Type Week ${updatedWeekNumber} to confirm`)
    .fill(`Week ${updatedWeekNumber}`);
  await confirmation.getByRole('button', { name: 'Delete Week' }).click();
  await expect(updatedWeek).toHaveCount(0);

  await page.goto('/admin/enrollments');
  await expect(
    page.getByRole('heading', { name: 'Enrollments' }),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Invite a Member' }),
  ).toBeEnabled();

  await page.goto('/seasons/unknown-real-stack-season');
  await expect(page.getByText('Season Not Found')).toBeVisible();
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
  await page.getByRole('button', { name: 'Create Season' }).click();
  const create = page.getByRole('dialog', { name: 'Create Season' });
  await create.getByLabel('Name').fill(seasonName);
  await create.getByLabel('URL Slug').fill(seasonSlug);
  await create.getByRole('button', { name: 'Create Season' }).click();
  const row = page.getByRole('row').filter({ hasText: seasonName });
  await expect(row).toBeVisible();

  await row
    .getByRole('button', { name: `More actions for ${seasonName}` })
    .click();
  await page.getByRole('menuitem', { name: 'Edit', exact: true }).click();
  const edit = page.getByRole('dialog', { name: `Edit ${seasonName}` });
  await edit
    .getByLabel('Resources URL')
    .fill('https://example.test/e2e-resources');
  await edit.getByRole('button', { name: 'Save Season' }).click();
  await expect(edit).toBeHidden();
  await row
    .getByRole('button', { name: `More actions for ${seasonName}` })
    .click();
  await page.getByRole('menuitem', { name: 'Edit', exact: true }).click();
  await expect(edit.getByLabel('Resources URL')).toHaveValue(
    'https://example.test/e2e-resources',
  );
  await page.keyboard.press('Escape');
  await expect(edit).toBeHidden();

  await row.getByRole('link', { name: seasonName }).click();
  await expect(page.getByRole('heading', { name: seasonName })).toBeVisible();
  await page.getByRole('button', { name: 'Edit Season' }).click();
  await page.getByRole('menuitem', { name: 'Close Season' }).click();
  const close = page.getByRole('dialog', {
    name: `Close Season: ${seasonName}`,
  });
  await close
    .getByLabel('Reason')
    .fill('Disposable real-stack acceptance workflow');
  await close.getByLabel(/Type E2E Release Season/).fill(seasonName);
  await close.getByRole('button', { name: 'Close Season' }).click();
  await expect(page.getByText('This season is read-only.')).toBeVisible();

  await page.getByRole('button', { name: 'Edit Season' }).click();
  await page.getByRole('menuitem', { name: 'Reopen Season' }).click();
  const reopen = page.getByRole('dialog', {
    name: `Reopen Season: ${seasonName}`,
  });
  await reopen
    .getByLabel('Reason')
    .fill('Reopen lifecycle acceptance workflow');
  await reopen.getByLabel(/Type E2E Release Season/).fill(seasonName);
  await reopen.getByRole('button', { name: 'Reopen Season' }).click();
  await expect(page.getByText('This season is read-only.')).toHaveCount(0);
  await page.getByRole('button', { name: 'Edit Season' }).click();
  await expect(
    page.getByRole('menuitem', { name: 'Close Season' }),
  ).toBeVisible();
  await page.keyboard.press('Escape');

  // Seasons are managed on the Seasons page, from each row's menu.
  await page.goto('/seasons');
  const emptySeason = page.getByRole('row').filter({ hasText: seasonName });
  await emptySeason
    .getByRole('button', { name: `More actions for ${seasonName}` })
    .click();
  await page.getByRole('menuitem', { name: 'Delete', exact: true }).click();
  const deletion = page.getByRole('alertdialog');
  await deletion.getByRole('textbox').fill(seasonName);
  await deletion.getByRole('button', { name: 'Delete Season' }).click();
  await expect(emptySeason).toHaveCount(0);

  // Account pages are for System Admins; a Director is turned away.
  await page.goto('/admin/users');
  await expect(page).toHaveURL(/\/forbidden$/);
});

async function emailLink(email: string, subject: string, pattern: RegExp) {
  const mailpit = requiredEnvironment('RSP_E2E_MAILPIT_URL');
  let link: string | undefined;
  await expect
    .poll(
      async () => {
        const response = await fetch(`${mailpit}/api/v1/messages`);
        const list = (await response.json()) as {
          messages: {
            ID: string;
            Subject: string;
            To: { Address: string }[];
          }[];
        };
        const message = list.messages.find(
          (m) =>
            m.Subject.includes(subject) &&
            m.To.some((r) => r.Address.toLowerCase() === email),
        );
        if (!message) return false;
        const body = (await (
          await fetch(`${mailpit}/api/v1/message/${message.ID}`)
        ).json()) as { Text: string };
        link = body.Text.match(pattern)?.[0];
        return Boolean(link);
      },
      { timeout: 20_000 },
    )
    .toBe(true);
  return link!;
}

// A stuck step fails by name after 30 seconds instead of hanging until the
// whole test times out.
test.use({ actionTimeout: 30_000 });

test('an invited new member verifies their email and joins the season', async ({
  page,
  browser,
}) => {
  test.setTimeout(180_000);
  const email = `invitation-${Date.now()}@example.test`;
  const password = 'RspInvitationE2eOnly-2026';
  // Provisioning and the earlier browser flows share one IP. Let the auth
  // bucket expire before testing an additional sign-in and new-account flow.
  await page.waitForTimeout(60_000);
  await signIn(page, 'DIRECTOR');
  await page.goto('/admin/enrollments');
  await chooseOption(page, 'Season', 'Development Season · Open');
  await page.getByRole('button', { name: 'Invite a Member' }).click();
  const invite = page.getByRole('dialog', { name: 'Invite a Member' });
  await invite.getByLabel('Name', { exact: true }).fill('Invited E2E Member');
  await invite.getByLabel('Email', { exact: true }).fill(email);
  await invite.getByRole('button', { name: 'Send Invitation' }).click();
  await expect(
    page.getByText(
      /^Invitation email queued for .+\. The recipient has 7 days to accept\.$/,
    ),
  ).toBeVisible();
  const link = await emailLink(
    email,
    'Your invitation to',
    /https?:\/\/[^\s]+\/invitations\/accept#token=[^\s]+/,
  );
  const recipientContext = await browser.newContext({
    baseURL: requiredEnvironment('RSP_E2E_BASE_URL'),
  });
  try {
    const recipient = await recipientContext.newPage();
    await recipient.goto(link);
    // A signed-out visitor with a good invitation goes straight to sign-up,
    // prefilled with the invited name and email.
    await expect(recipient).toHaveURL(/\/sign-in$/);
    await recipient.getByRole('tab', { name: 'Create Account' }).click();
    await recipient
      .getByLabel('Name', { exact: true })
      .fill('Invited E2E Member');
    await recipient.getByLabel('Email', { exact: true }).fill(email);
    await recipient.getByLabel('Password', { exact: true }).fill(password);
    await recipient.waitForTimeout(13_000);
    await recipient
      .getByRole('button', { name: 'Create Account', exact: true })
      .click();
    // The invitation link proved the inbox: no verification email, and the
    // new account is signed in and taken straight to the invitation.
    await expect(recipient).toHaveURL(/\/invitations\/accept$/);
    await recipient
      .getByRole('button', { name: 'Accept Invitation', exact: true })
      .click();
    // Accepting goes straight into the season.
    await expect(
      recipient.getByRole('heading', {
        name: 'Development Season',
        exact: true,
      }),
    ).toBeVisible();
    await page.reload();
    await page.getByPlaceholder('Search all columns').fill(email);
    await expect(
      page.getByText(email).filter({ visible: true }).first(),
    ).toBeVisible();
    // The accepted invitation shows on the new member's enrollment row.
    await expect(
      page.getByRole('row').filter({ hasText: email }),
    ).toContainText('Accepted');
  } finally {
    await recipientContext.close();
  }
});
