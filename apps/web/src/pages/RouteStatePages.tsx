import {
  IconAlertTriangle,
  IconLock,
  IconMailCheck,
  IconMoodSad,
  IconRefresh,
} from '@tabler/icons-react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useState } from 'react';
import type { Resolver } from 'react-hook-form';
import { useForm } from 'react-hook-form';
import {
  Link,
  isRouteErrorResponse,
  useLocation,
  useNavigate,
  useRouteError,
  useSearchParams,
} from 'react-router-dom';
import { z } from 'zod';

import { clearAccessToken } from '@/api/client';
import {
  isEmailVerificationError,
  requestPasswordReset,
  resetPassword,
  sendVerificationEmail,
  signInWithEmail,
  signInWithGoogle,
  signOut,
  signUpWithEmail,
  verifyTwoFactor,
} from '@/api/authClient';
import { usePageTitle } from '@/components/Common';
import styles from '@/styles/App.module.css';

function StateLayout({
  icon,
  title,
  message,
  children,
}: {
  icon: React.ReactNode;
  title: string;
  message: string;
  children?: React.ReactNode;
}) {
  return (
    <div className={styles.page}>
      <section className={styles.statusCard}>
        <div className={styles.statusContent}>
          {icon}
          <h1>{title}</h1>
          <p>{message}</p>
          <div
            className={styles.buttonRow}
            style={{ justifyContent: 'center' }}
          >
            {children}
          </div>
        </div>
      </section>
    </div>
  );
}

