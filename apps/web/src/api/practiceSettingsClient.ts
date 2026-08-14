import { apiRequest } from '@/api/client';
import type {
  PracticeGoalsEnablement,
  PracticeSettings,
  PracticeSettingsMutation,
} from '@/api/generated/models';

export function fetchPracticeSettings() {
  return apiRequest<PracticeSettings>('/me/practice-settings');
}

export function fetchUserPracticeSettings(userId: string) {
  return apiRequest<PracticeSettings>(
    `/users/${encodeURIComponent(userId)}/practice-settings`,
  );
}

export function patchPracticeSettings(settings: PracticeSettingsMutation) {
  return apiRequest<PracticeSettings>('/me/practice-settings', {
    method: 'PATCH',
    body: JSON.stringify(settings),
  });
}

export function enablePracticeGoals(
  userId: string,
  enablement: PracticeGoalsEnablement,
) {
  return apiRequest<PracticeSettings>(
    `/users/${encodeURIComponent(userId)}/practice-goals/enable`,
    {
      method: 'POST',
      body: JSON.stringify(enablement),
    },
  );
}
