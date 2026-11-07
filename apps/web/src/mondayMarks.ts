import type { BadgeTone } from '@/badgeTones';
import type {
  MondayUpdateMark,
  MondayUpdateMarkRecord,
} from '@/api/generated/models';

/** Mentor marks for a Monday update, strongest first. */
export const markOptions: { value: MondayUpdateMark; label: string }[] = [
  { value: 'way_ahead', label: 'Way ahead' },
  { value: 'doing_good', label: 'Doing good' },
  { value: 'on_track', label: 'On track' },
  { value: 'a_bit_behind', label: 'A bit behind' },
  { value: 'at_risk', label: 'At risk' },
  { value: 'kicked', label: 'Kicked' },
];

export function markLabel(mark: MondayUpdateMark) {
  return markOptions.find((option) => option.value === mark)?.label ?? mark;
}

/**
 * One soft badge colour per mark, like every other badge, strongest first:
 * way ahead purple, doing good blue, on track green, a bit behind yellow,
 * at risk orange, kicked red.
 */
export const markTone: Record<
  MondayUpdateMark,
  { badge: BadgeTone; dot: string }
> = {
  way_ahead: { badge: 'purple', dot: 'bg-purple' },
  doing_good: { badge: 'info', dot: 'bg-info' },
  on_track: { badge: 'success', dot: 'bg-success' },
  a_bit_behind: { badge: 'warning', dot: 'bg-warning' },
  at_risk: { badge: 'orange', dot: 'bg-orange' },
  kicked: { badge: 'danger', dot: 'bg-destructive' },
};

/** The fields of an update a latest-mark summary needs. */
export interface MarkedUpdate {
  startDate: string;
  endDate: string;
  createdAt: string;
  marks: MondayUpdateMarkRecord[];
}

/** A student's latest update and the newest mark saved on it, if any. */
export interface LatestMark {
  update: MarkedUpdate;
  mark?: MondayUpdateMarkRecord;
}

/**
 * The latest update by start date (then posting time) and its most recently
 * saved mark; normally one mentor marks, but if several did the newest wins.
 */
export function pickLatestMark(
  updates: MarkedUpdate[],
): LatestMark | undefined {
  let latest: MarkedUpdate | undefined;
  for (const update of updates)
    if (
      !latest ||
      update.startDate > latest.startDate ||
      (update.startDate === latest.startDate &&
        update.createdAt > latest.createdAt)
    )
      latest = update;
  if (!latest) return undefined;
  const mark = [...latest.marks].sort((left, right) =>
    right.updatedAt.localeCompare(left.updatedAt),
  )[0];
  return { update: latest, mark };
}

/**
 * Sort rank: At risk first through Way ahead, then an unmarked latest update,
 * then no update; anything without data sorts last.
 */
export function latestMarkRank(latest: LatestMark | undefined | null) {
  // Marks rank above every unmarked state, strongest highest.
  if (latest === null) return markOptions.length + 2;
  if (!latest) return markOptions.length + 1;
  if (!latest.mark) return markOptions.length;
  return (
    markOptions.length -
    1 -
    markOptions.findIndex((option) => option.value === latest.mark!.mark)
  );
}
