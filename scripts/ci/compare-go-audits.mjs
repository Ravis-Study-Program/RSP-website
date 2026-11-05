import { compareFindings } from './compare-security-findings.mjs';

export const scannerVersion = 'v1.6.0';

// govulncheck emits consecutive JSON objects, not a JSON array or JSONL.
export function parseMessages(text) {
  const messages = [];
  let depth = 0;
  let start = 0;
  let quoted = false;
  let escaped = false;
  for (let i = 0; i < text.length; i++) {
    const character = text[i];
    if (!depth) {
      if (/\s/.test(character)) continue;
      if (character !== '{') throw new Error('Invalid govulncheck stream');
      start = i;
    }
    if (quoted) {
      if (escaped) escaped = false;
      else if (character === '\\') escaped = true;
      else if (character === '"') quoted = false;
    } else if (character === '"') quoted = true;
    else if (character === '{') depth++;
    else if (character === '}' && --depth === 0) {
      messages.push(JSON.parse(text.slice(start, i + 1)));
    }
  }
  if (depth || !messages.length)
    throw new Error('Incomplete govulncheck stream');
  return messages;
}

export function normalizeGoAudit(text) {
  const messages = parseMessages(text);
  const configs = messages.filter((message) => message.config);
  const sboms = messages.filter((message) => message.SBOM);
  if (configs.length !== 1 || sboms.length !== 1) {
    throw new Error('Missing or duplicate govulncheck config/SBOM');
  }
  const config = configs[0].config;
  if (
    config.protocol_version !== 'v1.0.0' ||
    config.scanner_name !== 'govulncheck' ||
    config.scanner_version !== scannerVersion ||
    config.scan_level !== 'symbol' ||
    config.scan_mode !== 'source' ||
    !config.go_version ||
    !config.db ||
    !Array.isArray(sboms[0].SBOM.modules) ||
    !sboms[0].SBOM.modules.length
  ) {
    throw new Error('Unsupported or incomplete govulncheck config/SBOM');
  }
  const entries = new Map();
  for (const { osv } of messages) {
    if (osv) {
      if (!Number.isFinite(Date.parse(osv.modified))) {
        throw new Error('Invalid Go advisory timestamp');
      }
      entries.set(osv.id, osv);
    }
  }
  const findings = new Map();
  for (const message of messages) {
    if (
      Object.keys(message).length !== 1 ||
      !['config', 'SBOM', 'progress', 'osv', 'finding'].some(
        (key) => key in message,
      )
    ) {
      throw new Error('Unknown govulncheck message');
    }
    if (!message.finding) continue;
    const finding = message.finding;
    const frame = finding.trace?.[0];
    if (
      !/^GO-\d{4}-\d+$/.test(finding.osv ?? '') ||
      !entries.has(finding.osv) ||
      typeof frame?.module !== 'string' ||
      !frame.module ||
      ['version', 'package', 'function', 'receiver'].some(
        (field) =>
          frame[field] !== undefined && typeof frame[field] !== 'string',
      ) ||
      (frame.function && !frame.package)
    ) {
      throw new Error('Invalid govulncheck finding');
    }
    const key = JSON.stringify([finding.osv, frame.module]);
    if (!findings.has(key)) {
      findings.set(key, {
        id: finding.osv,
        name: frame.module,
        modified: entries.get(finding.osv).modified,
        exposures: [],
      });
    }
    // Ignore file/line and caller traces: a harmless refactor must not look like
    // a new vulnerability. Preserve module, package and symbol reachability.
    const exposure = JSON.stringify({
      version: frame.version ?? '',
      level: frame.function ? 'symbol' : frame.package ? 'package' : 'module',
      package: frame.package ?? '',
      function: frame.function ?? '',
      receiver: frame.receiver ?? '',
    });
    const exposures = findings.get(key).exposures;
    if (!exposures.includes(exposure)) exposures.push(exposure);
  }
  for (const finding of findings.values()) finding.exposures.sort();
  return { config, findings };
}

export function compareGoAudits(baseText, headText) {
  const base = normalizeGoAudit(baseText);
  const head = normalizeGoAudit(headText);
  for (const field of [
    'go_version',
    'scanner_version',
    'db',
    'db_last_modified',
  ]) {
    if (base.config[field] !== head.config[field]) {
      throw new Error(`Go scan configuration differs: ${field}`);
    }
  }
  return compareFindings(base.findings, head.findings, ['modified']);
}

export function renderGoComparison(result) {
  const lines = ['## Go baseline comparison (shadow mode)', ''];
  for (const [category, findings] of Object.entries(result)) {
    lines.push(`### ${category}: ${findings.length}`, '');
    for (const finding of findings) {
      const reachable = finding.exposures.some(
        (exposure) => JSON.parse(exposure).level === 'symbol',
      );
      const name = finding.name.replace(/[^a-zA-Z0-9/_.-]/g, '?');
      lines.push(
        `- ${finding.id} · ${name} · ${reachable ? 'reachable symbol' : 'not proven reachable'}`,
      );
    }
    lines.push('');
  }
  lines.push(
    'Both scans use one pinned scanner, Go toolchain, and local vulnerability database snapshot.',
    'Module/package findings remain visible but do not prove a reachable vulnerable symbol.',
    'Changed exposure includes new affected versions, packages, or symbols; it is not a severity judgment.',
    'Existing full-tree and secret gates remain strict; the separate remediation check verifies explicit targets.',
  );
  return `${lines.join('\n')}\n`;
}
