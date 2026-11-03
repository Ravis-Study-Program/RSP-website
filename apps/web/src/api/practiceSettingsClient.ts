import { apiRequest } from '@/api/client';
import type { PracticeGoals, PracticeSettings } from '@/api/generated/models';

export function fetchPracticeSettings() {
  return apiRequest<PracticeSettings>('/me/practice-settings');
}

export function fetchUserPracticeSettings(userId: string) {
  return apiRequest<PracticeSettings>(
    `/users/${encodeURIComponent(userId)}/practice-settings`,
  );
}

export function fetchProgrammePracticeGoals() {
  return apiRequest<PracticeGoals>('/practice-goals');
}

export function updateProgrammePracticeGoals(goals: PracticeGoals) {
  return apiRequest<PracticeGoals>('/practice-goals', {
    method: 'PUT',
    body: JSON.stringify(goals),
  });
}

function studentGoalsPath(seasonId: string, enrollmentId: string) {
  return `/seasons/${encodeURIComponent(seasonId)}/members/${encodeURIComponent(enrollmentId)}/practice-goals`;
}

/** Replaces a student's goals, or clears the override when `goals` is null. */
export function setStudentPracticeGoals(
  seasonId: string,
  enrollmentId: string,
  goals: PracticeGoals | null,
) {
  return apiRequest<PracticeSettings>(
    studentGoalsPath(seasonId, enrollmentId),
    goals
      ? { method: 'PUT', body: JSON.stringify(goals) }
      : { method: 'DELETE' },
  );
}
