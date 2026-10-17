import { Button, buttonVariants } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet';
import {
  IconAdjustments,
  IconBriefcase,
  IconCalendarEvent,
  IconDashboard,
  IconDots,
  IconLogout,
  IconMessage2,
  IconMoon,
  IconNotebook,
  IconSchool,
  IconSettings,
  IconSun,
  IconUsers,
} from '@tabler/icons-react';
import { useEffect, useMemo, useState, type ComponentType } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import {
  Link,
  NavLink,
  Outlet,
  useNavigate,
  useLocation,
} from 'react-router-dom';

import { signOut } from '@/api/authClient';
import { useCurrentUser } from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import { CommandPalette } from '@/components/CommandPalette';
import { useCurrentPageTitle } from '@/components/Common';
import { cn } from '@/lib/utils';
import { initials } from '@/utils';

const brandClass =
  'flex min-h-12 items-center gap-3 rounded-lg px-1.5 text-sidebar-foreground no-underline outline-none focus-visible:ring-3 focus-visible:ring-ring/50';
const avatarClass =
  'inline-grid size-9 shrink-0 place-items-center rounded-full bg-primary text-xs font-semibold text-primary-foreground';
const navLinkClass =
  'flex min-h-11 items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-sidebar-muted no-underline outline-none hover:bg-sidebar-accent hover:text-sidebar-foreground focus-visible:ring-3 focus-visible:ring-ring/50';
const navLinkActiveClass = 'bg-sidebar-accent text-sidebar-foreground';
const mobileNavLinkClass =
  'flex min-h-14 min-w-0 flex-col items-center justify-center gap-0.5 rounded-lg px-0.5 py-1.5 text-center text-[0.6875rem] leading-tight font-medium text-muted-foreground no-underline outline-none focus-visible:ring-3 focus-visible:ring-ring/50';
const mobileNavLinkActiveClass = 'bg-muted text-foreground';
const mobileNavLabelClass = 'block max-w-full truncate';
const mobileNavBarClass =
  'hidden max-[56rem]:grid fixed inset-x-0 bottom-0 z-30 grid-flow-col auto-cols-fr gap-1 border-t border-sidebar-border bg-sidebar px-1.5 pt-1.5 pb-[calc(0.4rem+env(safe-area-inset-bottom))]';
const sheetNavLinkClass =
  'flex min-h-11 items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-muted-foreground no-underline outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50';
const sheetNavLinkActiveClass = 'bg-muted text-foreground';

interface NavItem {
  label: string;
  to: string;
  icon: ComponentType<{ size?: number; 'aria-hidden'?: boolean }>;
  end?: boolean;
}

