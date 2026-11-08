import { AsyncLocalStorage } from 'node:async_hooks';

export interface DiscordProfileUpdate {
  authUserId: string;
  discordId: string;
  avatarUrl: string | null;
  occurredAt: string;
}

// Keep provider data inside the OAuth request that obtained it. It is published
// only after the callback succeeds and the account binding is checked.
export interface OAuthRequestState {
  update?: DiscordProfileUpdate;
  link?: { authUserId: string; providerId: string; accountId: string };
}
export const oauthRequestContext = new AsyncLocalStorage<OAuthRequestState>();

export function discordAvatarUrl(
  profile: Record<string, unknown>,
): string | null {
  if (typeof profile.id !== 'string' || !/^\d{1,20}$/.test(profile.id)) {
    throw new Error('Invalid Discord user ID');
  }
  if (profile.avatar === null) return null;
  if (
    typeof profile.avatar !== 'string' ||
    !/^(?:a_)?[a-f0-9]{32}$/.test(profile.avatar)
  ) {
    throw new Error('Invalid Discord avatar');
  }
  return `https://cdn.discordapp.com/avatars/${profile.id}/${profile.avatar}.png?size=256`;
}

export function captureDiscordProfile(
  authUserId: string,
  profile: Record<string, unknown>,
): void {
  const context = oauthRequestContext.getStore();
  if (!context) return;
  context.update = {
    authUserId,
    discordId: String(profile.id),
    avatarUrl: discordAvatarUrl(profile),
    occurredAt: new Date().toISOString(),
  };
}
