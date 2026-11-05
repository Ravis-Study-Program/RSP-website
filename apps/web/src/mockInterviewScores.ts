import type { MockRound } from '@/types';

/** A round passes at this score; an interview passes when every round does. */
export const passingScore = 5;

export const leetcodeScoreKeys = [
  'confirmQuestions',
  'algorithmDesign',
  'complexityAnalysis',
  'coding',
  'testing',
] as const;

// The criteria's names as interviewers know them.
const scoreLabels: Record<string, string> = {
  confirmQuestions: 'Confirming the Question',
  algorithmDesign: 'Algorithm Design',
  complexityAnalysis: 'Complexity Analysis',
  coding: 'Coding',
  testing: 'Testing',
  behavioural: 'Behavioural',
  custom: 'Custom',
};

/** One-line reminders of what each score is for, shown beside it. */
export const scoreHelp: Record<string, string> = {
  confirmQuestions:
    'Clarifying the problem, its inputs, constraints and edge cases before solving it.',
  algorithmDesign:
    'Finding a working approach and explaining it clearly before coding.',
  complexityAnalysis:
    "Working out and justifying the approach's time and space complexity.",
  coding: 'Turning the approach into correct, clean and readable code.',
  testing: 'Walking through examples and edge cases to find and fix bugs.',
  behavioural:
    'Answering with specific, well-structured examples from past experience.',
  custom: 'How well the interviewee did on what this round covered.',
};

export function scoreLabel(value: string) {
  return (
    scoreLabels[value] ??
    value
      .replaceAll(/([A-Z])/g, ' $1')
      .replace(/^./, (letter) => letter.toUpperCase())
  );
}

/**
 * A colour on a scale from danger (0) through warning (5) to success (10), so
 * scores read as a range rather than just pass or fail. The end colours are
 * the theme's status colours, which meet contrast in both themes.
 */
export function scoreColor(score: number) {
  return score <= 5
    ? `color-mix(in oklch, var(--destructive), var(--warning) ${score * 20}%)`
    : `color-mix(in oklch, var(--warning), var(--success) ${(score - 5) * 20}%)`;
}

/** A round's scores, LeetCode parts first in their interview order. */
export function orderedScores(round: MockRound) {
  const order: string[] = [...leetcodeScoreKeys];
  return Object.entries(round.scores).sort(
    ([left], [right]) =>
      (order.indexOf(left) + 1 || order.length + 1) -
      (order.indexOf(right) + 1 || order.length + 1),
  );
}