function itemsFor(role: string | undefined, seasonSlug?: string): NavItem[] {
  const seasonBase = seasonSlug ? `/seasons/${seasonSlug}` : '/seasons';
  if (role === 'student') {
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'Season', to: seasonBase, icon: IconSchool, end: true },
      { label: 'LeetCode', to: `${seasonBase}/leetcode`, icon: IconBriefcase },
      {
        label: 'Mock interviews',
        to: `${seasonBase}/mock-interviews`,
        icon: IconMessage2,
      },
      {
        label: 'Monday updates',
        to: `${seasonBase}/monday-updates`,
        icon: IconNotebook,
      },
      { label: 'People', to: `${seasonBase}/people`, icon: IconUsers },
    ];
  }
  if (role === 'mentor') {
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'My students', to: `${seasonBase}/mentees`, icon: IconUsers },
      {
        label: 'Mock interviews',
        to: `${seasonBase}/mock-interviews`,
        icon: IconMessage2,
      },
      {
        label: 'Monday updates',
        to: `${seasonBase}/monday-updates`,
        icon: IconNotebook,
      },
      { label: 'People', to: `${seasonBase}/people`, icon: IconSchool },
    ];
  }
  if (role === 'coordinator') {
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'Season', to: seasonBase, icon: IconCalendarEvent, end: true },
      { label: 'People', to: `${seasonBase}/people`, icon: IconUsers },
      { label: 'All students', to: `${seasonBase}/mentees`, icon: IconUsers },
      {
        label: 'Mentor teams',
        to: `${seasonBase}/mentor-teams`,
        icon: IconSchool,
      },
      {
        label: 'Mock interviews',
        to: `${seasonBase}/mock-interviews`,
        icon: IconMessage2,
      },
      {
        label: 'Monday updates',
        to: `${seasonBase}/monday-updates`,
        icon: IconNotebook,
      },
      {
        label: 'Season operations',
        to: '/admin/enrollments',
        icon: IconAdjustments,
      },
    ];
  }
  if (role === 'director' || role === 'system_admin') {
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'Seasons', to: '/seasons', icon: IconCalendarEvent },
      { label: 'People', to: '/graduates', icon: IconUsers },
      { label: 'LeetCode', to: '/leetcode', icon: IconBriefcase },
      { label: 'Administration', to: '/admin', icon: IconAdjustments },
    ];
  }
  if (role === 'graduate')
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'LeetCode', to: '/leetcode', icon: IconBriefcase },
      { label: 'Mock interviews', to: '/mock-interviews', icon: IconMessage2 },
      { label: 'Directory', to: '/graduates', icon: IconUsers },
    ];
  if (role === 'former_member')
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'LeetCode', to: '/leetcode', icon: IconBriefcase },
      { label: 'Mock interviews', to: '/mock-interviews', icon: IconMessage2 },
    ];
  return [{ label: 'Profile', to: '/profile', icon: IconUsers }];
}

function ThemeButton({
  theme,
  onToggle,
  inverse = false,
}: {
  theme: string;
  onToggle: () => void;
  inverse?: boolean;
}) {
  const Icon = theme === 'dark' ? IconSun : IconMoon;
  return (
    <Button
      variant="ghost"
      size={inverse ? 'default' : 'icon'}
      className={inverse ? cn(navLinkClass, 'w-full justify-start') : 'size-10'}
      type="button"
      onClick={onToggle}
      aria-label={`Use ${theme === 'dark' ? 'light' : 'dark'} theme`}
    >
      <Icon size={19} aria-hidden="true" />
      {inverse ? (
        <span>{theme === 'dark' ? 'Light theme' : 'Dark theme'}</span>
      ) : null}
    </Button>
  );
}

