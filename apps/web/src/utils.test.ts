import { afterEach, describe, expect, it } from 'vitest';

import {
  calendarDateKey,
  configureDisplayTimezone,
  formatDateTime,
  formatDateTimeInput,
  zonedDateTimeToUtc,
} from '@/utils';

describe('saved display timezone', () => {
  afterEach(() => configureDisplayTimezone('UTC'));

  it('moves a UTC Sunday boundary into Monday in Australia/Adelaide', () => {
    const instant = '2026-08-16T15:30:00.000Z';
    expect(calendarDateKey(instant, 'UTC')).toBe('2026-08-16');
    expect(calendarDateKey(instant, 'Australia/Adelaide')).toBe('2026-08-17');

    configureDisplayTimezone('Australia/Adelaide');
    expect(formatDateTime(instant)).toMatch(/(?:17 Aug|Aug 17).*2026/);
    expect(formatDateTimeInput(instant)).toBe('2026-08-17T01:00');
    expect(zonedDateTimeToUtc('2026-08-17T01:00')).toBe(instant);
  });
});
