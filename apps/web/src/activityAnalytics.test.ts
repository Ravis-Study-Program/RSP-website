import { activitySeries, difficultyBreakdown } from './activityAnalytics';
import type { Attempt, Season } from './types';

it('charts a short summer season across New Year using Adelaide dates', () => {
  const season = {
    startsAt: '2024-11-30T13:30:00Z',
    endsAt: '2025-02-28T13:29:59Z',
  } as Season;
  const attempts = [
    { attemptedAt: '2024-01-04T12:00:00Z', difficulty: 'Easy' },
    { attemptedAt: '2024-12-31T14:00:00Z', difficulty: 'Hard' },
  ] as Attempt[];
  const data = activitySeries(attempts, [], { season });
  expect(data).toHaveLength(14);
  expect(data[0]).toMatchObject({ week: '25 Nov 2024', attempts: 0 });
  expect(data.find((bucket) => bucket.week === '30 Dec 2024')?.attempts).toBe(
    1,
  );
  expect(data.reduce((n, d) => n + d.attempts, 0)).toBe(1);
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
  expect(activitySeries([], [])).toEqual([]);
});
