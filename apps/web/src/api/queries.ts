import {
  activityFilterQuery,
  emptyActivityFilters,
  type ActivityFilters,
} from '@/activityFilters';
import { practiceGoalMinutes } from '@/utils';
import { queryOptions, useQuery } from '@tanstack/react-query';

import {
  adaptAttemptPage,
  adaptCurrentUser,
  adaptMockInterviewPage,
  adaptMockParticipantPage,
  adaptPerson,
  adaptSeasonPage,
  adaptUserPage,
  indexProblems,
} from '@/api/adapters';
import { ApiError, apiRequest } from '@/api/client';
import { demoModeForEnvironment } from '@/api/demoMode';
import type {
  ActivitySummary,
  AttemptPage as ApiAttemptPage,
  Enrollment,
  EnrollmentCandidate,
  EnrollmentCandidatePage,
  EnrollmentPage,
  LeetcodeProblem,
  Me,
  MockInterviewPage as ApiMockInterviewPage,
  MockParticipantSummaryPage,
  MentorshipPage,
  Mentorship,
  ProblemPage,
  PracticeSettings,
  Participation,
  SeasonPage as ApiSeasonPage,
  User as ApiUser,
  UserPrivate,
  UserPage as ApiUserPage,
  Week,
  WeekPage,
} from '@/api/generated/models';
import { fetchPracticeSettings } from '@/api/practiceSettingsClient';
import {
  attempts,
  demoUser,
  mockInterviews,
  people,
  seasonWeeks,
  seasons,
} from '@/data/demo';
import type { CurrentUser, Page, Person } from '@/types';

export const demoMode = demoModeForEnvironment(
  import.meta.env.VITE_USE_DEMO_DATA,
  import.meta.env.MODE,
);

const wait = <T>(value: T) =>
  new Promise<T>((resolve) => window.setTimeout(() => resolve(value), 90));
const page = <T>(items: T[]): Page<T> => ({
  items,
  totalCount: items.length,
  pageInfo: { nextCursor: null, previousCursor: null, hasMore: false },
});

interface ApiPage<T> {
  items: T[];
  totalCount: number;
  pageInfo: {
    nextCursor: string | null;
    previousCursor: string | null;
    hasMore: boolean;
  };
}

/**
 * Resolve every cursor page when a screen needs a complete relationship graph
 * (participant selectors and season joins). The API cursor remains opaque and
 * is always passed back verbatim.
 */
export async function fetchAllPages<T>(path: string): Promise<ApiPage<T>> {
  const items: T[] = [];
  const seen = new Set<string>();
  let cursor: string | null = null;
  let totalCount = 0;
  do {
    const separator = path.includes('?') ? '&' : '?';
    const response: ApiPage<T> = await apiRequest<ApiPage<T>>(
      `${path}${cursor ? `${separator}cursor=${encodeURIComponent(cursor)}` : ''}`,
    );
    items.push(...response.items);
    totalCount = response.totalCount;
    const next: string | null = response.pageInfo.nextCursor ?? null;
    if (!response.pageInfo.hasMore || !next || seen.has(next)) break;
    seen.add(next);
    cursor = next;
  } while (cursor);
  return {
    items,
    totalCount: Math.max(totalCount, items.length),
    pageInfo: { nextCursor: null, previousCursor: null, hasMore: false },
  };
}

export const demoPracticeSettings: PracticeSettings = {
  goalsEnabled: true,
  easyMinutes: practiceGoalMinutes.Easy,
  mediumMinutes: practiceGoalMinutes.Medium,
  hardMinutes: practiceGoalMinutes.Hard,
};

