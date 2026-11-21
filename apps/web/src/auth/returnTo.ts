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
  const pathname = url.pathname.replace(/\/$/, '');
  if (
    url.origin !== 'https://rsp.test' ||
    ![
      '/jobs',
      '/jobs/applications',
      '/jobs/referrals',
      '/jobs/refer-candidates',
    ].includes(pathname)
  )
    return null;
  if (pathname === '/jobs/refer-candidates') return pathname;
  if (pathname === '/jobs/referrals') {
    if (url.searchParams.get('view') === 'refer')
      return '/jobs/refer-candidates';
    const company = url.searchParams.get('company');
    return (
      pathname +
      (company && /^[\w-]+$/.test(company)
        ? `?company=${encodeURIComponent(company)}`
        : '')
    );
  }
  const focus =
    url.searchParams.get('position') ?? url.searchParams.get('application');
  const key = pathname === '/jobs' ? 'position' : 'application';
  return (
    pathname +
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
