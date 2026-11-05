import type { BadgeTone } from '@/badgeTones';
import type { CurrentUser, Person, Season } from '@/types';

export const studentLevels = {
  novice: 'Novice',
  beginner: 'Beginner',
  intermediate: 'Intermediate',
  advanced: 'Advanced',
} as const;

/** Each level's badge colours, from grey for beginners up to purple. */
export const studentLevelTone: Record<keyof typeof studentLevels, BadgeTone> = {
  beginner: 'muted',
  novice: 'success',
  intermediate: 'info',
  advanced: 'purple',
};

export function studentLevelLabel(level?: Person['studentLevel']) {
  return level && level !== 'not_applicable' ? studentLevels[level] : '—';
}

export function canSetStudentLevel(user?: CurrentUser, season?: Season) {
  return Boolean(
    user &&
    season?.status === 'open' &&
    user.emailVerified &&
    user.accountState === 'active' &&
    (user.globalRoles.length ||
      user.seasonRoles.some(
        (membership) =>
          membership.seasonId === season.id &&
          membership.state === 'active' &&
          ['mentor', 'coordinator'].includes(membership.role),
      )),
  );
}
