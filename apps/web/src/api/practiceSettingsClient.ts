import { apiRequest } from '@/api/client';
import type { PracticeSettings } from '@/api/generated/models';

export function fetchPracticeSettings() {
  return apiRequest<PracticeSettings>('/me/practice-settings');
}

export function fetchUserPracticeSettings(userId: string) {
  return apiRequest<PracticeSettings>(
    `/users/${encodeURIComponent(userId)}/practice-settings`,
  );
}
