import type { UserPrivate } from '@/api/generated/models';
import type { Person } from '@/types';
import { initials } from '@/utils';

/**
 * An account from the System Admin user list, as the People table shows it.
 * Unlike the directory it includes suspended accounts and registered
 * nonmembers, so their state is shown as the status.
 */
export function accountToPerson(account: UserPrivate): Person {
  const activeMembership = account.seasonRoles.find(
    (membership) => membership.state === 'active',
  );
  return {
    id: account.id,
    name: account.name,
    slug: account.slug,
    initials: initials(account.name),
    avatarUrl: account.avatarUrl ?? null,
    roles: [
      ...new Set([
        ...account.globalRoles,
        ...account.seasonRoles.map((membership) => membership.role),
      ]),
    ],
    season: activeMembership?.seasonSlug ?? null,
    status:
      account.accountState === 'active'
        ? activeMembership
          ? 'active'
          : 'unassigned'
        : account.accountState,
    attempts: account.attemptCount,
    interviews: account.mockInterviewCount,
    email: account.email,
    lastActiveAt: null,
  };
}
