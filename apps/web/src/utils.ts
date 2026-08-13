import type { AttemptOutcome, Difficulty, MockInterview } from '@/types';

export function formatDate(value: string, options: Intl.DateTimeFormatOptions = {}) {
  return new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    ...options,
  }).format(new Date(value));
}

export function formatDateTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(new Date(value));
}

export function initials(name: string) {
  return name
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => part[0])
    .join('')
    .toUpperCase();
}

export function titleCase(value: string) {
  return value.replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

export function difficultyGoal(difficulty: Difficulty) {
  return difficulty === 'Easy' ? 20 : difficulty === 'Medium' ? 35 : 50;
}

export function outcomeLabel(outcome: AttemptOutcome) {
  const labels: Record<AttemptOutcome, string> = {
    independently_solved: 'Independently solved',
    solved_with_hints: 'Solved with hints',
    not_solved: 'Not solved',
    unknown: 'Outcome not recorded',
  };
  return labels[outcome];
}

export function interviewPassed(interview: MockInterview) {
  return interview.rounds.length > 0 && interview.rounds.every((round) => round.score >= 5);
}
