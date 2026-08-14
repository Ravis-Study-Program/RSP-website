import { IconArrowUpRight } from '@tabler/icons-react';
import { useEffect, type ReactNode } from 'react';
import { Link } from 'react-router-dom';

import styles from '@/styles/App.module.css';
import type { Person, Role } from '@/types';

export function usePageTitle(title: string) {
  useEffect(() => {
    document.title = `${title} · RSP`;
  }, [title]);
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
    <header className={styles.pageHeader}>
      <div>
        {eyebrow ? <p className={styles.eyebrow}>{eyebrow}</p> : null}
        <h1 className={styles.pageTitle}>{title}</h1>
        <p className={styles.lede}>{description}</p>
      </div>
      {actions ? <div className={styles.headerActions}>{actions}</div> : null}
    </header>
  );
}

export function RoleBadge({ role }: { role: Role }) {
  const label =
    role === 'system_admin'
      ? 'System Admin'
      : role[0].toUpperCase() + role.slice(1);
  return <span className={styles.badge}>{label}</span>;
}

export function PersonIdentity({
  person,
  privateView = false,
}: {
  person: Person;
  privateView?: boolean;
}) {
  return (
    <span className={styles.personIdentity}>
      <span className={styles.avatar} aria-hidden="true">
        {person.initials}
      </span>
      <span className={styles.personIdentityText}>
        <Link to={`/people/${person.slug}`}>{person.name}</Link>
        <span>
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
      className={styles.resourceLink}
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
    <article className={styles.metricCard}>
      <span className={styles.metricLabel}>{label}</span>
      <strong className={styles.metricValue}>{value}</strong>
      <span className={styles.helper}>{detail}</span>
    </article>
  );
}