function selectedDemoUser(): CurrentUser {
  const role = window.localStorage.getItem('rsp-demo-role');
  if (role === 'unverified')
    return {
      ...demoUser,
      emailVerified: false,
      globalRoles: [],
      seasonRoles: [],
      alumni: false,
    };
  if (role === 'student' || role === 'mentor' || role === 'coordinator') {
    return {
      ...demoUser,
      globalRoles: [],
      seasonRoles: [
        {
          seasonId: seasons[0].id,
          seasonSlug: seasons[0].slug,
          role,
          state: 'active',
        },
      ],
      alumni: role === 'mentor',
    };
  }
  if (role === 'graduate')
    return { ...demoUser, globalRoles: [], seasonRoles: [], alumni: true };
  if (role === 'former_member' || role === 'kicked')
    return {
      ...demoUser,
      globalRoles: [],
      seasonRoles: [
        {
          seasonId: seasons[1].id,
          seasonSlug: seasons[1].slug,
          role: role === 'kicked' ? 'student' : 'mentor',
          state: role === 'kicked' ? 'kicked' : 'completed',
        },
      ],
      alumni: false,
    };
  if (role === 'director')
    return {
      ...demoUser,
      globalRoles: ['director'],
      seasonRoles: [],
      alumni: true,
    };
  if (role === 'system_admin')
    return {
      ...demoUser,
      globalRoles: ['system_admin'],
      seasonRoles: [],
      alumni: true,
    };
  if (role === 'nonmember')
    return { ...demoUser, globalRoles: [], seasonRoles: [], alumni: false };
  if (role === 'suspended')
    return {
      ...demoUser,
      accountState: 'suspended',
      globalRoles: [],
      seasonRoles: [],
      alumni: false,
    };
  if (role === 'deletion_pending')
    return {
      ...demoUser,
      accountState: 'deletion_pending',
      globalRoles: [],
      seasonRoles: [],
      alumni: false,
    };
  return demoUser;
}

function demoAuthenticationError() {
  return new ApiError(401, {
    message: 'Sign in to continue.',
    requestId: 'demo',
  });
}

const demoProblems: LeetcodeProblem[] = [
  ...attempts.map((attempt, index) => ({
    id: attempt.problemId,
    number: index + 1,
    title: attempt.problem,
    link: `https://leetcode.com/problemset/?search=${encodeURIComponent(attempt.problem)}`,
    difficulty: (
      attempt.difficulty ?? 'Medium'
    ).toLowerCase() as LeetcodeProblem['difficulty'],
    categories: attempt.category ? [attempt.category] : [],
    premium: false,
  })),
];

async function getProblems(): Promise<ProblemPage> {
  const result = await fetchAllPages<LeetcodeProblem>(
    '/leetcode-problems?limit=100',
  );
  return {
    ...result,
    items: [...result.items].sort((left, right) => left.number - right.number),
  };
}

async function getUsers(): Promise<ApiUserPage> {
  return fetchAllPages<ApiUser>('/users?limit=100');
}

export const currentUserOptions = queryOptions({
  queryKey: ['me'],
  queryFn: async () => {
    if (!demoMode) return adaptCurrentUser(await apiRequest<Me>('/me'));
    if (window.localStorage.getItem('rsp-demo-auth-state') === 'signed-out')
      throw demoAuthenticationError();
    return wait(selectedDemoUser());
  },
  staleTime: 60_000,
});

export const seasonsOptions = queryOptions({
  queryKey: ['seasons'],
  queryFn: async () =>
    demoMode
      ? wait(page(seasons))
      : adaptSeasonPage(await fetchAllPages('/seasons?limit=100')),
  staleTime: demoMode ? Infinity : 0,
});

export const practiceSettingsOptions = queryOptions({
  queryKey: ['me', 'practice-settings'],
  queryFn: () =>
    demoMode ? wait(demoPracticeSettings) : fetchPracticeSettings(),
  staleTime: 60_000,
});

export function useCurrentUser() {
  return useQuery(currentUserOptions);
}

export function useSeasons(enabled = true) {
  return useQuery({
    ...seasonsOptions,
    enabled,
  });
}

export function usePracticeSettings() {
  return useQuery(practiceSettingsOptions);
}

export const seasonWeeksQueryKey = (seasonId?: string) =>
  ['season-weeks', seasonId] as const;
export const seasonEnrollmentsQueryKey = (seasonId?: string) =>
  ['season-enrollments', seasonId] as const;
export const seasonMentorshipsQueryKey = (seasonId?: string) =>
  ['season-mentorships', seasonId] as const;
export const enrollmentCandidatesQueryKey = (seasonId?: string) =>
  ['enrollment-candidates', seasonId] as const;

