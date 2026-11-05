import { compareFindings } from './compare-security-findings.mjs';

const severities = ['info', 'low', 'moderate', 'high', 'critical'];

export function normalizeAudit(report) {
  if (
    report.error ||
    !report.advisories ||
    typeof report.advisories !== 'object' ||
    Array.isArray(report.advisories) ||
    !report.metadata?.vulnerabilities
  ) {
    throw new Error('Incomplete pnpm audit report');
  }
  for (const severity of severities) {
    if (
      !Number.isInteger(report.metadata.vulnerabilities[severity]) ||
      report.metadata.vulnerabilities[severity] < 0
    ) {
      throw new Error(`Missing vulnerability count: ${severity}`);
    }
  }
  const findings = new Map();
  for (const advisory of Object.values(report.advisories)) {
    const { github_advisory_id: id, module_name: name, severity } = advisory;
    if (
      !/^GHSA-[\w-]+$/.test(id ?? '') ||
      typeof name !== 'string' ||
      !name ||
      !severities.includes(severity) ||
      report.metadata.vulnerabilities[severity] === 0 ||
      !Array.isArray(advisory.findings) ||
      !advisory.findings.length ||
      typeof advisory.vulnerable_versions !== 'string' ||
      typeof advisory.updated !== 'string'
    ) {
      throw new Error('Unrecognized pnpm advisory schema');
    }
    const exposures = new Set();
    for (const finding of advisory.findings) {
      if (
        typeof finding.version !== 'string' ||
        !Array.isArray(finding.paths) ||
        !finding.paths.length ||
        finding.paths.some((path) => typeof path !== 'string' || !path)
      ) {
        throw new Error(`Missing exposure data for ${id}`);
      }
      for (const path of finding.paths) {
        exposures.add(JSON.stringify([path, finding.version]));
      }
    }
    const key = JSON.stringify([id, name]);
    if (findings.has(key))
      throw new Error(`Duplicate advisory/package: ${key}`);
    findings.set(key, {
      id,
      name,
      severity,
      updated: advisory.updated,
      vulnerableVersions: advisory.vulnerable_versions,
      exposures: [...exposures].sort(),
    });
  }
  if (
    !findings.size &&
    Object.values(report.metadata.vulnerabilities).some((count) => count !== 0)
  ) {
    throw new Error('Vulnerability counts exist without advisories');
  }
  return findings;
}

export function compareAudits(baseReport, headReport) {
  return compareFindings(
    normalizeAudit(baseReport),
    normalizeAudit(headReport),
    ['severity', 'updated', 'vulnerableVersions'],
  );
}

export function renderComparison(result) {
  const lines = ['## pnpm baseline comparison (shadow mode)', ''];
  for (const [category, findings] of Object.entries(result)) {
    lines.push(`### ${category}: ${findings.length}`, '');
    for (const finding of findings) {
      // Keep registry metadata out of Markdown syntax.
      const name = finding.name.replace(/[^@a-zA-Z0-9/_.-]/g, '?');
      lines.push(`- ${finding.id} · ${name} · ${finding.severity}`);
    }
    lines.push('');
  }
  lines.push(
    'This report does not relax existing security gates.',
    'Changed exposure means a new dependency path or affected version, not proven greater exploitability.',
    'Partial exposure removals remain inherited; complete advisory/package removals appear under removed.',
    'Both dependency trees use one frozen npm audit response. The artifact retains its request, response, and SHA-256 manifest.',
    'This remains a shadow report, not a regression gate. The separate remediation check verifies explicit targets.',
  );
  return `${lines.join('\n')}\n`;
}
