import {
  QueryClient,
  QueryClientProvider,
  useQuery,
} from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import {
  Navigate,
  Outlet,
  RouterProvider,
  createBrowserRouter,
  createMemoryRouter,
  type RouteObject,
  useLocation,
  useParams,
} from 'react-router-dom';

import { getBrowserAuthSession } from '@/api/authClient';
import { demoMode, useCurrentUser } from '@/api/queries';
import { WorkspaceProvider } from '@/auth/WorkspaceContext';
import { AppShell } from '@/components/AppShell';
import { PageSkeleton } from '@/components/StatusViews';
import {
  ForbiddenPage,
  NoSeasonPage,
  NotFoundPage,
  AccountUnavailablePage,
  RouteErrorPage,
  SignInPage,
  ResetPasswordPage,
  TwoFactorPage,
  VerifyEmailPage,
} from '@/pages/RouteStatePages';

function ProtectedShell() {
  const user = useCurrentUser();
  const location = useLocation();
  const browserSession = useQuery({
    queryKey: ['auth-session'],
    queryFn: getBrowserAuthSession,
    enabled: user.isError && !demoMode,
    retry: false,
    staleTime: 30_000,
  });
  if (user.isError && !demoMode && browserSession.isLoading)
    return <PageSkeleton label="Checking account access" />;
  if (user.isError) {
    const sessionUser = browserSession.data?.user;
    if (sessionUser && !sessionUser.emailVerified)
      return (
        <Navigate
          to="/verify-email"
          replace
          state={{ email: sessionUser.email }}
        />
      );
    if (sessionUser?.accountState && sessionUser.accountState !== 'active')
      return (
        <Navigate
          to={`/account-unavailable?state=${sessionUser.accountState}`}
          replace
        />
      );
    return <Navigate to="/sign-in" replace />;
  }
  if (!user.isLoading && !user.data) return <Navigate to="/sign-in" replace />;
  if (user.data && !user.data.emailVerified)
    return <Navigate to="/verify-email" replace />;
  if (user.data && user.data.accountState !== 'active')
    return (
      <Navigate
        to={`/account-unavailable?state=${user.data.accountState}`}
        replace
      />
    );
  const isMember = Boolean(
    user.data &&
    (user.data.seasonRoles.length > 0 ||
      user.data.globalRoles.length > 0 ||
      user.data.alumni),
  );
  const nonmemberRoutes = ['/no-season', '/profile', '/settings'];
  if (user.data && !isMember && !nonmemberRoutes.includes(location.pathname))
    return <Navigate to="/no-season" replace />;
  const hasDirectoryAccess = Boolean(
    user.data &&
    (user.data.globalRoles.length > 0 ||
      user.data.alumni ||
      user.data.seasonRoles.some(
        (membership) => membership.state === 'active',
      )),
  );
  const formerMemberRoutes = [
    '/dashboard',
    '/practice',
    '/mock-interviews',
    '/profile',
    '/settings',
  ];
  if (
    user.data &&
    isMember &&
    !hasDirectoryAccess &&
    !formerMemberRoutes.includes(location.pathname)
  )
    return <Navigate to="/forbidden" replace />;
  return (
    <WorkspaceProvider>
      <AppShell />
    </WorkspaceProvider>
  );
}

function RequireAdmin() {
  const user = useCurrentUser();
  if (user.isLoading)
    return <PageSkeleton label="Checking administrator access" />;
  if (
    !user.data?.globalRoles.some(
      (role) => role === 'director' || role === 'system_admin',
    )
  )
    return <Navigate to="/forbidden" replace />;
  if (!user.data.mfaVerified) return <Navigate to="/two-factor" replace />;
  return <Outlet />;
}

function RequireSeasonOperations() {
  const user = useCurrentUser();
  if (user.isLoading)
    return <PageSkeleton label="Checking programme administration access" />;
  const allowed = Boolean(
    user.data &&
    (user.data.globalRoles.length > 0 ||
      user.data.seasonRoles.some(
        (membership) =>
          membership.role === 'coordinator' && membership.state === 'active',
      )),
  );
  if (!allowed) return <Navigate to="/forbidden" replace />;
  if (!user.data?.mfaVerified) return <Navigate to="/two-factor" replace />;
  return <Outlet />;
}

function RequireSystemAdmin() {
  const user = useCurrentUser();
  if (user.isLoading)
    return <PageSkeleton label="Checking System Admin access" />;
  if (!user.data?.globalRoles.includes('system_admin'))
    return <Navigate to="/forbidden" replace />;
  if (!user.data.mfaVerified) return <Navigate to="/two-factor" replace />;
  return <Outlet />;
}

function LegacySeasonRedirect({
  destination,
}: {
  destination: '' | 'people' | 'practice' | 'mentor-teams';
}) {
  const { slug } = useParams();
  return (
    <Navigate
      to={`/seasons/${slug}${destination ? `/${destination}` : ''}`}
      replace
    />
  );
}

function LegacyProfileRedirect() {
  const location = useLocation();
  const legacyUser = new URLSearchParams(location.search).get('user');
  return (
    <Navigate
      to={legacyUser ? `/people/${encodeURIComponent(legacyUser)}` : '/profile'}
      replace
    />
  );
}

