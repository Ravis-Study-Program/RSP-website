// Only Jobs introduces a public-to-authenticated return path. Keep the
// allowlist narrow so query strings cannot become external redirects.
export function safeJobsReturnTo(value: string | null): string | null {
  if (!value?.startsWith('/')) return null;
  let url: URL;
  try {
    url = new URL(value, 'https://rsp.test');
  } catch {
    return null;
  }
  if (
    url.origin !== 'https://rsp.test' ||
    !['/jobs', '/jobs/applications'].includes(url.pathname)
  )
    return null;
  const focus =
    url.searchParams.get('position') ?? url.searchParams.get('application');
  const key = url.pathname === '/jobs' ? 'position' : 'application';
  return (
    url.pathname +
    (focus && /^[\w-]+$/.test(focus)
      ? `?${key}=${encodeURIComponent(focus)}`
      : '')
  );
}

export function authReturnTo() {
  return (
    safeJobsReturnTo(sessionStorage.getItem('rsp-auth-return-to')) ??
    '/dashboard'
  );
}