function demoWeekPage(seasonId: string): WeekPage {
  const season = seasons.find((season) => season.id === seasonId);
  if (!season) return page([]);
  const start = new Date(season.startsAt).getTime();
  const items: Week[] = seasonWeeks.map((week) => ({
    id: `week_${seasonId}_${week.week}`,
    seasonId,
    number: week.week,
    startAt: new Date(start + (week.week - 1) * 7 * 86_400_000).toISOString(),
    endAt: new Date(start + week.week * 7 * 86_400_000 - 1).toISOString(),
    resourceUrl: `https://example.test/resources/week-${week.week}`,
  }));
  return page(items);
}

export function demoActivityWeekId(seasonId: string | undefined, at: string) {
  return seasonId
    ? demoWeekPage(seasonId).items.find(
        (week) => at >= week.startAt && at <= week.endAt,
      )?.id
    : undefined;
}

function demoEnrollmentPage(seasonId: string): EnrollmentPage {
  if (!seasons.some((season) => season.id === seasonId)) return page([]);
  const items: Enrollment[] = people
    .filter((person) =>
      person.roles.some(
        (role) =>
          role === 'student' || role === 'mentor' || role === 'coordinator',
      ),
    )
    .map((person) => ({
      id: `enrol_${seasonId}_${person.id}`,
      seasonId,
      userId: person.id,
      role: person.roles.find(
        (role) =>
          role === 'student' || role === 'mentor' || role === 'coordinator',
      ) as Enrollment['role'],
      studentLevel: person.roles.includes('student')
        ? 'beginner'
        : 'not_applicable',
      state: person.status === 'completed' ? 'completed' : 'active',
      assignmentState: 'active',
      removalReason: null,
    }));
  return page(items);
}

function demoMentorshipPage(seasonId: string): MentorshipPage {
  if (!seasons.some((season) => season.id === seasonId)) return page([]);
  const mentor = people.find((person) => person.roles.includes('mentor'));
  const student = people.find(
    (person) =>
      person.roles.includes('student') && person.status !== 'unassigned',
  );
  const items: Mentorship[] =
    mentor && student
      ? [
          {
            id: `mentorship_${seasonId}_${student.id}`,
            seasonId,
            mentorUserId: mentor.id,
            studentUserId: student.id,
          },
        ]
      : [];
  return page(items);
}

export function useSeasonWeeks(seasonId?: string) {
  return useQuery({
    queryKey: seasonWeeksQueryKey(seasonId),
    enabled: Boolean(seasonId),
    queryFn: () => {
      if (!seasonId) return page<Week>([]);
      return demoMode
        ? wait(demoWeekPage(seasonId))
        : fetchAllPages<Week>(
            `/seasons/${encodeURIComponent(seasonId)}/weeks?limit=100`,
          );
    },
  });
}

export function useSeasonEnrollments(seasonId?: string) {
  return useQuery({
    queryKey: seasonEnrollmentsQueryKey(seasonId),
    enabled: Boolean(seasonId),
    queryFn: () => {
      if (!seasonId) return page<Enrollment>([]);
      return demoMode
        ? wait(demoEnrollmentPage(seasonId))
        : fetchAllPages<Enrollment>(
            `/seasons/${encodeURIComponent(seasonId)}/members?limit=100`,
          );
    },
  });
}

export function useSeasonMentorships(seasonId?: string) {
  return useQuery({
    queryKey: seasonMentorshipsQueryKey(seasonId),
    enabled: Boolean(seasonId),
    queryFn: () => {
      if (!seasonId) return page<Mentorship>([]);
      return demoMode
        ? wait(demoMentorshipPage(seasonId))
        : fetchAllPages<Mentorship>(
            `/seasons/${encodeURIComponent(seasonId)}/mentorships?limit=100`,
          );
    },
  });
}

export function useEnrollmentCandidates(seasonId?: string, enabled = true) {
  return useQuery({
    queryKey: enrollmentCandidatesQueryKey(seasonId),
    enabled: Boolean(seasonId) && enabled,
    queryFn: async (): Promise<EnrollmentCandidatePage> => {
      if (!seasonId) return page<EnrollmentCandidate>([]);
      if (demoMode)
        return wait(
          page([
            {
              id: 'person_candidate',
              slug: 'priya-singh',
              name: 'Priya Singh',
              avatarUrl: null,
            },
          ]),
        );
      return fetchAllPages<EnrollmentCandidate>(
        `/seasons/${encodeURIComponent(seasonId)}/enrollment-candidates?limit=100`,
      );
    },
  });
}

