import {
  difficultyBreakdown,
  difficultySuccessRates,
} from './activityAnalytics';
import type { Attempt } from './types';

it('counts attempts by difficulty', () => {
  const attempts = [
    { attemptedAt: '2024-01-04T12:00:00Z', difficulty: 'Easy' },
    { attemptedAt: '2024-12-31T14:00:00Z', difficulty: 'Hard' },
  ] as Attempt[];
  expect(difficultyBreakdown(attempts)).toEqual([
    { name: 'Easy', value: 1 },
    { name: 'Hard', value: 1 },
  ]);
});

it('counts a success only when solved within the goal time', () => {
  const attempts = [
    { difficulty: 'Easy', outcome: 'solved', minutes: 20 },
    { difficulty: 'Easy', outcome: 'solved', minutes: 21 },
    { difficulty: 'Easy', outcome: 'unsolved', minutes: 5 },
    { difficulty: 'Easy', outcome: 'solved', minutes: null },
    { difficulty: 'Medium', outcome: 'solved', minutes: 35 },
    { difficulty: null, outcome: 'solved', minutes: 1 },
  ] as Attempt[];
  expect(difficultySuccessRates(attempts)).toEqual([
    { difficulty: 'Easy', count: 4, successes: 1 },
    { difficulty: 'Medium', count: 1, successes: 1 },
    { difficulty: 'Hard', count: 0, successes: 0 },
  ]);
});
