# Discord login and profile pictures

Members create their RSP account with email/password or Google, verify their
RSP email, and connect Discord in **Settings → Sign-In**. Discord cannot create
an RSP account or automatically link one by email. Its email may differ from
the RSP email; connecting it never changes the RSP email, name, roles or history.
Discord must have a verified email to connect. Connecting and disconnecting
require a sign-in within the last five minutes and revoke existing sessions.
Members must retain email/password or Google as a recovery sign-in method.

Once connected, **Continue with Discord** signs into that same account, including
RSP's existing two-factor challenge when enabled. The Discord button appears
only when the server has Discord credentials configured. Discord is not offered
on the Create Account tab.

## Profile pictures

Only a linked Discord account supplies the profile picture. The service uses
the profile returned by Discord during connection and each Discord login, stores
a static 256px Discord CDN URL, and displays initials if there is no avatar or
the image cannot load. Disconnecting Discord clears the picture. Email and
Google logins leave it unchanged. There is no image upload or storage service.

Discord does not tell apps when a picture changes, so the site checks while
members use it. When a member opens the app, the browser asks the auth service
(`POST /api/auth/discord/avatar/refresh`) once per tab session. The auth service
checks Discord at most once a day per member: it uses the encrypted OAuth grant
stored at connection (refreshing it when expired), reads the current profile,
and queues an avatar update only when the picture differs from the last one
sent. The Discord account row's `updated_at` records the last check, so a
Discord login counts as one and no migration is needed. If Discord is
unreachable or the member revoked access, the current picture stays until
their next Discord login. Inactive members are never checked. Better Auth's
`/get-access-token` and `/refresh-token` endpoints are disabled, so stored
Discord tokens never reach the browser.

Avatar synchronization uses the existing authenticated lifecycle outbox. Delivery
failures retry without blocking login, and an older event cannot overwrite a
newer avatar/disconnection. Browser and admin profile APIs cannot set avatar URLs.
Pictures use the existing `app.user_profiles.avatar_url` column. The existing
audit log tracks update ordering; no additional columns or migrations are needed.

## Test locally with a real Discord account

1. Create an application in the [Discord Developer Portal](https://discord.com/developers/applications).
2. In **OAuth2 → Redirects**, add `http://localhost:8080/api/auth/callback/discord`.
   If using another port, replace `8080` in both this redirect and `PUBLIC_ORIGIN`.
3. Add the application's client ID and client secret to your local, ignored `.env`:

   ```dotenv
   DISCORD_CLIENT_ID=your-application-client-id
   DISCORD_CLIENT_SECRET=your-application-client-secret
   ```

   Keep the secret out of Git and browser/Vite variables. No bot or server permissions
   are needed; the OAuth provider requests `identify` and `email`.

4. Run `docker compose up --build -d` and open <http://localhost:8080>.
   For another origin, also set `AUTH_ISSUER` and `AUTH_TRUSTED_ORIGINS` to that origin.
5. Create an email account and verify it through the message captured at
   <http://localhost:8025>. Sign in, then click **Connect Discord** in Settings.
6. After the connection signs you out, use **Continue with Discord**. Check that
   your RSP account/email are unchanged and your picture appears in the sidebar,
   profile and People directory.
7. Change your Discord avatar, sign out and sign in with Discord again. Then
   disconnect it from Settings: the picture should return to initials and Discord
   login should direct you to connect it first.

To run an isolated development stack, use a separate Compose project and ports.
Register `http://localhost:18080/api/auth/callback/discord`, set the credentials,
and start the site at <http://localhost:18080> (Mailpit: <http://localhost:18025>):

```sh
RSP_PORT=18080 PUBLIC_ORIGIN=http://localhost:18080 \
AUTH_ISSUER=http://localhost:18080 AUTH_TRUSTED_ORIGINS=http://localhost:18080 \
POSTGRES_PORT=15479 MAILPIT_SMTP_PORT=11025 MAILPIT_UI_PORT=18025 \
docker compose -f compose.yaml -p rsp-discord-dev up --build -d caddy
```

After changing credentials, rerun the same command with `auth` in place of `caddy`
to recreate the auth service with the updated environment.

## Automated verification

`apps/auth/test/discord-oauth.test.ts` exercises real PostgreSQL, the HTTP server,
OAuth state/cookies and callbacks with substituted Discord token/profile responses.
Run it only against a disposable auth database with `AUTH_DISCORD_LIVE_TEST=true`,
dummy paired Google and Discord credentials, `BETTER_AUTH_URL` and
`AUTH_TRUSTED_ORIGINS` set to `http://127.0.0.1:3001`, and
`IDENTITY_SERVICE_URL=http://127.0.0.1:4000`. It starts local listeners on those
ports. No real Discord credentials or external Discord requests are used.

Backend integration tests cover avatar update ordering, disconnects, and rejected
browser/admin avatar writes. Browser tests cover the sign-in and Settings controls
on desktop and mobile. A real Discord authorization still needs the manual test above.
