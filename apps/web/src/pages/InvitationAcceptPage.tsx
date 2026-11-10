import { Button, buttonVariants } from '@/components/ui/button';
import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useNavigate } from 'react-router-dom';
import { apiRequest, clearAccessToken, errorFromResponse } from '@/api/client';
import { useCurrentUser } from '@/api/queries';
import { getBrowserAuthSession } from '@/api/authClient';
import type {
  Enrollment,
  Invitation,
  InvitationInvitee,
} from '@/api/generated/models';
import { usePageTitle } from '@/components/Common';
import { AuthFrame } from '@/components/AuthFrame';
import {
  clearPendingInvitation,
  pendingInvitation,
  savePendingInvitation,
} from '@/pendingInvitation';

export function InvitationAcceptPage() {
  usePageTitle('Accept Invitation');
  const navigate = useNavigate();
  const client = useQueryClient();
  const [token] = useState(() => {
    const value = new URLSearchParams(window.location.hash.slice(1)).get(
      'token',
    );
    if (value) {
      savePendingInvitation(value);
      window.history.replaceState(
        window.history.state,
        '',
        window.location.pathname,
      );
    }
    return value ?? pendingInvitation();
  });
  const session = useQuery({
    queryKey: ['auth-session'],
    queryFn: getBrowserAuthSession,
    retry: false,
    // Coming back from sign-in, a cached signed-out session is stale.
    refetchOnMount: 'always',
  });
  const user = useCurrentUser();
  // Signed out: who the invitation is for, to start sign-up with it.
  const signedOut =
    session.isSuccess && !session.isFetching && !session.data?.user;
  const invitee = useQuery({
    queryKey: ['invitation-invitee', token],
    queryFn: async () => {
      const response = await fetch('/api/v2/invitations/invitee', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          Accept: 'application/json',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ token }),
      });
      if (!response.ok) throw await errorFromResponse(response);
      return (await response.json()) as InvitationInvitee;
    },
    enabled: signedOut && Boolean(token),
    retry: false,
  });
  useEffect(() => {
    if (signedOut && invitee.data)
      navigate('/sign-in', {
        replace: true,
        state: { signUp: true, ...invitee.data },
      });
  }, [signedOut, invitee.data, navigate]);
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
  // An invitation that cannot be used (accepted, expired, cancelled or for
  // another address) is forgotten, so the dashboard stops sending you here.
  useEffect(() => {
    if (preview.isError) clearPendingInvitation();
  }, [preview.isError]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function accept() {
    setBusy(true);
    setError('');
    try {
      const enrollment = await apiRequest<Enrollment>('/invitations/accept', {
        method: 'POST',
        body: JSON.stringify({ token }),
      });
      clearPendingInvitation();
      clearAccessToken();
      // Refresh everything but this invitation, which is now used up.
      await client.invalidateQueries({
        predicate: (query) => query.queryKey[0] !== 'invitation-preview',
      });
      // Straight into the season; a coordinator first sets up MFA.
      navigate(
        enrollment.assignmentState === 'pending_mfa'
          ? '/settings'
          : `/seasons/${preview.data?.seasonSlug}`,
        { replace: true },
      );
    } catch (e) {
      setError(
        e instanceof Error ? e.message : 'Unable to accept this invitation.',
      );
    } finally {
      setBusy(false);
    }
  }
  // A message sits closer to the title than to the button under it.
  const message = (text: string) => (
    <p className="-mt-2 mb-2 text-center text-sm text-muted-foreground">
      {text}
    </p>
  );
  const fullWidth = 'w-full';
  const body = !token ? (
    message('Open the invitation link from your email.')
  ) : session.isFetching || user.isLoading ? (
    <p role="status" className="text-center text-sm text-muted-foreground">
      Checking your account…
    </p>
  ) : !session.data?.user && !invitee.isError ? (
    // On its way to sign-up, prefilled with the invited name and email.
    <p role="status" className="text-center text-sm text-muted-foreground">
      Opening your invitation…
    </p>
  ) : !session.data?.user ? (
    <>
      <p
        role="alert"
        className="-mt-2 mb-2 text-center text-sm text-destructive"
      >
        {invitee.error?.message}
      </p>
      {message(
        'Sign in or create an account with the invited email address. Your invitation will be waiting when you return.',
      )}
      <Link
        to="/sign-in"
        className={buttonVariants({ variant: 'default', className: fullWidth })}
      >
        Sign in or create an account
      </Link>
    </>
  ) : !verified ? (
    <>
      {message('Verify your email address before accepting.')}
      <Link
        to="/verify-email"
        className={buttonVariants({ variant: 'default', className: fullWidth })}
      >
        Verify email
      </Link>
    </>
  ) : preview.isLoading ? (
    <p role="status" className="text-center text-sm text-muted-foreground">
      Checking the invitation…
    </p>
  ) : preview.isError ? (
    <p role="alert" className="text-center text-sm text-destructive">
      {preview.error.message} Check that you are signed in with the email
      address that received the invitation.
    </p>
  ) : preview.data ? (
    <>
      {message(`You have been invited to ${preview.data.seasonName}.`)}
      <Button
        type="submit"
        className={fullWidth}
        disabled={busy}
        onClick={() => void accept()}
      >
        {busy ? 'Joining…' : 'Accept Invitation'}
      </Button>
      {error ? (
        <p role="alert" className="text-center text-sm text-destructive">
          {error}
        </p>
      ) : null}
    </>
  ) : null;
  return (
    <AuthFrame title="Accept Your Invitation">
      <div className="flex flex-col gap-4">
        {body}
        <Button
          variant="ghost"
          type="button"
          className={fullWidth}
          onClick={() => {
            clearPendingInvitation();
            navigate('/profile');
          }}
        >
          Dismiss Invitation
        </Button>
      </div>
    </AuthFrame>
  );
}
