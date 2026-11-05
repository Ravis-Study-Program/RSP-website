import { execFile } from 'node:child_process';
import { createHash } from 'node:crypto';
import {
  appendFile,
  mkdir,
  mkdtemp,
  readFile,
  rm,
  writeFile,
} from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, join } from 'node:path';
import { pathToFileURL } from 'node:url';
import { promisify } from 'node:util';
import { gunzipSync } from 'node:zlib';
import {
  compareGoAudits,
  renderGoComparison,
  scannerVersion,
} from './compare-go-audits.mjs';

const execute = promisify(execFile);
const registry = 'https://vuln.go.dev';
const digest = (text) => createHash('sha256').update(text).digest('hex');

export function selectEntries(index, modules) {
  if (!Array.isArray(index) || !index.length)
    throw new Error('Empty Go database index');
  const selected = new Map();
  for (const entry of index) {
    if (typeof entry.path !== 'string' || !Array.isArray(entry.vulns)) {
      throw new Error('Invalid Go database module index');
    }
    for (const vuln of entry.vulns) {
      if (
        !/^GO-\d{4}-\d+$/.test(vuln.id) ||
        !Number.isFinite(Date.parse(vuln.modified))
      ) {
        throw new Error('Invalid Go database advisory index');
      }
      if (!modules.has(entry.path)) continue;
      if (selected.has(vuln.id) && selected.get(vuln.id) !== vuln.modified) {
        throw new Error('Conflicting Go advisory timestamps');
      }
      selected.set(vuln.id, vuln.modified);
    }
  }
  return selected;
}

export function validateEntry(entry, id, modified) {
  if (
    entry.id !== id ||
    entry.modified !== modified ||
    !Array.isArray(entry.affected)
  ) {
    throw new Error(`Go advisory changed or is incomplete: ${id}`);
  }
}

async function fetchDatabaseFile(path) {
  const response = await fetch(`${registry}/${path}.json.gz`, {
    signal: AbortSignal.timeout(60_000),
  });
  if (!response.ok)
    throw new Error(`Go database ${path}: HTTP ${response.status}`);
  return gunzipSync(Buffer.from(await response.arrayBuffer())).toString('utf8');
}

export async function replayGoSnapshot(directory) {
  const manifest = JSON.parse(
    await readFile(`${directory}/manifest.json`, 'utf8'),
  );
  if (
    ![
      'base.json',
      'head.json',
      'database/index/db.json',
      'database/index/modules.json',
    ].every((path) => manifest.files?.[path])
  ) {
    throw new Error('Incomplete Go snapshot manifest');
  }
  for (const [path, hash] of Object.entries(manifest.files)) {
    if (!/^(?:database\/(?:index|ID)\/)?[A-Za-z0-9_.-]+\.json$/.test(path)) {
      throw new Error('Invalid Go snapshot file path');
    }
    if (digest(await readFile(`${directory}/${path}`)) !== hash) {
      throw new Error(`Go snapshot integrity check failed: ${path}`);
    }
  }
  return compareGoAudits(
    await readFile(`${directory}/base.json`, 'utf8'),
    await readFile(`${directory}/head.json`, 'utf8'),
  );
}

export async function runGoShadow(base, head, output) {
  output = resolve(output);
  // Refuse stale records or a previous manifest after an incomplete retry.
  await mkdir(output);
  await mkdir(`${output}/database/index`, { recursive: true });
  await mkdir(`${output}/database/ID`, { recursive: true });
  const env = {
    ...process.env,
    GOTOOLCHAIN: process.env.GO_VERSION
      ? `go${process.env.GO_VERSION}`
      : 'auto',
    GOWORK: 'off',
    GOFLAGS: '-mod=readonly',
    GOOS: 'linux',
    GOARCH: 'amd64',
  };
  const files = {};
  async function save(path, text) {
    await writeFile(`${output}/${path}`, text);
    files[path] = digest(text);
  }
  async function command(binary, args, cwd, environment = env) {
    return execute(binary, args, {
      cwd,
      env: environment,
      timeout: 600_000,
      maxBuffer: 64 * 1024 * 1024,
    });
  }
  const { stdout: goVersion } = await command('go', ['env', 'GOVERSION'], head);
  env.GOTOOLCHAIN = goVersion.trim();
  const modules = new Set(['stdlib', 'toolchain']);
  const inputs = {};
  for (const [side, directory] of [
    ['base', base],
    ['head', head],
  ]) {
    const { stdout } = await command(
      'go',
      [
        'list',
        '-m',
        '-f',
        '{{.Path}}{{if .Replace}} {{.Replace.Path}}{{end}}',
        'all',
      ],
      directory,
    );
    const sideModules = [...new Set(stdout.trim().split(/\s+/))].sort();
    for (const path of sideModules) modules.add(path);
    await save(`${side}-modules.json`, JSON.stringify(sideModules));
    inputs[side] = {
      goModSha256: digest(await readFile(resolve(directory, 'go.mod'))),
      goSumSha256: digest(await readFile(resolve(directory, 'go.sum'))),
    };
  }
  const indexText = await fetchDatabaseFile('index/modules');
  const index = JSON.parse(indexText);
  const selected = selectEntries(index, modules);
  await save('database/index/modules.json', indexText);
  const modified = index
    .flatMap((entry) => entry.vulns.map((vuln) => vuln.modified))
    .sort()
    .at(-1);
  await save('database/index/db.json', JSON.stringify({ modified }));
  const pending = [...selected.entries()];
  await Promise.all(
    Array.from({ length: 8 }, async () => {
      while (pending.length) {
        const [id, date] = pending.pop();
        const text = await fetchDatabaseFile(`ID/${id}`);
        validateEntry(JSON.parse(text), id, date);
        await save(`database/ID/${id}.json`, text);
      }
    }),
  );

  const toolDirectory = await mkdtemp(join(tmpdir(), 'rsp-govulncheck-'));
  const database = pathToFileURL(`${output}/database`).href;
  try {
    await command(
      'go',
      ['install', `golang.org/x/vuln/cmd/govulncheck@${scannerVersion}`],
      head,
      {
        ...env,
        GOBIN: toolDirectory,
        GOFLAGS: '',
      },
    );
    for (const [side, directory] of [
      ['base', base],
      ['head', head],
    ]) {
      try {
        const { stdout, stderr } = await command(
          join(toolDirectory, 'govulncheck'),
          ['-json', '-db', database, './...'],
          directory,
        );
        await save(`${side}.json`, stdout);
        await writeFile(`${output}/${side}-stderr.txt`, stderr);
      } catch (error) {
        await writeFile(
          `${output}/${side}-stderr.txt`,
          error.stderr ?? String(error),
        );
        await writeFile(
          `${output}/${side}-incomplete.json`,
          error.stdout ?? '',
        );
        throw error;
      }
    }
  } finally {
    await rm(toolDirectory, { recursive: true, force: true });
  }
  const manifest = {
    registry,
    scannerVersion,
    goVersion: goVersion.trim(),
    database,
    inputs,
    files,
  };
  await writeFile(`${output}/manifest.json`, JSON.stringify(manifest, null, 2));
  const result = await replayGoSnapshot(output);
  await writeFile(`${output}/comparison.json`, JSON.stringify(result, null, 2));
  const summary = renderGoComparison(result);
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
    console.log(renderGoComparison(await replayGoSnapshot(process.argv[3])));
  } else {
    await runGoShadow(...process.argv.slice(2));
  }
}
