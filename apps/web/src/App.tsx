import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import {
  Navigate,
  Outlet,
  RouterProvider,
  createBrowserRouter,
  createMemoryRouter,
  type RouteObject,
  useParams,
} from 'react-router-dom';

import { useCurrentUser } from '@/api/queries';
import { WorkspaceProvider } from '@/auth/WorkspaceContext';
import { AppShell } from '@/components/AppShell';
import { PageSkeleton } from '@/components/StatusViews';
import {
  ForbiddenPage,
  NoSeasonPage,
  NotFoundPage,
  RouteErrorPage,
  SignInPage,
  VerifyEmailPage,
} from '@/pages/RouteStatePages';

function ProtectedShell() {
  const user = useCurrentUser();
  if (user.isError || (!user.isLoading && !user.data)) return <Navigate to="/sign-in" replace />;
  if (user.data && !user.data.emailVerified) return <Navigate to="/verify-email" replace />;
  return <WorkspaceProvider><AppShell /></WorkspaceProvider>;
}

function RequireAdmin() {
  const user = useCurrentUser();
  if (user.isLoading) return <PageSkeleton label="Checking administrator access" />;
  if (!user.data?.globalRoles.some((role) => role === 'director' || role === 'system_admin')) return <Navigate to="/forbidden" replace />;
  if (!user.data.mfaVerified) return <Navigate to="/forbidden" replace />;
  return <Outlet />;
}

function LegacySeasonRedirect({ destination }: { destination: '' | 'people' | 'practice' | 'mentor-teams' }) {
  const { slug } = useParams();
  return <Navigate to={`/seasons/${slug}${destination ? `/${destination}` : ''}`} replace />;
}

export const routeObjects: RouteObject[] = [
  { path: '/sign-in', element: <SignInPage />, errorElement: <RouteErrorPage /> },
  { path: '/verify-email', element: <VerifyEmailPage />, errorElement: <RouteErrorPage /> },
  {
    element: <ProtectedShell />,
    errorElement: <RouteErrorPage />,
    children: [
      { path: '/', element: <Navigate to="/dashboard" replace /> },
      { path: '/dashboard', lazy: async () => ({ Component: (await import('@/pages/DashboardPage')).DashboardPage }) },
      { path: '/seasons', lazy: async () => ({ Component: (await import('@/pages/SeasonsPage')).SeasonsPage }) },
      { path: '/seasons/:slug', lazy: async () => ({ Component: (await import('@/pages/SeasonWorkspacePage')).SeasonWorkspacePage }) },
      { path: '/seasons/:slug/practice', lazy: async () => ({ Component: (await import('@/pages/PracticePage')).PracticePage }) },
      { path: '/seasons/:slug/mock-interviews', lazy: async () => ({ Component: (await import('@/pages/MockInterviewsPage')).MockInterviewsPage }) },
      { path: '/seasons/:slug/people', lazy: async () => ({ Component: (await import('@/pages/PeoplePage')).PeoplePage }) },
      { path: '/seasons/:slug/mentees', lazy: async () => ({ Component: (await import('@/pages/MenteesPage')).MenteesPage }) },
      { path: '/seasons/:slug/mentor-teams', lazy: async () => ({ Component: (await import('@/pages/MentorTeamsPage')).MentorTeamsPage }) },
      { path: '/practice', lazy: async () => ({ Component: (await import('@/pages/PracticePage')).PracticePage }) },
      { path: '/mock-interviews', lazy: async () => ({ Component: (await import('@/pages/MockInterviewsPage')).MockInterviewsPage }) },
      { path: '/graduates', lazy: async () => ({ Component: (await import('@/pages/PeoplePage')).PeoplePage }) },
      { path: '/people/:slug', lazy: async () => ({ Component: (await import('@/pages/ProfilePage')).ProfilePage }) },
      { path: '/profile', lazy: async () => ({ Component: (await import('@/pages/ProfilePage')).ProfilePage }) },
      { path: '/settings', lazy: async () => ({ Component: (await import('@/pages/SettingsPage')).SettingsPage }) },
      { path: '/no-season', element: <NoSeasonPage /> },
      { path: '/forbidden', element: <ForbiddenPage /> },
      {
        element: <RequireAdmin />,
        children: [
          { path: '/admin', lazy: async () => ({ Component: (await import('@/pages/AdminPage')).AdminPage }) },
          { path: '/admin/seasons', lazy: async () => ({ Component: (await import('@/pages/AdminPage')).AdminSeasonsPage }) },
          { path: '/admin/weeks', lazy: async () => ({ Component: (await import('@/pages/AdminPage')).AdminWeeksPage }) },
          { path: '/admin/users', lazy: async () => ({ Component: (await import('@/pages/AdminPage')).AdminUsersPage }) },
          { path: '/admin/enrollments', lazy: async () => ({ Component: (await import('@/pages/AdminPage')).AdminEnrollmentsPage }) },
          { path: '/admin/mentorships', lazy: async () => ({ Component: (await import('@/pages/AdminPage')).AdminMentorshipsPage }) },
        ],
      },
      { path: '/overview', element: <Navigate to="/dashboard" replace /> },
      { path: '/users', element: <Navigate to="/graduates" replace /> },
      { path: '/mentors', element: <Navigate to="/graduates" replace /> },
      { path: '/leetcode', element: <Navigate to="/practice" replace /> },
      { path: '/admin/season-weeks', element: <Navigate to="/admin/weeks" replace /> },
      { path: '/seasons/:slug/overview', element: <LegacySeasonRedirect destination="" /> },
      { path: '/seasons/:slug/users', element: <LegacySeasonRedirect destination="people" /> },
      { path: '/seasons/:slug/leetcode', element: <LegacySeasonRedirect destination="practice" /> },
      { path: '/seasons/:slug/mentors', element: <LegacySeasonRedirect destination="mentor-teams" /> },
      { path: '/seasons/:slug/profile', element: <Navigate to="/profile" replace /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
];

export function AppProviders({ children }: { children: ReactNode }) {
  const [queryClient] = useState(() => new QueryClient({ defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } } }));
  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
}

export function App() {
  const [router] = useState(() => createBrowserRouter(routeObjects));
  return <AppProviders><RouterProvider router={router} /></AppProviders>;
}

export function createTestRouter(initialEntries: string[]) {
  return createMemoryRouter(routeObjects, { initialEntries });
}
