import { apiRequest } from '@/api/client';
import {
  fetchPracticeSettings,
  fetchProgrammePracticeGoals,
  fetchUserPracticeSettings,
  setStudentPracticeGoals,
  updateProgrammePracticeGoals,
} from '@/api/practiceSettingsClient';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

const goals = { easyMinutes: 12, mediumMinutes: 28, hardMinutes: 45 };

it('fetches the goals for the current user or an encoded member ID', async () => {
  await fetchPracticeSettings();
  await fetchUserPracticeSettings('user/with space');
  expect(apiRequest).toHaveBeenNthCalledWith(1, '/me/practice-settings');
  expect(apiRequest).toHaveBeenNthCalledWith(
    2,
    '/users/user%2Fwith%20space/practice-settings',
  );
});

it('reads and replaces the programme goals', async () => {
  vi.mocked(apiRequest).mockClear();
  await fetchProgrammePracticeGoals();
  await updateProgrammePracticeGoals(goals);
  expect(apiRequest).toHaveBeenNthCalledWith(1, '/practice-goals');
  expect(apiRequest).toHaveBeenNthCalledWith(2, '/practice-goals', {
    method: 'PUT',
    body: JSON.stringify(goals),
  });
});

it('sets or clears a student override on their encoded enrollment', async () => {
  vi.mocked(apiRequest).mockClear();
  await setStudentPracticeGoals('season/1', 'member 2', goals);
  await setStudentPracticeGoals('season/1', 'member 2', null);
  const path = '/seasons/season%2F1/members/member%202/practice-goals';
  expect(apiRequest).toHaveBeenNthCalledWith(1, path, {
    method: 'PUT',
    body: JSON.stringify(goals),
  });
  expect(apiRequest).toHaveBeenNthCalledWith(2, path, { method: 'DELETE' });
});
