import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { compareAudits } from './compare-pnpm-audits.mjs';
import {
  combineTrees,
  replaySnapshot,
  runShadow,
  splitSnapshot,
} from './pnpm-shadow.mjs';

const tree = {
  dependencies: {
    apps__web: {
      version: '1.0.0',
      dependencies: { example: { version: '1.0.0' } },
      requires: { example: '1.0.0' },
    },
  },
};

function snapshot(paths) {
  return {
    advisories: {
      123: {
        github_advisory_id: 'GHSA-aaaa-bbbb-cccc',
        module_name: 'example',
        severity: 'high',
        updated: '2026-01-01',
        vulnerable_versions: '<2.0.0',
        findings: [{ version: '1.0.0', paths }],
      },
    },
    metadata: {
      vulnerabilities: { info: 0, low: 0, moderate: 0, high: 1, critical: 0 },
    },
  };
}

test('union namespaces importers without changing their dependency graphs', () => {
  const request = combineTrees({ base: tree, head: tree });
  assert.deepEqual(Object.keys(request.dependencies), [
    'base__apps__web',
    'head__apps__web',
  ]);
  assert.deepEqual(
    request.dependencies.base__apps__web,
    tree.dependencies.apps__web,
  );
  assert.deepEqual(request.requires, {
    base__apps__web: '1.0.0',
    head__apps__web: '1.0.0',
  });
  assert.throws(() => combineTrees({ base: {}, head: tree }), /Missing base/);
});

test('one advisory record applies to both trees without a publication race', () => {
  const request = combineTrees({ base: tree, head: tree });
  const reports = splitSnapshot(
    snapshot(['base__apps__web>example', 'head__apps__web>example']),
    request,
  );
  assert.deepEqual(reports.base, reports.head);
  assert.deepEqual(reports.base.advisories[123].findings[0].paths, [
    'apps__web>example',
  ]);
  assert.equal(compareAudits(reports.base, reports.head).inherited.length, 1);
});

test('side-specific findings remain added or removed after partition', () => {
  const request = combineTrees({ base: tree, head: tree });
  for (const side of ['base', 'head']) {
    const reports = splitSnapshot(
      snapshot([`${side}__apps__web>example`]),
      request,
    );
    const result = compareAudits(reports.base, reports.head);
    assert.equal(result[side === 'base' ? 'removed' : 'added'].length, 1);
    assert.equal(result.inherited.length, 0);
  }
});

test('different affected versions retain their own side and exposure', () => {
  const head = structuredClone(tree);
  head.dependencies.apps__web.dependencies.example.version = '1.1.0';
  head.dependencies.apps__web.requires.example = '1.1.0';
  const request = combineTrees({ base: tree, head });
  const response = snapshot(['base__apps__web>example']);
  response.advisories[123].findings.push({
    version: '1.1.0',
    paths: ['head__apps__web>example'],
  });
  const reports = splitSnapshot(response, request);
  assert.equal(reports.base.advisories[123].findings[0].version, '1.0.0');
  assert.equal(reports.head.advisories[123].findings[0].version, '1.1.0');
  assert.equal(
    compareAudits(reports.base, reports.head).changedExposure.length,
    1,
  );
});

test('a valid empty snapshot produces two clean reports', () => {
  const request = combineTrees({ base: tree, head: tree });
  const response = snapshot([]);
  response.advisories = {};
  response.metadata.vulnerabilities.high = 0;
  const reports = splitSnapshot(response, request);
  assert.deepEqual(compareAudits(reports.base, reports.head), {
    added: [],
    changedExposure: [],
    inherited: [],
    removed: [],
  });
});

test('unknown importer paths and malformed snapshots fail closed', () => {
  const request = combineTrees({ base: tree, head: tree });
  for (const invalid of [
    snapshot(['unknown>example']),
    snapshot(['base__apps__web']),
    { error: 'registry unavailable' },
    {},
  ]) {
    assert.throws(() => splitSnapshot(invalid, request));
  }
});

test('offline replay checks request and response hashes', async (context) => {
  const directory = await mkdtemp(join(tmpdir(), 'rsp-snapshot-'));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const request = JSON.stringify(combineTrees({ base: tree, head: tree }));
  const response = JSON.stringify(snapshot(['head__apps__web>example']));
  const hash = (text) => createHash('sha256').update(text).digest('hex');
  await writeFile(`${directory}/request.json`, request);
  await writeFile(`${directory}/snapshot.json`, response);
  await writeFile(
    `${directory}/manifest.json`,
    JSON.stringify({
      requestSha256: hash(request),
      snapshotSha256: hash(response),
    }),
  );
  assert.equal((await replaySnapshot(directory)).added.length, 1);
  for (const [name, original] of [
    ['request', request],
    ['snapshot', response],
  ]) {
    await writeFile(`${directory}/${name}.json`, `${original} `);
    await assert.rejects(replaySnapshot(directory), /integrity check failed/);
    await writeFile(`${directory}/${name}.json`, original);
  }
});

test('pnpm capture includes dev dependencies and makes one upstream request', async (context) => {
  const directory = await mkdtemp(join(tmpdir(), 'rsp-capture-'));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const summaryPath = process.env.GITHUB_STEP_SUMMARY;
  delete process.env.GITHUB_STEP_SUMMARY;
  context.after(() => {
    if (summaryPath) process.env.GITHUB_STEP_SUMMARY = summaryPath;
  });
  context.mock.method(console, 'log', () => {});
  await writeFile(
    `${directory}/package.json`,
    JSON.stringify({
      dependencies: { example: '1.0.0' },
      devDependencies: { 'dev-example': '1.0.0' },
    }),
  );
  await writeFile(`${directory}/.npmrc`, 'production=true\n');
  await writeFile(
    `${directory}/pnpm-lock.yaml`,
    `lockfileVersion: '9.0'
importers:
  .:
    dependencies:
      example: {specifier: 1.0.0, version: 1.0.0}
    devDependencies:
      dev-example: {specifier: 1.0.0, version: 1.0.0}
packages:
  example@1.0.0: {resolution: {integrity: sha512-ZmFrZQ==}}
  dev-example@1.0.0: {resolution: {integrity: sha512-ZmFrZQ==}}
snapshots:
  example@1.0.0: {}
  dev-example@1.0.0: {}
`,
  );
  let calls = 0;
  context.mock.method(globalThis, 'fetch', async (_url, options) => {
    calls++;
    const request = JSON.parse(options.body);
    for (const side of ['base', 'head']) {
      const dependencies = request.dependencies[`${side}__.`].dependencies;
      assert.ok(dependencies.example);
      assert.ok(dependencies['dev-example']);
    }
    const response = snapshot([]);
    response.advisories = {};
    response.metadata.vulnerabilities.high = 0;
    return new Response(JSON.stringify(response));
  });
  await runShadow(directory, directory, `${directory}/output`);
  assert.equal(calls, 1);
});
