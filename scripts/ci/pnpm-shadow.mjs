import { execFile } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile, appendFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { promisify } from 'node:util';
import {
  compareAudits,
  normalizeAudit,
  renderComparison,
} from './compare-pnpm-audits.mjs';

const execute = promisify(execFile);
const endpoint = '/-/npm/v1/security/audits';
const registry = 'https://registry.npmjs.org';
const sides = ['base', 'head'];
const emptyCounts = () => ({
  info: 0,
  low: 0,
  moderate: 0,
  high: 0,
  critical: 0,
});
const digest = (text) => createHash('sha256').update(text).digest('hex');

export function combineTrees(trees) {
  const dependencies = {};
  for (const side of sides) {
    const tree = trees[side];
    if (
      !tree?.dependencies ||
      typeof tree.dependencies !== 'object' ||
      Array.isArray(tree.dependencies) ||
      !Object.keys(tree.dependencies).length
    ) {
      throw new Error(`Missing ${side} audit tree`);
    }
    for (const [name, dependency] of Object.entries(tree.dependencies)) {
      if (name.includes('>') || typeof dependency.version !== 'string') {
        throw new Error('Unrecognized pnpm audit tree');
      }
      dependencies[`${side}__${name}`] = dependency;
    }
  }
  return {
    dependencies,
    requires: Object.fromEntries(
      Object.entries(dependencies).map(([name, value]) => [
        name,
        value.version,
      ]),
    ),
    dev: false,
    install: [],
    remove: [],
    metadata: {},
  };
}

export function splitSnapshot(snapshot, request) {
  normalizeAudit(snapshot);
  const reports = Object.fromEntries(
    sides.map((side) => [
      side,
      {
        advisories: {},
        metadata: { vulnerabilities: emptyCounts() },
      },
    ]),
  );
  for (const [id, advisory] of Object.entries(snapshot.advisories)) {
    const findings = { base: [], head: [] };
    for (const finding of advisory.findings) {
      const paths = { base: [], head: [] };
      for (const path of finding.paths) {
        const root = path.split('>')[0];
        if (!Object.hasOwn(request.dependencies, root)) {
          throw new Error(`Unknown snapshot dependency path: ${root}`);
        }
        const side = sides.find((value) => root.startsWith(`${value}__`));
        if (!side || !path.includes('>')) {
          throw new Error('Snapshot finding lacks an importer path');
        }
        paths[side].push(path.slice(`${side}__`.length));
      }
      for (const side of sides) {
        if (paths[side].length) {
          findings[side].push({ ...finding, paths: paths[side] });
        }
      }
    }
    for (const side of sides) {
      if (findings[side].length) {
        reports[side].advisories[id] = {
          ...advisory,
          findings: findings[side],
        };
        // These are derived affected-path counts, not npm's dependency totals.
        reports[side].metadata.vulnerabilities[advisory.severity] += findings[
          side
        ].reduce((sum, finding) => sum + finding.paths.length, 0);
      }
    }
  }
  return reports;
}

async function captureTree(directory, output, side) {
  let tree;
  const server = createServer(async (request, response) => {
    try {
      if (request.method !== 'POST' || request.url !== endpoint || tree) {
        throw new Error('Unexpected pnpm registry request');
      }
      const chunks = [];
      let size = 0;
      for await (const chunk of request) {
        size += chunk.length;
        if (size > 16 * 1024 * 1024)
          throw new Error('Audit tree exceeds 16 MiB');
        chunks.push(chunk);
      }
      tree = JSON.parse(Buffer.concat(chunks).toString('utf8'));
      // This response only captures pnpm's lockfile graph. It is never a scan result.
      response.writeHead(200, { 'Content-Type': 'application/json' });
      response.end(
        JSON.stringify({
          actions: [],
          advisories: {},
          metadata: { vulnerabilities: emptyCounts() },
        }),
      );
    } catch {
      response.writeHead(400);
      response.end('Invalid audit tree request');
    }
  });
  await new Promise((done) => server.listen(0, '127.0.0.1', done));
  const env = {
    ...process.env,
    npm_config_manage_package_manager_versions: 'false',
    npm_config_package_manager_strict: 'false',
    // pnpm turns both false into its default all-dependency audit. Setting
    // production and dev both true instead silently excludes dev dependencies.
    npm_config_production: 'false',
    npm_config_dev: 'false',
    npm_config_optional: 'true',
    npm_config_only: '',
  };
  const options = {
    cwd: directory,
    env,
    timeout: 60_000,
    maxBuffer: 16 * 1024 * 1024,
  };
  try {
    const { stdout: version } = await execute('pnpm', ['--version'], options);
    if (
      process.env.PNPM_VERSION &&
      version.trim() !== process.env.PNPM_VERSION
    ) {
      throw new Error('Scanner does not match required pnpm version');
    }
    await writeFile(`${output}/${side}-version.txt`, version);
    await execute(
      'pnpm',
      [
        'audit',
        '--json',
        `--registry=http://127.0.0.1:${server.address().port}`,
      ],
      options,
    );
    if (!tree) throw new Error('pnpm did not submit an audit tree');
    return { tree, version };
  } finally {
    await new Promise((done) => server.close(done));
  }
}

