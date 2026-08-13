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
import { Link, NavLink, Outlet } from 'react-router-dom';

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
      { label: 'Mock interviews', to: `${seasonBase}/mock-interviews`, icon: IconMessage2 },
      { label: 'People', to: `${seasonBase}/people`, icon: IconUsers },
    ];
  }
  if (role === 'mentor') {
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'My mentees', to: `${seasonBase}/mentees`, icon: IconUsers },
      { label: 'Mock interviews', to: `${seasonBase}/mock-interviews`, icon: IconMessage2 },
      { label: 'People', to: `${seasonBase}/people`, icon: IconSchool },
    ];
  }
  if (role === 'coordinator') {
    return [
      { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
      { label: 'Season', to: seasonBase, icon: IconCalendarEvent, end: true },
      { label: 'People', to: `${seasonBase}/people`, icon: IconUsers },
      { label: 'Mentor teams', to: `${seasonBase}/mentor-teams`, icon: IconSchool },
      { label: 'Mock interviews', to: `${seasonBase}/mock-interviews`, icon: IconMessage2 },
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
  return [
    { label: 'Dashboard', to: '/dashboard', icon: IconDashboard },
    { label: 'Practice', to: '/practice', icon: IconBriefcase },
    { label: 'Mock interviews', to: '/mock-interviews', icon: IconMessage2 },
    { label: 'Directory', to: '/graduates', icon: IconUsers },
  ];
}

function ThemeButton({ theme, onToggle, inverse = false }: { theme: string; onToggle: () => void; inverse?: boolean }) {
  const Icon = theme === 'dark' ? IconSun : IconMoon;
  return (
    <button className={inverse ? styles.navLink : styles.iconButton} type="button" onClick={onToggle} aria-label={`Use ${theme === 'dark' ? 'light' : 'dark'} theme`}>
      <Icon size={19} aria-hidden="true" />
      {inverse ? <span>{theme === 'dark' ? 'Light theme' : 'Dark theme'}</span> : null}
    </button>
  );
}

export function AppShell() {
  const userQuery = useCurrentUser();
  const { workspaces, activeWorkspace, setActiveWorkspaceId } = useWorkspace();
  const [theme, setTheme] = useState(() => localStorage.getItem('rsp-theme') ?? (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'));
  const navItems = useMemo(() => itemsFor(activeWorkspace?.role, activeWorkspace?.seasonSlug), [activeWorkspace]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem('rsp-theme', theme);
  }, [theme]);

  const workspaceSelect = (className: string, showLabel: boolean) => (
    <div className={showLabel ? styles.workspaceField : undefined}>
      {showLabel ? <label htmlFor="workspace-select">Workspace</label> : <label className={styles.visuallyHidden} htmlFor="workspace-select-mobile">Workspace</label>}
      <select
        id={showLabel ? 'workspace-select' : 'workspace-select-mobile'}
        className={className}
        value={activeWorkspace?.id ?? ''}
        disabled={!workspaces.length}
        onChange={(event) => setActiveWorkspaceId(event.target.value)}
      >
        {!workspaces.length ? <option>Loading workspace…</option> : null}
        {workspaces.map((workspace) => <option key={workspace.id} value={workspace.id}>{workspace.label}</option>)}
      </select>
    </div>
  );

  const nav = (mobile = false) => (
    <nav className={mobile ? styles.mobileNav : styles.nav} aria-label={mobile ? 'Mobile navigation' : 'Primary navigation'}>
      {navItems.slice(0, mobile ? 4 : undefined).map((item) => {
        const Icon = item.icon;
        return (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            className={({ isActive }) => `${styles.navLink} ${isActive ? styles.navLinkActive : ''}`}
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
      <a className={styles.skipLink} href="#main-content" tabIndex={0}>Skip to main content</a>
      <aside className={styles.sidebar}>
        <Link className={styles.brand} to="/dashboard" aria-label="RSP home">
          <img src="/assets/rsp-logo.png" alt="" />
          <span className={styles.brandCopy}><strong>RSP Workspace</strong><span>Ready · supported · progressing</span></span>
        </Link>
        {workspaceSelect(styles.workspaceSelect, true)}
        {nav()}
        <div className={styles.sidebarFooter}>
          <Link className={styles.navLink} to="/settings"><IconSettings size={19} aria-hidden="true" /> Settings</Link>
          <ThemeButton theme={theme} onToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')} inverse />
          <div className={styles.userSummary}>
            <span className={styles.avatar}>{initials(userQuery.data?.name ?? 'RSP')}</span>
            <span className={styles.userSummaryText}>
              <strong>{userQuery.data?.name ?? 'Loading account…'}</strong>
              <span>{activeWorkspace?.role?.replace('_', ' ') ?? 'Member'}</span>
            </span>
          </div>
          <a className={styles.navLink} href="/api/auth/sign-out"><IconLogout size={19} aria-hidden="true" /> Sign out</a>
          <span className={styles.commandHint}>Quick navigation <span className={styles.kbd}>⌘ K</span></span>
        </div>
      </aside>
      <header className={styles.mobileHeader}>
        <Link className={styles.brand} to="/dashboard" aria-label="RSP home">
          <img src="/assets/rsp-logo.png" alt="" />
          <span className={styles.brandCopy}><strong>RSP</strong><span>Workspace</span></span>
        </Link>
        {workspaceSelect(styles.mobileWorkspace, false)}
        <ThemeButton theme={theme} onToggle={() => setTheme(theme === 'dark' ? 'light' : 'dark')} />
      </header>
      <main id="main-content" className={styles.main} tabIndex={-1}>
        <Outlet />
      </main>
      {nav(true)}
      <CommandPalette />
    </div>
  );
}
