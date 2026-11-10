import { expect, test } from '@playwright/test';

test('Discord is for returning members and explains unlinked accounts', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-auth-state', 'signed-out'),
  );
  await page.goto('/sign-in?provider=discord&error=signup_disabled');
  await expect(
    page.getByText(
      'Sign in with email or Google, then connect Discord in Settings.',
    ),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Continue with Discord' }),
  ).toBeVisible();
  await page.getByRole('tab', { name: 'Create Account' }).click();
  await expect(
    page.getByRole('button', { name: 'Continue with Discord' }),
  ).toHaveCount(0);
  await page.getByRole('tab', { name: 'Sign In', exact: true }).click();
  let body: Record<string, unknown> | undefined;
  await page.route('**/api/auth/sign-in/social', async (route) => {
    body = route.request().postDataJSON();
    await route.fulfill({
      status: 400,
      json: { message: 'Sign-in could not start. Try again.' },
    });
  });
  await page.getByRole('button', { name: 'Continue with Discord' }).click();
  await expect(
    page.getByText('Sign-in could not start. Try again.'),
  ).toBeVisible();
  expect(body).toMatchObject({
    provider: 'discord',
    callbackURL: '/dashboard',
    disableRedirect: true,
  });
});

test('Settings connects Discord and displays a recoverable stale-session error', async ({
  page,
}) => {
  await page.addInitScript(() => {
    localStorage.setItem('rsp-demo-role', 'student');
    localStorage.removeItem('rsp-demo-auth-state');
  });
  await page.route('**/api/auth/link-social', async (route) => {
    expect(route.request().postDataJSON()).toMatchObject({
      provider: 'discord',
    });
    await route.fulfill({
      status: 403,
      json: { message: 'Sign in again before connecting Discord.' },
    });
  });
  await page.goto('/settings');
  await expect(
    page.getByText(
      /^Sign in with your Discord account\. Your Discord profile picture will be shown on this site\./,
    ),
  ).toBeVisible();
  await page
    .getByRole('button', { name: 'Connect Discord', exact: true })
    .click();
  await expect(
    page.getByText('Sign in again before connecting Discord.'),
  ).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Connect Discord', exact: true }),
  ).toBeEnabled();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
});

test('signing out after a Discord change explains why across the top', async ({
  page,
}) => {
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-auth-state', 'signed-out'),
  );
  for (const [param, change] of [
    ['discordConnected', 'Discord was connected'],
    ['discordDisconnected', 'Discord was disconnected'],
  ]) {
    await page.goto(`/sign-in?${param}=true`);
    const notice = page.getByRole('status').filter({ hasText: change });
    await expect(notice).toHaveText(
      `${change}, so you were signed out on all devices. Sign in again to continue.`,
    );
    // It sits above the sign-in card, not inside it.
    const card = page.getByRole('heading', { name: 'Welcome to RSP' });
    expect((await notice.boundingBox())!.y).toBeLessThan(
      (await card.boundingBox())!.y,
    );
  }
});
