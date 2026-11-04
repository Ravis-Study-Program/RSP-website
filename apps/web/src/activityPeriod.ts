import { useParams, useSearchParams } from 'react-router-dom';
import type { Participation } from '@/api/generated/models';
import { useSeasons, useUserParticipation } from '@/api/queries';
import type { Season } from '@/types';

export function useActivitySeason() {
  const { slug } = useParams();
  const [params] = useSearchParams();
  const seasons = useSeasons();
  const selectedId = params.get('seasonId');
  const season = seasons.data?.items.find((item) =>
    slug ? item.slug === slug : item.id === selectedId,
  );
  return { seasons, season, hasSelection: Boolean(slug || selectedId) };
}

/**
 * The seasons a member can have activity in: those where they were a student
 * at any point, or with `anyRole`, every season they took part in. Practice
 * and received mocks only belong to seasons where the member was a student, so
 * offering other seasons would show empty views and reveal seasons such as
 * test seasons. A selected season stays listed so the picker never goes blank.
 */
export function activitySeasons(
  seasons: Season[],
  participation: Participation[],
  { anyRole = false, selectedId }: { anyRole?: boolean; selectedId?: string },
) {
  const ids = new Set(
    participation
      .filter(
        (entry) =>
          anyRole ||
          entry.role === 'student' ||
          entry.periods.some((period) => period.role === 'student'),
      )
      .map((entry) => entry.seasonId),
  );
  return seasons.filter(
    (season) => ids.has(season.id) || season.id === selectedId,
  );
}

/** `activitySeasons` for one member, loading their participation. */
export function useActivitySeasons(
  userId: string | undefined,
  seasons: Season[],
  options: { anyRole?: boolean; selectedId?: string } = {},
) {
  const participation = useUserParticipation(userId);
  return activitySeasons(seasons, participation.data?.items ?? [], options);
}
