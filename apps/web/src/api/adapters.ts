import type {
  Attempt as ApiAttempt,
  AttemptPage as ApiAttemptPage,
  LeetcodeProblem,
  Me,
  MockInterview as ApiMockInterview,
  MockInterviewPage as ApiMockInterviewPage,
  MockParticipantSummary,
  MockParticipantSummaryPage,
  MockRound as ApiMockRound,
  Season as ApiSeason,
  SeasonPage as ApiSeasonPage,
  User as ApiUser,
  UserPage as ApiUserPage,
} from '@/api/generated/models';
import type {
  Attempt,
  CurrentUser,
  Difficulty,
  MockInterview,
  MockRound,
  Page,
  Person,
  Role,
  Season,
} from '@/types';
import { initials } from '@/utils';

type ApiPage<T> = {
  items: T[];
  pageInfo: {
    nextCursor?: string | null;
    previousCursor?: string | null;
    hasMore: boolean;
  };
  totalCount: number;
};

function adaptPage<TApi, TView>(
  source: ApiPage<TApi>,
  adapt: (item: TApi) => TView,
): Page<TView> {
  return {
    items: source.items.map(adapt),
    pageInfo: {
      nextCursor: source.pageInfo.nextCursor ?? null,
      previousCursor: source.pageInfo.previousCursor ?? null,
      hasMore: source.pageInfo.hasMore,
    },
    totalCount: source.totalCount,
  };
}

export function adaptCurrentUser(user: Me): CurrentUser {
  return {
    id: user.id,
    name: user.name,
    slug: user.slug,
    avatarUrl: user.avatarUrl ?? null,
    email: user.email,
    timezone: user.timezone,
    timezoneConfigured: user.timezoneConfigured,
    emailVerified: user.emailVerified,
    mfaVerified: user.mfaVerified,
    accountState: user.accountState,
    globalRoles: [...user.globalRoles],
    seasonRoles: user.seasonRoles.map((membership) => ({ ...membership })),
    alumni: user.alumni,
    attemptCount: user.attemptCount,
    mockInterviewCount: user.mockInterviewCount,
  };
}

export function adaptSeason(season: ApiSeason): Season {
  return {
    id: season.id,
    slug: season.slug,
    name: season.name,
    status: season.status,
    startsAt: season.startAt,
    endsAt: season.endAt,
    location: season.location,
    imageUrl: season.imageUrl,
    resourcesUrl: season.resourcesUrl,
    memberCount: null,
    weekCount: null,
    summary: season.location
      ? `${season.status === 'open' ? 'Current programme' : 'Completed programme'} at ${season.location}.`
      : season.status === 'open'
        ? 'Current programme.'
        : 'Completed programme history.',
  };
}

export function adaptSeasonPage(source: ApiSeasonPage): Page<Season> {
  return adaptPage(source, adaptSeason);
}

export interface PersonContext {
  roles?: Role[];
  season?: string | null;
  status?: Person['status'];
  attempts?: number | null;
  interviews?: number | null;
  email?: string;
  lastActiveAt?: string | null;
}

export function adaptPerson(
  user: ApiUser,
  context: PersonContext = {},
): Person {
  const derivedRoles = new Set<Role>(user.globalRoles);
  user.seasonRoles.forEach((membership) => derivedRoles.add(membership.role));
  if (
    user.seasonRoles.some(
      (membership) =>
        membership.role === 'student' && membership.state === 'completed',
    )
  )
    derivedRoles.add('graduate');
  const primaryMembership =
    user.seasonRoles.find((membership) => membership.state === 'active') ??
    user.seasonRoles[0];
  const derivedStatus =
    primaryMembership?.state === 'active'
      ? 'active'
      : primaryMembership?.state === 'completed'
        ? 'completed'
        : null;
  return {
    id: user.id,
    name: user.name,
    slug: user.slug,
    initials: initials(user.name),
    avatarUrl: user.avatarUrl ?? null,
    roles: context.roles ?? [...derivedRoles],
    season: context.season ?? primaryMembership?.seasonSlug ?? null,
    status: context.status ?? derivedStatus,
    attempts: context.attempts ?? user.attemptCount,
    interviews: context.interviews ?? user.mockInterviewCount,
    ...(context.email ? { email: context.email } : {}),
    lastActiveAt: context.lastActiveAt ?? null,
  };
}

export function adaptCurrentPerson(
  user: CurrentUser | Me,
  seasons: Season[] = [],
): Person {
  const roles = new Set<Role>(user.globalRoles);
  user.seasonRoles.forEach((membership) => roles.add(membership.role));
  if (user.alumni) roles.add('graduate');
  const firstMembership = user.seasonRoles[0];
  const season = seasons.find((item) => item.id === firstMembership?.seasonId);
  return {
    id: user.id,
    name: user.name,
    slug: user.slug,
    initials: initials(user.name),
    avatarUrl: user.avatarUrl ?? null,
    roles: [...roles],
    season: season?.name ?? firstMembership?.seasonSlug ?? null,
    status: user.accountState === 'active' ? 'active' : null,
    attempts: user.attemptCount,
    interviews: user.mockInterviewCount,
    email: user.email,
    lastActiveAt: null,
  };
}

