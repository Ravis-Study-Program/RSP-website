import {
  IconAdjustments,
  IconCalendarEvent,
  IconCode,
  IconDashboard,
  IconMessage2,
  IconNotebook,
  IconUsers,
} from '@tabler/icons-react';
import type { ComponentType } from 'react';

import { getSelectedSeasonSlug } from '@/selectedSeason';
import type { CurrentUser, Season, SeasonRole } from '@/types';

export interface NavItem {
  label: string;
  description: string;
  to: string;
  icon: ComponentType<{ size?: number; 'aria-hidden'?: boolean }>;
  section?: string;
  beta?: boolean;
}

export function preferredMembership(user?: CurrentUser, role?: SeasonRole) {
  const memberships = user?.seasonRoles.filter(
    (membership) => !role || membership.role === role,
  );
  return (
    memberships?.find((membership) => membership.state === 'active') ??
    memberships?.find((membership) => membership.state === 'completed')
  );
}

export function defaultSeason(user?: CurrentUser, seasons: Season[] = []) {
  const membership = preferredMembership(user);
  return (
    seasons.find((season) => season.id === membership?.seasonId) ??
    (user?.globalRoles.length
      ? (seasons.find((season) => season.status === 'open') ?? seasons[0])
      : undefined)
  );
}

/** The season Mentor Teams opens on: the one selected, else the default. */
export function mentorTeamsSeason(user?: CurrentUser, seasons: Season[] = []) {
  return (
    seasons.find((season) => season.slug === getSelectedSeasonSlug()) ??
    defaultSeason(user, seasons)
  );
}

/** Account capabilities are additive; visiting a season never switches roles. */
export function navigationItems(
  user?: CurrentUser,
  seasons: Season[] = [],
): NavItem[] {
  if (!user) return [];
  const jobs: NavItem = {
    label: 'Jobs',
    beta: true,
    description: 'Open positions and your applications',
    to: '/jobs',
    icon: IconBriefcase,
  };
  if (!user.seasonRoles.length && !user.globalRoles.length && !user.alumni)
    return [
      {
        label: 'Profile',
        description: 'Your profile and account details',
        to: '/profile',
        icon: IconUsers,
      },
      jobs,
    ];

  const items: NavItem[] = [
    {
      label: 'Dashboard',
      description: 'Your activity and program summary',
      to: '/dashboard',
      icon: IconDashboard,
    },
    {
      label: 'LeetCode',
      description: 'Problem attempt history',
      to: '/leetcode',
      icon: IconCode,
      section: 'leetcode',
    },
    {
      label: 'Mock Interviews',
      description: 'Received and given interviews',
      to: '/mock-interviews',
      icon: IconMessage2,
      section: 'mock-interviews',
    },
  ];
  items.push(jobs);
  const isManager = user.globalRoles.length > 0;
  const membership = preferredMembership(user);
  // Directors, System Admins and coordinators stay in the season they last
  // chose; everyone else uses their own.
  const selected = getSelectedSeasonSlug();
  const managesSeasons =
    isManager ||
    user.seasonRoles.some(
      (entry) => entry.role === 'coordinator' && entry.state === 'active',
    );
  const seasonSlug =
    (managesSeasons && seasons.some((season) => season.slug === selected)
      ? selected
      : undefined) ??
    membership?.seasonSlug ??
    defaultSeason(user, seasons)?.slug;
  const mentor = preferredMembership(user, 'mentor');
  const coordinator = preferredMembership(user, 'coordinator');
  const canBrowse =
    isManager ||
    user.alumni ||
    user.seasonRoles.some((entry) => entry.state === 'active');
  const updatesMembership = preferredMembership({
    ...user,
    seasonRoles: user.seasonRoles.filter(
      (entry) => entry.role !== 'coordinator' || entry.state === 'active',
    ),
  });
  // A coordinator stays in the season they last chose among those they run.
  const coordinatedSelected = user.seasonRoles.some(
    (entry) =>
      entry.role === 'coordinator' &&
      entry.state === 'active' &&
      entry.seasonSlug === selected,
  )
    ? selected
    : undefined;
  const updatesSeasonSlug = isManager
    ? seasonSlug
    : (coordinatedSelected ?? updatesMembership?.seasonSlug);

  if (updatesSeasonSlug)
    items.push({
      label: 'Monday Updates',
      description: 'Weekly updates and reviews',
      to: `/seasons/${updatesSeasonSlug}/monday-updates`,
      icon: IconNotebook,
      section: 'monday-updates',
    });
  // Coordinators and admins manage students from People, so theirs opens on
  // the season they run.
  const managedSeasonSlug = isManager
    ? seasonSlug
    : (coordinatedSelected ?? coordinator?.seasonSlug);
  if (canBrowse || managedSeasonSlug)
    items.push({
      label: 'People',
      description: 'Everyone, and the students and staff you manage',
      to: managedSeasonSlug
        ? `/seasons/${managedSeasonSlug}/people`
        : '/people',
      icon: IconUsers,
      section: 'people',
    });
  // Mentors keep their own list, admins who mentor included.
  if (mentor)
    items.push({
      label: 'My Students',
      description: 'Assigned students and mentoring actions',
      to: isManager
        ? `/seasons/${mentor.seasonSlug}/my-students`
        : `/seasons/${mentor.seasonSlug}/mentees`,
      icon: IconUsers,
    });
  if (canBrowse || mentor || coordinator)
    items.push({
      label: 'Seasons',
      description: 'Program schedules, resources and season history',
      to: '/seasons',
      icon: IconCalendarEvent,
    });
  if (isManager)
    items.push({
      label: 'Administration',
      description: 'Program administration',
      to: '/admin',
      icon: IconAdjustments,
    });
  else if (coordinator?.state === 'active')
    items.push({
      label: 'Season Operations',
      description: 'Weeks, enrollments and mentorships',
      to: '/admin/enrollments',
      icon: IconAdjustments,
    });
  return items;
}

export function navigationItemActive(item: NavItem, pathname: string) {
  if (item.to === '/jobs')
    return (
      pathname === '/jobs' ||
      pathname === '/jobs/applications' ||
      pathname === '/jobs/referrals'
    );
  if (item.section)
    return (
      pathname === item.to ||
      new RegExp(`^/seasons/[^/]+/${item.section}$`).test(pathname) ||
      (item.section === 'people' && pathname === '/graduates')
    );
  if (item.to === '/seasons') return /^\/seasons(?:\/[^/]+)?$/.test(pathname);
  if (item.to.startsWith('/admin'))
    return pathname === '/admin' || pathname.startsWith('/admin/');
  return pathname === item.to;
}
