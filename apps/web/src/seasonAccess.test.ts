import { describe, expect, it } from 'vitest';
import { canRenameSelf } from '@/seasonAccess';
import type { CurrentUser } from '@/types';

function member(
  roles: Array<CurrentUser['seasonRoles'][number]['role']>,
  globalRoles: CurrentUser['globalRoles'] = [],
): CurrentUser {
  return {
    id: 'user-1',
    name: 'Jane Doe',
    slug: 'jane-doe',
    avatarUrl: null,
    email: 'jane@example.com',
    timezone: 'Australia/Adelaide',
    timezoneConfigured: true,
    emailVerified: true,
    mfaVerified: false,
    accountState: 'active',
    globalRoles,
    seasonRoles: roles.map((role, index) => ({
      seasonId: `season-${index}`,
      seasonSlug: `season-${index}`,
      role,
      state: 'active',
    })),
    alumni: false,
    attemptCount: 0,
    mockInterviewCount: 0,
  };
}

describe('canRenameSelf', () => {
  it('locks the name for students only', () => {
    expect(canRenameSelf(member(['student']))).toBe(false);
    expect(canRenameSelf(member([]))).toBe(true);
    expect(canRenameSelf(member(['mentor']))).toBe(true);
    expect(canRenameSelf(member(['student', 'mentor']))).toBe(true);
    expect(canRenameSelf(member(['student', 'coordinator']))).toBe(true);
    expect(canRenameSelf(member(['student'], ['director']))).toBe(true);
    expect(canRenameSelf(member(['student'], ['system_admin']))).toBe(true);
    expect(canRenameSelf(undefined)).toBe(false);
  });
});