export function AppShell() {
  const userQuery = useCurrentUser();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const { workspaces, activeWorkspace, setActiveWorkspaceId } = useWorkspace();
  const pageTitle = useCurrentPageTitle();
  const [theme, setTheme] = useState(
    () =>
      localStorage.getItem('rsp-theme') ??
      (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'),
  );
  const navItems = useMemo(
    () => itemsFor(activeWorkspace?.role, activeWorkspace?.seasonSlug),
    [activeWorkspace],
  );
  const homePath = workspaces.length ? '/dashboard' : '/profile';
  const handleSignOut = async () => {
    try {
      await signOut();
    } finally {
      queryClient.clear();
      navigate('/sign-in', { replace: true });
    }
  };

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('rsp-theme', theme);
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute('content', theme === 'dark' ? '#171717' : '#ffffff');
  }, [theme]);

  const workspaceSelect = (className: string, showLabel: boolean) => {
    if (!workspaces.length) return null;
    const workspaceItems = workspaces.map((workspace) => ({
      label: workspace.label,
      value: workspace.id,
    }));
    return (
      <div className={showLabel ? 'mt-6 mb-4 grid gap-1.5' : 'min-w-0 flex-1'}>
        {showLabel ? (
          <label
            className="text-xs font-medium text-sidebar-muted"
            htmlFor="workspace-select"
          >
            Workspace
          </label>
        ) : (
          <label className="sr-only" htmlFor="workspace-select-mobile">
            Workspace
          </label>
        )}
        <Select
          items={workspaceItems}
          value={activeWorkspace?.id ?? ''}
          onValueChange={(value) => setActiveWorkspaceId(value ?? '')}
        >
          <SelectTrigger
            id={showLabel ? 'workspace-select' : 'workspace-select-mobile'}
            className={cn(
              'border-sidebar-border bg-sidebar text-sidebar-foreground',
              className,
            )}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {workspaceItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
    );
  };

  return (
    <div className="min-h-svh">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-lg focus:bg-primary focus:px-3.5 focus:py-2.5 focus:text-sm focus:font-medium focus:text-primary-foreground focus:outline-none focus:ring-3 focus:ring-ring/50"
      >
        Skip to main content
      </a>
      <aside className="fixed inset-y-0 left-0 z-20 flex w-sidebar flex-col border-r border-sidebar-border bg-sidebar p-3 text-sidebar-foreground max-[56rem]:hidden">
        <Link className={brandClass} to={homePath} aria-label="RSP home">
          <img
            src="/assets/rsp-logo.png"
            alt=""
            className="size-10 rounded-sm object-contain"
          />
          <span className="grid leading-tight">
            <strong className="text-base font-semibold tracking-tight">
              Ravi’s Study Program
            </strong>
            <span className="mt-0.5 text-xs text-sidebar-muted">
              Member workspace
            </span>
          </span>
        </Link>
        {workspaceSelect('h-11 w-full', true)}
        <NavLinks items={navItems} />
        <div className="mt-auto grid gap-3 pt-4">
          <Link className={navLinkClass} to="/settings">
            <IconSettings size={19} aria-hidden="true" /> Settings
          </Link>
          <ThemeButton
            theme={theme}
            onToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
            inverse
          />
          <div className="flex items-center gap-3 border-t border-sidebar-border px-1.5 py-3">
            <span className={avatarClass}>
              {initials(userQuery.data?.name ?? 'RSP')}
            </span>
            <span className="grid min-w-0 leading-snug">
              <strong className="truncate text-sm font-semibold">
                {userQuery.data?.name ?? 'Loading account…'}
              </strong>
              <span className="truncate text-xs text-sidebar-muted">
                {activeWorkspace?.role?.replace('_', ' ') ?? 'Personal account'}
              </span>
            </span>
          </div>
          <Button
            variant="ghost"
            className={cn(navLinkClass, 'w-full justify-start')}
            type="button"
            onClick={() => void handleSignOut()}
          >
            <IconLogout size={19} aria-hidden="true" /> Sign out
          </Button>
        </div>
      </aside>
      <header className="sticky top-0 z-30 hidden min-h-16 items-center justify-between gap-3 border-b border-sidebar-border bg-sidebar px-3.5 py-2 text-sidebar-foreground max-[56rem]:flex">
        <Link
          className={cn(brandClass, 'shrink-0')}
          to={homePath}
          aria-label="RSP home"
        >
          <img
            src="/assets/rsp-logo.png"
            alt=""
            className="size-9 rounded-sm object-contain"
          />
          <strong className="text-base font-semibold tracking-tight max-[400px]:sr-only">
            RSP
          </strong>
        </Link>
        {workspaceSelect('h-10 min-w-0 flex-1 truncate', false)}
        <div className="flex shrink-0 items-center gap-1">
          <Link
            to="/settings"
            aria-label="Account settings"
            className={buttonVariants({
              variant: 'ghost',
              size: 'icon',
              className: 'size-10',
            })}
          >
            <IconSettings size={19} aria-hidden="true" />
          </Link>
          <ThemeButton
            theme={theme}
            onToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          />
        </div>
      </header>
      <div className="min-w-0 min-[56rem]:ml-sidebar">
        <header className="flex h-14 items-center justify-between gap-2 border-b border-border px-4 min-[56rem]:h-16 min-[56rem]:gap-4 min-[56rem]:px-8">
          <div className="flex min-w-0 items-center gap-3 text-sm">
            <span className="text-muted-foreground">Workspace</span>
            <span className="text-border" aria-hidden="true">
              /
            </span>
            <span className="min-w-0 truncate font-medium">
              {pageTitle ||
                navItems.find((item) => location.pathname === item.to)?.label}
            </span>
          </div>
          <CommandPalette />
        </header>
        <main
          id="main-content"
          className="min-h-[calc(100svh-4rem)] px-3.5 pt-5 pb-26 min-[56rem]:px-8 min-[56rem]:pt-8 min-[56rem]:pb-16 xl:px-12"
          tabIndex={-1}
        >
          <Outlet />
        </main>
      </div>
      <NavLinks items={navItems} mobile />
    </div>
  );
}

