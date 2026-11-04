import assert from 'node:assert/strict';
import test from 'node:test';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import {
  imageVulnerabilities,
  collectReports,
  marker,
  renderReport,
} from './security-tracker.mjs';

test('tracker ignores raw reports and publishes no secret context', async (context) => {
  const directory = await mkdtemp(join(tmpdir(), 'rsp-tracker-'));
  context.after(() => rm(directory, { recursive: true, force: true }));
  await writeFile(join(directory, 'go.txt'), 'private scanner error');
  await writeFile(join(directory, 'pnpm.txt'), 'private registry error');
  await writeFile(
    join(directory, 'api.json'),
    JSON.stringify({
      Results: [
        {
          Secrets: [{ Match: 'private secret', Code: 'private source' }],
        },
      ],
    }),
  );
  const reports = await collectReports(directory);
  assert.deepEqual(reports, [{ name: 'api.json', text: '[]' }]);
});

test('published image data excludes secret matches and source context', () => {
  const findings = imageVulnerabilities({
    Results: [
      {
        Target: 'private/source',
        Secrets: [{ Match: 'secret-value', Code: 'private source context' }],
        Vulnerabilities: [
          {
            VulnerabilityID: 'CVE-2026-1234',
            PkgName: 'libssl3',
            InstalledVersion: '1',
            Severity: 'HIGH',
            Description: 'private context',
          },
        ],
      },
    ],
  });
  assert.deepEqual(findings, [
    {
      id: 'CVE-2026-1234',
      package: 'libssl3',
      installed: '1',
      fixed: '',
      severity: 'HIGH',
    },
  ]);
  assert.doesNotMatch(JSON.stringify(findings), /secret-value|private/);
});

const input = {
  results: {
    dependencies: { result: 'failure' },
    containers: { result: 'success' },
  },
  reports: [],
  runUrl: 'https://github.com/example/repo/actions/runs/1',
  sha: 'abc123',
};

test('missing reports and failed jobs never imply a clean scan', () => {
  const body = renderReport(input);
  assert.ok(body.startsWith(marker));
  assert.match(body, /dependencies \| failure/);
  assert.match(body, /containers \| success/);
  assert.match(body, /No scanner reports available/);
  assert.match(body, /scanner\/build errors/);
  assert.match(body, /actions\/runs\/1/);
});

test('scanner metadata cannot inject HTML and long reports are truncated', () => {
  const body = renderReport({
    ...input,
    reports: [
      { name: 'pnpm.txt', text: '<script>&unsafe</script>' + 'x'.repeat(6000) },
    ],
  });
  assert.ok(!body.includes('<script>'));
  assert.match(body, /&lt;script&gt;&amp;unsafe/);
  assert.match(body, /Truncated/);
  assert.ok(body.length < 8000);
});
