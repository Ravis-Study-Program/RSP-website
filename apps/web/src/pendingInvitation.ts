/**
 * The invitation being accepted, kept in localStorage rather than
 * sessionStorage: signing up sends a verification email whose link opens in a
 * new tab, and the invitation has to be waiting there after sign-in.
 */
const pendingInvitationKey = 'rsp-pending-invitation';
// Invitations last 7 days; a saved token is no use after that.
const pendingInvitationTtl = 7 * 86400000;

export function pendingInvitation(): string {
  try {
    const saved = JSON.parse(
      localStorage.getItem(pendingInvitationKey) ?? 'null',
    ) as { token?: string; savedAt?: number } | null;
    if (
      saved?.token &&
      saved.savedAt &&
      Date.now() - saved.savedAt < pendingInvitationTtl
    )
      return saved.token;
    localStorage.removeItem(pendingInvitationKey);
  } catch {
    // Storage blocked or malformed: no invitation to resume.
  }
  return '';
}

export function savePendingInvitation(token: string) {
  try {
    localStorage.setItem(
      pendingInvitationKey,
      JSON.stringify({ token, savedAt: Date.now() }),
    );
  } catch {
    // Storage blocked: the invitation lasts only while this page is open.
  }
}

export function clearPendingInvitation() {
  try {
    localStorage.removeItem(pendingInvitationKey);
  } catch {
    // Nothing saved.
  }
}
