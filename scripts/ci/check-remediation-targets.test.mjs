import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import {
  checkRemediation,
  evaluateTargets,
  parseIntent,
} from './check-remediation-targets.mjs';
import { combineTrees } from './pnpm-shadow.mjs';

const target = { id: 'GHSA-aaaa-bbbb-cccc', name: 'example' };
const refs = {
  number: 1,
  base: { sha: 'a'.repeat(40) },
  head: { sha: 'b'.repeat(40) },
};
const event = { pull_request: refs };
const block = (targets) =>
  `\`\`\`security-remediation\n${JSON.stringify(targets)}\n\`\`\``;
const pr = (body = '', labels = []) => ({ ...refs, body, labels });
const comparison = (overrides = {}) => ({
  added: [],
  changedExposure: [],
  inherited: [],
  removed: [],
  ...overrides,
});

function successfulScans(context) {
  const previous = process.env.SECURITY_SCAN_RESULTS;
  process.env.SECURITY_SCAN_RESULTS = JSON.stringify({
    'pnpm-security-shadow': { result: 'success' },
  });
  context.after(() => {
    if (previous === undefined) delete process.env.SECURITY_SCAN_RESULTS;
    else process.env.SECURITY_SCAN_RESULTS = previous;
  });
}

test('ordinary PRs opt out; labels or a valid body block declare remediation', () => {
  assert.equal(parseIntent(pr(), event).required, false);
  assert.equal(
    parseIntent({ ...pr(), head: { sha: 'c'.repeat(40) } }, event).required,
    false,
  );
  assert.equal(
    parseIntent(pr(block({ pnpm: [target] })), event).required,
    true,
  );
  assert.throws(() =>
    parseIntent(pr('', [{ name: 'security-remediation' }]), event),
  );
});

test('invalid, empty, duplicate, unknown, oversized or unterminated targets fail closed', () => {
  for (const body of [
    block({}),
    block({ pnpm: [] }),
    block({ pnpm: [target, target] }),
    block({ unknown: [target] }),
    block({ pnpm: [{ ...target, path: '*' }] }),
    block({ pnpm: [{ ...target, name: '' }] }),
    block({ pnpm: [{ ...target, id: '*' }] }),
    block({ pnpm: [{ ...target, name: '*' }] }),
    block({ images: [{ ...target, image: 'unknown' }] }),
    `${block({ pnpm: [target] })}\n${block({ pnpm: [target] })}`,
    '```security-remediation\n{}',
    '```security-remediation\nnot json\n```',
    `\`\`\`security-remediation\n${' '.repeat(17_000)}{}\n\`\`\``,
  ]) {
    assert.throws(() => parseIntent(pr(body), event));
  }
});

test('stale PR refs cannot attach target results to a newer commit', () => {
  const intent = pr(block({ pnpm: [target] }));
  assert.throws(
    () => parseIntent({ ...intent, head: { sha: 'c'.repeat(40) } }, event),
    /refs changed/,
  );
  assert.throws(
    () => parseIntent({ ...intent, base: { sha: 'c'.repeat(40) } }, event),
    /refs changed/,
  );
});

test('only a witnessed complete removal resolves a target', () => {
  for (const [category, status] of [
    ['removed', 'resolved'],
    ['inherited', 'still-present'],
    ['changedExposure', 'still-present'],
    ['added', 'not-in-base'],
  ]) {
    const result = evaluateTargets(
      { pnpm: [target] },
      { pnpm: comparison({ [category]: [target] }) },
    );
    assert.equal(result[0].status, status);
  }
  assert.equal(
    evaluateTargets({ pnpm: [target] }, { pnpm: comparison() })[0].status,
    'not-in-base',
  );
  assert.throws(
    () => evaluateTargets({ pnpm: [target] }, {}),
    /Missing verified/,
  );
});

test('image targets must disappear from every location in the named image', () => {
  const imageTarget = { id: 'CVE-2026-1234', name: 'example', image: 'api' };
  const partial = comparison({
    removed: [imageTarget],
    inherited: [imageTarget],
  });
  assert.equal(
    evaluateTargets({ images: [imageTarget] }, { images: partial })[0].status,
    'still-present',
  );
  const otherImage = { ...imageTarget, image: 'worker' };
  const fixed = comparison({ removed: [imageTarget], inherited: [otherImage] });
  assert.equal(
    evaluateTargets({ images: [imageTarget] }, { images: fixed })[0].status,
    'resolved',
  );
});

test('Go package/module findings prevent a false reachability-only resolution', () => {
  const goTarget = { id: 'GO-2026-1234', name: 'example.com/lib' };
  const finding = { ...goTarget, exposures: ['package-only'] };
  assert.equal(
    evaluateTargets(
      { go: [goTarget] },
      { go: comparison({ inherited: [finding] }) },
    )[0].status,
    'still-present',
  );
});

