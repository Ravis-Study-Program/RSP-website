import { Badge } from '@/components/ui/badge';
import {
  Card,
  CardContent,
  CardHeader,
  CardDescription,
} from '@/components/ui/card';

import { IconArrowUpRight } from '@tabler/icons-react';
import { useEffect, useSyncExternalStore, type ReactNode } from 'react';
import { Link } from 'react-router-dom';

import type { Person, Role } from '@/types';

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
  actions,
}: {
  eyebrow?: string;
  title: string;
  description: string;
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
        <h1 className="text-2xl font-semibold tracking-tight md:text-3xl">
          {title}
        </h1>
        <p className="mt-2 max-w-[68ch] text-base text-muted-foreground">
          {description}
        </p>
      </div>
      {actions ? (
        <div className="flex flex-wrap items-center gap-2.5 self-end min-[56rem]:self-center">
          {actions}
        </div>
      ) : null}
    </header>
  );
}

export function RoleBadge({ role }: { role: Role }) {
  const label =
    role === 'system_admin'
      ? 'System Admin'
      : role[0].toUpperCase() + role.slice(1);
  return <Badge variant="secondary">{label}</Badge>;
}

export function PersonIdentity({
  person,
  privateView = false,
  seasonId,
}: {
  person: Person;
  privateView?: boolean;
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
        <span className="truncate text-sm text-muted-foreground">
          {privateView && person.email
            ? person.email
            : person.roles.map((role) => role.replace('_', ' ')).join(' · ')}
        </span>
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
  detail: string;
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
        <p className="mt-1 text-sm text-muted-foreground">{detail}</p>
      </CardContent>
    </Card>
  );
}
