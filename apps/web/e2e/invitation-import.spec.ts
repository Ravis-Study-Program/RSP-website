import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }, testInfo) => {
  test.skip(
    testInfo.project.name.includes('mobile'),
    'Desktop spreadsheet import',
  );
  await page.addInitScript(() =>
    localStorage.setItem('rsp-demo-role', 'director'),
  );
});

test('administrators invite people from a spreadsheet, with bad rows held back', async ({
  page,
}) => {
  await page.goto('/admin/enrollments');
  await page.getByRole('button', { name: 'Import Spreadsheet' }).click();
  const dialog = page.getByRole('dialog', { name: 'Import Invitations' });
  await dialog.getByLabel('Spreadsheet').setInputFiles({
    name: 'season.csv',
    mimeType: 'text/csv',
    buffer: Buffer.from(
      [
        'Name,Role,Email,Mentor,Season,Slug',
        'Alex Chen,student,alex@example.com,Noah Williams,Summer 2025/26,alex-c',
        'Sam Lee,Mentor,sam@example.com,,summer-2025-26,',
        '"Lee, Jordan",student,not-an-email,,Summer 2025/26,',
        'Kai,teacher,kai@example.com,,Summer 2025/26,',
        'Alex Again,student,ALEX@example.com,,summer-2025-26,',
      ].join('\n'),
    ),
  });
  const rows = dialog.getByRole('table', { name: 'Rows in season.csv' });
  await expect(rows.getByRole('row')).toHaveCount(6);
  await expect(
    rows.getByRole('row').filter({ hasText: 'Alex Again' }),
  ).toContainText('New');
  await expect(rows).toContainText('Lee, Jordan');
  await expect(
    rows.getByRole('row').filter({ hasText: 'Alex Chen' }),
  ).toContainText('alex-c');
  await expect(
    rows.getByRole('row').filter({ hasText: 'not-an-email' }),
  ).toContainText('Error: Add a valid email address.');
  await expect(rows).toContainText(
    'Role must be student, mentor or coordinator.',
  );
  await expect(
    rows.getByRole('row').filter({ hasText: 'Alex Chen' }),
  ).toContainText('New');
  await dialog.getByRole('button', { name: 'Import 3 People' }).click();
  await expect(dialog.getByRole('status')).toHaveText('3 invited.');
  await expect(
    rows.getByRole('row').filter({ hasText: 'Alex Chen' }),
  ).toContainText('Invited');
  await dialog.getByRole('button', { name: 'Done' }).click();
  await expect(dialog).toBeHidden();
});

test('a mass import creates seasons first, then invites people to them', async ({
  page,
}) => {
  await page.goto('/admin');
  await page.getByRole('button', { name: 'Mass Import' }).click();
  const dialog = page.getByRole('dialog', { name: 'Mass Import' });
  await dialog.getByLabel('Spreadsheets').setInputFiles([
    {
      name: 'people.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(
        'name,role,email,mentor,season\nAlex Chen,student,alex@example.com,,Autumn 2027\nSam Lee,mentor,sam@example.com,,summer-2025-26',
      ),
    },
    {
      name: 'seasons.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from(
        [
          'season,location,week,start,end,resources',
          'Autumn 2027,Adelaide,1,2027-03-01,2027-03-07,"Arrays',
          '- https://leetcode.com/problems/two-sum/"',
          'Autumn 2027,,2,8/3/2027,14/3/2027,',
        ].join('\n'),
      ),
    },
    {
      name: 'notes.csv',
      mimeType: 'text/csv',
      buffer: Buffer.from('a,b\n1,2'),
    },
  ]);
  const seasonsTable = dialog.getByRole('table', {
    name: 'Seasons in seasons.csv',
  });
  await expect(seasonsTable).toContainText('autumn-2027');
  await expect(seasonsTable.getByRole('row')).toHaveCount(3);
  await expect(seasonsTable).toContainText('Arrays');
  await expect(seasonsTable).toContainText('14/03/27');
  await expect(seasonsTable).toContainText('New');
  const people = dialog.getByRole('table', { name: 'Rows in people.csv' });
  await expect(
    people.getByRole('row').filter({ hasText: 'Alex Chen' }),
  ).toContainText('New');
  await expect(dialog.getByRole('region', { name: 'notes.csv' })).toContainText(
    'Not recognised',
  );
  await dialog
    .getByRole('button', { name: 'Create 1 Season and Import 2 People' })
    .click();
  await expect(dialog.getByRole('status')).toHaveText(
    '1 season created, 2 invited.',
  );
  await expect(seasonsTable).toContainText('Created');
});