test('an incomplete downloaded artifact fails instead of resolving a target', async (context) => {
  successfulScans(context);
  const directory = await mkdtemp(join(tmpdir(), 'rsp-targets-'));
  context.after(() => rm(directory, { recursive: true, force: true }));
  await writeFile(
    `${directory}/pr.json`,
    JSON.stringify(pr(block({ pnpm: [target] }))),
  );
  await writeFile(`${directory}/event.json`, JSON.stringify(event));
  let downloads = 0;
  const download = async (_program, args) => {
    downloads++;
    assert.ok(args.includes('pnpm-security-shadow'));
    // A fabricated derived result is not evidence without a verified snapshot.
    const path = args[args.indexOf('--dir') + 1];
    await writeFile(
      `${path}/comparison.json`,
      JSON.stringify(comparison({ removed: [target] })),
    );
    return { stdout: '', stderr: '' };
  };
  await assert.rejects(
    checkRemediation(
      `${directory}/pr.json`,
      `${directory}/event.json`,
      `${directory}/output`,
      download,
    ),
  );
  assert.equal(downloads, 1);
  assert.ok(await readFile(`${directory}/output/error.txt`, 'utf8'));
});

test('a complete frozen pnpm snapshot proves resolution through the CLI path', async (context) => {
  successfulScans(context);
  const directory = await mkdtemp(join(tmpdir(), 'rsp-resolved-'));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const summary = process.env.GITHUB_STEP_SUMMARY;
  delete process.env.GITHUB_STEP_SUMMARY;
  context.after(() => {
    if (summary) process.env.GITHUB_STEP_SUMMARY = summary;
  });
  context.mock.method(console, 'log', () => {});
  await writeFile(
    `${directory}/pr.json`,
    JSON.stringify(pr(block({ pnpm: [target] }))),
  );
  await writeFile(`${directory}/event.json`, JSON.stringify(event));
  const tree = (version) => ({
    dependencies: {
      app: { version: '1.0.0', dependencies: { example: { version } } },
    },
  });
  const request = JSON.stringify(
    combineTrees({ base: tree('1.0.0'), head: tree('2.0.0') }),
  );
  const snapshot = JSON.stringify({
    advisories: {
      123: {
        github_advisory_id: target.id,
        module_name: target.name,
        severity: 'high',
        vulnerable_versions: '<2.0.0',
        updated: '2026-01-01',
        findings: [{ version: '1.0.0', paths: ['base__app>example'] }],
      },
    },
    metadata: {
      vulnerabilities: { info: 0, low: 0, moderate: 0, high: 1, critical: 0 },
    },
  });
  const hash = (text) => createHash('sha256').update(text).digest('hex');
  const download = async (_program, args) => {
    const path = args[args.indexOf('--dir') + 1];
    await writeFile(`${path}/request.json`, request);
    await writeFile(`${path}/snapshot.json`, snapshot);
    await writeFile(
      `${path}/manifest.json`,
      JSON.stringify({
        requestSha256: hash(request),
        snapshotSha256: hash(snapshot),
      }),
    );
    return { stdout: '', stderr: '' };
  };
  await checkRemediation(
    `${directory}/pr.json`,
    `${directory}/event.json`,
    `${directory}/output`,
    download,
  );
  const results = JSON.parse(
    await readFile(`${directory}/output/results.json`, 'utf8'),
  );
  assert.equal(results[0].status, 'resolved');
});

test('failed, skipped or missing snapshot jobs cannot reuse an older artifact', async (context) => {
  const directory = await mkdtemp(join(tmpdir(), 'rsp-target-refresh-'));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const previous = process.env.SECURITY_SCAN_RESULTS;
  context.after(() => {
    if (previous === undefined) delete process.env.SECURITY_SCAN_RESULTS;
    else process.env.SECURITY_SCAN_RESULTS = previous;
  });
  await writeFile(
    `${directory}/pr.json`,
    JSON.stringify(pr(block({ pnpm: [target] }))),
  );
  await writeFile(`${directory}/event.json`, JSON.stringify(event));
  let downloads = 0;
  for (const status of ['failure', 'skipped', 'missing']) {
    process.env.SECURITY_SCAN_RESULTS = JSON.stringify(
      status === 'missing'
        ? null
        : { 'pnpm-security-shadow': { result: status } },
    );
    await assert.rejects(
      checkRemediation(
        `${directory}/pr.json`,
        `${directory}/event.json`,
        `${directory}/${status}`,
        async () => {
          downloads++;
        },
      ),
      /Incomplete snapshot job/,
    );
  }
  assert.equal(downloads, 0);
});
