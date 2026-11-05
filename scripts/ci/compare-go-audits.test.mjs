import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdtemp, rm, writeFile, mkdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import {
  compareGoAudits,
  normalizeGoAudit,
  parseMessages,
  renderGoComparison,
} from './compare-go-audits.mjs';
import {
  replayGoSnapshot,
  selectEntries,
  validateEntry,
} from './go-security-shadow.mjs';

const id = 'GO-2026-1234';
const modified = '2026-01-01T00:00:00Z';
const config = {
  protocol_version: 'v1.0.0',
  scanner_name: 'govulncheck',
  scanner_version: 'v1.6.0',
  go_version: 'go1.26.10',
  scan_level: 'symbol',
  scan_mode: 'source',
  db: 'file:///snapshot',
  db_last_modified: modified,
};
const moduleFrame = { module: 'example.com/lib', version: 'v1.0.0' };

function stream(frames = [], overrides = {}) {
  return [
    { config: { ...config, ...overrides } },
    { SBOM: { modules: [{ path: 'example.com/app' }] } },
    { osv: { id, modified } },
    ...frames.map((frame) => ({ finding: { osv: id, trace: [frame] } })),
  ]
    .map((message) => JSON.stringify(message, null, 2))
    .join('\n');
}

test('Go JSON stream parser handles escaped quotes and braces inside strings', () => {
  const text = JSON.stringify({
    progress: { message: 'quote " and slash \\ and } {' },
  });
  assert.equal(parseMessages(`${text}\n${text}`).length, 2);
  for (const invalid of [
    '',
    '{',
    `${text}\n{"finding":`,
    '[]',
    `${text}garbage`,
  ]) {
    assert.throws(() => parseMessages(invalid));
  }
});

test('Go clean, inherited, added and removed findings compare by advisory/module', () => {
  const clean = stream();
  const finding = stream([moduleFrame]);
  assert.equal(compareGoAudits(clean, finding).added.length, 1);
  assert.equal(compareGoAudits(finding, clean).removed.length, 1);
  assert.equal(compareGoAudits(finding, finding).inherited.length, 1);
  assert.equal(compareGoAudits(clean, clean).added.length, 0);
  const rename = compareGoAudits(
    finding,
    stream([{ ...moduleFrame, module: 'other/lib' }]),
  );
  assert.equal(rename.added.length, 1);
  assert.equal(rename.removed.length, 1);
});

test('new Go package/symbol reachability and versions flag changed exposure', () => {
  for (const frame of [
    { ...moduleFrame, package: 'example.com/lib' },
    { ...moduleFrame, package: 'example.com/lib', function: 'Unsafe' },
    { ...moduleFrame, version: 'v1.1.0' },
  ]) {
    assert.equal(
      compareGoAudits(stream([moduleFrame]), stream([moduleFrame, frame]))
        .changedExposure.length,
      1,
    );
  }
});

test('Go trace positions and duplicate traces do not create false regressions', () => {
  const frame = {
    ...moduleFrame,
    package: 'example.com/lib',
    function: 'Unsafe',
  };
  const moved = { ...frame, position: { filename: 'new.go', line: 100 } };
  assert.equal(
    compareGoAudits(stream([frame]), stream([moved, moved])).inherited.length,
    1,
  );
});

test('Go summaries distinguish imported packages from reachable symbols', () => {
  const imported = compareGoAudits(
    stream(),
    stream([{ ...moduleFrame, package: 'example.com/lib' }]),
  );
  assert.match(renderGoComparison(imported), /not proven reachable/);
  const called = compareGoAudits(
    stream(),
    stream([
      { ...moduleFrame, package: 'example.com/lib', function: 'Unsafe' },
    ]),
  );
  assert.match(renderGoComparison(called), /reachable symbol/);
});

test('Go scan errors, mismatched tools/databases, and incomplete reports fail closed', () => {
  for (const invalid of [
    '{}',
    JSON.stringify({ config }),
    stream().replace('"SBOM"', '"unknown"'),
    stream([{}]),
    stream([moduleFrame]).replaceAll(id, 'unknown'),
    stream([], { scanner_version: 'v2.0.0' }),
    stream([], { scan_level: 'package' }),
    `${stream()}\n{"error":"scan failed"}`,
  ]) {
    assert.throws(() => normalizeGoAudit(invalid));
  }
  for (const overrides of [
    { go_version: 'go1.27.0' },
    { db: 'file:///other' },
    { db_last_modified: '2026-02-01' },
  ]) {
    assert.throws(
      () => compareGoAudits(stream(), stream([], overrides)),
      /configuration differs/,
    );
  }
});

test('snapshot selects only union modules and detects advisory changes during download', () => {
  const index = [
    { path: 'stdlib', vulns: [{ id, modified }] },
    { path: 'other', vulns: [{ id: 'GO-2026-9999', modified }] },
  ];
  assert.deepEqual([...selectEntries(index, new Set(['stdlib'])).keys()], [id]);
  validateEntry({ id, modified, affected: [] }, id, modified);
  assert.throws(() =>
    validateEntry({ id, modified: '2026-02-01', affected: [] }, id, modified),
  );
  assert.throws(() =>
    selectEntries(
      [{ path: 'stdlib', vulns: [{ id: '../escape', modified }] }],
      new Set(['stdlib']),
    ),
  );
});

test('Go offline replay verifies scans and database hashes', async (context) => {
  const directory = await mkdtemp(join(tmpdir(), 'rsp-go-replay-'));
  context.after(() => rm(directory, { recursive: true, force: true }));
  await mkdir(`${directory}/database/index`, { recursive: true });
  const contents = {
    'base.json': stream(),
    'head.json': stream([moduleFrame]),
    'database/index/modules.json': '[]',
    'database/index/db.json': '{}',
  };
  const files = {};
  for (const [path, text] of Object.entries(contents)) {
    await writeFile(`${directory}/${path}`, text);
    files[path] = createHash('sha256').update(text).digest('hex');
  }
  await writeFile(`${directory}/manifest.json`, JSON.stringify({ files }));
  assert.equal((await replayGoSnapshot(directory)).added.length, 1);
  await writeFile(`${directory}/database/index/modules.json`, '[{}]');
  await assert.rejects(replayGoSnapshot(directory), /integrity check failed/);
});
