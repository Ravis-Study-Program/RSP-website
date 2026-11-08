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
import { cn } from '@/lib/utils';

import { studentLevelLabel, studentLevelTone } from '@/studentLevels';
import type { Person, Role } from '@/types';
import { formatDate } from '@/utils';
import { sortRoles } from '@/sortOrder';

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

/** The centred column, at most 92rem wide, that every page sits in. */
export function Page({
  as: Element = 'div',
  className,
  children,
}: {
  as?: 'div' | 'main';
  className?: string;
  children: ReactNode;
}) {
  return (
    <Element className={cn('mx-auto w-full max-w-[92rem]', className)}>
      {children}
    </Element>
  );
}

/**
 * A titled section of a page: an `h2` that labels the section, optional
 * actions on the right, then the content 12px below. Spacing between
 * sections belongs to the page's stack.
 */
export function PageSection({
  title,
  titleId,
  actions,
  className,
  children,
}: {
  title: ReactNode;
  titleId: string;
  actions?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  return (
    <section
      className={cn('flex flex-col', className)}
      aria-labelledby={titleId}
    >
      <div className="mb-3 flex flex-wrap items-end justify-between gap-x-4 gap-y-1">
        <h2 id={titleId} className="text-xl font-semibold tracking-tight">
          {title}
        </h2>
        {actions}
      </div>
      {children}
    </section>
  );
}

/** A member's initials in an ink circle. Decorative: the name sits beside it. */
export function Avatar({
  initials,
  size = 'md',
}: {
  initials: string;
  size?: 'md' | 'lg';
}) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'inline-grid shrink-0 place-items-center rounded-full bg-primary font-semibold text-primary-foreground',
        size === 'lg' ? 'size-18 text-xl' : 'size-9 text-xs',
      )}
    >
      {initials}
    </span>
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
      <Avatar initials={person.initials} />
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
            {sortRoles(person.roles)
              .map((role) => role.replace('_', ' '))
              .join(' · ')}
          </span>
        ) : null}
      </span>
    </span>
  );
}

/** A block link to an outside resource, such as a week's materials. */
export function ResourceLink({
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
  labelAside,
  to,
}: {
  label: string;
  value: ReactNode;
  detail?: string;
  /** Sits on the same row as the label, at the far edge of the card. */
  labelAside?: ReactNode;
  /** Makes the card open this page. An aside link stays clickable on top. */
  to?: string;
}) {
  return (
    <Card className={cn(to && 'relative transition-colors hover:bg-muted/40')}>
      <CardHeader>
        <div className="flex items-center justify-between gap-3">
          <CardDescription>{label}</CardDescription>
          {labelAside ? (
            <div className="relative z-20">{labelAside}</div>
          ) : null}
        </div>
      </CardHeader>
      <CardContent>
        <strong className="block text-3xl font-semibold tracking-tight tabular-nums">
          {value}
        </strong>
        {detail ? (
          <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
        ) : null}
      </CardContent>
      {to ? (
        <Link
          to={to}
          aria-label={`Open ${label}`}
          className="absolute inset-0 z-10 rounded-xl focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        />
      ) : null}
    </Card>
  );
}

/**
 * Bracketed detail after a label or title, such as "(optional)" or
 * "(minutes)": the brackets and their text in normal-weight muted grey.
 */
export function LabelHint({ children }: { children: ReactNode }) {
  return (
    <span className="font-normal text-muted-foreground">({children})</span>
  );
}
