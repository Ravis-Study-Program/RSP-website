#!/usr/bin/env node
/* global Buffer, Headers, URL, console, fetch, process, setTimeout */

import { createHmac } from 'node:crypto';
import { spawn } from 'node:child_process';
import { chmod, readFile, writeFile } from 'node:fs/promises';

const TOTP_SECRET_FILE = '.fake-data-totp-secrets.json';

const origin = requiredUrl(
  'RSP_FAKE_DATA_ORIGIN',
  process.env.RSP_FAKE_DATA_ORIGIN ?? 'http://localhost:8080',
);
const mailpit = requiredUrl(
  'RSP_FAKE_DATA_MAILPIT_URL',
  process.env.RSP_FAKE_DATA_MAILPIT_URL ?? 'http://127.0.0.1:8025',
);

if (process.env.APP_ENV !== 'development') fail('APP_ENV must be development');
if (!isLoopback(origin.hostname) || !isLoopback(mailpit.hostname))
  fail('fake data can only target loopback services');

const users = [
  {
    role: 'student',
    name: 'Fake Student',
    email: 'fake.student@rsp.local',
    password: 'RSP-local-Student-2026!',
    mfa: false,
  },
  {
    role: 'mentor',
    name: 'Fake Mentor',
    email: 'fake.mentor@rsp.local',
    password: 'RSP-local-Mentor-2026!',
    mfa: false,
  },
  {
    role: 'coordinator',
    name: 'Fake Coordinator',
    email: 'fake.coordinator@rsp.local',
    password: 'RSP-local-Coordinator-2026!',
    mfa: true,
  },
  {
    role: 'site-admin',
    name: 'Fake Site Admin',
    email: 'fake.site-admin@rsp.local',
    password: 'RSP-local-SiteAdmin-2026!',
    mfa: true,
  },
];

class CookieJar {
  values = new Map();

  absorb(response) {
    const setCookies =
      typeof response.headers.getSetCookie === 'function'
        ? response.headers.getSetCookie()
        : [];
    for (const setCookie of setCookies) {
      const [pair] = setCookie.split(';', 1);
      const separator = pair?.indexOf('=') ?? -1;
      if (!pair || separator < 1) continue;
      const name = pair.slice(0, separator);
      const value = pair.slice(separator + 1);
      if (value) this.values.set(name, value);
      else this.values.delete(name);
    }
  }

  header() {
    return [...this.values.entries()]
      .map(([name, value]) => `${name}=${value}`)
      .join('; ');
  }
}

async function request(path, { body, jar, expected = [200] } = {}) {
  const headers = new Headers({
    accept: 'application/json, application/problem+json',
    origin: origin.origin,
  });
  if (body !== undefined) headers.set('content-type', 'application/json');
  if (jar?.values.size) headers.set('cookie', jar.header());
  const response = await fetch(`${origin.origin}/api/auth${path}`, {
    method: body === undefined ? 'GET' : 'POST',
    headers,
    redirect: 'manual',
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  jar?.absorb(response);
  if (!expected.includes(response.status)) {
    const detail = (await response.text()).slice(0, 500);
    fail(`${path} returned HTTP ${response.status}: ${detail}`);
  }
  return response;
}

async function createAndVerify(user) {
  const existing = await request('/sign-in/email', {
    body: { email: user.email, password: user.password },
    expected: [200, 401, 403, 404, 429],
  });
  if (existing.status === 200) {
    await existing.arrayBuffer();
    return;
  }
  if (existing.status === 429)
    fail(`sign-in is rate limited for ${user.email}; try again in a minute`);

  const response = await request('/sign-up/email', {
    body: {
      name: user.name,
      email: user.email,
      password: user.password,
      callbackURL: '/sign-in?verified=true',
    },
    expected: [200, 201, 400, 409, 422],
  });

  if (!response.ok) {
    fail(
      `${user.email} already exists or could not be created; reset the local database before running just fake-data again`,
    );
  }

  const verificationUrl = await pollVerificationUrl(user.email);
  const verification = await fetch(verificationUrl, { redirect: 'manual' });
  if (![200, 302].includes(verification.status)) {
    fail(`verification for ${user.email} returned HTTP ${verification.status}`);
  }
}

async function pollVerificationUrl(email) {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    const response = await fetch(`${mailpit.origin}/api/v1/messages`);
    if (response.ok) {
      const list = await response.json();
      const message = list.messages?.find(
        (candidate) =>
          candidate.Subject === 'Verify your RSP email' &&
          candidate.To?.some(
            (recipient) => recipient.Address?.toLowerCase() === email,
          ),
      );
      if (message?.ID) {
        const messageResponse = await fetch(
          `${mailpit.origin}/api/v1/message/${message.ID}`,
        );
        const body = await messageResponse.json();
        const verificationUrl = body.Text?.match(
          /https?:\/\/[^\s]+\/api\/auth\/verify-email\?[^\s]+/,
        )?.[0];
        if (verificationUrl) return verificationUrl;
      }
    }
    await sleep(250);
  }
  fail(`Mailpit did not capture verification email for ${email}`);
}

async function configureMfa(user) {
  const jar = new CookieJar();
  const signIn = await request('/sign-in/email', {
    body: { email: user.email, password: user.password, rememberMe: true },
    jar,
  });
  const signInResult = await signIn.json();
  if (signInResult.twoFactorRedirect === true) return undefined;

  const enable = await request('/two-factor/enable', {
    body: { password: user.password },
    jar,
  });
  const setup = await enable.json();
  const secret = setup.totpURI
    ? new URL(setup.totpURI).searchParams.get('secret')
    : null;
  if (!secret) fail(`MFA setup returned no TOTP secret for ${user.email}`);

  const untilBoundary = 30_000 - (Date.now() % 30_000);
  if (untilBoundary < 1_500) await sleep(untilBoundary + 250);
  await request('/two-factor/verify-totp', {
    body: { code: totp(secret), trustDevice: false },
    jar,
  });
  return secret;
}

async function loadStoredMfaSecrets() {
  try {
    const content = await readFile(TOTP_SECRET_FILE, 'utf8');
    const values = JSON.parse(content);
    return new Map(
      Object.entries(values).filter(
        ([email, secret]) =>
          typeof email === 'string' &&
          typeof secret === 'string' &&
          /^[A-Z2-7]+=*$/.test(secret),
      ),
    );
  } catch (error) {
    if (error?.code === 'ENOENT') return new Map();
    throw error;
  }
}

async function storeMfaSecret(email, secret) {
  const stored = await loadStoredMfaSecrets();
  stored.set(email, secret);
  await writeFile(
    TOTP_SECRET_FILE,
    `${JSON.stringify(Object.fromEntries(stored), null, 2)}\n`,
    { mode: 0o600 },
  );
  await chmod(TOTP_SECRET_FILE, 0o600);
}

async function run(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: process.cwd(),
      env: process.env,
      stdio: 'inherit',
    });
    child.once('error', reject);
    child.once('exit', (code, signal) => {
      if (code === 0) resolve();
      else reject(new Error(`${command} exited with ${code ?? signal}`));
    });
  });
}

