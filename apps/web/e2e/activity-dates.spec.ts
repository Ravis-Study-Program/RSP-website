import { expect, test } from '@playwright/test';

test('former students can record mocks today in Adelaide and filter their personal history', async ({
  page,
}, testInfo) => {
  await page.clock.setFixedTime(new Date('2026-09-06T07:30:00Z'));
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'kicked'),
  );
  await page.goto('/mock-interviews');
  await expect(
    page.getByRole('heading', { name: 'Mock interviews', exact: true }),
  ).toBeVisible();
  await page.getByRole('button', { name: 'New interview' }).click();
  const create = page.getByRole('dialog', { name: 'Record mock interview' });
  await expect(create.getByText(/Today in Adelaide: 2026-09-06/)).toBeVisible();
  await expect(create.locator('input[type="date"]')).toHaveCount(0);
  await expect(create.getByLabel('Round type')).toHaveAttribute(
    'id',
    /^round-type-[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/,
  );
  await create.getByLabel('Interviewee').selectOption('person_amelia');
  await create.getByLabel('Time (Adelaide)').fill('23:00');
  await create.getByRole('button', { name: 'Save interview' }).click();
  await expect(create.getByText(/Future times are not allowed/)).toBeVisible();
  await create.getByLabel('Time (Adelaide)').fill('16:30');
  await create.screenshot({
    path: testInfo.outputPath('adelaide-recording.png'),
  });
  await create.getByRole('button', { name: 'Save interview' }).click();
  await expect(create).toBeHidden();
  await page.getByRole('tab', { name: 'Given' }).click();
  await expect(
    page
      .getByRole(
        testInfo.project.name.includes('mobile') ? 'heading' : 'link',
        { name: 'Amelia Chen', exact: true },
      )
      .first(),
  ).toBeVisible();
  await page.getByRole('button', { name: 'Edit details' }).first().click();
  const edit = page.getByRole('dialog', { name: 'Edit interview details' });
  await expect(edit.getByText(/Recorded:/)).toBeVisible();
  await expect(
    edit.locator('input[type="date"], input[type="time"]'),
  ).toHaveCount(0);
  await edit.getByLabel('Duration (minutes)').fill('65');
  await edit.getByRole('button', { name: 'Save interview' }).click();
  await expect(edit).toBeHidden();
  await page.getByLabel('Year (Adelaide time)').selectOption('2025');
  await expect(page.getByRole('button', { name: 'Edit details' })).toHaveCount(
    0,
  );
  await page.getByLabel('Year (Adelaide time)').selectOption('2026');
  await expect(
    page
      .getByText(/65 min/)
      .filter({ visible: true })
      .first(),
  ).toBeVisible();
  await page.goto('/practice');
  await expect(
    page.getByRole('heading', { name: 'Problem practice', exact: true }),
  ).toBeVisible();
  await page.getByLabel('Year (Adelaide time)').selectOption('2025');
  await expect(
    page.getByText('Binary Tree Level Order Traversal', { exact: true }),
  ).toHaveCount(0);
  await page.goto('/graduates');
  await expect(page).toHaveURL(/\/forbidden$/);
});
