import assert from 'node:assert/strict';
import { execFile } from 'node:child_process';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { promisify } from 'node:util';
import { normalizeGoAudit } from './compare-go-audits.mjs';
import { normalizeImageAudit } from './compare-image-audits.mjs';

const execute = promisify(execFile);
const options = { timeout: 300_000, maxBuffer: 32 * 1024 * 1024 };
const skip = process.env.SECURITY_CONTRACT_TESTS !== 'true';

test(
  'pinned govulncheck emits the source-stream contract',
  { skip },
  async (context) => {
    const cwd = await mkdtemp(join(tmpdir(), 'rsp-go-contract-'));
    context.after(() => rm(cwd, { recursive: true, force: true }));
    await writeFile(
      join(cwd, 'go.mod'),
      'module example.com/contract\n\ngo 1.26.0\n',
    );
    await writeFile(
      join(cwd, 'main.go'),
      'package main\nimport "fmt"\nfunc main() { fmt.Println("contract") }\n',
    );
    const { stdout } = await execute('govulncheck', ['-json', './...'], {
      ...options,
      cwd,
    });
    const report = normalizeGoAudit(stdout);
    assert.equal(report.config.scan_mode, 'source');
  },
);

test(
  'pinned Trivy emits the complete image-inventory contract',
  { skip },
  async () => {
    const image = 'alpine:3.24';
    await execute('docker', ['pull', image], options);
    const { stdout } = await execute(
      'trivy',
      [
        'image',
        '--scanners',
        'vuln',
        '--list-all-pkgs',
        '--format',
        'json',
        '--exit-code',
        '0',
        '--image-src',
        'docker',
        image,
      ],
      options,
    );
    normalizeImageAudit(JSON.parse(stdout), 'contract');
  },
);