async function runCapture(command, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: process.cwd(),
      env: process.env,
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (chunk) => {
      stdout += chunk;
    });
    child.stderr.on('data', (chunk) => {
      stderr += chunk;
    });
    child.once('error', reject);
    child.once('exit', (code, signal) => {
      if (code === 0) resolve(stdout);
      else
        reject(
          new Error(`${command} exited with ${code ?? signal}: ${stderr}`),
        );
    });
  });
}

async function fakeFixtureExists() {
  const result = await runCapture('docker', [
    'compose',
    'exec',
    '-T',
    'postgres',
    'psql',
    '--username',
    process.env.POSTGRES_USER ?? 'rsp',
    '--dbname',
    process.env.POSTGRES_DB ?? 'rsp',
    '--tuples-only',
    '--no-align',
    '--command',
    `SELECT EXISTS (SELECT 1 FROM app.seasons WHERE id = 'dev-season')
       AND (SELECT count(*) FROM app.users WHERE email IN (${users
         .map((user) => `'${user.email}'`)
         .join(', ')})) = ${users.length};`,
  ]);
  return result.trim() === 't';
}

function totp(secret, at = Date.now()) {
  const message = Buffer.alloc(8);
  message.writeBigUInt64BE(BigInt(Math.floor(at / 30_000)));
  const digest = createHmac('sha1', decodeBase32(secret))
    .update(message)
    .digest();
  const offset = (digest.at(-1) ?? 0) & 0x0f;
  const binary =
    (((digest[offset] ?? 0) & 0x7f) << 24) |
    ((digest[offset + 1] ?? 0) << 16) |
    ((digest[offset + 2] ?? 0) << 8) |
    (digest[offset + 3] ?? 0);
  return String(binary % 1_000_000).padStart(6, '0');
}

function decodeBase32(value) {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
  let bits = '';
  for (const character of value.replaceAll('=', '').toUpperCase()) {
    const index = alphabet.indexOf(character);
    if (index < 0) fail('Better Auth returned an invalid Base32 TOTP secret');
    bits += index.toString(2).padStart(5, '0');
  }
  const bytes = [];
  for (let index = 0; index + 8 <= bits.length; index += 8)
    bytes.push(Number.parseInt(bits.slice(index, index + 8), 2));
  return Buffer.from(bytes);
}

function requiredUrl(name, value) {
  try {
    const parsed = new URL(value);
    if (
      !['http:', 'https:'].includes(parsed.protocol) ||
      parsed.pathname !== '/'
    )
      throw new Error();
    return parsed;
  } catch {
    fail(`${name} must be an HTTP origin`);
  }
}

function isLoopback(hostname) {
  return ['localhost', '127.0.0.1', '[::1]', '::1'].includes(hostname);
}

function sleep(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

function fail(message) {
  throw new Error(`fake data generation failed: ${message}`);
}

const mfaSecrets = new Map();
for (const [email, secret] of await loadStoredMfaSecrets()) {
  const user = users.find((candidate) => candidate.email === email);
  if (user) mfaSecrets.set(user.role, secret);
}
if (!(await fakeFixtureExists())) {
  for (const user of users) await createAndVerify(user);
  for (const user of users.filter(({ mfa }) => mfa)) {
    const secret = await configureMfa(user);
    if (secret) {
      mfaSecrets.set(user.role, secret);
      await storeMfaSecret(user.email, secret);
    }
  }

  await run('go', [
    'run',
    './backend/cmd/rspctl',
    'seed',
    '--student-email',
    users[0].email,
    '--mentor-email',
    users[1].email,
    '--coordinator-email',
    users[2].email,
    '--site-admin-email',
    users[3].email,
    '--allow-existing',
  ]);
}

console.log('\nFake local accounts');
for (const user of users) {
  console.log(`  ${user.role}: ${user.email} / ${user.password}`);
  const secret = mfaSecrets.get(user.role);
  if (secret) console.log(`    TOTP secret: ${secret}`);
  else if (user.mfa)
    console.log(
      '    TOTP secret is not stored; reset fake data to generate a new one.',
    );
}
