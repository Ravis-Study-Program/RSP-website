import { activitySeries, difficultyBreakdown } from './activityAnalytics';
import type { Attempt, Season } from './types';

it('charts the selected historical year using Adelaide dates and includes empty months', () => {
  const attempts = [
    { attemptedAt: '2024-01-04T12:00:00Z', difficulty: 'Easy' },
    { attemptedAt: '2024-12-31T14:00:00Z', difficulty: 'Hard' },
  ] as Attempt[];
  const data = activitySeries(attempts, [], { year: 2024 });
  expect(data).toHaveLength(12);
  expect(data[0]).toMatchObject({ week: 'Jan 2024', attempts: 1 });
  expect(data.reduce((n, d) => n + d.attempts, 0)).toBe(1);
  expect(activitySeries(attempts, [], { year: 2025 })[0].attempts).toBe(1);
  expect(difficultyBreakdown(attempts)).toEqual([
    { name: 'Easy', value: 1 },
    { name: 'Hard', value: 1 },
  ]);
});

it('uses season dates even with no activity and keeps the full all-time range', () => {
  const season = {
    startsAt: '2024-01-01T00:00:00Z',
    endsAt: '2024-01-21T00:00:00Z',
  } as Season;
  expect(activitySeries([], [], { season })).toHaveLength(3);
  const data = activitySeries([
    { attemptedAt: '2023-02-01T00:00:00Z' },
    { attemptedAt: '2025-02-01T00:00:00Z' },
  ] as Attempt[]);
  expect(data).toHaveLength(25);
  expect(data[0].attempts).toBe(1);
  expect(data.at(-1)?.attempts).toBe(1);
  expect(activitySeries([], [], { season, year: 2025 })).toEqual([]);
});
