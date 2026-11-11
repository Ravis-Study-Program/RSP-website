import { accountLifecycle, auth, pool } from '../auth.js';
import { config } from '../config.js';
import { discordAvatarUrl } from './discord-profile.js';

/** How long a Discord profile picture is trusted before it is checked again. */
export const DISCORD_AVATAR_REFRESH_HOURS = 24;

export type DiscordAvatarRefresh =
  /** Not linked, Discord is off, or it was checked within the last day. */
  | 'skipped'
  /** Discord has the picture RSP already shows. */
  | 'unchanged'
  /** A new picture (or its removal) was sent to the API. */
  | 'updated'
  /** Discord could not be reached or the stored grant no longer works. */
  | 'unavailable';

/**
 * Refreshes a member's Discord profile picture while they use the site, at
 * most once a day, using the encrypted OAuth grant stored when they connected
 * Discord. Discord does not tell apps when a picture changes, so this checks.
 *
 * The Discord account row's `updated_at` records the last check: a Discord
 * sign-in or token refresh also updates it, and both are as fresh as a check.
 */
export async function refreshDiscordAvatar(
  authUserId: string,
): Promise<DiscordAvatarRefresh> {
  if (!config.discord) return 'skipped';
  // Claim the check first, so concurrent visits make one Discord request.
  const claimed = await pool.query<{ id: string; account_id: string }>(
    `UPDATE accounts SET updated_at = now()
      WHERE user_id = $1 AND provider_id = 'discord'
        AND updated_at < now() - make_interval(hours => $2)
      RETURNING id, account_id`,
    [authUserId, DISCORD_AVATAR_REFRESH_HOURS],
  );
  const account = claimed.rows[0];
  if (!account) return 'skipped';
  const discordId = account.account_id;

  let profile: Record<string, unknown>;
  try {
    // Refreshes and re-encrypts an expired grant as needed.
    const tokens = await auth.api.getAccessToken({
      body: { accountId: account.id, userId: authUserId },
    });
    const provider = (await auth.$context).socialProviders.find(
      (item) => item.id === 'discord',
    );
    const info = await provider?.getUserInfo({
      accessToken: tokens.accessToken,
    });
    if (!info?.data) return 'unavailable';
    profile = info.data as Record<string, unknown>;
  } catch {
    // A revoked grant or a Discord outage keeps the current picture; the
    // next sign-in with Discord stores a working grant again.
    return 'unavailable';
  }
  if (profile.id !== discordId) return 'unavailable';
  const avatarUrl = discordAvatarUrl(profile);

  // Skip unchanged pictures, so a daily check adds no API audit entry.
  const last = await pool.query<{ avatar_url: string | null }>(
    `SELECT payload->>'avatarUrl' AS avatar_url
       FROM lifecycle_outbox
      WHERE target = 'identity'
        AND payload->>'type' = 'discord_avatar_updated'
        AND payload->>'authUserId' = $1
      ORDER BY created_at DESC
      LIMIT 1`,
    [authUserId],
  );
  if (last.rows[0] && last.rows[0].avatar_url === avatarUrl) {
    return 'unchanged';
  }

  const user = await pool.query<{ security_version: number }>(
    'SELECT security_version FROM users WHERE id = $1',
    [authUserId],
  );
  await accountLifecycle.queueIdentityEvent({
    type: 'discord_avatar_updated',
    authUserId,
    avatarUrl,
    securityVersion: user.rows[0]!.security_version,
    occurredAt: new Date().toISOString(),
  });
  return 'updated';
}
