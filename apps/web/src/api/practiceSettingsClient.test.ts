import { apiRequest } from '@/api/client';
import {
  enablePracticeGoals,
  fetchPracticeSettings,
  fetchUserPracticeSettings,
  patchPracticeSettings,
} from '@/api/practiceSettingsClient';

vi.mock('@/api/client', () => ({ apiRequest: vi.fn() }));

const apiRequestMock = vi.mocked(apiRequest);

describe('practice settings API', () => {
  beforeEach(() => apiRequestMock.mockReset());

  it('loads and updates the current member settings', async () => {
    apiRequestMock.mockResolvedValueOnce({
      goalsEnabled: true,
      easyMinutes: 10,
      mediumMinutes: 20,
      hardMinutes: 45,
    });
    apiRequestMock.mockResolvedValueOnce({
      goalsEnabled: true,
      easyMinutes: 25,
      mediumMinutes: 40,
      hardMinutes: 60,
    });

    await fetchPracticeSettings();
    await patchPracticeSettings({
      easyMinutes: 25,
      mediumMinutes: 40,
      hardMinutes: 60,
    });

    expect(apiRequestMock).toHaveBeenNthCalledWith(1, '/me/practice-settings');
    expect(apiRequestMock).toHaveBeenNthCalledWith(2, '/me/practice-settings', {
      method: 'PATCH',
      body: JSON.stringify({
        easyMinutes: 25,
        mediumMinutes: 40,
        hardMinutes: 60,
      }),
    });
  });

  it('uses the season-scoped mentor/admin goal enablement endpoint', async () => {
    apiRequestMock.mockResolvedValueOnce({
      goalsEnabled: false,
      easyMinutes: 10,
      mediumMinutes: 20,
      hardMinutes: 45,
    });
    apiRequestMock.mockResolvedValueOnce({
      goalsEnabled: true,
      easyMinutes: 10,
      mediumMinutes: 20,
      hardMinutes: 45,
    });

    await fetchUserPracticeSettings('user/with space');
    await enablePracticeGoals('user/with space', {
      seasonId: 'season-2026-s2',
    });

    expect(apiRequestMock).toHaveBeenNthCalledWith(
      1,
      '/users/user%2Fwith%20space/practice-settings',
    );
    expect(apiRequestMock).toHaveBeenNthCalledWith(
      2,
      '/users/user%2Fwith%20space/practice-goals/enable',
      {
        method: 'POST',
        body: JSON.stringify({ seasonId: 'season-2026-s2' }),
      },
    );
  });
});