export function SignInPage() {
  usePageTitle('Sign in');
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const [mode, setMode] = useState<'sign-in' | 'sign-up'>('sign-in');
  const [requestError, setRequestError] = useState(
    searchParams.has('error') ? 'Google sign-in could not be completed.' : '',
  );
  const [googlePending, setGooglePending] = useState(false);
  const [resetPending, setResetPending] = useState(false);
  const [resetMessage, setResetMessage] = useState('');
  const schema = z.object({
    name: z.string(),
    email: z.string().email('Enter a valid email address.'),
    password: z.string().min(1, 'Enter your password.'),
  });
  type Values = z.infer<typeof schema>;
  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<Values>({
    resolver: zodResolver(schema as never) as Resolver<Values>,
    defaultValues: { name: '', email: '', password: '' },
  });
  const submit = handleSubmit(async (values) => {
    setRequestError('');
    if (mode === 'sign-up' && values.name.trim().length < 2) {
      setError('name', { message: 'Enter your name.' });
      return;
    }
    if (mode === 'sign-up' && values.password.length < 12) {
      setError('password', { message: 'Use at least 12 characters.' });
      return;
    }
    try {
      if (mode === 'sign-up') {
        await signUpWithEmail(
          values.name.trim(),
          values.email,
          values.password,
        );
        navigate('/verify-email', { state: { email: values.email } });
        return;
      }
      const result = await signInWithEmail(values.email, values.password);
      clearAccessToken();
      navigate(result.twoFactorRedirect ? '/two-factor' : '/dashboard', {
        replace: true,
      });
    } catch (error) {
      if (isEmailVerificationError(error)) {
        navigate('/verify-email', {
          state: { email: values.email },
          replace: true,
        });
        return;
      }
      setRequestError(
        error instanceof Error ? error.message : 'Unable to continue.',
      );
    }
  });
  const google = async () => {
    setGooglePending(true);
    setRequestError('');
    try {
      await signInWithGoogle();
    } catch (error) {
      setRequestError(
        error instanceof Error
          ? error.message
          : 'Unable to start Google sign-in.',
      );
      setGooglePending(false);
    }
  };
  const forgotPassword = async () => {
    const email =
      (
        document.getElementById('auth-email') as HTMLInputElement | null
      )?.value.trim() ?? '';
    if (!email || !z.string().email().safeParse(email).success) {
      setRequestError('Enter your email address first.');
      return;
    }
    setResetPending(true);
    setRequestError('');
    try {
      await requestPasswordReset(email);
    } catch {
      // Keep the response indistinguishable so this form cannot enumerate accounts.
    } finally {
      setResetPending(false);
      setResetMessage(
        'If an account exists for that address, a password-reset email is on its way.',
      );
    }
  };

  return (
    <StateLayout
      icon={<img src="/assets/rsp-logo.png" alt="RSP" width="72" height="72" />}
      title="Welcome to RSP"
      message="Sign in with Google or your verified email and password to continue."
    >
      <div className={styles.form} style={{ width: '100%', textAlign: 'left' }}>
        {searchParams.get('verified') === 'true' ? (
          <p className={styles.inlineNotice}>
            Email verified. Sign in to continue.
          </p>
        ) : null}
        {searchParams.get('securityChanged') === 'true' ? (
          <p className={styles.inlineNotice}>
            Your security settings were updated and all sessions were revoked.
            Sign in again to continue.
          </p>
        ) : null}
        {searchParams.get('reset') === 'true' ? (
          <p className={styles.inlineNotice}>
            Password reset complete. Sign in with your new password.
          </p>
        ) : null}
        {searchParams.get('passwordConnected') === 'true' ? (
          <p className={styles.inlineNotice}>
            Password sign-in was connected. Sign in again to continue.
          </p>
        ) : null}
        {requestError ? (
          <p className={styles.fieldError} role="alert">
            {requestError}
          </p>
        ) : null}
        {resetMessage ? <p role="status">{resetMessage}</p> : null}
        <button
          className={styles.button}
          type="button"
          disabled={googlePending}
          onClick={() => void google()}
        >
          {googlePending ? 'Opening Google…' : 'Continue with Google'}
        </button>
        <div
          className={styles.tabsList}
          role="group"
          aria-label="Email account action"
        >
          <button
            className={styles.tab}
            type="button"
            aria-pressed={mode === 'sign-in'}
            onClick={() => setMode('sign-in')}
          >
            Sign in
          </button>
          <button
            className={styles.tab}
            type="button"
            aria-pressed={mode === 'sign-up'}
            onClick={() => setMode('sign-up')}
          >
            Create account
          </button>
        </div>
        <form className={styles.form} noValidate onSubmit={submit}>
          {mode === 'sign-up' ? (
            <div className={styles.field}>
              <label htmlFor="auth-name">Name</label>
              <input
                id="auth-name"
                className={styles.input}
                autoComplete="name"
                aria-invalid={Boolean(errors.name)}
                {...register('name')}
              />
              {errors.name ? (
                <p className={styles.fieldError}>{errors.name.message}</p>
              ) : null}
            </div>
          ) : null}
          <div className={styles.field}>
            <label htmlFor="auth-email">Email</label>
            <input
              id="auth-email"
              className={styles.input}
              type="email"
              autoComplete="email"
              aria-invalid={Boolean(errors.email)}
              {...register('email')}
            />
            {errors.email ? (
              <p className={styles.fieldError}>{errors.email.message}</p>
            ) : null}
          </div>
          <div className={styles.field}>
            <label htmlFor="auth-password">Password</label>
            <input
              id="auth-password"
              className={styles.input}
              type="password"
              autoComplete={
                mode === 'sign-up' ? 'new-password' : 'current-password'
              }
              aria-invalid={Boolean(errors.password)}
              {...register('password')}
            />
            {errors.password ? (
              <p className={styles.fieldError}>{errors.password.message}</p>
            ) : null}
          </div>
          <button
            className={styles.buttonSecondary}
            type="submit"
            disabled={isSubmitting}
          >
            {isSubmitting
              ? 'Please wait…'
              : mode === 'sign-up'
                ? 'Create account'
                : 'Sign in with email'}
          </button>
          {mode === 'sign-in' ? (
            <button
              className={styles.buttonQuiet}
              type="button"
              disabled={resetPending}
              onClick={() => void forgotPassword()}
            >
              {resetPending ? 'Requesting…' : 'Forgot password?'}
            </button>
          ) : null}
        </form>
      </div>
    </StateLayout>
  );
}

