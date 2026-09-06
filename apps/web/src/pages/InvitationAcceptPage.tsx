import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { apiRequest, clearAccessToken } from '@/api/client';
import { useCurrentUser } from '@/api/queries';
import { getBrowserAuthSession, signOut } from '@/api/authClient';
import type { Enrollment, Invitation } from '@/api/generated/models';
import { PageHeader, usePageTitle } from '@/components/Common';
import styles from '@/styles/App.module.css';

export const pendingInvitationKey = 'rsp-pending-invitation';
export function InvitationAcceptPage() {
  usePageTitle('Accept invitation');
  const navigate = useNavigate();
  const client = useQueryClient();
  const [token] = useState(() => {
    const value = new URLSearchParams(window.location.hash.slice(1)).get(
      'token',
    );
    if (value) {
      sessionStorage.setItem(pendingInvitationKey, value);
      window.history.replaceState(
        window.history.state,
        '',
        window.location.pathname,
      );
    }
    return value ?? sessionStorage.getItem(pendingInvitationKey) ?? '';
  });
  const session = useQuery({
    queryKey: ['auth-session'],
    queryFn: getBrowserAuthSession,
    retry: false,
  });
  const user = useCurrentUser();
  const verified = Boolean(
    session.data?.user?.emailVerified && user.data?.emailVerified,
  );
  const preview = useQuery({
    queryKey: ['invitation-preview', token, user.data?.id],
    queryFn: () =>
      apiRequest<Invitation>('/invitations/preview', {
        method: 'POST',
        body: JSON.stringify({ token }),
      }),
    enabled: verified && Boolean(token),
    retry: false,
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [accepted, setAccepted] = useState<Enrollment>();
  async function accept() {
    setBusy(true);
    setError('');
    try {
      const enrollment = await apiRequest<Enrollment>('/invitations/accept', {
        method: 'POST',
        body: JSON.stringify({ token }),
      });
      sessionStorage.removeItem(pendingInvitationKey);
      clearAccessToken();
      await client.invalidateQueries();
      setAccepted(enrollment);
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Unable to accept this invitation.',
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className={styles.page}>
      <PageHeader
        eyebrow="RSP"
        title="Accept your invitation"
        description="Join your season using the email address that received the invitation."
      />
      {!token ? (
        <p>Open the invitation link from your email.</p>
      ) : accepted ? (
        <div className={styles.panel}>
          <h2>You have joined {preview.data?.seasonName ?? 'the season'}.</h2>
          {accepted.assignmentState === 'pending_mfa' ? (
            <p>Set up MFA in Settings to activate your coordinator access.</p>
          ) : null}
          <Link
            className={styles.buttonPrimary}
            to={
              accepted.assignmentState === 'pending_mfa'
                ? '/settings'
                : `/seasons/${preview.data?.seasonSlug}`
            }
          >
            Continue
          </Link>
        </div>
      ) : session.isLoading || user.isLoading ? (
        <p role="status">Checking your account…</p>
      ) : !session.data?.user ? (
        <div className={styles.panel}>
          <p>
            Sign in or create an account with the invited email address. Your
            invitation will be waiting when you return.
          </p>
          <Link className={styles.buttonPrimary} to="/sign-in">
            Sign in or create an account
          </Link>
        </div>
      ) : !verified ? (
        <div className={styles.panel}>
          <p>Verify your email address before accepting.</p>
          <Link className={styles.buttonPrimary} to="/verify-email">
            Verify email
          </Link>
        </div>
      ) : preview.isLoading ? (
        <p role="status">Checking the invitation…</p>
      ) : preview.isError ? (
        <p role="alert">
          {preview.error.message} Check that you are signed in with the email
          address that received the invitation.
        </p>
      ) : preview.data ? (
        <div className={styles.panel}>
          <h2>{preview.data.seasonName}</h2>
          <p>
            {preview.data.email} has been invited as a {preview.data.role}.
          </p>
          <button
            className={styles.buttonPrimary}
            disabled={busy}
            onClick={() => void accept()}
          >
            {busy ? 'Joining…' : 'Accept invitation'}
          </button>
          {error ? <p role="alert">{error}</p> : null}
        </div>
      ) : null}
      {session.data?.user && !accepted ? (
        <button
          className={styles.buttonSecondary}
          onClick={() => {
            void signOut()
              .then(() => {
                clearAccessToken();
                client.clear();
                window.location.assign('/sign-in');
              })
              .catch(() => setError('Could not sign out. Please try again.'));
          }}
        >
          Use another account
        </button>
      ) : null}
      <button
        className={styles.buttonQuiet}
        onClick={() => {
          sessionStorage.removeItem(pendingInvitationKey);
          navigate('/profile');
        }}
      >
        Dismiss invitation
      </button>
    </main>
  );
}