const MOBILE_NAV_LIMIT = 5;
const MOBILE_NAV_VISIBLE = 4;

const mobileNavShortLabels: Record<string, string> = {
  Administration: 'Admin',
  'Mock interviews': 'Mocks',
  'Monday updates': 'Updates',
  'Season operations': 'Operations',
  'All students': 'Students',
  'Mentor teams': 'Teams',
  'My students': 'Students',
};

function NavLinks({
  items,
  mobile = false,
}: {
  items: NavItem[];
  mobile?: boolean;
}) {
  const { pathname } = useLocation();
  const [moreOpen, setMoreOpen] = useState(false);
  const collapse = mobile && items.length > MOBILE_NAV_LIMIT;
  const visibleItems = collapse ? items.slice(0, MOBILE_NAV_VISIBLE) : items;
  const overflowItems = collapse ? items.slice(MOBILE_NAV_VISIBLE) : [];
  const moreActive = overflowItems.some((item) =>
    item.end
      ? pathname === item.to
      : pathname === item.to || pathname.startsWith(`${item.to}/`),
  );

  const renderLink = (
    item: NavItem,
    baseClass: string,
    activeClass: string,
    labelClass?: string,
    onNavigate?: () => void,
  ) => {
    const Icon = item.icon;
    const mobileBar = labelClass !== undefined;
    return (
      <NavLink
        key={item.to}
        to={item.to}
        end={item.end}
        onClick={onNavigate}
        aria-label={mobileBar ? item.label : undefined}
        title={mobileBar ? item.label : undefined}
        className={({ isActive }) => cn(baseClass, isActive && activeClass)}
      >
        <Icon size={19} aria-hidden={true} />
        <span className={labelClass}>
          {mobileBar
            ? (mobileNavShortLabels[item.label] ?? item.label)
            : item.label}
        </span>
      </NavLink>
    );
  };

  if (!mobile) {
    return (
      <nav className="mt-1.5 grid gap-1" aria-label="Primary navigation">
        {items.map((item) =>
          renderLink(item, navLinkClass, navLinkActiveClass),
        )}
      </nav>
    );
  }

  return (
    <nav className={mobileNavBarClass} aria-label="Mobile navigation">
      {visibleItems.map((item) =>
        renderLink(
          item,
          mobileNavLinkClass,
          mobileNavLinkActiveClass,
          mobileNavLabelClass,
        ),
      )}
      {collapse ? (
        <Sheet open={moreOpen} onOpenChange={setMoreOpen}>
          <SheetTrigger
            render={
              <button
                type="button"
                className={cn(
                  mobileNavLinkClass,
                  moreActive && mobileNavLinkActiveClass,
                )}
              />
            }
          >
            <IconDots size={19} aria-hidden={true} />
            <span className={mobileNavLabelClass}>More</span>
          </SheetTrigger>
          <SheetContent
            side="bottom"
            className="max-h-[80svh] overflow-y-auto pb-[calc(1rem+env(safe-area-inset-bottom))]"
          >
            <SheetHeader>
              <SheetTitle>More</SheetTitle>
            </SheetHeader>
            <nav aria-label="More pages" className="grid gap-1">
              {overflowItems.map((item) =>
                renderLink(
                  item,
                  sheetNavLinkClass,
                  sheetNavLinkActiveClass,
                  undefined,
                  () => setMoreOpen(false),
                ),
              )}
            </nav>
          </SheetContent>
        </Sheet>
      ) : null}
    </nav>
  );
}
