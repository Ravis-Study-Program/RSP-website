import { execFile } from 'node:child_process';
import { appendFile, mkdir, readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { promisify } from 'node:util';
import { replaySnapshot } from './pnpm-shadow.mjs';
import { replayGoSnapshot } from './go-security-shadow.mjs';
import { replayImageSnapshot } from './image-security-shadow.mjs';
import { imageNames } from './compare-image-audits.mjs';

const execute = promisify(execFile);
const scanners = {
  pnpm: { artifact: 'pnpm-security-shadow', replay: replaySnapshot },
  go: { artifact: 'go-security-shadow', replay: replayGoSnapshot },
  images: { artifact: 'image-security-shadow', replay: replayImageSnapshot },
};

export function parseIntent(pr, event) {
  const expected = event.pull_request;
  if (!expected || pr.number !== expected.number)
    throw new Error('Mismatched PR event');
  const body = pr.body ?? '';
  const openings = body.match(/^```security-remediation\b/gm) ?? [];
  const blocks = [
    ...body.matchAll(/^```security-remediation\s*\n([\s\S]*?)^```\s*$/gm),
  ];
  const labelled = pr.labels?.some(
    (label) => label.name === 'security-remediation',
  );
  const required = Boolean(labelled || openings.length);
  if (
    required &&
    (!/^[a-f0-9]{40}$/.test(pr.head?.sha ?? '') ||
      !/^[a-f0-9]{40}$/.test(pr.base?.sha ?? '') ||
      pr.head?.sha !== expected.head?.sha ||
      pr.base?.sha !== expected.base?.sha)
  ) {
    throw new Error(
      'PR refs changed; push or rebase to obtain fresh snapshots',
    );
  }
  if (required && (openings.length !== 1 || blocks.length !== 1)) {
    throw new Error(
      'Remediation PRs need exactly one security-remediation JSON block',
    );
  }
  const targets = {};
  let count = 0;
  if (required) {
    if (blocks[0][1].length > 16_384)
      throw new Error('Target block exceeds 16 KiB');
    const specification = JSON.parse(blocks[0][1]);
    if (
      !specification ||
      Array.isArray(specification) ||
      typeof specification !== 'object'
    ) {
      throw new Error('Target specification must be an object');
    }
    for (const [scanner, items] of Object.entries(specification)) {
      if (!Object.hasOwn(scanners, scanner) || !Array.isArray(items)) {
        throw new Error('Unknown scanner or invalid target list');
      }
      const seen = new Set();
      targets[scanner] = items.map((target) => {
        const fields =
          scanner === 'images' ? ['id', 'name', 'image'] : ['id', 'name'];
        const pattern =
          scanner === 'pnpm'
            ? /^GHSA-[a-z0-9]{4}-[a-z0-9]{4}-[a-z0-9]{4}$/
            : scanner === 'go'
              ? /^GO-\d{4}-\d+$/
              : /^[A-Za-z0-9_.:-]+$/;
        if (
          !target ||
          Object.keys(target).length !== fields.length ||
          !fields.every((field) => typeof target[field] === 'string') ||
          !Object.keys(target).every((field) => fields.includes(field)) ||
          !pattern.test(target.id) ||
          !/^[A-Za-z0-9@_./+:-]{1,512}$/.test(target.name) ||
          (scanner === 'images' && !imageNames.includes(target.image))
        ) {
          throw new Error(
            'Invalid remediation target; exact advisory and package/module names are required',
          );
        }
        const canonical = { id: target.id, name: target.name };
        if (scanner === 'images') canonical.image = target.image;
        const key = JSON.stringify(canonical);
        if (seen.has(key)) throw new Error('Duplicate remediation target');
        seen.add(key);
        count++;
        return canonical;
      });
    }
    if (!count || count > 100)
      throw new Error('Specify between 1 and 100 remediation targets');
  }
  return {
    required,
    targets,
    refs: { pr: pr.number, base: pr.base.sha, head: pr.head.sha },
  };
}

export function evaluateTargets(targets, comparisons) {
  const results = [];
  for (const [scanner, items] of Object.entries(targets)) {
    if (!items.length) continue;
    const comparison = comparisons[scanner];
    if (
      !comparison ||
      !['added', 'changedExposure', 'inherited', 'removed'].every((category) =>
        Array.isArray(comparison[category]),
      )
    )
      throw new Error(`Missing verified comparison: ${scanner}`);
    for (const target of items) {
      const matches = (finding) =>
        finding.id === target.id &&
        finding.name === target.name &&
        (scanner !== 'images' || finding.image === target.image);
      const inBase = [
        ...comparison.removed,
        ...comparison.inherited,
        ...comparison.changedExposure,
      ].some(matches);
      const inHead = [
        ...comparison.added,
        ...comparison.inherited,
        ...comparison.changedExposure,
      ].some(matches);
      const status = !inBase
        ? 'not-in-base'
        : inHead
          ? 'still-present'
          : 'resolved';
      results.push({ scanner, ...target, status });
    }
  }
  return results;
}

export function renderTargets(results) {
  const lines = ['## Explicit remediation targets', ''];
  for (const target of results) {
    const name = target.name.replace(/[^@a-zA-Z0-9/_.-]/g, '?');
    lines.push(
      `- ${target.status}: ${target.scanner}/${target.image ?? name} · ${target.id} · ${name}`,
    );
  }
  lines.push(
    '',
    'Resolution requires presence on the base and complete absence from the PR report.',
    'Go targets include module/package findings, not just reachable symbols. Image targets include lower severities and unfixed findings.',
    'Existing regression and secret gates remain unchanged. Metadata edits require a fresh CI run.',
  );
  return `${lines.join('\n')}\n`;
}

export async function checkRemediation(
  prPath,
  eventPath,
  output,
  run = execute,
) {
  output = resolve(output);
  await mkdir(output);
  try {
    const intent = parseIntent(
      JSON.parse(await readFile(prPath, 'utf8')),
      JSON.parse(await readFile(eventPath, 'utf8')),
    );
    intent.checkedAt = new Date().toISOString();
    intent.sourceRun = process.env.GITHUB_RUN_ID;
    await writeFile(`${output}/intent.json`, JSON.stringify(intent, null, 2));
    const comparisons = {};
    const scanResults = JSON.parse(process.env.SECURITY_SCAN_RESULTS ?? 'null');
    for (const [scanner, items] of Object.entries(intent.targets)) {
      if (!items.length) continue;
      if (scanResults?.[scanners[scanner].artifact]?.result !== 'success') {
        throw new Error(`Incomplete snapshot job: ${scanner}`);
      }
      const directory = `${output}/snapshots/${scanner}`;
      await mkdir(directory, { recursive: true });
      await run(
        'gh',
        [
          'run',
          'download',
          process.env.GITHUB_RUN_ID,
          '--repo',
          process.env.GITHUB_REPOSITORY,
          '--name',
          scanners[scanner].artifact,
          '--dir',
          directory,
        ],
        { timeout: 120_000 },
      );
      comparisons[scanner] = await scanners[scanner].replay(directory);
    }
    const results = evaluateTargets(intent.targets, comparisons);
    await writeFile(`${output}/results.json`, JSON.stringify(results, null, 2));
    const summary = intent.required
      ? renderTargets(results)
      : '## Remediation targets\n\nNot a declared remediation PR. Existing security gates still apply.\n';
    await writeFile(`${output}/summary.md`, summary);
    if (process.env.GITHUB_STEP_SUMMARY)
      await appendFile(process.env.GITHUB_STEP_SUMMARY, summary);
    console.log(summary);
    if (results.some((target) => target.status !== 'resolved')) {
      throw new Error(
        'One or more remediation targets remain unresolved or lack base evidence',
      );
    }
  } catch (error) {
    await writeFile(`${output}/error.txt`, String(error.message));
    throw error;
  }
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  await checkRemediation(...process.argv.slice(2));
}
