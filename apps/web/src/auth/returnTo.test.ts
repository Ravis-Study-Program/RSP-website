import { safeJobsReturnTo } from '@/auth/returnTo';

it('only accepts Jobs return paths and safe focus identifiers', () => {
  expect(safeJobsReturnTo('/jobs')).toBe('/jobs');
  expect(safeJobsReturnTo('/jobs/applications?application=abc-123')).toBe(
    '/jobs/applications?application=abc-123',
  );
  expect(
    safeJobsReturnTo(
      '/jobs/referrals?company=company-Google&redirect=https://evil.example',
    ),
  ).toBe('/jobs/referrals?company=company-Google');
  expect(safeJobsReturnTo('/jobs/refer-candidates')).toBe(
    '/jobs/refer-candidates',
  );
  expect(safeJobsReturnTo('/jobs/referrals?view=refer')).toBe(
    '/jobs/refer-candidates',
  );
  for (const input of [
    'https://evil.example/jobs',
    '//evil.example/jobs',
    '//%/jobs',
    '/admin',
    '/jobs/../admin',
    'javascript:alert(1)',
  ])
    expect(safeJobsReturnTo(input)).toBeNull();
  expect(safeJobsReturnTo('/jobs?redirect=https://evil.example')).toBe('/jobs');
});
