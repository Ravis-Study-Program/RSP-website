#!/usr/bin/env node

import { createHmac } from 'node:crypto';
import { appendFile } from 'node:fs/promises';
import { spawn } from 'node:child_process';

function fail(message) {
  throw new Error(`real-stack E2E provisioning failed: ${message}`);
}

function required(name) {
  const value = process.env[name]?.trim();
  if (!value) fail(`${name} is required`);
  return value;
}

function requireLoopbackUrl(name, value) {
  const parsed = new URL(value);
  if (!['http:', 'postgresql:', 'postgres:'].includes(parsed.protocol)) {
    fail(`${name} must use a local HTTP or PostgreSQL URL`);
  }
  if (!['127.0.0.1', 'localhost', '[::1]', '::1'].includes(parsed.hostname)) {
    fail(`${name} must resolve directly to loopback`);
  }
  return parsed;
}

if (process.env.RSP_E2E_ALLOW_DATABASE_WRITE !== 'true') {
  fail('set RSP_E2E_ALLOW_DATABASE_WRITE=true for the disposable E2E database');
}
if (process.env.APP_ENV !== 'test') {
  fail('APP_ENV must be test');
}
const composeProject = required('COMPOSE_PROJECT_NAME');
if (!/^rsp-real-e2e(?:-|$)/.test(composeProject)) {
  fail('COMPOSE_PROJECT_NAME must start with rsp-real-e2e');
}

const baseUrl = required('RSP_E2E_BASE_URL').replace(/\/$/, '');
const mailpitUrl = required('RSP_E2E_MAILPIT_URL').replace(/\/$/, '');
requireLoopbackUrl('RSP_E2E_BASE_URL', baseUrl);
requireLoopbackUrl('RSP_E2E_MAILPIT_URL', mailpitUrl);

const users = [
  {
    role: 'student',
    name: 'E2E Student',
    email: required('RSP_E2E_STUDENT_EMAIL').toLowerCase(),
    password: required('RSP_E2E_STUDENT_PASSWORD'),
    mfa: false,
  },
  {
    role: 'coordinator',
    name: 'E2E Coordinator',
    email: required('RSP_E2E_COORDINATOR_EMAIL').toLowerCase(),
    password: required('RSP_E2E_COORDINATOR_PASSWORD'),
    mfa: true,
  },
  {
    role: 'director',
    name: 'E2E Director',
    email: required('RSP_E2E_DIRECTOR_EMAIL').toLowerCase(),
    password: required('RSP_E2E_DIRECTOR_PASSWORD'),
    mfa: true,
  },
];

if (new Set(users.map(({ email }) => email)).size !== users.length) {
  fail('E2E user email addresses must be distinct');
}
for (const user of users) {
  if (user.password.length < 12)
    fail(`${user.role} password must contain at least 12 characters`);
}

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
    origin: baseUrl,
  });
  if (body !== undefined) headers.set('content-type', 'application/json');
  if (jar?.values.size) headers.set('cookie', jar.header());
  const response = await fetch(`${baseUrl}/api/auth${path}`, {
    method: body === undefined ? 'GET' : 'POST',
    headers,
    redirect: 'manual',
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
  });
  jar?.absorb(response);
  if (!expected.includes(response.status)) {
    const detail = (await response.text()).slice(0, 1_000);
    fail(`${path} returned HTTP ${response.status}: ${detail}`);
  }
  return response;
}