export function useAttempts(
  enabled = true,
  seasonId?: string,
  filters: ActivityFilters = emptyActivityFilters,
) {
  return useQuery({
    queryKey: ['problem-attempts', { seasonId, filters }],
    enabled,
    queryFn: async () => {
      if (demoMode)
        return wait(
          page(
            attempts
              .filter((item) => {
                const season = seasons.find((item) => item.id === seasonId);
                return (
                  !seasonId ||
                  Boolean(
                    season &&
                    item.attemptedAt >= season.startsAt &&
                    item.attemptedAt <= season.endsAt,
                  )
                );
              })
              .map((item) => ({
                ...item,
                weekId: demoActivityWeekId(seasonId, item.attemptedAt),
              })),
          ),
        );
      const [attemptPage, problemPage] = await Promise.all([
        fetchAllPages<ApiAttemptPage['items'][number]>(
          `/problem-attempts?limit=100${seasonId ? `&seasonId=${encodeURIComponent(seasonId)}` : ''}${activityFilterQuery(filters)}`,
        ),
        getProblems(),
      ]);
      const adapted = adaptAttemptPage(attemptPage, problemPage.items);
      return {
        ...adapted,
        items: [...adapted.items].sort((left, right) =>
          right.attemptedAt.localeCompare(left.attemptedAt),
        ),
      };
    },
  });
}

export function useLeetcodeProblems() {
  return useQuery({
    queryKey: ['leetcode-problems'],
    queryFn: () => (demoMode ? wait(page(demoProblems)) : getProblems()),
    staleTime: 5 * 60_000,
  });
}

export function usePeople(enabled = true) {
  return useQuery({
    queryKey: ['people'],
    enabled,
    queryFn: async () =>
      demoMode ? wait(page(people)) : adaptUserPage(await getUsers()),
  });
}

export function useAdminUsers(enabled = true) {
  return useQuery({
    queryKey: ['admin-users'],
    enabled,
    queryFn: async (): Promise<ApiPage<UserPrivate>> => {
      if (!demoMode)
        return fetchAllPages<UserPrivate>('/admin/users?limit=100');
      return wait(
        page(
          people.map((person) => ({
            id: person.id,
            slug: person.slug,
            name: person.name,
            avatarUrl: person.avatarUrl,
            timezone: 'Australia/Adelaide',
            timezoneConfigured: true,
            globalRoles: person.roles.filter(
              (role): role is UserPrivate['globalRoles'][number] =>
                role === 'director' || role === 'system_admin',
            ),
            seasonRoles: [],
            attemptCount: person.attempts ?? 0,
            mockInterviewCount: person.interviews ?? 0,
            email: person.email ?? `${person.slug}@example.test`,
            accountState: 'active',
          })),
        ),
      );
    },
  });
}

export function useMockParticipants() {
  return useQuery({
    queryKey: ['mock-interview-participants'],
    queryFn: async () =>
      demoMode
        ? wait(page(people))
        : adaptMockParticipantPage(
            await fetchAllPages<MockParticipantSummaryPage['items'][number]>(
              '/mock-interviews/participants?limit=100',
            ),
          ),
  });
}

export function useUserProfile(identifier?: string, seasonId?: string) {
  return useQuery({
    queryKey: ['users', identifier, { seasonId }],
    enabled: Boolean(identifier),
    queryFn: async () => {
      if (!identifier) return null;
      if (demoMode)
        return wait(
          people.find(
            (person) => person.id === identifier || person.slug === identifier,
          ) ?? null,
        );
      return adaptPerson(
        await apiRequest<ApiUser>(
          `/users/${encodeURIComponent(identifier)}${seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : ''}`,
        ),
      );
    },
  });
}

