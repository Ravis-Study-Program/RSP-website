import { calendarDateKey } from '@/utils';
import { programmeTimezone } from '@/activityDates';
import type { MockInterviewDayCount } from '@/api/generated/models';
import type { Attempt, Season } from '@/types';

/** `mockDays` are server-aggregated programme-timezone day counts. */
export function activitySeries(
  attempts: Attempt[],
  mockDays: MockInterviewDayCount[] = [],
  period?: { season?: Season },
) {
  const dates = [
    ...attempts.map((a) => calendarDateKey(a.attemptedAt, programmeTimezone)),
    ...mockDays.map((d) => d.date),
  ].sort();
  const start = period?.season
    ? calendarDateKey(period.season.startsAt, programmeTimezone)
    : dates[0];
  const end = period?.season
    ? calendarDateKey(period.season.endsAt, programmeTimezone)
    : dates.at(-1);
  if (!start || !end || end < start) return [];
  const monthly = Date.parse(end) - Date.parse(start) > 120 * 86_400_000;
  const key = (day: string) => {
    if (monthly) return day.slice(0, 7);
    const date = new Date(day);
    date.setUTCDate(date.getUTCDate() - ((date.getUTCDay() + 6) % 7));
    return date.toISOString().slice(0, 10);
  };
  const buckets = new Map<
    string,
    { week: string; attempts: number; interviews: number }
  >();
  const cursor = new Date(monthly ? `${start.slice(0, 7)}-01` : key(start));
  while (cursor.toISOString().slice(0, 10) <= end) {
    const bucketKey = key(cursor.toISOString().slice(0, 10));
    buckets.set(bucketKey, {
      week: new Intl.DateTimeFormat('en-AU', {
        month: 'short',
        year: 'numeric',
        ...(monthly ? {} : { day: 'numeric' }),
        timeZone: 'UTC',
      }).format(cursor),
      attempts: 0,
      interviews: 0,
    });
    if (monthly) cursor.setUTCMonth(cursor.getUTCMonth() + 1);
    else cursor.setUTCDate(cursor.getUTCDate() + 7);
  }
  const add = (day: string, field: 'attempts' | 'interviews', count = 1) => {
    if (day < start || day > end) return;
    const bucket = buckets.get(key(day));
    if (bucket) bucket[field] += count;
  };
  for (const a of attempts)
    add(calendarDateKey(a.attemptedAt, programmeTimezone), 'attempts');
  for (const d of mockDays) add(d.date, 'interviews', d.count);
  return [...buckets.values()];
}

export function difficultyBreakdown(attempts: Attempt[]) {
  return ['Easy', 'Medium', 'Hard', 'Unknown']
    .map((name) => ({
      name,
      value: attempts.filter((a) => (a.difficulty ?? 'Unknown') === name)
        .length,
    }))
    .filter((item) => item.value > 0);
}
