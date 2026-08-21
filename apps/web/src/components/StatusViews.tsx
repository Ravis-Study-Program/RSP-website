import { IconAlertTriangle, IconInbox, IconRefresh } from '@tabler/icons-react';

import styles from '@/styles/App.module.css';

export function PageSkeleton({ label = 'Loading page' }: { label?: string }) {
  return (
    <div className={styles.page} aria-busy="true" aria-label={label}>
      <span className={`${styles.skeleton} ${styles.skeletonLabel}`}>
        Loading
      </span>
      <span className={`${styles.skeleton} ${styles.skeletonTitle}`}>
        Loading
      </span>
      <div className={styles.metricGrid}>
        {[1, 2, 3].map((item) => (
          <span
            key={item}
            className={`${styles.skeleton} ${styles.skeletonMetric}`}
          >
            Loading
          </span>
        ))}
      </div>
      <span className={`${styles.skeleton} ${styles.skeletonPanel}`}>
        Loading
      </span>
      <span className={styles.visuallyHidden} role="status">
        {label}
      </span>
    </div>
  );
}

export function EmptyState({
  title,
  message,
  filtered = false,
  action,
}: {
  title: string;
  message: string;
  filtered?: boolean;
  action?: React.ReactNode;
}) {
  return (
    <div className={styles.tableState}>
      <div className={styles.statusContent}>
        <IconInbox size={30} aria-hidden="true" />
        <h2>{filtered ? `No matches for ${title.toLowerCase()}` : title}</h2>
        <p>{message}</p>
        {action}
      </div>
    </div>
  );
}

export function ErrorState({
  title = 'We could not load this section',
  message = 'The rest of your workspace is still available. Try this request again.',
  onRetry,
}: {
  title?: string;
  message?: string;
  onRetry?: () => void;
}) {
  return (
    <div className={styles.tableState} role="alert">
      <div className={styles.statusContent}>
        <IconAlertTriangle size={30} aria-hidden="true" />
        <h2>{title}</h2>
        <p>{message}</p>
        {onRetry ? (
          <button
            className={styles.buttonSecondary}
            type="button"
            onClick={onRetry}
          >
            <IconRefresh size={18} aria-hidden="true" /> Retry
          </button>
        ) : null}
      </div>
    </div>
  );
}

export function InlineNotice({
  children,
  tone = 'info',
}: {
  children: React.ReactNode;
  tone?: 'info' | 'warning' | 'danger' | 'success';
}) {
  const className =
    tone === 'warning'
      ? styles.noticeWarning
      : tone === 'danger'
        ? styles.noticeDanger
        : tone === 'success'
          ? styles.noticeSuccess
          : styles.notice;
  return (
    <div className={className} role={tone === 'danger' ? 'alert' : 'status'}>
      {children}
    </div>
  );
}
