import { apiRequest } from '@/api/client';
import {
  fetchPracticeSettings,
  fetchUserPracticeSettings,
} from '@/api/practiceSettingsClient';
vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

it('fetches the fixed goals for the current user or an encoded member ID', async () => {
  await fetchPracticeSettings();
  await fetchUserPracticeSettings('user/with space');
  expect(apiRequest).toHaveBeenNthCalledWith(1, '/me/practice-settings');
  expect(apiRequest).toHaveBeenNthCalledWith(
    2,
    '/users/user%2Fwith%20space/practice-settings',
  );
});
