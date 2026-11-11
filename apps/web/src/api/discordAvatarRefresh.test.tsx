import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';

import { refreshDiscordAvatar } from '@/api/authClient';
import { useDiscordAvatarRefresh } from '@/api/queries';

// `pnpm test` runs in demo mode, where the refresh is off; test the real path.
vi.mock('@/api/demoMode', () => ({ demoModeForEnvironment: () => false }));
vi.mock('@/api/authClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/api/authClient')>()),
  refreshDiscordAvatar: vi.fn(),
}));

function setup() {
  const client = new QueryClient();
  const invalidate = vi.spyOn(client, 'invalidateQueries');
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  return { invalidate, wrapper };
}

describe('Discord profile picture refresh', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.mocked(refreshDiscordAvatar).mockReset();
  });

  it('checks once per visit and reloads the member when the picture changed', async () => {
    vi.mocked(refreshDiscordAvatar).mockResolvedValue({ result: 'updated' });
    const { invalidate, wrapper } = setup();
    const { rerender } = renderHook(
      ({ id }: { id?: string }) => useDiscordAvatarRefresh(id),
      { wrapper, initialProps: { id: undefined as string | undefined } },
    );
    expect(refreshDiscordAvatar).not.toHaveBeenCalled();
    rerender({ id: 'member' });
    await waitFor(() =>
      expect(invalidate).toHaveBeenCalledWith({ queryKey: ['me'] }),
    );
    // Moving between pages, or a second app shell in the same tab, does not ask again.
    renderHook(() => useDiscordAvatarRefresh('member'), { wrapper });
    expect(refreshDiscordAvatar).toHaveBeenCalledTimes(1);
  });

  it('leaves the member alone when nothing changed or the check failed', async () => {
    vi.mocked(refreshDiscordAvatar).mockResolvedValueOnce({
      result: 'unchanged',
    });
    const { invalidate, wrapper } = setup();
    renderHook(() => useDiscordAvatarRefresh('one'), { wrapper });
    vi.mocked(refreshDiscordAvatar).mockRejectedValueOnce(new Error('down'));
    renderHook(() => useDiscordAvatarRefresh('two'), { wrapper });
    await waitFor(() => expect(refreshDiscordAvatar).toHaveBeenCalledTimes(2));
    expect(invalidate).not.toHaveBeenCalled();
  });
});
