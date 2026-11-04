import assert from 'node:assert/strict';
import test from 'node:test';
import { compareAudits, renderComparison } from './compare-pnpm-audits.mjs';

function advisory(overrides = {}) {
  return {
    github_advisory_id: 'GHSA-aaaa-bbbb-cccc',
    module_name: 'example',
    severity: 'high',
    updated: '2026-01-01',
    vulnerable_versions: '<2.0.0',
    findings: [{ version: '1.0.0', paths: ['apps__web>example'] }],
    ...overrides,
  };
}

function report(...advisories) {
  const vulnerabilities = {
    info: 0,
    low: 0,
    moderate: 0,
    high: 0,
    critical: 0,
  };
  for (const item of advisories) vulnerabilities[item.severity]++;
  return {
    advisories: Object.fromEntries(
      advisories.map((item, index) => [index, item]),
    ),
    metadata: { vulnerabilities },
  };
}

test('unchanged findings are inherited; complete removals are resolved', () => {
  const base = report(advisory());
  assert.equal(compareAudits(base, base).inherited.length, 1);
  assert.equal(compareAudits(base, report()).removed.length, 1);
  assert.equal(compareAudits(report(), report()).added.length, 0);
});

test('new advisories are not hidden by equal finding counts', () => {
  const result = compareAudits(
    report(advisory()),
    report(advisory({ github_advisory_id: 'GHSA-dddd-eeee-ffff' })),
  );
  assert.equal(result.added.length, 1);
  assert.equal(result.removed.length, 1);
});

test('package renames do not inherit another package baseline', () => {
  const result = compareAudits(
    report(advisory()),
    report(advisory({ module_name: 'renamed' })),
  );
  assert.equal(result.added.length, 1);
  assert.equal(result.removed.length, 1);
});

test('additional vulnerable paths and versions flag changed exposure', () => {
  for (const findings of [
    [{ version: '1.0.0', paths: ['apps__web>example', 'apps__auth>example'] }],
    [{ version: '1.1.0', paths: ['apps__web>example'] }],
  ]) {
    const result = compareAudits(
      report(advisory()),
      report(advisory({ findings })),
    );
    assert.equal(result.changedExposure.length, 1);
    assert.equal(result.changedExposure[0].addedExposures.length, 1);
  }
});

test('duplicate and reordered paths do not create false regressions', () => {
  const result = compareAudits(
    report(advisory()),
    report(
      advisory({
        findings: [
          {
            version: '1.0.0',
            paths: ['apps__web>example', 'apps__web>example'],
          },
        ],
      }),
    ),
  );
  assert.equal(result.inherited.length, 1);
});

test('partial exposure removal does not claim complete remediation', () => {
  const result = compareAudits(
    report(
      advisory({
        findings: [
          {
            version: '1.0.0',
            paths: ['apps__web>example', 'apps__auth>example'],
          },
        ],
      }),
    ),
    report(advisory()),
  );
  assert.equal(result.inherited.length, 1);
  assert.equal(result.removed.length, 0);
  assert.equal(result.changedExposure.length, 0);
});

test('advisory database drift is incomparable, not a PR regression', () => {
  for (const override of [
    { updated: '2026-02-01' },
    { severity: 'critical' },
    { vulnerable_versions: '<3.0.0' },
  ]) {
    assert.throws(
      () => compareAudits(report(advisory()), report(advisory(override))),
      /Advisory data changed/,
    );
  }
});

test('registry errors, missing fields, and incomplete findings fail closed', () => {
  for (const invalid of [
    {},
    { error: { code: 'ECONNRESET' } },
    { ...report(), metadata: {} },
    { ...report(), metadata: report(advisory()).metadata },
    report(advisory({ findings: [] })),
    report(advisory({ findings: [{ version: '1.0.0', paths: [] }] })),
    report(advisory({ severity: 'unknown' })),
  ]) {
    assert.throws(() => compareAudits(report(), invalid));
  }
});

test('summary states shadow mode and the frozen response source', () => {
  const summary = renderComparison(compareAudits(report(), report(advisory())));
  assert.match(summary, /added: 1/);
  assert.match(summary, /does not relax/);
  assert.match(summary, /one frozen npm audit response/);
});