export function useUserAttempts(
  userId?: string,
  enabled = true,
  seasonId?: string,
  filters: ActivityFilters = emptyActivityFilters,
) {
  return useQuery({
    queryKey: ['problem-attempts', 'user', userId, { seasonId, filters }],
    enabled: Boolean(userId) && enabled,
    queryFn: async () => {
      if (!userId) return page([]);
      if (demoMode)
        return wait(
          page(
            attempts
              .filter((attempt) => {
                const season = seasons.find((item) => item.id === seasonId);
                return (
                  !seasonId ||
                  Boolean(
                    season &&
                    attempt.attemptedAt >= season.startsAt &&
                    attempt.attemptedAt <= season.endsAt,
                  )
                );
              })
              .map((attempt) => ({
                ...attempt,
                weekId: demoActivityWeekId(seasonId, attempt.attemptedAt),
              })),
          ),
        );
      const [attemptPage, problemPage] = await Promise.all([
        fetchAllPages<ApiAttemptPage['items'][number]>(
          `/problem-attempts?limit=100&userId=${encodeURIComponent(userId)}${seasonId ? `&seasonId=${encodeURIComponent(seasonId)}` : ''}${activityFilterQuery(filters)}`,
        ),
        getProblems(),
      ]);
      const adapted = adaptAttemptPage(attemptPage, problemPage.items);
      return {
        ...adapted,
        items: [...adapted.items].sort((left, right) =>
          right.attemptedAt.localeCompare(left.attemptedAt),
        ),
      };
    },
  });
}

