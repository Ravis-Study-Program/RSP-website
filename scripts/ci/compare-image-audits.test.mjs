import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { mkdir, mkdtemp, readFile, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import {
  compareImageAudits,
  imageNames,
  normalizeImageAudit,
  trivyVersion,
} from './compare-image-audits.mjs';
import {
  replayImageSnapshot,
  runImageShadow,
} from './image-security-shadow.mjs';

const hash = (text) => createHash('sha256').update(text).digest('hex');
const imageID = (image, side) => `sha256:${hash(`${image}/${side}`)}`;
const vulnerability = {
  VulnerabilityID: 'CVE-2026-1234',
  PkgName: 'example',
  InstalledVersion: '1.0',
  FixedVersion: '1.1',
  Severity: 'HIGH',
};

function report(vulns = [vulnerability], id = imageID('api', 'head')) {
  return {
    SchemaVersion: 2,
    Trivy: { Version: trivyVersion },
    ArtifactType: 'container_image',
    ArtifactName: 'rsp-api:ci',
    Metadata: { ImageID: id, OS: { Family: 'alpine', Name: '3.24' } },
    Results: [
      {
        Class: 'os-pkgs',
        Type: 'alpine',
        Target: 'rsp-api:ci (alpine 3.24)',
        Packages: [{ Name: 'example', Version: '1.0' }],
        Vulnerabilities: structuredClone(vulns),
      },
    ],
  };
}

const reports = (vulns = []) =>
  Object.fromEntries(imageNames.map((image) => [image, report(vulns)]));

test('image tag and ID changes do not change vulnerability identities', () => {
  const base = reports([vulnerability]);
  const head = structuredClone(base);
  for (const image of imageNames) {
    head[image].ArtifactName = `rsp-${image}:another-tag`;
    head[image].Metadata.ImageID = imageID(image, 'another');
    head[image].Results[0].Target = `rsp-${image}:another-tag (alpine 3.24)`;
  }
  assert.equal(compareImageAudits(base, head).inherited.length, 6);
});

test('equal counts and findings in another image cannot hide new advisories', () => {
  const base = reports();
  base.api = report();
  const head = reports();
  head.worker = report();
  const moved = compareImageAudits(base, head);
  assert.equal(moved.added.length, 1);
  assert.equal(moved.removed.length, 1);
  head.worker = report([
    { ...vulnerability, VulnerabilityID: 'CVE-2026-9999' },
  ]);
  assert.equal(compareImageAudits(base, head).added.length, 1);
});

test('changed image exposure tracks versions, severity, paths and distro', () => {
  for (const override of [
    { InstalledVersion: '1.0.1' },
    { FixedVersion: '1.2' },
    { Severity: 'CRITICAL' },
    { PkgPath: '/new/path' },
  ]) {
    assert.equal(
      compareImageAudits(
        reports([vulnerability]),
        reports([{ ...vulnerability, ...override }]),
      ).changedExposure.length,
      6,
    );
  }
  const head = reports([vulnerability]);
  head.api.Metadata.OS.Name = '3.25';
  assert.equal(
    compareImageAudits(reports([vulnerability]), head).changedExposure.length,
    1,
  );
});

test('library file identities stay separate and duplicate records collapse', () => {
  const base = report();
  base.Results.push({
    Class: 'lang-pkgs',
    Type: 'gobinary',
    Target: 'usr/bin/service',
    Vulnerabilities: [vulnerability, vulnerability],
  });
  const head = structuredClone(base);
  head.Results[1].Target = 'usr/bin/another-service';
  assert.equal(normalizeImageAudit(base, 'api').size, 2);
  assert.notDeepEqual(
    [...normalizeImageAudit(base, 'api').keys()],
    [...normalizeImageAudit(head, 'api').keys()],
  );
});

test('missing inventories, image sets, wrong scanners and invalid findings fail closed', () => {
  for (const mutate of [
    (value) => {
      value.Results = [];
    },
    (value) => {
      value.Results[0].Packages = [];
    },
    (value) => {
      value.Metadata.OS.Eosl = true;
    },
    (value) => {
      value.Trivy.Version = '0.1.0';
    },
    (value) => {
      value.Results[0].Type = 'jar';
    },
    (value) => {
      value.Results[0].Vulnerabilities = {};
    },
    (value) => {
      value.Results[0].Vulnerabilities[0].FixedVersion = {};
    },
    (value) => {
      value.Results[0].Vulnerabilities[0].Severity = 'INVALID';
    },
  ]) {
    const invalid = report();
    mutate(invalid);
    assert.throws(() => normalizeImageAudit(invalid, 'api'));
  }
  assert.throws(() =>
    normalizeImageAudit(report(), 'api', imageID('wrong', 'head')),
  );
  assert.throws(() => compareImageAudits({}, reports()));
  assert.equal(compareImageAudits(reports(), reports()).added.length, 0);
});

test('lower severity and unfixed image findings remain visible to target checks', () => {
  const result = compareImageAudits(
    reports([vulnerability]),
    reports([{ ...vulnerability, Severity: 'LOW', FixedVersion: '' }]),
  );
  assert.equal(result.removed.length, 0);
  assert.equal(result.changedExposure.length, 6);
});

function fakeCommands(fault) {
  const scans = [];
  let downloads = 0;
  let builds = 0;
  const run = async (program, args, options) => {
    assert.ok(
      !Object.keys(options.env).some((name) => name.startsWith('TRIVY_')),
    );
    if (program === 'bash') {
      builds++;
      assert.equal(options.env.IMAGE_TAG, 'security-base');
    } else if (program === 'docker') {
      const [image, tag] = args.at(-1).slice(4).split(':');
      return {
        stdout: JSON.stringify([
          {
            Id: imageID(image, tag === 'ci' ? 'head' : 'base'),
            Os: 'linux',
            Architecture: fault === 'platform' ? 'arm64' : 'amd64',
          },
        ]),
        stderr: '',
      };
    } else if (args[0] === '--version') {
      const Version = fault === 'version' ? '0.1.0' : trivyVersion;
      return { stdout: JSON.stringify({ Version }), stderr: '' };
    } else if (args.includes('--download-db-only')) {
      downloads++;
      const cache = args[args.indexOf('--cache-dir') + 1];
      await mkdir(`${cache}/db`, { recursive: true });
      await writeFile(`${cache}/db/trivy.db`, 'frozen-db');
      await writeFile(`${cache}/db/metadata.json`, '{"Version":2}');
    } else {
      scans.push(args);
      if (fault === 'scanner') throw new Error('scanner failed');
      const path = args[args.indexOf('--output') + 1];
      await writeFile(
        path,
        JSON.stringify(report([vulnerability], args.at(-1))),
      );
      if (fault === 'database') {
        const cache = args[args.indexOf('--cache-dir') + 1];
        await writeFile(`${cache}/db/trivy.db`, 'changed-db');
      }
    }
    return { stdout: '', stderr: '' };
  };
  return { run, scans, counts: () => ({ downloads, builds }) };
}

async function fixture(context) {
  const directory = await mkdtemp(join(tmpdir(), 'rsp-images-'));
  context.after(() => rm(directory, { recursive: true, force: true }));
  const summary = process.env.GITHUB_STEP_SUMMARY;
  delete process.env.GITHUB_STEP_SUMMARY;
  context.after(() => {
    if (summary) process.env.GITHUB_STEP_SUMMARY = summary;
  });
  context.mock.method(console, 'log', () => {});
  return `${directory}/output`;
}

test('image pilot builds only base, freezes one DB and scans 12 immutable IDs', async (context) => {
  const output = await fixture(context);
  const fake = fakeCommands();
  await runImageShadow('.', output, fake.run);
  assert.deepEqual(fake.counts(), { downloads: 1, builds: 1 });
  assert.equal(fake.scans.length, 12);
  for (const args of fake.scans) {
    for (const flag of [
      '--skip-db-update',
      '--skip-java-db-update',
      '--offline-scan',
      '--skip-version-check',
      '--list-all-pkgs',
    ]) {
      assert.ok(args.includes(flag));
    }
    assert.match(args.at(-1), /^sha256:/);
    assert.equal(args[args.indexOf('--exit-code') + 1], '0');
    assert.equal(
      args[args.indexOf('--severity') + 1],
      'UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL',
    );
    assert.ok(!args.includes('--ignore-unfixed'));
  }
  // Comparison replay deliberately needs no multi-gigabyte database binary.
  await rm(`${output}/cache/db/trivy.db`);
  assert.equal((await replayImageSnapshot(output)).inherited.length, 6);
  await writeFile(`${output}/cache/db/metadata.json`, 'tampered');
  await assert.rejects(replayImageSnapshot(output), /integrity check failed/);
});

test('image scan failures, database changes and platform mismatches never look clean', async (context) => {
  for (const fault of ['scanner', 'database', 'platform', 'version']) {
    await context.test(fault, async (child) => {
      const output = await fixture(child);
      await assert.rejects(
        runImageShadow('.', output, fakeCommands(fault).run),
      );
      assert.ok(await readFile(`${output}/error.txt`, 'utf8'));
      await assert.rejects(replayImageSnapshot(output));
    });
  }
});
