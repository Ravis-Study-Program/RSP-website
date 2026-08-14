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

  it('loads and updates the current member settings with a revision', async () => {
    apiRequestMock.mockResolvedValueOnce({
      premiumOptIn: false,
      goalsEnabled: true,
      easyMinutes: 20,
      mediumMinutes: 35,
      hardMinutes: 50,
      revision: 3,
    });
    apiRequestMock.mockResolvedValueOnce({
      premiumOptIn: true,
      goalsEnabled: true,
      easyMinutes: 25,
      mediumMinutes: 40,
      hardMinutes: 60,
      revision: 4,
    });

    await fetchPracticeSettings();
    await patchPracticeSettings({
      premiumOptIn: true,
      easyMinutes: 25,
      mediumMinutes: 40,
      hardMinutes: 60,
      revision: 3,
    });

    expect(apiRequestMock).toHaveBeenNthCalledWith(1, '/me/practice-settings');
    expect(apiRequestMock).toHaveBeenNthCalledWith(2, '/me/practice-settings', {
      method: 'PATCH',
      body: JSON.stringify({
        premiumOptIn: true,
        easyMinutes: 25,
        mediumMinutes: 40,
        hardMinutes: 60,
        revision: 3,
      }),
    });
  });

  it('uses the season-scoped mentor/admin goal enablement endpoint', async () => {
    apiRequestMock.mockResolvedValueOnce({
      premiumOptIn: false,
      goalsEnabled: false,
      easyMinutes: 20,
      mediumMinutes: 35,
      hardMinutes: 50,
      revision: 4,
    });
    apiRequestMock.mockResolvedValueOnce({
      premiumOptIn: false,
      goalsEnabled: true,
      easyMinutes: 20,
      mediumMinutes: 35,
      hardMinutes: 50,
      revision: 2,
    });

    await fetchUserPracticeSettings('user/with space');
    await enablePracticeGoals('user/with space', {
      seasonId: 'season-2026-s2',
      revision: 4,
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
        body: JSON.stringify({ seasonId: 'season-2026-s2', revision: 4 }),
      },
    );
  });
});
