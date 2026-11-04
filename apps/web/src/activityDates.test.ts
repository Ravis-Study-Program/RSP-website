import { describe, expect, it } from 'vitest';

import {
  defaultMondayUpdateRange,
  programmeTimezone,
  mondayUpdateWeeks,
  completedMondayUpdateWeeks,
  mondayReminderWeek,
} from '@/activityDates';
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
    ['Saturday', '2026-10-17', '2026-10-05', '2026-10-11'],
    ['Sunday', '2026-10-18', '2026-10-05', '2026-10-11'],
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
      startDate: '2026-09-21',
      endDate: '2026-09-27',
    });
  });

  it('keeps the previous completed week on an Adelaide Saturday', () => {
    // 2026-10-16 is still Friday in UTC but already Saturday evening in Adelaide.
    const now = new Date('2026-10-16T20:00:00Z');
    expect(calendarDateKey(now, 'UTC')).toBe('2026-10-16');
    expect(calendarDateKey(now, programmeTimezone)).toBe('2026-10-17');
    expect(defaultMondayUpdateRange(now)).toEqual({
      startDate: '2026-10-05',
      endDate: '2026-10-11',
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

describe('numbered programme reporting weeks', () => {
  const weeks = [
    {
      id: 'w1',
      seasonId: 's',
      number: 1,
      startAt: '2026-10-04T13:30:00Z',
      endAt: '2026-10-11T13:29:59Z',
      resourceUrl: '',
    },
    {
      id: 'w2',
      seasonId: 's',
      number: 2,
      startAt: '2026-10-11T13:30:00Z',
      endAt: '2026-10-18T13:29:59Z',
      resourceUrl: '',
    },
  ];
  it('uses the Adelaide Monday and keeps programme numbering', () => {
    expect(mondayUpdateWeeks(weeks)[0]).toEqual({
      id: 'w1',
      number: 1,
      startDate: '2026-10-05',
      endDate: '2026-10-11',
    });
  });
  it('makes earlier weeks available only after Sunday ends', () => {
    expect(
      completedMondayUpdateWeeks(weeks, new Date('2026-10-11T13:29:59Z')),
    ).toHaveLength(0);
    expect(
      completedMondayUpdateWeeks(weeks, new Date('2026-10-11T13:30:00Z')).map(
        (w) => w.id,
      ),
    ).toEqual(['w1']);
    expect(
      completedMondayUpdateWeeks(weeks, new Date('2026-11-01T00:00:00Z')),
    ).toHaveLength(2);
  });
  it('reminds on Monday only, during an active week, about the previous week', () => {
    expect(
      mondayReminderWeek(weeks, new Date('2026-10-11T13:30:00Z'))?.number,
    ).toBe(1);
    expect(
      mondayReminderWeek(weeks, new Date('2026-10-12T13:30:00Z')),
    ).toBeUndefined();
    expect(
      mondayReminderWeek(weeks, new Date('2026-10-18T13:30:00Z')),
    ).toBeUndefined();
    expect(
      mondayReminderWeek(weeks, new Date('2026-10-04T13:30:00Z')),
    ).toBeUndefined();
  });
});
