import {
  IconAdjustments,
  IconBriefcase,
  IconCalendarEvent,
  IconDashboard,
  IconLogout,
  IconMessage2,
  IconMoon,
  IconSchool,
  IconSettings,
  IconSun,
  IconUsers,
} from '@tabler/icons-react';
import { useEffect, useMemo, useState, type ComponentType } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom';

import { signOut } from '@/api/authClient';
import { useCurrentUser } from '@/api/queries';
import { useWorkspace } from '@/auth/WorkspaceContext';
import { CommandPalette } from '@/components/CommandPalette';
import styles from '@/styles/App.module.css';
import { initials } from '@/utils';

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
      { label: 'Practice', to: `${seasonBase}/practice`, icon: IconBriefcase },
      {
        label: 'Mock interviews',
        to: `${seasonBase}/mock-interviews`,
        icon: IconMessage2,
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
      { label: 'Practice', to: '/practice', icon: IconBriefcase },
      { label: 'Administration', to: '/admin', icon: IconAdjustments },
    ];
  }
  if (role === 'graduate')
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'Practice', to: '/practice', icon: IconBriefcase },
      { label: 'Mock interviews', to: '/mock-interviews', icon: IconMessage2 },
      { label: 'Directory', to: '/graduates', icon: IconUsers },
    ];
  if (role === 'former_member')
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'Practice', to: '/practice', icon: IconBriefcase },
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
    <button
      className={inverse ? styles.navLink : styles.iconButton}
      type="button"
      onClick={onToggle}
      aria-label={`Use ${theme === 'dark' ? 'light' : 'dark'} theme`}
    >
      <Icon size={19} aria-hidden="true" />
      {inverse ? (
        <span>{theme === 'dark' ? 'Light theme' : 'Dark theme'}</span>
      ) : null}
    </button>
  );
}

export function AppShell() {
  const userQuery = useCurrentUser();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { workspaces, activeWorkspace, setActiveWorkspaceId } = useWorkspace();
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
      ?.setAttribute('content', theme === 'dark' ? '#080808' : '#121212');
  }, [theme]);

  const workspaceSelect = (className: string, showLabel: boolean) => {
    if (!workspaces.length) return null;
    return (
      <div className={showLabel ? styles.workspaceField : undefined}>
        {showLabel ? (
          <label htmlFor="workspace-select">Workspace</label>
        ) : (
          <label
            className={styles.visuallyHidden}
            htmlFor="workspace-select-mobile"
          >
            Workspace
          </label>
        )}
        <select
          id={showLabel ? 'workspace-select' : 'workspace-select-mobile'}
          className={className}
          value={activeWorkspace?.id ?? ''}
          onChange={(event) => setActiveWorkspaceId(event.target.value)}
        >
          {workspaces.map((workspace) => (
            <option key={workspace.id} value={workspace.id}>
              {workspace.label}
            </option>
          ))}
        </select>
      </div>
    );
  };

  const nav = (mobile = false) => (
    <nav
      className={mobile ? styles.mobileNav : styles.nav}
      aria-label={mobile ? 'Mobile navigation' : 'Primary navigation'}
    >
      {navItems.map((item) => {
        const Icon = item.icon;
        return (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className={({ isActive }) =>
              `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`
            }
          >
            <Icon size={19} aria-hidden={true} />
            <span>{item.label}</span>
          </NavLink>
        );
      })}
    </nav>
  );

  return (
    <div className={styles.appShell}>
      <a className={styles.skipLink} href="#main-content" tabIndex={0}>
        Skip to main content
      </a>
      <aside className={styles.sidebar}>
        <Link className={styles.brand} to={homePath} aria-label="RSP home">
          <img src="/assets/rsp-logo.png" alt="" />
          <span className={styles.brandCopy}>
            <strong>RSP Workspace</strong>
          </span>
        </Link>
        {workspaceSelect(styles.workspaceSelect, true)}
        {nav()}
        <div className={styles.sidebarFooter}>
          <Link className={styles.navLink} to="/settings">
            <IconSettings size={19} aria-hidden="true" /> Settings
          </Link>
          <ThemeButton
            theme={theme}
            onToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
            inverse
          />
          <div className={styles.userSummary}>
            <span className={styles.avatar}>
              {initials(userQuery.data?.name ?? 'RSP')}
            </span>
            <span className={styles.userSummaryText}>
              <strong>{userQuery.data?.name ?? 'Loading account…'}</strong>
              <span>
                {activeWorkspace?.role?.replace('_', ' ') ?? 'Personal account'}
              </span>
            </span>
          </div>
          <button
            className={styles.navLink}
            type="button"
            onClick={() => void handleSignOut()}
          >
            <IconLogout size={19} aria-hidden="true" /> Sign out
          </button>
          <span className={styles.commandHint}>
            Quick navigation <span className={styles.kbd}>⌘ K</span>
          </span>
        </div>
      </aside>
      <header className={styles.mobileHeader}>
        <Link className={styles.brand} to={homePath} aria-label="RSP home">
          <img src="/assets/rsp-logo.png" alt="" />
          <span className={styles.brandCopy}>
            <strong>RSP</strong>
            <span>Workspace</span>
          </span>
        </Link>
        {workspaceSelect(styles.mobileWorkspace, false)}
        <div className={styles.inline}>
          <Link
            className={styles.iconButton}
            to="/settings"
            aria-label="Account settings"
          >
            <IconSettings size={19} aria-hidden="true" />
          </Link>
          <ThemeButton
            theme={theme}
            onToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
          />
        </div>
      </header>
      <main id="main-content" className={styles.main} tabIndex={-1}>
        <Outlet />
      </main>
      {nav(true)}
      <CommandPalette />
    </div>
  );
}
