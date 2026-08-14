import type {
  Attempt,
  AttemptOutcome,
  Difficulty,
  MockInterview,
} from '@/types';

let displayTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;

export function configureDisplayTimezone(timezone: string) {
  try {
    new Intl.DateTimeFormat(undefined, { timeZone: timezone }).format();
    displayTimezone = timezone;
  } catch {
    displayTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  }
}

export function calendarDateKey(
  value: string | Date,
  timezone = displayTimezone,
) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    timeZone: timezone,
  }).formatToParts(new Date(value));
  const values = Object.fromEntries(
    parts.map((part) => [part.type, part.value]),
  );
  return `${values.year}-${values.month}-${values.day}`;
}

function zonedDateTimeParts(value: string | Date, timezone = displayTimezone) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
    timeZone: timezone,
  }).formatToParts(new Date(value));
  return Object.fromEntries(parts.map((part) => [part.type, part.value]));
}

export function formatDateTimeInput(
  value: string | Date,
  timezone = displayTimezone,
) {
  const parts = zonedDateTimeParts(value, timezone);
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}

/** Convert a datetime-local control value in the member's saved zone to UTC. */
export function zonedDateTimeToUtc(value: string, timezone = displayTimezone) {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?$/.exec(
    value,
  );
  if (!match) throw new RangeError('Invalid local date and time.');
  const desired = Date.UTC(
    Number(match[1]),
    Number(match[2]) - 1,
    Number(match[3]),
    Number(match[4]),
    Number(match[5]),
    Number(match[6] ?? 0),
  );
  let guess = desired;
  for (let index = 0; index < 4; index += 1) {
    const parts = zonedDateTimeParts(new Date(guess), timezone);
    const represented = Date.UTC(
      Number(parts.year),
      Number(parts.month) - 1,
      Number(parts.day),
      Number(parts.hour),
      Number(parts.minute),
      Number(parts.second),
    );
    const correction = desired - represented;
    guess += correction;
    if (correction === 0) break;
  }
  return new Date(guess).toISOString();
}

function calendarDay(value: string | Date, timezone = displayTimezone) {
  const [year, month, day] = calendarDateKey(value, timezone)
    .split('-')
    .map(Number);
  return Date.UTC(year, month - 1, day);
}

export function formatDate(
  value: string,
  options: Intl.DateTimeFormatOptions = {},
) {
  return new Intl.DateTimeFormat(undefined, {
    day: 'numeric',
    month: 'short',
    year: 'numeric',
    timeZone: displayTimezone,
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
    timeZone: displayTimezone,
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
  return value
    .replaceAll('_', ' ')
    .replace(/\b\w/g, (letter) => letter.toUpperCase());
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
  return (
    interview.rounds.length > 0 &&
    interview.rounds.every((round) => round.score !== null && round.score >= 5)
  );
}

export function recentActivitySeries(
  attempts: Attempt[],
  interviews: MockInterview[] = [],
  userId?: string,
) {
  const today = new Date();
  const todayCalendarDay = calendarDay(today);
  const day = new Date(todayCalendarDay).getUTCDay();
  const thisMonday = todayCalendarDay - ((day + 6) % 7) * 86_400_000;
  const buckets = Array.from({ length: 6 }, (_, index) => {
    const start = thisMonday - (5 - index) * 7 * 86_400_000;
    const end = start + 7 * 86_400_000;
    return {
      start,
      end,
      week: new Intl.DateTimeFormat(undefined, {
        day: 'numeric',
        month: 'short',
        timeZone: 'UTC',
      }).format(new Date(start)),
      attempts: 0,
      interviews: 0,
    };
  });
  attempts.forEach((attempt) => {
    const time = calendarDay(attempt.attemptedAt);
    const bucket = buckets.find(
      (item) => time >= item.start && time < item.end,
    );
    if (bucket) bucket.attempts += 1;
  });
  interviews.forEach((interview) => {
    if (
      userId &&
      interview.interviewer.id !== userId &&
      interview.interviewee.id !== userId
    )
      return;
    const time = calendarDay(interview.occurredAt);
    const bucket = buckets.find(
      (item) => time >= item.start && time < item.end,
    );
    if (bucket) bucket.interviews += 1;
  });
  return buckets.map(({ week, attempts: count, interviews: mockCount }) => ({
    week,
    attempts: count,
    interviews: mockCount,
  }));
}
