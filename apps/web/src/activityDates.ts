import type { Week } from '@/api/generated/models';
import { calendarDateKey } from '@/utils';

export const programTimezone = 'Australia/Adelaide';

export interface MondayUpdateRange {
  startDate: string; // YYYY-MM-DD
  endDate: string; // YYYY-MM-DD
}

/** Shift a YYYY-MM-DD calendar date by whole days without touching local time. */
function shiftCalendarDate(dateKey: string, days: number) {
  const date = new Date(`${dateKey}T00:00:00Z`);
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

/** Calendar weekday of a YYYY-MM-DD date, 1 = Monday … 7 = Sunday. */
function calendarWeekday(dateKey: string) {
  const weekday = new Date(`${dateKey}T00:00:00Z`).getUTCDay();
  return weekday === 0 ? 7 : weekday;
}

/**
 * The Monday-to-Sunday range a Monday update defaults to, in Australia/Adelaide:
 * always the previous completed week, including on weekends.
 */
export function defaultMondayUpdateRange(
  now: Date = new Date(),
): MondayUpdateRange {
  const today = calendarDateKey(now, programTimezone);
  const weekday = calendarWeekday(today);
  const daysBack = weekday + 6;
  const startDate = shiftCalendarDate(today, -daysBack);
  return { startDate, endDate: shiftCalendarDate(startDate, 6) };
}

export interface MondayUpdateWeek extends MondayUpdateRange {
  id: string;
  number: number;
}

/** Program numbering is shared with the schedule; reporting dates are fixed. */
export function mondayUpdateWeeks(weeks: Week[]): MondayUpdateWeek[] {
  return weeks
    .map((week) => {
      const day = calendarDateKey(week.startAt, programTimezone);
      const startDate = shiftCalendarDate(day, 1 - calendarWeekday(day));
      return {
        id: week.id,
        number: week.number,
        startDate,
        endDate: shiftCalendarDate(startDate, 6),
      };
    })
    .sort(
      (a, b) => a.startDate.localeCompare(b.startDate) || a.number - b.number,
    );
}

export function completedMondayUpdateWeeks(weeks: Week[], now = new Date()) {
  const today = calendarDateKey(now, programTimezone);
  return mondayUpdateWeeks(weeks).filter((week) => week.endDate < today);
}

export function sameUpdateWeek(
  update: MondayUpdateRange,
  week: MondayUpdateRange,
) {
  return update.startDate === week.startDate && update.endDate === week.endDate;
}

/** A Monday reminder is due only during an active program week. */
export function mondayReminderWeek(weeks: Week[], now = new Date()) {
  const today = calendarDateKey(now, programTimezone);
  if (
    calendarWeekday(today) !== 1 ||
    !weeks.some(
      (week) =>
        calendarDateKey(week.startAt, programTimezone) <= today &&
        calendarDateKey(week.endAt, programTimezone) >= today,
    )
  )
    return undefined;
  const previous = defaultMondayUpdateRange(now);
  return mondayUpdateWeeks(weeks).find((week) =>
    sameUpdateWeek(week, previous),
  );
}
