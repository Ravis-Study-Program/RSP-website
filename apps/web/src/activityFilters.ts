import { useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import type { Attempt, MockInterview } from '@/types';
import { interviewPassed } from '@/utils';

export interface ActivityFilters {
  difficulties: string[];
  categories: string[];
  weeks: string[];
  interviewers: string[];
  passed: string;
}
export const emptyActivityFilters: ActivityFilters = {
  difficulties: [],
  categories: [],
  weeks: [],
  interviewers: [],
  passed: '',
};
const keys = {
  difficulties: 'difficulty',
  categories: 'category',
  weeks: 'weekId',
  interviewers: 'interviewerId',
  passed: 'passed',
} as const;

export function activityFilterQuery(filters: ActivityFilters) {
  const params = new URLSearchParams();
  for (const [field, key] of Object.entries(keys)) {
    const value = filters[field as keyof ActivityFilters];
    for (const item of typeof value === 'string'
      ? value
        ? [value]
        : []
      : value)
      params.append(key, item);
  }
  return params.size ? `&${params}` : '';
}

export function useActivityFilters(seasonId?: string) {
  const [params, setParams] = useSearchParams();
  const filters = useMemo<ActivityFilters>(
    () => ({
      difficulties: params.getAll('difficulty'),
      categories: params.getAll('category'),
      weeks: seasonId ? params.getAll('weekId') : [],
      interviewers: params.getAll('interviewerId'),
      passed: params.get('passed') ?? '',
    }),
    [params, seasonId],
  );
  const update = (change: Partial<ActivityFilters>) => {
    const next = new URLSearchParams(params);
    for (const [field, value] of Object.entries(change)) {
      const key = keys[field as keyof ActivityFilters];
      next.delete(key);
      for (const item of typeof value === 'string'
        ? value
          ? [value]
          : []
        : value)
        next.append(key, item);
    }
    setParams(next, { replace: true });
  };
  return [filters, update] as const;
}

export function filterPractice(attempts: Attempt[], filters: ActivityFilters) {
  return attempts.filter(
    (a) =>
      (!filters.difficulties.length ||
        filters.difficulties.includes(a.difficulty?.toLowerCase() ?? '')) &&
      (!filters.categories.length ||
        (a.categories ?? (a.category ? [a.category] : [])).some((c) =>
          filters.categories.includes(c),
        )) &&
      (!filters.weeks.length || filters.weeks.includes(a.weekId ?? '')),
  );
}
export function filterMocks(mocks: MockInterview[], filters: ActivityFilters) {
  return mocks.filter(
    (m) =>
      (!filters.interviewers.length ||
        filters.interviewers.includes(m.interviewer.id)) &&
      (!filters.passed || interviewPassed(m) === (filters.passed === 'true')) &&
      (!filters.weeks.length || filters.weeks.includes(m.weekId ?? '')),
  );
}
