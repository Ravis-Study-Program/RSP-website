import { describe, expect, it } from 'vitest';

import { defaultMondayUpdateRange, programmeTimezone } from '@/activityDates';
import { calendarDateKey } from '@/utils';

/** Midday-ish Adelaide instants, so each case lands on the calendar date named. */
const adelaideInstant = (dateKey: string) => new Date(`${dateKey}T04:00:00Z`);

describe('defaultMondayUpdateRange', () => {
  const weekdays = [
    ['Monday', '2026-10-12', '2026-10-05', '2026-10-11'],
    ['Tuesday', '2026-10-13', '2026-10-05', '2026-10-11'],
    ['Wednesday', '2026-10-14', '2026-10-05', '2026-10-11'],
    ['Thursday', '2026-10-15', '2026-10-05', '2026-10-11'],
    ['Friday', '2026-10-16', '2026-10-05', '2026-10-11'],
    ['Saturday', '2026-10-17', '2026-10-12', '2026-10-18'],
    ['Sunday', '2026-10-18', '2026-10-12', '2026-10-18'],
  ] as const;

  it.each(weekdays)(
    '%s defaults to %s … %s … %s',
    (weekday, today, startDate, endDate) => {
      const now = adelaideInstant(today);
      expect(calendarDateKey(now, programmeTimezone)).toBe(today);
      expect(defaultMondayUpdateRange(now)).toEqual({ startDate, endDate });
    },
  );

  it('uses the Adelaide calendar date across the October DST change', () => {
    // 2026-10-04 02:00 ACST becomes 03:00 ACDT, inside this Monday-Sunday range.
    const now = adelaideInstant('2026-10-03');
    expect(calendarDateKey(now, 'UTC')).toBe('2026-10-03');
    expect(defaultMondayUpdateRange(now)).toEqual({
      startDate: '2026-09-28',
      endDate: '2026-10-04',
    });
  });

  it('treats a Saturday in Adelaide as the current week, not the UTC Friday', () => {
    // 2026-10-16 is still Friday in UTC but already Saturday evening in Adelaide.
    const now = new Date('2026-10-16T20:00:00Z');
    expect(calendarDateKey(now, 'UTC')).toBe('2026-10-16');
    expect(calendarDateKey(now, programmeTimezone)).toBe('2026-10-17');
    expect(defaultMondayUpdateRange(now)).toEqual({
      startDate: '2026-10-12',
      endDate: '2026-10-18',
    });
  });

  it('treats an Adelaide Monday as the previous week, not the UTC Sunday', () => {
    const now = new Date('2026-10-18T20:00:00Z');
    expect(calendarDateKey(now, 'UTC')).toBe('2026-10-18');
    expect(calendarDateKey(now, programmeTimezone)).toBe('2026-10-19');
    expect(defaultMondayUpdateRange(now)).toEqual({
      startDate: '2026-10-12',
      endDate: '2026-10-18',
    });
  });

  it('ignores the browser timezone and the instant time of day', () => {
    const morning = new Date('2026-10-15T00:30:00Z');
    const evening = new Date('2026-10-15T13:00:00Z');
    const range = { startDate: '2026-10-05', endDate: '2026-10-11' };
    expect(calendarDateKey(morning, programmeTimezone)).toBe('2026-10-15');
    expect(calendarDateKey(evening, programmeTimezone)).toBe('2026-10-15');
    expect(defaultMondayUpdateRange(morning)).toEqual(range);
    expect(defaultMondayUpdateRange(evening)).toEqual(range);
  });
});
