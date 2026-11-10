import { useEffect, type ReactNode } from 'react';
import { selectSeason, useSelectedSeasonSlug } from '@/selectedSeason';
import { useNavigate, useParams } from 'react-router-dom';

import { useCurrentUser, useSeasons } from '@/api/queries';
import type { Season } from '@/types';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

const allValue = 'all';

/**
 * The seasons this person manages (all of them for a Director or System Admin,
 * the ones they coordinate for a coordinator); empty for anyone else.
 */
export function useManagedSeasons(): Season[] {
  const seasons = useSeasons();
  const user = useCurrentUser();
  const coordinated = new Set(
    user.data?.seasonRoles
      .filter(
        (membership) =>
          membership.role === 'coordinator' && membership.state === 'active',
      )
      .map((membership) => membership.seasonId) ?? [],
  );
  return (seasons.data?.items ?? []).filter(
    (item) =>
      Boolean(user.data?.globalRoles.length) || coordinated.has(item.id),
  );
}

/**
 * A season dropdown for the top-left of a season's tables, switching to the
 * same page in another season. Only Directors, System Admins and coordinators
 * get one (coordinators for the seasons they coordinate); everyone else gets
 * null and sees no switch.
 */
export function useSeasonSwitcher(
  page: string,
  /** Adds an "All seasons" choice that opens this path instead. */
  allSeasons?: string,
  /** The seasons to offer instead of the ones this person manages, for
   *  anyone with some (such as the seasons a mentor has mentored in). */
  offered?: Season[],
): ReactNode {
  const { slug } = useParams();
  const navigate = useNavigate();
  const managedSeasons = useManagedSeasons();
  const managed = offered ?? managedSeasons;
  const selected = useSelectedSeasonSlug();
  // Being on a season's page makes it the selected season everywhere.
  useEffect(() => {
    if (slug && managed.some((item) => item.slug === slug)) selectSeason(slug);
  }, [slug, managed]);
  if (!managed.length) return null;
  const items = [
    ...(allSeasons ? [{ label: 'All seasons', value: allValue }] : []),
    ...managed.map((item) => ({ label: item.name, value: item.slug })),
  ];
  const current =
    slug ??
    (allSeasons
      ? allValue
      : managed.some((item) => item.slug === selected)
        ? selected
        : '');
  return (
    <Select
      items={items}
      value={current}
      onValueChange={(value) => {
        if (!value) return;
        if (value === allValue && allSeasons) {
          navigate(allSeasons);
          return;
        }
        selectSeason(value);
        navigate(`/seasons/${value}/${page}`);
      }}
    >
      <SelectTrigger aria-label="Season" className="w-64">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  );
}
