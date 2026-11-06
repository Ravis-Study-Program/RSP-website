import type { PracticeGoals } from '@/utils';
import type { Attempt, Difficulty } from '@/types';

export function difficultyBreakdown(attempts: Attempt[]) {
  return ['Easy', 'Medium', 'Hard', 'Unknown']
    .map((name) => ({
      name,
      value: attempts.filter((a) => (a.difficulty ?? 'Unknown') === name)
        .length,
    }))
    .filter((item) => item.value > 0);
}

/**
 * Per-difficulty counts and successes, where a success is an attempt solved
 * independently within that difficulty's practice goal.
 */
/** A success is a solved attempt within its difficulty's goal. */
export function difficultySuccessRates(
  attempts: Attempt[],
  goals: PracticeGoals,
) {
  return (['Easy', 'Medium', 'Hard'] as Difficulty[]).map((difficulty) => {
    const matching = attempts.filter((a) => a.difficulty === difficulty);
    return {
      difficulty,
      count: matching.length,
      successes: matching.filter(
        (a) =>
          a.outcome === 'solved' &&
          a.minutes !== null &&
          a.minutes <= goals[difficulty],
      ).length,
    };
  });
}
