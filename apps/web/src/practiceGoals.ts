import type { PracticeGoals } from '@/api/generated/models';
import type { Attempt, Difficulty } from '@/types';

/** Easy, Medium and Hard time goals in minutes. */
export type GoalMinutes = Record<Difficulty, number>;

export const goalDifficulties: Difficulty[] = ['Easy', 'Medium', 'Hard'];

/**
 * The programme goals before a Director or System Admin changes them. They
 * match `practice.DefaultGoals` in the backend; demo mode uses them directly.
 */
export const defaultGoalMinutes: GoalMinutes = {
  Easy: 10,
  Medium: 25,
  Hard: 40,
};

/** Each goal is a whole number of minutes in this range. */
export const goalMinutesRange = { min: 1, max: 180 } as const;

export function goalMinutesFrom(goals: PracticeGoals): GoalMinutes {
  return {
    Easy: goals.easyMinutes,
    Medium: goals.mediumMinutes,
    Hard: goals.hardMinutes,
  };
}

export function practiceGoalsFrom(goals: GoalMinutes): PracticeGoals {
  return {
    easyMinutes: goals.Easy,
    mediumMinutes: goals.Medium,
    hardMinutes: goals.Hard,
  };
}

/** The goals as "10 / 25 / 40", in difficulty order. */
export function goalSummary(goals: GoalMinutes) {
  return goalDifficulties.map((difficulty) => goals[difficulty]).join(' / ');
}

export function validGoalMinutes(minutes: number) {
  return (
    Number.isInteger(minutes) &&
    minutes >= goalMinutesRange.min &&
    minutes <= goalMinutesRange.max
  );
}

/** Whether a timed attempt took longer than its difficulty's goal. */
export function overGoal(
  goals: GoalMinutes | undefined,
  attempt: Pick<Attempt, 'difficulty' | 'minutes'>,
) {
  return Boolean(
    goals &&
    attempt.difficulty &&
    attempt.minutes !== null &&
    attempt.minutes > goals[attempt.difficulty],
  );
}
