import { useEffect } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import type { Participation } from '@/api/generated/models';
import { useSeasons, useUserParticipation } from '@/api/queries';
import type { Season } from '@/types';

const seasonPreferenceKey = 'rsp-activity-season';

/** Remembers a table's season choice, or All time, for every table. */
export function rememberActivitySeason(seasonId: string | undefined) {
  try {
    localStorage.setItem(seasonPreferenceKey, seasonId ?? 'all');
  } catch {
    // Storage can be unavailable; the choice then lasts for this page only.
  }
}

/** The remembered season ID, 'all' for All time, or null if none. */
export function rememberedActivitySeason() {
  try {
    return localStorage.getItem(seasonPreferenceKey);
  } catch {
    return null;
  }
}

/**
 * The season a LeetCode or mock interview table shows. With `section`, the
 * season last chosen in any table is applied unless the address names a
 * season (`?seasonId`) or weeks; season links are only a default,
 * so the remembered choice (including All time) replaces them.
 */
export function useActivitySeason(section?: 'leetcode' | 'mock-interviews') {
  const { slug } = useParams();
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const seasons = useSeasons();
  const selectedId = params.get('seasonId');
  const season = seasons.data?.items.find((item) =>
    slug ? item.slug === slug : item.id === selectedId,
  );
  useEffect(() => {
    // An address that names a season or its weeks (a link, or going back)
    // is explicit; only plain table and season links are defaults.
    if (!section || !seasons.data || selectedId || params.has('weekId')) return;
    const remembered = rememberedActivitySeason();
    if (!remembered) return;
    const target = seasons.data.items.find((item) => item.id === remembered);
    if (remembered !== 'all' && (!target || target.slug === slug)) return;
    if (remembered === 'all' && !slug) return;
    // Weeks belong to the season being left.
    const next = new URLSearchParams(params);
    next.delete('weekId');
    next.delete('year');
    if (target && !slug) next.set('seasonId', target.id);
    navigate(
      {
        pathname:
          target && slug ? `/seasons/${target.slug}/${section}` : `/${section}`,
        search: next.toString(),
      },
      { replace: true },
    );
  }, [navigate, params, section, seasons.data, selectedId, slug]);
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
