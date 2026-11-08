import { demoUser, seasons } from '@/data/demo';
import { navigationItems, navigationItemActive } from '@/navigation';
import type { CurrentUser, SeasonRole } from '@/types';

function userWith(...seasonRoles: CurrentUser['seasonRoles']): CurrentUser {
  return { ...demoUser, globalRoles: [], alumni: false, seasonRoles };
}

function membership(
  role: SeasonRole,
  index = 0,
  state: CurrentUser['seasonRoles'][number]['state'] = 'active',
) {
  return {
    role,
    state,
    seasonId: seasons[index].id,
    seasonSlug: seasons[index].slug,
  };
}

describe('shared navigation', () => {
  it.each(['student', 'mentor', 'coordinator'] as const)(
    'keeps the same personal activity routes for a %s',
    (role) => {
      const items = navigationItems(userWith(membership(role)), seasons);
      expect(items).toEqual(
        expect.arrayContaining([
          expect.objectContaining({ label: 'Dashboard', to: '/dashboard' }),
          expect.objectContaining({ label: 'LeetCode', to: '/leetcode' }),
          expect.objectContaining({
            label: 'Mock Interviews',
            to: '/mock-interviews',
          }),
          expect.objectContaining({ label: 'People', to: '/people' }),
          expect.objectContaining({ label: 'Seasons', to: '/seasons' }),
        ]),
      );
      if (role === 'student')
        expect(items.map((item) => item.label)).not.toEqual(
          expect.arrayContaining([
            'My Students',
            'All Students',
            'Administration',
            'Season Operations',
          ]),
        );
    },
  );

  it('adds both staff responsibilities when a member has different roles across seasons', () => {
    const items = navigationItems(
      userWith(membership('mentor'), membership('coordinator', 1)),
      seasons,
    );
    expect(items).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          label: 'My Students',
          to: '/seasons/summer-2025-26/mentees',
        }),
        expect.objectContaining({
          label: 'All Students',
          to: '/seasons/summer-2024-25/mentees',
        }),
        expect.objectContaining({ label: 'Season Operations' }),
      ]),
    );
  });

  it('prefers active staff membership over earlier completed membership', () => {
    const items = navigationItems(
      userWith(membership('mentor', 1, 'completed'), membership('mentor')),
      seasons,
    );
    expect(items.find((item) => item.label === 'My Students')?.to).toBe(
      '/seasons/summer-2025-26/mentees',
    );
  });

  it('keeps completed mentors historical review and personal activity together', () => {
    const items = navigationItems(
      userWith(membership('mentor', 1, 'completed')),
      seasons,
    );
    expect(items.map((item) => item.label)).toEqual([
      'Dashboard',
      'LeetCode',
      'Mock Interviews',
      'Jobs',
      'My Students',
      'Monday Updates',
      'Seasons',
    ]);
    expect(items.find((item) => item.label === 'My Students')?.to).toBe(
      '/seasons/summer-2024-25/mentees',
    );
  });

  it('does not expose operations or Monday reviews to completed coordinators', () => {
    const items = navigationItems(
      userWith(membership('coordinator', 1, 'completed')),
      seasons,
    );
    expect(items.map((item) => item.label)).toEqual([
      'Dashboard',
      'LeetCode',
      'Mock Interviews',
      'Jobs',
      'Seasons',
      'All Students',
    ]);
  });

  it.each(['kicked', 'withdrawn'] as const)(
    'limits a %s member to personal pages',
    (state) => {
      const items = navigationItems(
        userWith(membership('coordinator', 0, state)),
        seasons,
      );
      expect(items.map((item) => item.to)).toEqual([
        '/dashboard',
        '/leetcode',
        '/mock-interviews',
        '/jobs',
      ]);
    },
  );

  it('keeps global administration when the account also has a season role', () => {
    const items = navigationItems(demoUser, seasons);
    expect(items).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ label: 'All Students' }),
        expect.objectContaining({ label: 'Administration', to: '/admin' }),
        expect.objectContaining({
          label: 'Mock Interviews',
          to: '/mock-interviews',
        }),
      ]),
    );
  });

  it('does not invent a season link for an administrator when no seasons exist', () => {
    const items = navigationItems({ ...demoUser, seasonRoles: [] });
    expect(items.some((item) => item.to.includes('undefined'))).toBe(false);
    expect(items.find((item) => item.label === 'Administration')).toBeDefined();
    expect(
      items.find((item) => item.label === 'Monday Updates'),
    ).toBeUndefined();
  });

  it('gives a verified nonmember their profile and Jobs', () => {
    expect(navigationItems(userWith()).map((item) => item.to)).toEqual([
      '/profile',
      '/jobs',
    ]);
  });

  it('marks shared activity navigation active on historical season URLs', () => {
    const items = navigationItems(userWith(membership('mentor')), seasons);
    const leetcode = items.find((item) => item.to === '/leetcode')!;
    expect(
      navigationItemActive(leetcode, '/seasons/summer-2024-25/leetcode'),
    ).toBe(true);
    expect(navigationItemActive(leetcode, '/mock-interviews')).toBe(false);
  });
});
