import { execFile } from 'node:child_process';
import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { appendFile, mkdir, readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { promisify } from 'node:util';
import {
  compareImageAudits,
  imageNames,
  normalizeImageAudit,
  renderImageComparison,
  trivyVersion,
} from './compare-image-audits.mjs';

const execute = promisify(execFile);
const digest = (text) => createHash('sha256').update(text).digest('hex');
const databaseFile = 'cache/db/trivy.db';
const metadataFile = 'cache/db/metadata.json';

async function hashFile(path) {
  const hash = createHash('sha256');
  for await (const chunk of createReadStream(path)) hash.update(chunk);
  return hash.digest('hex');
}

export async function replayImageSnapshot(directory) {
  const manifest = JSON.parse(
    await readFile(`${directory}/manifest.json`, 'utf8'),
  );
  const required = [
    metadataFile,
    ...['base', 'head'].flatMap((side) =>
      imageNames.map((image) => `${side}-${image}.json`),
    ),
  ];
  if (
    manifest.trivyVersion !== trivyVersion ||
    manifest.coverage !== 'all-vulnerabilities' ||
    !/^[a-f0-9]{64}$/.test(manifest.databaseSha256 ?? '') ||
    !required.every((path) => manifest.files?.[path])
  ) {
    throw new Error('Incomplete image snapshot manifest');
  }
  for (const [path, hash] of Object.entries(manifest.files)) {
    if (!required.includes(path))
      throw new Error('Unknown image snapshot file');
    if (digest(await readFile(`${directory}/${path}`)) !== hash) {
      throw new Error(`Image snapshot integrity check failed: ${path}`);
    }
  }
  const reports = { base: {}, head: {} };
  for (const side of ['base', 'head']) {
    for (const image of imageNames) {
      const report = JSON.parse(
        await readFile(`${directory}/${side}-${image}.json`, 'utf8'),
      );
      const id = manifest.images?.[side]?.[image];
      if (!id) throw new Error(`Missing image identity: ${side}/${image}`);
      normalizeImageAudit(report, image, id);
      reports[side][image] = report;
    }
  }
  return compareImageAudits(reports.base, reports.head);
}

// CI already built the PR's rsp-*:ci images. Reuse those instead of rebuilding.
export async function runImageShadow(base, output, run = execute) {
  output = resolve(output);
  await mkdir(output);
  const env = Object.fromEntries(
    Object.entries(process.env).filter(([name]) => !name.startsWith('TRIVY_')),
  );
  env.DOCKER_DEFAULT_PLATFORM = 'linux/amd64';
  const binary = process.env.TRIVY_BINARY || 'trivy';
  const files = {};
  async function command(program, args, cwd = output, environment = env) {
    return run(program, args, {
      cwd,
      env: environment,
      timeout: 2_400_000,
      maxBuffer: 64 * 1024 * 1024,
    });
  }
  async function inspect(tag) {
    const images = {};
    for (const image of imageNames) {
      const { stdout } = await command('docker', [
        'image',
        'inspect',
        `rsp-${image}:${tag}`,
      ]);
      const [info] = JSON.parse(stdout);
      if (
        !/^sha256:[a-f0-9]{64}$/.test(info?.Id ?? '') ||
        info.Os !== 'linux' ||
        info.Architecture !== 'amd64'
      ) {
        throw new Error(`Missing or unsupported image: ${image}/${tag}`);
      }
      images[image] = info.Id;
    }
    return images;
  }
  try {
    const { stdout } = await command(binary, ['--version', '--format', 'json']);
    if (JSON.parse(stdout).Version !== trivyVersion)
      throw new Error('Unexpected Trivy version');
    const images = { head: await inspect('ci') };
    const buildScript = fileURLToPath(
      new URL('./containers.sh', import.meta.url),
    );
    await command('bash', [buildScript, 'build'], resolve(base), {
      ...env,
      IMAGE_TAG: 'security-base',
    });
    images.base = await inspect('security-base');

    const cache = `${output}/cache`;
    await command(binary, [
      'image',
      '--cache-dir',
      cache,
      '--download-db-only',
      '--skip-version-check',
      '--no-progress',
    ]);
    const databaseSha256 = await hashFile(`${output}/${databaseFile}`);
    files[metadataFile] = digest(await readFile(`${output}/${metadataFile}`));
    const metadata = JSON.parse(
      await readFile(`${output}/cache/db/metadata.json`, 'utf8'),
    );
    if (metadata.Version !== 2)
      throw new Error('Unsupported Trivy database schema');
    for (const side of ['base', 'head']) {
      for (const image of imageNames) {
        const path = `${side}-${image}.json`;
        const { stderr } = await command(binary, [
          'image',
          '--cache-dir',
          cache,
          '--skip-db-update',
          '--skip-java-db-update',
          '--offline-scan',
          '--image-src',
          'docker',
          '--scanners',
          'vuln',
          '--severity',
          'UNKNOWN,LOW,MEDIUM,HIGH,CRITICAL',
          '--list-all-pkgs',
          '--format',
          'json',
          '--exit-code',
          '0',
          '--skip-version-check',
          '--no-progress',
          '--output',
          `${output}/${path}`,
          images[side][image],
        ]);
        await writeFile(`${output}/${side}-${image}-stderr.txt`, stderr);
        const text = await readFile(`${output}/${path}`, 'utf8');
        normalizeImageAudit(JSON.parse(text), image, images[side][image]);
        files[path] = digest(text);
      }
    }
    if (
      (await hashFile(`${output}/${databaseFile}`)) !== databaseSha256 ||
      digest(await readFile(`${output}/${metadataFile}`)) !==
        files[metadataFile]
    ) {
      throw new Error('Trivy database changed during comparison');
    }
    const manifest = {
      trivyVersion,
      coverage: 'all-vulnerabilities',
      databaseSha256,
      images,
      files,
    };
    await writeFile(
      `${output}/manifest.json`,
      JSON.stringify(manifest, null, 2),
    );
    const result = await replayImageSnapshot(output);
    await writeFile(
      `${output}/comparison.json`,
      JSON.stringify(result, null, 2),
    );
    const summary = renderImageComparison(result);
    await writeFile(`${output}/summary.md`, summary);
    if (process.env.GITHUB_STEP_SUMMARY)
      await appendFile(process.env.GITHUB_STEP_SUMMARY, summary);
    console.log(summary);
  } catch (error) {
    await writeFile(
      `${output}/error.txt`,
      error.stderr || error.stack || String(error),
    );
    if (process.env.GITHUB_STEP_SUMMARY) {
      await appendFile(
        process.env.GITHUB_STEP_SUMMARY,
        '\n**Image shadow comparison failed. Coverage is incomplete; inspect the artifact and logs.**\n',
      );
    }
    throw error;
  }
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  if (process.argv[2] === '--replay') {
    console.log(
      renderImageComparison(await replayImageSnapshot(process.argv[3])),
    );
  } else {
    await runImageShadow(...process.argv.slice(2));
  }
}
