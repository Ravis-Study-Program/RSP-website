import {
  IconAdjustments,
  IconBriefcase,
  IconCalendarEvent,
  IconDashboard,
  IconMessage2,
  IconNotebook,
  IconSchool,
  IconUsers,
} from '@tabler/icons-react';
import type { ComponentType } from 'react';

import type { CurrentUser, Season, SeasonRole } from '@/types';

export interface NavItem {
  label: string;
  description: string;
  to: string;
  icon: ComponentType<{ size?: number; 'aria-hidden'?: boolean }>;
  section?: string;
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

/** Account capabilities are additive; visiting a season never switches roles. */
export function navigationItems(
  user?: CurrentUser,
  seasons: Season[] = [],
): NavItem[] {
  if (!user) return [];
  if (!user.seasonRoles.length && !user.globalRoles.length && !user.alumni)
    return [
      {
        label: 'Profile',
        description: 'Your profile and account details',
        to: '/profile',
        icon: IconUsers,
      },
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
      icon: IconBriefcase,
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
  const isManager = user.globalRoles.length > 0;
  const membership = preferredMembership(user);
  const seasonSlug =
    membership?.seasonSlug ?? defaultSeason(user, seasons)?.slug;
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
  const updatesSeasonSlug = isManager
    ? seasonSlug
    : updatesMembership?.seasonSlug;

  if (updatesSeasonSlug)
    items.push({
      label: 'Monday Updates',
      description: 'Weekly updates and reviews',
      to: `/seasons/${updatesSeasonSlug}/monday-updates`,
      icon: IconNotebook,
      section: 'monday-updates',
    });
  if (canBrowse)
    items.push({
      label: 'People',
      description: 'Everyone, across every season',
      to: '/people',
      icon: IconUsers,
      section: 'people',
    });
  if (canBrowse || mentor || coordinator)
    items.push({
      label: 'Seasons',
      description: 'Program schedules, resources and season history',
      to: '/seasons',
      icon: IconCalendarEvent,
    });
  if (mentor && !isManager)
    items.push({
      label: 'My Students',
      description: 'Assigned students and mentoring actions',
      to: `/seasons/${mentor.seasonSlug}/mentees`,
      icon: IconUsers,
    });
  const managedSeasonSlug = isManager ? seasonSlug : coordinator?.seasonSlug;
  if (managedSeasonSlug) {
    items.push({
      label: 'All Students',
      description: 'Students and their progress',
      to: `/seasons/${managedSeasonSlug}/mentees`,
      icon: IconUsers,
    });
    if (isManager || coordinator?.state === 'active')
      items.push({
        label: 'Mentor Teams',
        description: 'Mentorship assignments',
        to: `/seasons/${managedSeasonSlug}/mentor-teams`,
        icon: IconSchool,
        section: 'mentor-teams',
      });
  }
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