export function ResetPasswordPage() {
  usePageTitle('Reset password');
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const token = searchParams.get('token') ?? '';
  const [password, setPassword] = useState('');
  const [confirmation, setConfirmation] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!token)
      return setError(
        'This reset link is missing its token. Request a new email.',
      );
    if (password.length < 12) return setError('Use at least 12 characters.');
    if (password !== confirmation)
      return setError('The passwords do not match.');
    setPending(true);
    setError('');
    try {
      await resetPassword(token, password);
      navigate('/sign-in?reset=true', { replace: true });
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'This link could not be used. Request a new reset email.',
      );
      setPending(false);
    }
  };
  return (
    <StateLayout
      icon={<IconLock size={48} aria-hidden="true" />}
      title="Choose a new password"
      message="Reset links are single-use. Completing this change revokes existing sessions."
    >
      <form
        className={styles.form}
        style={{ width: '100%', textAlign: 'left' }}
        onSubmit={(event) => void submit(event)}
      >
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor="reset-password">New password</label>
          <input
            id="reset-password"
            className={styles.input}
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
          />
        </div>
        <div className={styles.field}>
          <label htmlFor="reset-password-confirmation">
            Confirm new password
          </label>
          <input
            id="reset-password-confirmation"
            className={styles.input}
            type="password"
            autoComplete="new-password"
            value={confirmation}
            onChange={(event) => setConfirmation(event.target.value)}
          />
        </div>
        <button className={styles.button} type="submit" disabled={pending}>
          {pending ? 'Resetting…' : 'Reset password'}
        </button>
        <Link className={styles.buttonSecondary} to="/sign-in">
          Back to sign in
        </Link>
      </form>
    </StateLayout>
  );
}

export function VerifyEmailPage() {
  usePageTitle('Verify email');
  const navigate = useNavigate();
  const location = useLocation();
  const stateEmail = (location.state as { email?: unknown } | null)?.email;
  const [email, setEmail] = useState(
    typeof stateEmail === 'string' ? stateEmail : '',
  );
  const [status, setStatus] = useState('');
  const [pending, setPending] = useState(false);
  const resend = async () => {
    if (!email) {
      setStatus('Enter the email address you registered with.');
      return;
    }
    setPending(true);
    try {
      await sendVerificationEmail(email);
      setStatus('A new verification email has been sent.');
    } catch (error) {
      setStatus(
        error instanceof Error ? error.message : 'Unable to resend the email.',
      );
    } finally {
      setPending(false);
    }
  };
  const anotherAccount = async () => {
    await signOut();
    navigate('/sign-in', { replace: true });
  };
  return (
    <StateLayout
      icon={<IconMailCheck size={48} aria-hidden="true" />}
      title="Check your email"
      message="Protected programme access begins after your email address is verified."
    >
      <div className={styles.form} style={{ width: '100%', textAlign: 'left' }}>
        {status ? <p role="status">{status}</p> : null}
        <div className={styles.field}>
          <label htmlFor="verification-email">Email</label>
          <input
            id="verification-email"
            className={styles.input}
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
          />
        </div>
        <button
          className={styles.button}
          type="button"
          disabled={pending}
          onClick={() => void resend()}
        >
          {pending ? 'Sending…' : 'Resend verification email'}
        </button>
        <button
          className={styles.buttonSecondary}
          type="button"
          onClick={() => void anotherAccount()}
        >
          Use another account
        </button>
      </div>
    </StateLayout>
  );
}

