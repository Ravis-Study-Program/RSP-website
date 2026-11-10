import { afterEach, describe, expect, it } from 'vitest';

import {
  supportedTimezones,
  calendarDateKey,
  configureDisplayTimezone,
  formatDateTime,
  formatDateTimeInput,
  isValidEmail,
  problemName,
  zonedDateTimeToUtc,
} from '@/utils';

describe('saved display timezone', () => {
  afterEach(() => configureDisplayTimezone('UTC'));

  it('moves a UTC Sunday boundary into Monday in Australia/Adelaide', () => {
    const instant = '2026-08-16T15:30:00.000Z';
    expect(calendarDateKey(instant, 'UTC')).toBe('2026-08-16');
    expect(calendarDateKey(instant, 'Australia/Adelaide')).toBe('2026-08-17');

    configureDisplayTimezone('Australia/Adelaide');
    expect(formatDateTime(instant)).toBe('17/08/26, 01:00');
    expect(formatDateTimeInput(instant)).toBe('2026-08-17T01:00');
    expect(zonedDateTimeToUtc('2026-08-17T01:00')).toBe(instant);
  });
});

describe('problemName', () => {
  it('prefixes the number once', () => {
    expect(problemName({ number: 1, title: 'Two Sum' })).toBe('1. Two Sum');
    expect(
      problemName({ number: 3379, title: '3379. Transformed Array' }),
    ).toBe('3379. Transformed Array');
  });

  it('keeps titles that only start with a digit', () => {
    expect(problemName({ number: 15, title: '3Sum' })).toBe('15. 3Sum');
  });
});

it('lists time zones from the smallest UTC offset to the largest', () => {
  const zones = supportedTimezones('Australia/Adelaide');
  const at = (zone: string) => zones.indexOf(zone);
  expect(at('Pacific/Honolulu')).toBeGreaterThanOrEqual(0);
  expect(at('Pacific/Honolulu')).toBeLessThan(at('UTC'));
  expect(at('UTC')).toBeLessThan(at('Australia/Adelaide'));
});

describe('isValidEmail', () => {
  it('needs a dotted domain', () => {
    expect(isValidEmail('name@example.com')).toBe(true);
    expect(isValidEmail(' a.b@mail.co.uk ')).toBe(true);
    expect(isValidEmail('coord-maddie@mail')).toBe(false);
    expect(isValidEmail('name@example.')).toBe(false);
    expect(isValidEmail('no at.com')).toBe(false);
  });
});
