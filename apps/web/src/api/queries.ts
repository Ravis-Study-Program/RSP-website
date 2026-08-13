import { queryOptions, useQuery } from '@tanstack/react-query';

import { apiRequest } from '@/api/client';
import {
  attempts,
  demoUser,
  mockInterviews,
  people,
  recommendation,
  seasons,
} from '@/data/demo';
import type { Attempt, CurrentUser, MockInterview, Page, Person, Recommendation, Season } from '@/types';

export const demoMode = import.meta.env.DEV || import.meta.env.VITE_USE_DEMO_DATA === 'true';

const wait = <T,>(value: T) => new Promise<T>((resolve) => window.setTimeout(() => resolve(value), 90));
const page = <T,>(items: T[]): Page<T> => ({
  items,
  totalCount: items.length,
  pageInfo: { nextCursor: null, previousCursor: null, hasMore: false },
});

export const currentUserOptions = queryOptions({
  queryKey: ['me'],
  queryFn: () => (demoMode ? wait(demoUser) : apiRequest<CurrentUser>('/me')),
  staleTime: 60_000,
});

export const seasonsOptions = queryOptions({
  queryKey: ['seasons'],
  queryFn: () => (demoMode ? wait(page(seasons)) : apiRequest<Page<Season>>('/seasons?limit=100')),
});

export function useCurrentUser() {
  return useQuery(currentUserOptions);
}

export function useSeasons() {
  return useQuery(seasonsOptions);
}

export function useAttempts() {
  return useQuery({
    queryKey: ['problem-attempts'],
    queryFn: () =>
      demoMode
        ? wait(page(attempts))
        : apiRequest<Page<Attempt>>('/problem-attempts?limit=100&sort=-attemptedAt'),
  });
}

export function useRecommendation() {
  return useQuery({
    queryKey: ['recommendations', 'current'],
    queryFn: () =>
      demoMode ? wait(recommendation) : apiRequest<Recommendation>('/recommendations/current'),
  });
}

export function usePeople() {
  return useQuery({
    queryKey: ['people'],
    queryFn: () => (demoMode ? wait(page(people)) : apiRequest<Page<Person>>('/users?limit=100')),
  });
}

export function useMockInterviews() {
  return useQuery({
    queryKey: ['mock-interviews'],
    queryFn: () =>
      demoMode
        ? wait(page(mockInterviews))
        : apiRequest<Page<MockInterview>>('/mock-interviews?limit=100&sort=-occurredAt'),
  });
}
