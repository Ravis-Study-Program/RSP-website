import type { BadgeTone } from '@/badgeTones';
import { programTimezone, type MondayUpdateRange } from '@/activityDates';
import { calendarDateKey } from '@/utils';

/**
 * Where a week's Monday update stands for a student, shared by the Monday
 * Updates page and the student lists.
 */
export type StudentUpdateStatus =
  'Submitted' | 'Submitted (Late)' | 'Not submitted' | 'Overdue' | 'Kicked';

export const updateStatusTones: Record<StudentUpdateStatus, BadgeTone> = {
  Submitted: 'success',
  'Submitted (Late)': 'success',
  'Not submitted': 'warning',
  Overdue: 'danger',
  Kicked: 'danger',
};

/**
 * A week's update is due by the end of the Monday after it ends, in
 * Adelaide: the Adelaide calendar date it is due, as yyyy-mm-dd.
 */
export function dueDateKey(range: MondayUpdateRange) {
  const monday = new Date(`${range.endDate}T00:00:00Z`);
  monday.setUTCDate(monday.getUTCDate() + 1);
  return monday.toISOString().slice(0, 10);
}

/** Whether a date is past the week's due date, in Adelaide. */
export function pastDue(
  range: MondayUpdateRange,
  at: string | Date = new Date(),
) {
  return calendarDateKey(at, programTimezone) > dueDateKey(range);
}

/** An update posted, or last edited, after its due date is late. */
export function isLate(
  update: { createdAt: string; editedAt?: string | null },
  range: MondayUpdateRange,
) {
  return pastDue(range, update.editedAt ?? update.createdAt);
}

/**
 * A week's status. A student kicked that week, with nothing posted, shows as
 * kicked rather than overdue; their mentor still marks it.
 */
export function weekStatus(
  range: MondayUpdateRange,
  update?: { createdAt: string; editedAt?: string | null },
  kickedAt?: string | null,
): StudentUpdateStatus {
  if (update) return isLate(update, range) ? 'Submitted (Late)' : 'Submitted';
  if (kickedAt) return 'Kicked';
  return pastDue(range) ? 'Overdue' : 'Not submitted';
}
