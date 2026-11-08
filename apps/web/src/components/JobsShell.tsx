import { createContext, useContext, useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, Navigate, Outlet, useLocation } from 'react-router-dom';
import { IconMoon, IconSun } from '@tabler/icons-react';
import { getBrowserAuthSession } from '@/api/authClient';
import { currentUserOptions, demoMode } from '@/api/queries';
import { AppShell } from '@/components/AppShell';
import { Button, buttonVariants } from '@/components/ui/button';
import { PageSkeleton, ErrorState } from '@/components/StatusViews';
import type { CurrentUser } from '@/types';

const JobsUserContext = createContext<CurrentUser | undefined>(undefined);
// eslint-disable-next-line react-refresh/only-export-components
export const useJobsUser = () => useContext(JobsUserContext);

export function JobsShell() {
  const location = useLocation();
  const membersOnly = ['/jobs/applications', '/jobs/referrals'].includes(
    location.pathname,
  );
  const session = useQuery({
    queryKey: ['auth-session'],
    queryFn: getBrowserAuthSession,
    enabled: !demoMode,
    retry: false,
  });
  const user = useQuery({
    ...currentUserOptions,
    enabled: demoMode || Boolean(session.data?.user?.emailVerified),
    retry: false,
  });
  const signedIn =
    (demoMode || session.data?.user?.emailVerified) &&
    user.data?.emailVerified &&
    user.data.accountState === 'active'
      ? user.data
      : undefined;
  useEffect(() => {
    if (signedIn) sessionStorage.removeItem('rsp-auth-return-to');
    else if (membersOnly)
      sessionStorage.setItem(
        'rsp-auth-return-to',
        location.pathname + location.search,
      );
  }, [signedIn, membersOnly, location.pathname, location.search]);
  if (
    (!demoMode && session.isLoading) ||
    ((demoMode || session.data?.user?.emailVerified) && user.isLoading)
  )
    return <PageSkeleton label="Loading Jobs" />;
  // Auth outages must not turn a private application screen into an empty
  // signed-out state, while the public board remains browsable.
  if (membersOnly && !signedIn) {
    if (session.data?.user && !session.data.user.emailVerified)
      return (
        <Navigate
          to="/verify-email"
          state={{ email: session.data.user.email }}
          replace
        />
      );
    if (
      session.data?.user?.accountState &&
      session.data.user.accountState !== 'active'
    )
      return (
        <Navigate
          to={`/account-unavailable?state=${session.data.user.accountState}`}
          replace
        />
      );
    if (session.isError || (session.data?.user?.emailVerified && user.isError))
      return (
        <ErrorState
          onRetry={() => {
            void session.refetch();
            void user.refetch();
          }}
        />
      );
    return (
      <Navigate
        to={`/sign-in?returnTo=${encodeURIComponent(location.pathname + location.search)}`}
        replace
      />
    );
  }
  return (
    <JobsUserContext.Provider value={signedIn}>
      {signedIn ? <AppShell /> : <PublicJobsShell />}
    </JobsUserContext.Provider>
  );
}

function PublicJobsShell() {
  const [theme, setTheme] = useState(
    () =>
      localStorage.getItem('rsp-theme') ??
      (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'),
  );
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('rsp-theme', theme);
  }, [theme]);
  const Icon = theme === 'dark' ? IconSun : IconMoon;
  return (
    <div className="min-h-svh">
      <header className="flex min-h-16 items-center justify-between gap-3 border-b bg-sidebar px-4 py-3 md:px-8">
        <Link
          to="/jobs"
          className="flex min-w-0 items-center gap-3 font-semibold text-foreground no-underline"
        >
          <img src="/assets/rsp-logo.png" alt="" className="size-10" />
          <span>Ravi’s Study Program</span>
        </Link>
        <div className="flex shrink-0 items-center gap-2">
          <Button
            variant="ghost"
            size="icon"
            aria-label={`Use ${theme === 'dark' ? 'light' : 'dark'} theme`}
            onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          >
            <Icon size={19} aria-hidden="true" />
          </Button>
          <Link
            className={buttonVariants({ variant: 'outline' })}
            to="/sign-in?returnTo=%2Fjobs"
          >
            Sign in
          </Link>
        </div>
      </header>
      <main id="main-content" className="p-4 md:p-8">
        <Outlet />
      </main>
    </div>
  );
}
