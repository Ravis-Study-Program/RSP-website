import type {
  Attempt,
  AttemptOutcome,
  CurrentUser,
  Difficulty,
  MockInterview,
} from '@/types';
import type {
  LeetcodeProblem,
  MockInterviewDayCount,
} from '@/api/generated/models';

let displayTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone;

export function supportedTimezones(...preferred: string[]) {
  let zones: string[] = [];
  try {
    zones = Intl.supportedValuesOf('timeZone');
  } catch {
    // Older browsers may not expose the complete IANA timezone list.
  }
  zones = zones.filter((zone) => !zone.startsWith('Etc/'));
  return Array.from(
    new Set([
      ...zones,
      ...preferred.filter((zone) => zone && !zone.startsWith('Etc/')),
      'UTC',
    ]),
  ).sort(
    // Smallest UTC offset first, then by name within an offset.
    (left, right) =>
      timezoneOffsetMinutes(left) - timezoneOffsetMinutes(right) ||
      left.localeCompare(right),
  );
}

/** The zone's current UTC offset as sign, hours and minutes. */
function timezoneOffset(timezone: string) {
  try {
    const offsetName = new Intl.DateTimeFormat('en', {
      timeZone: timezone,
      timeZoneName: 'longOffset',
    })
      .formatToParts()
      .find((part) => part.type === 'timeZoneName')?.value;
    const match = /^GMT([+-])(\d{1,2})(?::(\d{2}))?$/.exec(offsetName ?? '');
    if (match)
      return {
        sign: match[1] as '+' | '-',
        hours: Number(match[2]),
        minutes: Number(match[3] ?? '0'),
      };
  } catch {
    // An invalid or unsupported timezone reads as UTC.
  }
  return { sign: '+' as const, hours: 0, minutes: 0 };
}

function timezoneOffsetMinutes(timezone: string) {
  const { sign, hours, minutes } = timezoneOffset(timezone);
  return (sign === '-' ? -1 : 1) * (hours * 60 + minutes);
}

export function formatTimezoneOption(timezone: string, label = timezone) {
  const { sign, hours, minutes } = timezoneOffset(timezone);
  const offset = `${sign}${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`;
  return `${offset} ${label.replaceAll('_', ' ')}`;
}

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

// Every date on the site reads dd/mm/yy, whatever the browser's locale.
const dayMonthYear = {
  day: '2-digit',
  month: '2-digit',
  year: '2-digit',
} as const;

/** A date as dd/mm/yy in the member's saved timezone. */
export function formatDate(value: string | number | Date) {
  return new Intl.DateTimeFormat('en-GB', {
    ...dayMonthYear,
    timeZone: displayTimezone,
  }).format(new Date(value));
}

/** A date and 24-hour time as "dd/mm/yy, hh:mm" in the saved timezone. */
export function formatDateTime(value: string | number | Date) {
  return new Intl.DateTimeFormat('en-GB', {
    ...dayMonthYear,
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
    timeZone: displayTimezone,
  }).format(new Date(value));
}

/** A calendar day already in UTC, such as a chart bucket, as dd/mm/yy. */
export function formatCalendarDay(value: number | Date) {
  return new Intl.DateTimeFormat('en-GB', {
    ...dayMonthYear,
    timeZone: 'UTC',
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

/**
 * "1. Two Sum". Some catalogue titles already start with their number, so a
 * matching prefix is not repeated.
 */
export function problemName({
  number,
  title,
}: Pick<LeetcodeProblem, 'number' | 'title'>) {
  const prefix = `${number}. `;
  return title.startsWith(prefix) ? title : `${prefix}${title}`;
}

/** Minutes per difficulty that an attempt aims to finish within. */
export type PracticeGoals = Record<Difficulty, number>;

/** The program's goals until someone changes them; the API has the rest. */
/** The hours a week the program expects of each student by default. */
export const defaultWeeklyHours = 30;

/**
 * What a student is expected to do each week besides their hours: LeetCode
 * problems solved by difficulty, and mock interviews (given and received
 * together: 2 of each). Fixed for now; not yet editable.
 */
export const weeklyLeetCodeTargets = { easy: 3, medium: 5, hard: 2, mocks: 4 };

/** What a student is expected to do each week, by default. */
export const defaultWeeklyTargets = {
  hours: defaultWeeklyHours,
  ...weeklyLeetCodeTargets,
};
export type WeeklyTargets = typeof defaultWeeklyTargets;

/** A student's weekly targets from their weekly hours. */
export function weeklyTargetsFor(hours: number): WeeklyTargets {
  return { hours, ...weeklyLeetCodeTargets };
}

/** Whether a week's count meets its target; a target of 0 is not shown. */
export function meetsTarget(count: number | null | undefined, target?: number) {
  return Boolean(target && count != null && count >= target);
}

export const defaultPracticeGoals: PracticeGoals = {
  Easy: 10,
  Medium: 25,
  Hard: 40,
};

/** Directors, system admins and active coordinators set practice goals. */
export function canSetPracticeGoals(user?: CurrentUser) {
  return Boolean(
    user &&
    (user.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    ) ||
      user.seasonRoles.some(
        (membership) =>
          membership.role === 'coordinator' && membership.state === 'active',
      )),
  );
}

export function outcomeLabel(outcome: AttemptOutcome) {
  const labels: Record<AttemptOutcome, string> = {
    solved: 'Solved',
    unsolved: 'Unsolved',
  };
  return labels[outcome];
}

export function interviewPassed(interview: MockInterview) {
  return (
    interview.rounds.length > 0 &&
    interview.rounds.every((round) => round.score !== null && round.score >= 5)
  );
}

/** `mockDays` are server-aggregated, already limited to the subject's interviews. */
export function recentActivitySeries(
  attempts: Attempt[],
  mockDays: MockInterviewDayCount[] = [],
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
      week: formatCalendarDay(start),
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
  mockDays.forEach(({ date, count }) => {
    const time = Date.parse(date);
    const bucket = buckets.find(
      (item) => time >= item.start && time < item.end,
    );
    if (bucket) bucket.interviews += count;
  });
  return buckets.map(({ week, attempts: count, interviews: mockCount }) => ({
    week,
    attempts: count,
    interviews: mockCount,
  }));
}

/** Rich text counts as empty when it carries no text, only markup. */
export function hasRichText(html: string | null | undefined) {
  return (
    (html ?? '')
      .replace(/<[^>]*>/g, ' ')
      .replace(/&nbsp;/gi, ' ')
      .trim().length > 0
  );
}

/**
 * When something was recorded, from a date and an optional HH:MM time in a
 * zone. With no time, today's date keeps the current time (never the future)
 * and an earlier day uses noon.
 */
export function recordedDateTime(
  date: string,
  time: string,
  timezone = displayTimezone,
) {
  if (time) return zonedDateTimeToUtc(`${date}T${time}`, timezone);
  return date === calendarDateKey(new Date(), timezone)
    ? new Date().toISOString()
    : zonedDateTimeToUtc(`${date}T12:00`, timezone);
}

/** An optional HH:MM time field: empty, or a valid 24-hour time. */
export const optionalTimePattern = /^(?:[01]\d|2[0-3]):[0-5]\d$|^$/;

/** Whether `email` looks like name@example.com: a domain needs a dot. */
export function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@.]+(?:\.[^\s@.]+)+$/.test(email.trim());
}
