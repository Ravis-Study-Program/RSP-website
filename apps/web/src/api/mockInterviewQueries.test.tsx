import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';

import { clearAccessToken } from '@/api/client';
import { useMockInterviews, useSharedMockInterviews } from '@/api/queries';

function jwt() {
  const payload = btoa(
    JSON.stringify({ sub: 'user', exp: Math.floor(Date.now() / 1000) + 300 }),
  );
  return `header.${payload}.signature`;
}

function json(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'Content-Type': 'application/json' },
  });
}

const emptyPage = (nextCursor: string | null = null) => ({
  items: [],
  totalCount: 250,
  pageInfo: { nextCursor, previousCursor: null, hasMore: Boolean(nextCursor) },
});

/** Records every API path requested, answering mock pages from `pages`. */
function stubApi(pages: Record<string, ReturnType<typeof emptyPage>> = {}) {
  const paths: string[] = [];
  vi.stubGlobal(
    'fetch',
    vi.fn((input: string) => {
      if (input === '/api/auth/token')
        return Promise.resolve(json({ token: jwt() }));
      const path = input.replace(/^\/api\/v2/, '');
      paths.push(path);
      const cursor = new URL(path, 'http://x').searchParams.get('cursor');
      return Promise.resolve(json(pages[cursor ?? 'first'] ?? emptyPage()));
    }),
  );
  return paths;
}

function wrapper(
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } }),
) {
  return function QueryWrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>{children}</QueryClientProvider>
    );
  };
}

afterEach(() => {
  clearAccessToken();
  vi.unstubAllGlobals();
});

describe('mock interview loading', () => {
  it('asks the server for only the caller’s own tab', async () => {
    const paths = stubApi();
    const { result } = renderHook(
      () => useMockInterviews({ mode: 'received' }),
      { wrapper: wrapper() },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const interviewPaths = paths.filter((path) =>
      path.startsWith('/mock-interviews'),
    );
    expect(interviewPaths).toEqual([
      '/mock-interviews?limit=100&mode=received',
    ]);
  });

  it('scopes a person’s history by user instead of loading the platform', async () => {
    const paths = stubApi();
    const { result } = renderHook(
      () => useMockInterviews({ userId: 'user-1' }),
      { wrapper: wrapper() },
    );
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(paths).toContain(
      '/mock-interviews?limit=100&mode=all&userId=user-1',
    );
  });

  it('loads the shared list one page at a time, on request', async () => {
    const paths = stubApi({
      first: emptyPage('cursor-2'),
      'cursor-2': emptyPage('cursor-3'),
    });
    const { result } = renderHook(() => useSharedMockInterviews(true), {
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // A further page exists, but nothing fetches it until asked.
    expect(result.current.hasNextPage).toBe(true);
    expect(paths.filter((path) => path.startsWith('/mock-interviews'))).toEqual(
      ['/mock-interviews?limit=100&mode=all'],
    );

    await act(() => result.current.fetchNextPage());
    expect(paths.filter((path) => path.startsWith('/mock-interviews'))).toEqual(
      [
        '/mock-interviews?limit=100&mode=all',
        '/mock-interviews?limit=100&mode=all&cursor=cursor-2',
      ],
    );
  });

  it('reuses the cached season list across interview requests', async () => {
    const paths = stubApi();
    const shared = wrapper();
    const received = renderHook(() => useMockInterviews({ mode: 'received' }), {
      wrapper: shared,
    });
    await waitFor(() => expect(received.result.current.isSuccess).toBe(true));
    const given = renderHook(() => useMockInterviews({ mode: 'given' }), {
      wrapper: shared,
    });
    await waitFor(() => expect(given.result.current.isSuccess).toBe(true));

    expect(paths.filter((path) => path.startsWith('/seasons'))).toHaveLength(1);
  });
});