export function useSeasonPeople(seasonId?: string, seasonName?: string) {
  return useQuery<Page<Person>>({
    queryKey: ['season-people', seasonId],
    enabled: demoMode || Boolean(seasonId),
    queryFn: async () => {
      if (demoMode) {
        if (!seasonId) return page([]);
        const enrollments = demoEnrollmentPage(seasonId).items;
        const mentorships = demoMentorshipPage(seasonId).items;
        return wait(
          page(
            enrollments.flatMap((enrollment) => {
              const person = people.find(
                (item) => item.id === enrollment.userId,
              );
              if (!person) return [];
              return [
                {
                  ...person,
                  season: seasonName ?? person.season,
                  enrollmentId: enrollment.id,
                  enrollmentState: enrollment.state,
                  studentLevel: enrollment.studentLevel,
                  seasonRole: enrollment.role,
                  mentorshipMentorId: mentorships.find(
                    (item) => item.studentUserId === person.id,
                  )?.mentorUserId,
                },
              ];
            }),
          ),
        );
      }
      if (!seasonId) return page([]);
      const [enrollmentPage, mentorshipPage] = await Promise.all([
        fetchAllPages<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members?limit=100`,
        ),
        fetchAllPages<Mentorship>(
          `/seasons/${encodeURIComponent(seasonId)}/mentorships?limit=100&includeEnded=true`,
        ),
      ]);
      const mentorForStudent = new Map(
        mentorshipPage.items
          .filter((mentorship) => !mentorship.endedAt)
          .map((mentorship) => [
            mentorship.studentUserId,
            mentorship.mentorUserId,
          ]),
      );
      const assignedStudents = new Set(mentorForStudent.keys());
      const items = enrollmentPage.items.flatMap((enrollment) => {
        const member = enrollment.member;
        if (!member) return [];
        const user = {
          id: member.id,
          name: member.name,
          slug: member.slug,
          initials: member.name
            .split(' ')
            .map((part) => part[0])
            .slice(0, 2)
            .join(''),
          avatarUrl: member.avatarUrl,
          roles: [enrollment.role],
          attempts: member.activity.attemptCount,
          interviews: member.activity.mockInterviewCount,
          mocksReceived: member.activity.mocksReceived,
          mocksConducted: member.activity.mocksConducted,
          lastActiveAt: member.activity.lastActivityAt,
        };
        return [
          {
            ...user,
            roles: [...new Set([...user.roles, enrollment.role])],
            season: seasonName ?? null,
            status:
              enrollment.state === 'completed'
                ? ('completed' as const)
                : enrollment.state === 'active' &&
                    enrollment.role === 'student' &&
                    !assignedStudents.has(enrollment.userId)
                  ? ('unassigned' as const)
                  : enrollment.state === 'active'
                    ? ('active' as const)
                    : enrollment.state,
            previousMentorIds: mentorshipPage.items
              .filter((m) => m.studentUserId === enrollment.userId)
              .map((m) => m.mentorUserId),
            enrollmentId: enrollment.id,
            enrollmentState: enrollment.state,
            studentLevel: enrollment.studentLevel,
            seasonRole: enrollment.role,
            ...(mentorForStudent.has(enrollment.userId)
              ? { mentorshipMentorId: mentorForStudent.get(enrollment.userId) }
              : {}),
          },
        ];
      });
      return page(items);
    },
  });
}

export function useMockInterviews(
  enabled = true,
  seasonId?: string,
  userId?: string,
  filters: ActivityFilters = emptyActivityFilters,
) {
  return useQuery({
    queryKey: ['mock-interviews', { seasonId, userId, filters }],
    enabled,
    queryFn: async () => {
      if (demoMode)
        return wait(
          page(
            mockInterviews
              .filter((item) => {
                const season = seasons.find((item) => item.id === seasonId);
                return (
                  (!userId ||
                    item.interviewee.id === userId ||
                    item.interviewer.id === userId) &&
                  (!seasonId ||
                    Boolean(
                      season &&
                      item.occurredAt >= season.startsAt &&
                      item.occurredAt <= season.endsAt,
                    ))
                );
              })
              .map((item) => ({
                ...item,
                weekId: demoActivityWeekId(seasonId, item.occurredAt),
              })),
          ),
        );
      const [interviewPage, seasonPage, problemPage] = await Promise.all([
        fetchAllPages<ApiMockInterviewPage['items'][number]>(
          `/mock-interviews?limit=100&mode=all${userId ? `&userId=${encodeURIComponent(userId)}` : ''}${seasonId ? `&seasonId=${encodeURIComponent(seasonId)}` : ''}${activityFilterQuery(filters)}`,
        ),
        fetchAllPages<ApiSeasonPage['items'][number]>('/seasons?limit=100'),
        getProblems(),
      ]);
      const adaptedSeasons = adaptSeasonPage(seasonPage).items;
      const adapted = adaptMockInterviewPage(interviewPage, {
        users: new Map(),
        seasons: new Map(adaptedSeasons.map((season) => [season.id, season])),
        problems: indexProblems(problemPage.items),
      });
      return {
        ...adapted,
        items: [...adapted.items].sort((left, right) =>
          right.occurredAt.localeCompare(left.occurredAt),
        ),
      };
    },
  });
}

export function useActivitySummary(userId?: string, seasonId?: string) {
  return useQuery({
    queryKey: ['activity-summary', { userId, seasonId }],
    enabled: Boolean(userId || seasonId),
    queryFn: async (): Promise<ActivitySummary> => {
      if (demoMode) {
        const matchingAttempts = attempts.filter(
          (a) => !seasonId || a.seasonId === seasonId,
        );
        const matchingMocks = mockInterviews.filter(
          (m) =>
            (!seasonId || m.seasonId === seasonId) &&
            (!userId ||
              m.interviewee.id === userId ||
              m.interviewer.id === userId),
        );
        const dates = [
          ...matchingAttempts.map((a) => a.attemptedAt),
          ...matchingMocks.map((m) => m.occurredAt),
        ].sort();
        return {
          attemptCount: matchingAttempts.length,
          mockInterviewCount: matchingMocks.length,
          mocksReceived: matchingMocks.filter(
            (m) => !userId || m.interviewee.id === userId,
          ).length,
          mocksConducted: matchingMocks.filter(
            (m) => !userId || m.interviewer.id === userId,
          ).length,
          lastActivityAt: dates.at(-1) ?? null,
        };
      }
      return apiRequest<ActivitySummary>(
        userId
          ? `/users/${encodeURIComponent(userId)}/activity-summary${seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : ''}`
          : `/seasons/${encodeURIComponent(seasonId!)}/activity-summary`,
      );
    },
  });
}

export function useUserParticipation(
  userId?: string,
  enabled = true,
  seasonId?: string,
) {
  return useQuery({
    queryKey: ['participation', userId, { seasonId }],
    enabled: Boolean(userId) && enabled,
    queryFn: async (): Promise<Page<Participation>> => {
      if (demoMode)
        return page(
          seasons.flatMap((season) =>
            demoEnrollmentPage(season.id)
              .items.filter((e) => e.userId === userId)
              .map((e) => ({
                ...e,
                seasonName: season.name,
                seasonSlug: season.slug,
                startAt: season.startsAt,
                endAt: season.endsAt,
                lastStudentLevel: e.studentLevel,
                periods: [
                  {
                    role: e.role,
                    startedAt: season.startsAt,
                    endedAt: season.status === 'closed' ? season.endsAt : null,
                  },
                ],
              })),
          ),
        );
      return apiRequest<Page<Participation>>(
        `/users/${encodeURIComponent(userId!)}/participation${seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : ''}`,
      );
    },
  });
}
