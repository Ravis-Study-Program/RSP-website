import { IconAlertTriangle, IconLock, IconMailCheck, IconMoodSad, IconRefresh } from '@tabler/icons-react';
import { Link, isRouteErrorResponse, useRouteError } from 'react-router-dom';

import { usePageTitle } from '@/components/Common';
import styles from '@/styles/App.module.css';

function StateLayout({ icon, title, message, children }: { icon: React.ReactNode; title: string; message: string; children?: React.ReactNode }) {
  return <div className={styles.page}><section className={styles.statusCard}><div className={styles.statusContent}>{icon}<h1>{title}</h1><p>{message}</p><div className={styles.buttonRow} style={{ justifyContent: 'center' }}>{children}</div></div></section></div>;
}

export function SignInPage() {
  usePageTitle('Sign in');
  return <StateLayout icon={<img src="/assets/rsp-logo.png" alt="RSP" width="72" height="72" />} title="Welcome to RSP" message="Sign in with Google or your verified email and password to continue."><a className={styles.button} href="/api/auth/sign-in/google">Continue with Google</a><a className={styles.buttonSecondary} href="/api/auth/sign-in">Use email and password</a></StateLayout>;
}

export function VerifyEmailPage() {
  usePageTitle('Verify email');
  return <StateLayout icon={<IconMailCheck size={48} aria-hidden="true" />} title="Check your email" message="Protected programme access begins after your email address is verified."><button className={styles.button} type="button">Resend verification email</button><a className={styles.buttonSecondary} href="/api/auth/sign-out">Use another account</a></StateLayout>;
}

export function ForbiddenPage() {
  usePageTitle('Access denied');
  return <StateLayout icon={<IconLock size={48} aria-hidden="true" />} title="You do not have access" message="This workspace or action is outside your current programme role."><Link className={styles.button} to="/dashboard">Back to dashboard</Link></StateLayout>;
}

export function NoSeasonPage() {
  usePageTitle('No season access');
  return <StateLayout icon={<IconMoodSad size={48} aria-hidden="true" />} title="No season access yet" message="Your account is ready, but a Coordinator has not approved a season enrolment."><Link className={styles.button} to="/settings">Account settings</Link></StateLayout>;
}

export function NotFoundPage() {
  usePageTitle('Page not found');
  return <StateLayout icon={<IconMoodSad size={48} aria-hidden="true" />} title="Page not found" message="The link may be outdated, or this page may have moved."><Link className={styles.button} to="/dashboard">Go to dashboard</Link></StateLayout>;
}

export function RouteErrorPage() {
  const error = useRouteError();
  const status = isRouteErrorResponse(error) ? error.status : 500;
  const message = isRouteErrorResponse(error) ? error.statusText : error instanceof Error ? error.message : 'An unexpected error interrupted this page.';
  return <StateLayout icon={<IconAlertTriangle size={48} aria-hidden="true" />} title={`Something went wrong (${status})`} message={message}><button className={styles.button} type="button" onClick={() => window.location.reload()}><IconRefresh size={18} aria-hidden="true" /> Reload page</button><Link className={styles.buttonSecondary} to="/dashboard">Dashboard</Link></StateLayout>;
}