export const routeObjects: RouteObject[] = [
  {
    path: '/sign-in',
    element: <SignInPage />,
    errorElement: <RouteErrorPage />,
  },
  {
    path: '/verify-email',
    element: <VerifyEmailPage />,
    errorElement: <RouteErrorPage />,
  },
  {
    path: '/two-factor',
    element: <TwoFactorPage />,
    errorElement: <RouteErrorPage />,
  },
  {
    path: '/reset-password',
    element: <ResetPasswordPage />,
    errorElement: <RouteErrorPage />,
  },
  {
    path: '/account-unavailable',
    element: <AccountUnavailablePage />,
    errorElement: <RouteErrorPage />,
  },
  {
    element: <ProtectedShell />,
    errorElement: <RouteErrorPage />,
    children: [
      { path: '/', element: <Navigate to="/dashboard" replace /> },
      {
        path: '/dashboard',
        lazy: async () => ({
          Component: (await import('@/pages/DashboardPage')).DashboardPage,
        }),
      },
      {
        path: '/seasons',
        lazy: async () => ({
          Component: (await import('@/pages/SeasonsPage')).SeasonsPage,
        }),
      },
      {
        path: '/seasons/:slug',
        lazy: async () => ({
          Component: (await import('@/pages/SeasonWorkspacePage'))
            .SeasonWorkspacePage,
        }),
      },
      {
        path: '/seasons/:slug/practice',
        lazy: async () => ({
          Component: (await import('@/pages/PracticePage')).PracticePage,
        }),
      },
      {
        path: '/seasons/:slug/mock-interviews',
        lazy: async () => ({
          Component: (await import('@/pages/MockInterviewsPage'))
            .MockInterviewsPage,
        }),
      },
      {
        path: '/seasons/:slug/people',
        lazy: async () => ({
          Component: (await import('@/pages/PeoplePage')).PeoplePage,
        }),
      },
      {
        path: '/seasons/:slug/mentees',
        lazy: async () => ({
          Component: (await import('@/pages/MenteesPage')).MenteesPage,
        }),
      },
      {
        path: '/seasons/:slug/mentor-teams',
        lazy: async () => ({
          Component: (await import('@/pages/MentorTeamsPage')).MentorTeamsPage,
        }),
      },
      {
        path: '/practice',
        lazy: async () => ({
          Component: (await import('@/pages/PracticePage')).PracticePage,
        }),
      },
      {
        path: '/mock-interviews',
        lazy: async () => ({
          Component: (await import('@/pages/MockInterviewsPage'))
            .MockInterviewsPage,
        }),
      },
      {
        path: '/graduates',
        lazy: async () => ({
          Component: (await import('@/pages/PeoplePage')).PeoplePage,
        }),
      },
      {
        path: '/people/:slug',
        lazy: async () => ({
          Component: (await import('@/pages/ProfilePage')).ProfilePage,
        }),
      },
      {
        path: '/profile',
        lazy: async () => ({
          Component: (await import('@/pages/ProfileRoutePage'))
            .ProfileRoutePage,
        }),
      },
      {
        path: '/settings',
        lazy: async () => ({
          Component: (await import('@/pages/SettingsPage')).SettingsPage,
        }),
      },
      { path: '/no-season', element: <NoSeasonPage /> },
      { path: '/forbidden', element: <ForbiddenPage /> },
      {
        element: <RequireAdmin />,
        children: [
          {
            path: '/admin',
            lazy: async () => ({
              Component: (await import('@/pages/AdminPage')).AdminPage,
            }),
          },
          {
            path: '/admin/seasons',
            lazy: async () => ({
              Component: (await import('@/pages/AdminPage')).AdminSeasonsPage,
            }),
          },
        ],
      },
      {
        element: <RequireSystemAdmin />,
        children: [
          {
            path: '/admin/users',
            lazy: async () => ({
              Component: (await import('@/pages/AdminPage')).AdminUsersPage,
            }),
          },
        ],
      },
      {
        element: <RequireSeasonOperations />,
        children: [
          {
            path: '/admin/weeks',
            lazy: async () => ({
              Component: (await import('@/pages/AdminPage')).AdminWeeksPage,
            }),
          },
          {
            path: '/admin/enrollments',
            lazy: async () => ({
              Component: (await import('@/pages/AdminPage'))
                .AdminEnrollmentsPage,
            }),
          },
          {
            path: '/admin/mentorships',
            lazy: async () => ({
              Component: (await import('@/pages/AdminPage'))
                .AdminMentorshipsPage,
            }),
          },
        ],
      },
      { path: '/overview', element: <Navigate to="/dashboard" replace /> },
      { path: '/users', element: <Navigate to="/graduates" replace /> },
      { path: '/mentors', element: <Navigate to="/graduates" replace /> },
      { path: '/leetcode', element: <Navigate to="/practice" replace /> },
      {
        path: '/admin/season-weeks',
        element: <Navigate to="/admin/weeks" replace />,
      },
      {
        path: '/seasons/:slug/overview',
        element: <LegacySeasonRedirect destination="" />,
      },
      {
        path: '/seasons/:slug/users',
        element: <LegacySeasonRedirect destination="people" />,
      },
      {
        path: '/seasons/:slug/leetcode',
        element: <LegacySeasonRedirect destination="practice" />,
      },
      {
        path: '/seasons/:slug/mentors',
        element: <LegacySeasonRedirect destination="mentor-teams" />,
      },
      { path: '/seasons/:slug/profile', element: <LegacyProfileRedirect /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
];

export function AppProviders({ children }: { children: ReactNode }) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
      }),
  );
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

export function App() {
  const [router] = useState(() => createBrowserRouter(routeObjects));
  return (
    <AppProviders>
      <RouterProvider router={router} />
    </AppProviders>
  );
}

export function createTestRouter(initialEntries: string[]) {
  return createMemoryRouter(routeObjects, { initialEntries });
}