export function adaptUserPage(source: ApiUserPage): Page<Person> {
  return adaptPage(source, (user) => adaptPerson(user));
}

export function adaptMockParticipant(
  participant: MockParticipantSummary,
): Person {
  return {
    id: participant.id,
    name: participant.name,
    slug: participant.slug,
    initials: initials(participant.name),
    avatarUrl: participant.avatarUrl ?? null,
    roles: [],
    season: null,
    status: 'active',
    attempts: null,
    interviews: null,
    lastActiveAt: null,
  };
}

export function adaptMockParticipantPage(
  source: MockParticipantSummaryPage,
): Page<Person> {
  return adaptPage(source, adaptMockParticipant);
}

export function displayDifficulty(
  value: LeetcodeProblem['difficulty'],
): Difficulty {
  return value === 'easy' ? 'Easy' : value === 'medium' ? 'Medium' : 'Hard';
}

export function indexProblems(
  problems: LeetcodeProblem[],
): Map<string, LeetcodeProblem> {
  return new Map(problems.map((problem) => [problem.id, problem]));
}

export function adaptAttempt(
  attempt: ApiAttempt,
  problems: ReadonlyMap<string, LeetcodeProblem>,
): Attempt {
  const problem = problems.get(attempt.problemId);
  return {
    id: attempt.id,
    problemId: attempt.problemId,
    problem: problem?.title ?? `Problem ${attempt.problemId}`,
    problemUrl: problem?.link,
    difficulty: problem ? displayDifficulty(problem.difficulty) : null,
    category: problem?.categories.join(', ') || null,
    categories: problem?.categories ?? [],
    weekId: attempt.weekId,
    outcome: attempt.outcome,
    confidence: attempt.confidence ?? null,
    minutes: attempt.minutes,
    attemptedAt: attempt.attemptedAt,
    seasonId: attempt.seasonId,
    notes: attempt.notes,
  };
}

export function adaptAttemptPage(
  source: ApiAttemptPage,
  problems: LeetcodeProblem[],
): Page<Attempt> {
  const problemIndex = indexProblems(problems);
  return adaptPage(source, (attempt) => adaptAttempt(attempt, problemIndex));
}

function scoresForRound(round: ApiMockRound): Record<string, number> {
  return Object.fromEntries(
    Object.entries(round.scores).filter(
      (entry): entry is [string, number] => typeof entry[1] === 'number',
    ),
  );
}

export function adaptMockRound(
  round: ApiMockRound,
  problems: ReadonlyMap<string, LeetcodeProblem>,
): MockRound {
  const scores = scoresForRound(round);
  const values = Object.values(scores);
  const problem = round.problemId ? problems.get(round.problemId) : undefined;
  return {
    id: round.id,
    type: round.type === 'behavioural' ? 'behavioural' : 'technical',
    apiType: round.type,
    ...(round.problemId ? { problemId: round.problemId } : {}),
    ...(round.link ? { link: round.link } : {}),
    title:
      problem?.title ??
      (round.type === 'behavioural'
        ? 'Behavioural round'
        : round.type === 'leetcode'
          ? 'LeetCode round'
          : 'Custom round'),
    score: values.length > 0 ? Math.min(...values) : null,
    scores,
    notes: round.content ?? '',
    reviewed: round.reviewed,
    intervieweeComment: round.intervieweeComment,
  };
}

export interface MockInterviewContext {
  users: ReadonlyMap<string, Person>;
  seasons: ReadonlyMap<string, Season>;
  problems: ReadonlyMap<string, LeetcodeProblem>;
}

export function adaptMockInterview(
  interview: ApiMockInterview,
  context: MockInterviewContext,
): MockInterview {
  const comments = interview.rounds
    .map((round) => round.intervieweeComment)
    .filter(Boolean);
  return {
    id: interview.id,
    interviewee:
      context.users.get(interview.intervieweeId) ??
      adaptMockParticipant(interview.interviewee),
    interviewer:
      context.users.get(interview.interviewerId) ??
      adaptMockParticipant(interview.interviewer),
    seasonId: interview.seasonId,
    season: interview.seasonId
      ? (context.seasons.get(interview.seasonId)?.name ?? null)
      : null,
    occurredAt: interview.occurredAt,
    durationMinutes: interview.durationMinutes,
    notes: interview.notes,
    rounds: interview.rounds.map((round) =>
      adaptMockRound(round, context.problems),
    ),
    reviewStatus: interview.rounds.every((round) => round.reviewed)
      ? 'reviewed'
      : 'pending',
    ...(comments.length > 0 ? { reviewComments: comments.join('\n\n') } : {}),
  };
}

export function adaptMockInterviewPage(
  source: ApiMockInterviewPage,
  context: MockInterviewContext,
): Page<MockInterview> {
  return adaptPage(source, (interview) =>
    adaptMockInterview(interview, context),
  );
}