export function TwoFactorPage() {
  usePageTitle('Two-factor verification');
  const navigate = useNavigate();
  const [method, setMethod] = useState<'totp' | 'backup-code'>('totp');
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    setPending(true);
    setError('');
    try {
      await verifyTwoFactor(code.trim(), method);
      clearAccessToken();
      navigate('/dashboard', { replace: true });
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : 'The verification code was not accepted.',
      );
      setPending(false);
    }
  };
  return (
    <StateLayout
      icon={<IconLock size={48} aria-hidden="true" />}
      title="Complete two-factor verification"
      message="Enter a current authenticator code or one of your single-use backup codes."
    >
      <form
        className={styles.form}
        style={{ width: '100%', textAlign: 'left' }}
        onSubmit={(event) => void submit(event)}
      >
        {error ? (
          <p className={styles.fieldError} role="alert">
            {error}
          </p>
        ) : null}
        <div className={styles.field}>
          <label htmlFor="two-factor-code">
            {method === 'totp' ? 'Authenticator code' : 'Backup code'}
          </label>
          <input
            id="two-factor-code"
            className={styles.input}
            value={code}
            onChange={(event) => setCode(event.target.value)}
            inputMode={method === 'totp' ? 'numeric' : 'text'}
            autoComplete="one-time-code"
            required
          />
        </div>
        <button className={styles.button} type="submit" disabled={pending}>
          {pending ? 'Verifying…' : 'Verify'}
        </button>
        <button
          className={styles.buttonSecondary}
          type="button"
          onClick={() => {
            setMethod(method === 'totp' ? 'backup-code' : 'totp');
            setCode('');
          }}
        >
          {method === 'totp'
            ? 'Use a backup code'
            : 'Use an authenticator code'}
        </button>
      </form>
    </StateLayout>
  );
}

export function ForbiddenPage() {
  usePageTitle('Access denied');
  return (
    <StateLayout
      icon={<IconLock size={48} aria-hidden="true" />}
      title="You do not have access"
      message="This workspace or action is outside your current programme role."
    >
      <Link className={styles.button} to="/dashboard">
        Back to dashboard
      </Link>
    </StateLayout>
  );
}

export function NoSeasonPage() {
  usePageTitle('No season access');
  return (
    <StateLayout
      icon={<IconMoodSad size={48} aria-hidden="true" />}
      title="No season access yet"
      message="Your account is ready, but a Coordinator has not approved a season enrolment."
    >
      <Link className={styles.button} to="/settings">
        Account settings
      </Link>
    </StateLayout>
  );
}

export function AccountUnavailablePage() {
  usePageTitle('Account unavailable');
  const [searchParams] = useSearchParams();
  const state = searchParams.get('state');
  const message =
    state === 'deletion_pending'
      ? 'Your deletion request is in its 30-day recovery period. Use the verified recovery link from your email to restore access.'
      : state === 'deleted'
        ? 'This account has been deleted and its personal details have been removed.'
        : 'This account is suspended. Contact an RSP administrator if you believe this is a mistake.';
  return (
    <StateLayout
      icon={<IconLock size={48} aria-hidden="true" />}
      title="Account unavailable"
      message={message}
    >
      <Link className={styles.buttonSecondary} to="/sign-in">
        Return to sign in
      </Link>
    </StateLayout>
  );
}

export function NotFoundPage() {
  usePageTitle('Page not found');
  return (
    <StateLayout
      icon={<IconMoodSad size={48} aria-hidden="true" />}
      title="Page not found"
      message="The link may be outdated, or this page may have moved."
    >
      <Link className={styles.button} to="/dashboard">
        Go to dashboard
      </Link>
    </StateLayout>
  );
}

export function RouteErrorPage() {
  const error = useRouteError();
  const status = isRouteErrorResponse(error) ? error.status : 500;
  const message = isRouteErrorResponse(error)
    ? error.statusText
    : error instanceof Error
      ? error.message
      : 'An unexpected error interrupted this page.';
  return (
    <StateLayout
      icon={<IconAlertTriangle size={48} aria-hidden="true" />}
      title={`Something went wrong (${status})`}
      message={message}
    >
      <button
        className={styles.button}
        type="button"
        onClick={() => window.location.reload()}
      >
        <IconRefresh size={18} aria-hidden="true" /> Reload page
      </button>
      <Link className={styles.buttonSecondary} to="/dashboard">
        Dashboard
      </Link>
    </StateLayout>
  );
}
