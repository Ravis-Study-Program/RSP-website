import { calendarDateKey } from '@/utils';

export const programmeTimezone = 'Australia/Adelaide';

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
 * the current week on Saturday or Sunday, otherwise the previous week.
 */
export function defaultMondayUpdateRange(
  now: Date = new Date(),
): MondayUpdateRange {
  const today = calendarDateKey(now, programmeTimezone);
  const weekday = calendarWeekday(today);
  // Monday is 0 days into its own week from Tuesday onwards, and the current
  // week is 7 days ahead of the previous one for Monday to Friday.
  const daysBack = weekday <= 5 ? weekday + 6 : weekday - 1;
  const startDate = shiftCalendarDate(today, -daysBack);
  return { startDate, endDate: shiftCalendarDate(startDate, 6) };
}