export async function replaySnapshot(directory) {
  const manifest = JSON.parse(
    await readFile(`${directory}/manifest.json`, 'utf8'),
  );
  const request = await readFile(`${directory}/request.json`, 'utf8');
  const snapshot = await readFile(`${directory}/snapshot.json`, 'utf8');
  if (
    digest(request) !== manifest.requestSha256 ||
    digest(snapshot) !== manifest.snapshotSha256
  ) {
    throw new Error('Snapshot integrity check failed');
  }
  const reports = splitSnapshot(JSON.parse(snapshot), JSON.parse(request));
  return compareAudits(reports.base, reports.head);
}

export async function runShadow(base, head, output) {
  output = resolve(output);
  await mkdir(output, { recursive: true });
  const trees = {};
  const versions = [];
  for (const [side, directory] of [
    ['base', base],
    ['head', head],
  ]) {
    const captured = await captureTree(directory, output, side);
    trees[side] = captured.tree;
    versions.push(captured.version);
  }
  if (versions[0] !== versions[1])
    throw new Error('pnpm scanner versions differ');
  const request = combineTrees(trees);
  const requestText = JSON.stringify(request);
  await writeFile(`${output}/request.json`, requestText);
  // Exactly one upstream request evaluates the union of both dependency graphs.
  // Both reports below derive from this frozen response, never separate scans.
  const response = await fetch(`${registry}${endpoint}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: requestText,
    signal: AbortSignal.timeout(60_000),
  });
  if (!response.ok)
    throw new Error(`npm audit snapshot failed: HTTP ${response.status}`);
  const snapshotText = await response.text();
  await writeFile(`${output}/snapshot.json`, snapshotText);
  const reports = splitSnapshot(JSON.parse(snapshotText), request);
  const manifest = {
    registry,
    pnpmVersion: versions[0].trim(),
    capturedAt: new Date().toISOString(),
    requestSha256: digest(requestText),
    snapshotSha256: digest(snapshotText),
    baseLockSha256: digest(await readFile(resolve(base, 'pnpm-lock.yaml'))),
    headLockSha256: digest(await readFile(resolve(head, 'pnpm-lock.yaml'))),
  };
  for (const side of sides) {
    await writeFile(
      `${output}/${side}.json`,
      JSON.stringify(reports[side], null, 2),
    );
  }
  const result = compareAudits(reports.base, reports.head);
  await writeFile(`${output}/manifest.json`, JSON.stringify(manifest, null, 2));
  await writeFile(`${output}/comparison.json`, JSON.stringify(result, null, 2));
  const summary = `${renderComparison(result)}\nSnapshot SHA-256: \`${manifest.snapshotSha256}\`\n`;
  await writeFile(`${output}/summary.md`, summary);
  if (process.env.GITHUB_STEP_SUMMARY) {
    await appendFile(process.env.GITHUB_STEP_SUMMARY, summary);
  }
  console.log(summary);
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  if (process.argv[2] === '--replay') {
    console.log(renderComparison(await replaySnapshot(process.argv[3])));
  } else {
    await runShadow(...process.argv.slice(2));
  }
}
