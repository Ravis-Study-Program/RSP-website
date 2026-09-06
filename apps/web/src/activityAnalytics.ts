import { calendarDateKey } from '@/utils';
import { programmeTimezone } from '@/activityDates';
import type { Attempt, MockInterview, Season } from '@/types';

export function activitySeries(
  attempts: Attempt[],
  mocks: MockInterview[] = [],
  period?: { season?: Season },
) {
  const dates = [
    ...attempts.map((a) => a.attemptedAt),
    ...mocks.map((m) => m.occurredAt),
  ]
    .map((d) => calendarDateKey(d, programmeTimezone))
    .sort();
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
  for (const [items, field] of [
    [attempts.map((a) => a.attemptedAt), 'attempts'],
    [mocks.map((m) => m.occurredAt), 'interviews'],
  ] as const) {
    for (const at of items) {
      const day = calendarDateKey(at, programmeTimezone);
      if (day < start || day > end) continue;
      const bucket = buckets.get(key(day));
      if (bucket) bucket[field]++;
    }
  }
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
