import type { BadgeTone } from '@/badgeTones';
import { StatusBadge } from '@/components/StatusBadge';
import {
  Card,
  CardContent,
  CardHeader,
  CardDescription,
} from '@/components/ui/card';

import { IconArrowUpRight } from '@tabler/icons-react';
import { useEffect, useSyncExternalStore, type ReactNode } from 'react';
import { Link } from 'react-router-dom';

import { studentLevelLabel, studentLevelTone } from '@/studentLevels';
import type { Person, Role } from '@/types';
import { formatDate } from '@/utils';

let currentPageTitle = '';
const pageTitleListeners = new Set<() => void>();

function setCurrentPageTitle(title: string) {
  if (currentPageTitle === title) return;
  currentPageTitle = title;
  pageTitleListeners.forEach((listener) => listener());
}

function subscribeToPageTitle(listener: () => void) {
  pageTitleListeners.add(listener);
  return () => {
    pageTitleListeners.delete(listener);
  };
}

export function usePageTitle(title: string) {
  useEffect(() => {
    document.title = `${title} · RSP`;
    setCurrentPageTitle(title);
    return () => setCurrentPageTitle('');
  }, [title]);
}

export function useCurrentPageTitle() {
  return useSyncExternalStore(
    subscribeToPageTitle,
    () => currentPageTitle,
    () => currentPageTitle,
  );
}

export function PageHeader({
  eyebrow,
  title,
  description,
  status,
  actions,
}: {
  eyebrow?: string;
  title: string;
  description?: string;
  status?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="mb-6 flex flex-col gap-5 min-[56rem]:flex-row min-[56rem]:items-start min-[56rem]:justify-between">
      <div>
        {eyebrow ? (
          <p className="mb-1.5 text-xs font-semibold text-muted-foreground">
            {eyebrow}
          </p>
        ) : null}
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-2xl font-semibold tracking-tight md:text-3xl">
            {title}
          </h1>
          {status}
        </div>
        {description ? (
          <p className="mt-2 max-w-[68ch] text-base text-muted-foreground">
            {description}
          </p>
        ) : null}
      </div>
      {actions ? (
        <div className="flex flex-wrap items-center gap-2.5 self-end min-[56rem]:self-center">
          {actions}
        </div>
      ) : null}
    </header>
  );
}

export function RoleBadge({
  role,
  className,
}: {
  role: Role;
  className?: string;
}) {
  const label =
    role === 'system_admin'
      ? 'System Admin'
      : role[0].toUpperCase() + role.slice(1);
  return <StatusBadge className={className}>{label}</StatusBadge>;
}

// A colour per season, starting at 20/21 and repeating every seven seasons.
const seasonTones: BadgeTone[] = [
  'danger',
  'orange',
  'warning',
  'success',
  'info',
  'purple',
  'pink',
];

/**
 * A season named by the two years it spans, "24/25", in its colour (24/25
 * is blue). The full name shows on hover.
 */
export function SeasonBadge({
  name,
  startsAt,
}: {
  name: string;
  startsAt: string;
}) {
  if (Number.isNaN(Date.parse(startsAt))) return name;
  const start = Number(formatDate(startsAt).slice(-2));
  const label = `${String(start).padStart(2, '0')}/${String((start + 1) % 100).padStart(2, '0')}`;
  return (
    <StatusBadge
      title={name}
      aria-label={`Season ${label}: ${name}`}
      tone={seasonTones[(((start - 20) % 7) + 7) % 7]}
    >
      {label}
    </StatusBadge>
  );
}

/** A student level, coloured by level; nothing when there is none. */
export function StudentLevelBadge({
  level,
}: {
  level?: Person['studentLevel'];
}) {
  if (!level || level === 'not_applicable') return null;
  return (
    <StatusBadge tone={studentLevelTone[level]}>
      {studentLevelLabel(level)}
    </StatusBadge>
  );
}

export function PersonIdentity({
  person,
  privateView = false,
  showRoles = true,
  seasonId,
}: {
  person: Person;
  privateView?: boolean;
  showRoles?: boolean;
  seasonId?: string;
}) {
  return (
    <span className="flex min-w-0 items-center gap-3">
      <span
        className="inline-grid size-9 shrink-0 place-items-center rounded-full bg-primary text-xs font-semibold text-primary-foreground"
        aria-hidden="true"
      >
        {person.initials}
      </span>
      <span className="grid min-w-0">
        <Link
          className="truncate font-medium text-foreground"
          to={`/people/${person.slug}${seasonId ? `?seasonId=${encodeURIComponent(seasonId)}` : ''}`}
        >
          {person.name}
        </Link>
        {privateView && person.email ? (
          <span className="truncate text-sm text-muted-foreground">
            {person.email}
          </span>
        ) : showRoles ? (
          <span className="truncate text-sm text-muted-foreground">
            {person.roles.map((role) => role.replace('_', ' ')).join(' · ')}
          </span>
        ) : null}
      </span>
    </span>
  );
}

export function ExternalLink({
  href,
  children,
}: {
  href: string;
  children: ReactNode;
}) {
  return (
    <a
      className="flex min-h-12 items-center justify-between gap-3 rounded-lg border border-border bg-muted/50 px-3.5 py-3 font-semibold text-foreground no-underline outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50"
      href={href}
      target="_blank"
      rel="noreferrer"
    >
      {children}
      <IconArrowUpRight size={17} aria-hidden="true" />
    </a>
  );
}

export function MetricCard({
  label,
  value,
  detail,
}: {
  label: string;
  value: ReactNode;
  detail?: string;
}) {
  return (
    <Card>
      <CardHeader>
        <CardDescription>{label}</CardDescription>
      </CardHeader>
      <CardContent>
        <strong className="block text-3xl font-semibold tracking-tight tabular-nums">
          {value}
        </strong>
        {detail ? (
          <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
        ) : null}
      </CardContent>
    </Card>
  );
}
