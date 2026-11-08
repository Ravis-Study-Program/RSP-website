import type { Attempt, Difficulty } from '@/types';

/** Program order. Beginner is where a student starts. */
export const levelOrder = [
  'beginner',
  'novice',
  'intermediate',
  'advanced',
] as const;

export type ProgramLevel = (typeof levelOrder)[number];

export type LevelCounts = {
  easy: number;
  medium: number;
  hard: number;
  received: number;
  given: number;
};

type Mix = { easy: number; medium: number; hard: number };
type MockMix = { received: number; given: number };

/** Counted tasks required to be awarded the level. Beginner and Novice have none. */
const countedTargets: Record<
  ProgramLevel,
  { leetcode: Mix; mocks: MockMix } | null
> = {
  beginner: null,
  novice: null,
  intermediate: {
    leetcode: { easy: 15, medium: 25, hard: 10 },
    mocks: { received: 10, given: 10 },
  },
  advanced: {
    leetcode: { easy: 30, medium: 50, hard: 20 },
    mocks: { received: 20, given: 20 },
  },
};

export type ProgressBar = {
  done: number;
  target: number;
  parts: Array<{ label: string; done: number; target: number }>;
};

export function isProgramLevel(
  level: string | undefined,
): level is ProgramLevel {
  return levelOrder.some((item) => item === level);
}

export function nextLevel(level: ProgramLevel): ProgramLevel | null {
  const index = levelOrder.indexOf(level);
  return levelOrder[index + 1] ?? null;
}

/** One solved problem counts once, at the difficulty recorded on that attempt. */
export function solvedDifficultyCounts(
  attempts: Attempt[],
): Pick<LevelCounts, 'easy' | 'medium' | 'hard'> {
  const seen = new Map<string, Difficulty>();
  for (const attempt of attempts) {
    if (attempt.outcome !== 'solved' || !attempt.difficulty) continue;
    if (!seen.has(attempt.problemId)) {
      seen.set(attempt.problemId, attempt.difficulty);
    }
  }
  const counts = { easy: 0, medium: 0, hard: 0 };
  for (const difficulty of seen.values()) {
    if (difficulty === 'Easy') counts.easy += 1;
    if (difficulty === 'Medium') counts.medium += 1;
    if (difficulty === 'Hard') counts.hard += 1;
  }
  return counts;
}

function bar(
  parts: Array<{ label: string; done: number; target: number }>,
): ProgressBar {
  return {
    done: parts.reduce(
      (sum, part) => sum + Math.min(part.done, part.target),
      0,
    ),
    target: parts.reduce((sum, part) => sum + part.target, 0),
    parts,
  };
}

function barsFor(
  level: ProgramLevel,
  counts: LevelCounts,
): { leetcode: ProgressBar; mocks: ProgressBar } | null {
  const targets = countedTargets[level];
  if (!targets) return null;
  return {
    leetcode: bar([
      { label: 'Easy', done: counts.easy, target: targets.leetcode.easy },
      { label: 'Medium', done: counts.medium, target: targets.leetcode.medium },
      { label: 'Hard', done: counts.hard, target: targets.leetcode.hard },
    ]),
    mocks: bar([
      {
        label: 'As interviewee',
        done: counts.received,
        target: targets.mocks.received,
      },
      {
        label: 'As interviewer',
        done: counts.given,
        target: targets.mocks.given,
      },
    ]),
  };
}

/** Counted bars for being awarded this level. Null when the level has no numbers. */
export function barsForLevel(
  level: ProgramLevel,
  counts: LevelCounts,
): { leetcode: ProgressBar; mocks: ProgressBar } | null {
  return barsFor(level, counts);
}
