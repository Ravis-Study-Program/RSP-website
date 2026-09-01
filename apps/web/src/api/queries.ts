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
import type { CurrentUser, Page } from '@/types';

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
async function fetchAllPages<T>(path: string): Promise<ApiPage<T>> {
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
  easyMinutes: 20,
  mediumMinutes: 35,
  hardMinutes: 50,
  revision: 1,
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
  if (role === 'former_member')
    return {
      ...demoUser,
      globalRoles: [],
      seasonRoles: [
        {
          seasonId: seasons[1].id,
          seasonSlug: seasons[1].slug,
          role: 'mentor',
          state: 'completed',
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
    code: 'authentication_required',
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
    revision: 1,
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
  const items: Week[] = seasonWeeks.map((week) => ({
    id: `week_${seasonId}_${week.week}`,
    seasonId,
    number: week.week,
    startAt: week.startsAt,
    endAt: new Date(
      new Date(week.startsAt).getTime() + 6 * 86_400_000,
    ).toISOString(),
    resourceUrl: `https://example.test/resources/week-${week.week}`,
    revision: 1,
  }));
  return page(items);
}

function demoEnrollmentPage(seasonId: string): EnrollmentPage {
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
      revision: 1,
    }));
  return page(items);
}

function demoMentorshipPage(seasonId: string): MentorshipPage {
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
            revision: 1,
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
              revision: 1,
            },
          ]),
        );
      return fetchAllPages<EnrollmentCandidate>(
        `/seasons/${encodeURIComponent(seasonId)}/enrollment-candidates?limit=100`,
      );
    },
  });
}

export function useAttempts(enabled = true) {
  return useQuery({
    queryKey: ['problem-attempts'],
    enabled,
    queryFn: async () => {
      if (demoMode) return wait(page(attempts));
      const [attemptPage, problemPage] = await Promise.all([
        fetchAllPages<ApiAttemptPage['items'][number]>(
          '/problem-attempts?limit=100',
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
            revision: person.revision ?? 1,
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

export function useUserProfile(identifier?: string) {
  return useQuery({
    queryKey: ['users', identifier],
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
        await apiRequest<ApiUser>(`/users/${encodeURIComponent(identifier)}`),
      );
    },
  });
}

export function useUserAttempts(userId?: string, enabled = true) {
  return useQuery({
    queryKey: ['problem-attempts', 'user', userId],
    enabled: Boolean(userId) && enabled,
    queryFn: async () => {
      if (!userId) return page([]);
      if (demoMode) return wait(page(attempts));
      const [attemptPage, problemPage] = await Promise.all([
        fetchAllPages<ApiAttemptPage['items'][number]>(
          `/problem-attempts?limit=100&userId=${encodeURIComponent(userId)}`,
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
  return useQuery({
    queryKey: ['season-people', seasonId],
    enabled: demoMode || Boolean(seasonId),
    queryFn: async () => {
      if (demoMode) return wait(page(people));
      if (!seasonId) return page([]);
      const [userPage, enrollmentPage, mentorshipPage] = await Promise.all([
        getUsers(),
        fetchAllPages<Enrollment>(
          `/seasons/${encodeURIComponent(seasonId)}/members?limit=100`,
        ),
        fetchAllPages<Mentorship>(
          `/seasons/${encodeURIComponent(seasonId)}/mentorships?limit=100`,
        ),
      ]);
      const users = new Map(
        adaptUserPage(userPage).items.map((user) => [user.id, user]),
      );
      const mentorForStudent = new Map(
        mentorshipPage.items.map((mentorship) => [
          mentorship.studentUserId,
          mentorship.mentorUserId,
        ]),
      );
      const assignedStudents = new Set(mentorForStudent.keys());
      const items = enrollmentPage.items.flatMap((enrollment) => {
        const user = users.get(enrollment.userId);
        if (!user) return [];
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
                    : null,
            enrollmentId: enrollment.id,
            enrollmentRevision: enrollment.revision,
            enrollmentState: enrollment.state,
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

export function useMockInterviews(enabled = true) {
  return useQuery({
    queryKey: ['mock-interviews'],
    enabled,
    queryFn: async () => {
      if (demoMode) return wait(page(mockInterviews));
      const [interviewPage, seasonPage, problemPage] = await Promise.all([
        fetchAllPages<ApiMockInterviewPage['items'][number]>(
          '/mock-interviews?limit=100&mode=all',
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
