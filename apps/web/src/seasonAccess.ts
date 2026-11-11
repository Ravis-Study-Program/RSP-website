import type { useCurrentUser } from '@/api/queries';

type CurrentUser = NonNullable<ReturnType<typeof useCurrentUser>['data']>;

/** What the signed-in member may do in one season. */
export function seasonAccess(user: CurrentUser | undefined, seasonId: string) {
  const hasSeasonAccess = Boolean(
    user &&
    (user.globalRoles.length > 0 ||
      user.alumni ||
      user.seasonRoles.some(
        (membership) =>
          membership.state === 'active' ||
          (membership.seasonId === seasonId &&
            membership.state === 'completed'),
      )),
  );
  const isGlobalManager = Boolean(
    user?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ),
  );
  const isCoordinator = Boolean(
    user?.seasonRoles.some(
      (membership) =>
        membership.seasonId === seasonId &&
        membership.role === 'coordinator' &&
        membership.state === 'active',
    ),
  );
  return { hasSeasonAccess, isGlobalManager, isCoordinator };
}
