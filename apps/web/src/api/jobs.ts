import { useQuery, useQueryClient } from '@tanstack/react-query';
import { errorFromResponse, getAccessToken } from '@/api/client';
import { demoMode } from '@/api/queries';
import type {
  JobApplication,
  JobCompany,
  JobCompanyList,
  JobPosition,
  JobPositionProgress,
  JobReferralBoard,
  JobReferralCandidate,
  JobReferralCounts,
} from '@/api/generated/models';

export class JobRequestError extends Error {
  constructor(
    message: string,
    public existingPositionId?: string,
  ) {
    super(message);
  }
}

export async function jobRequest<T>(
  path: string,
  method = 'GET',
  body?: unknown,
  userId?: string,
): Promise<T> {
  if (demoMode) {
    const { demoJobRequest } = await import('@/data/jobsDemo');
    return demoJobRequest(path, method, body, userId) as T;
  }
  const publicRead =
    method === 'GET' && /^\/jobs\/(companies|positions)(?:[/?]|$)/.test(path);
  const response = await fetch(`/api/v2${path}`, {
    method,
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json',
      ...(body ? { 'Content-Type': 'application/json' } : {}),
      ...(!publicRead
        ? { Authorization: `Bearer ${await getAccessToken()}` }
        : {}),
    },
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
  if (!response.ok) {
    if (response.status === 409) {
      const duplicate = (await response
        .clone()
        .json()
        .catch(() => null)) as {
        message?: string;
        existingPositionId?: string;
      } | null;
      if (duplicate?.existingPositionId)
        throw new JobRequestError(
          duplicate.message ?? 'This position is already listed.',
          duplicate.existingPositionId,
        );
    }
    throw await errorFromResponse(response);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

async function allJobsPages<T>(path: string, userId?: string) {
  const items: T[] = [];
  const cursors = new Set<string>();
  let cursor: string | null = null;
  do {
    const page: {
      items: T[];
      pageInfo: { nextCursor: string | null; hasMore: boolean };
    } = await jobRequest(
      `${path}?limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ''}`,
      'GET',
      undefined,
      userId,
    );
    items.push(...page.items);
    cursor = page.pageInfo.hasMore ? page.pageInfo.nextCursor : null;
    if (cursor && cursors.has(cursor))
      throw new Error('Could not load the next Jobs page. Try again.');
    if (cursor) cursors.add(cursor);
  } while (cursor);
  return items;
}

export function useJobPositions() {
  return useQuery({
    queryKey: ['jobs', 'positions'],
    queryFn: () => allJobsPages<JobPosition>('/jobs/positions'),
  });
}
export function useJobCompanies() {
  return useQuery({
    queryKey: ['jobs', 'companies'],
    queryFn: async (): Promise<JobCompany[]> =>
      (await jobRequest<JobCompanyList>('/jobs/companies')).items,
  });
}

export function useJobPositionProgress(positionId: string, enabled = true) {
  return useQuery({
    queryKey: ['jobs', 'position-progress', positionId],
    queryFn: () =>
      jobRequest<JobPositionProgress>(
        `/jobs/positions/${encodeURIComponent(positionId)}/progress`,
      ),
    enabled,
  });
}

export function useJobApplications(userId?: string) {
  return useQuery({
    queryKey: ['jobs', 'applications', userId],
    queryFn: () => allJobsPages<JobApplication>('/me/job-applications', userId),
    enabled: Boolean(userId),
  });
}

export function useJobReferralBoard(userId?: string) {
  return useQuery({
    queryKey: ['jobs', 'referrals', userId],
    queryFn: () =>
      jobRequest<JobReferralBoard>('/jobs/referrals', 'GET', undefined, userId),
    enabled: Boolean(userId),
  });
}

export function useJobReferralCounts(userId?: string) {
  return useQuery({
    queryKey: ['jobs', 'referral-counts', userId],
    queryFn: async () =>
      (
        await jobRequest<JobReferralCounts>(
          '/jobs/referrals/counts',
          'GET',
          undefined,
          userId,
        )
      ).items,
    enabled: Boolean(userId),
  });
}

export function useJobReferralCandidates(userId: string, companyId?: string) {
  return useQuery({
    queryKey: ['jobs', 'referral-candidates', userId, companyId],
    queryFn: () =>
      allJobsPages<JobReferralCandidate>(
        `/jobs/referrals/companies/${companyId}/candidates`,
        userId,
      ),
    enabled: Boolean(companyId),
  });
}

export function useJobActions(userId?: string) {
  const client = useQueryClient();
  return async <T>(path: string, method: string, body?: unknown) => {
    const result = await jobRequest<T>(path, method, body, userId);
    await client.invalidateQueries({ queryKey: ['jobs'] });
    return result;
  };
}