async function pollVerificationUrl(email) {
  for (let attempt = 0; attempt < 40; attempt += 1) {
    const listResponse = await fetch(`${mailpitUrl}/api/v1/messages`);
    if (listResponse.ok) {
      const list = await listResponse.json();
      const message = list.messages?.find(
        (candidate) =>
          candidate.Subject === 'Verify your RSP email' &&
          candidate.To?.some(
            (recipient) => recipient.Address?.toLowerCase() === email,
          ),
      );
      if (message?.ID) {
        const messageResponse = await fetch(
          `${mailpitUrl}/api/v1/message/${message.ID}`,
        );
        if (!messageResponse.ok)
          fail(`Mailpit could not read verification message for ${email}`);
        const body = await messageResponse.json();
        const verificationUrl = body.Text?.match(
          /https?:\/\/[^\s]+\/api\/auth\/verify-email\?[^\s]+/,
        )?.[0];
        if (!verificationUrl) fail(`verification URL was absent for ${email}`);
        return verificationUrl;
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  fail(`Mailpit did not capture a verification email for ${email}`);
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
  for (let index = 0; index + 8 <= bits.length; index += 8) {
    bytes.push(Number.parseInt(bits.slice(index, index + 8), 2));
  }
  return Buffer.from(bytes);
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

async function signUpAndVerify(user) {
  await request('/sign-up/email', {
    body: {
      name: user.name,
      email: user.email,
      password: user.password,
      callbackURL: '/sign-in?verified=true',
    },
  });
  const verificationUrl = await pollVerificationUrl(user.email);
  requireLoopbackUrl('verification URL', verificationUrl);
  const verification = await fetch(verificationUrl, { redirect: 'manual' });
  if (![200, 302].includes(verification.status)) {
    fail(
      `verification callback for ${user.email} returned HTTP ${verification.status}`,
    );
  }
}

async function configureMfa(user) {
  const jar = new CookieJar();
  await request('/sign-in/email', {
    body: { email: user.email, password: user.password, rememberMe: true },
    jar,
  });
  const enable = await request('/two-factor/enable', {
    body: { password: user.password },
    jar,
  });
  const setup = await enable.json();
  const secret = setup.totpURI
    ? new URL(setup.totpURI).searchParams.get('secret')
    : null;
  if (
    !secret ||
    !Array.isArray(setup.backupCodes) ||
    setup.backupCodes.length === 0
  ) {
    fail(`Better Auth returned an incomplete MFA setup for ${user.email}`);
  }
  const untilBoundary = 30_000 - (Date.now() % 30_000);
  if (untilBoundary < 1_500)
    await new Promise((resolve) => setTimeout(resolve, untilBoundary + 250));
  await request('/two-factor/verify-totp', {
    body: { code: totp(secret), trustDevice: false },
    jar,
  });
  return secret;
}

function run(command, args, extraEnvironment = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, {
      cwd: process.cwd(),
      env: { ...process.env, ...extraEnvironment },
      stdio: 'inherit',
    });
    child.once('error', reject);
    child.once('exit', (code, signal) => {
      if (code === 0) resolve();
      else reject(new Error(`${command} exited with ${code ?? signal}`));
    });
  });
}

for (const user of users) await signUpAndVerify(user);

const mfaSecrets = new Map();
for (const user of users.filter(({ mfa }) => mfa)) {
  mfaSecrets.set(user.role, await configureMfa(user));
}

await run('docker', [
  'compose',
  '-p',
  composeProject,
  '-f',
  'compose.yaml',
  '-f',
  'deploy/compose.real-e2e.yaml',
  '--profile',
  'dev',
  'exec',
  '-T',
  '-e',
  'APP_ENV=test',
  'api-dev',
  'go',
  'run',
  './backend/cmd/rspctl',
  'seed',
  '--student-email',
  users.find(({ role }) => role === 'student').email,
  '--coordinator-email',
  users.find(({ role }) => role === 'coordinator').email,
  '--director-email',
  users.find(({ role }) => role === 'director').email,
]);

const outputFile = process.env.RSP_E2E_OUTPUT_ENV?.trim();
if (outputFile) {
  const lines = [
    `RSP_E2E_COORDINATOR_TOTP_SECRET=${mfaSecrets.get('coordinator')}`,
    `RSP_E2E_DIRECTOR_TOTP_SECRET=${mfaSecrets.get('director')}`,
  ];
  await appendFile(outputFile, `${lines.join('\n')}\n`, { mode: 0o600 });
}

process.stdout.write(
  'Real-stack E2E users were created, email-verified, MFA-configured, and linked to deterministic domain fixtures.\n',
);
