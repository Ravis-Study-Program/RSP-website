import { compareFindings } from './compare-security-findings.mjs';

export const trivyVersion = '0.70.0';
export const imageNames = ['api', 'auth', 'worker', 'web', 'migrations', 'ops'];

export function normalizeImageAudit(report, image, expectedID) {
  if (
    report.SchemaVersion !== 2 ||
    report.Trivy?.Version !== trivyVersion ||
    report.ArtifactType !== 'container_image' ||
    !/^sha256:[a-f0-9]{64}$/.test(report.Metadata?.ImageID ?? '') ||
    (expectedID && report.Metadata.ImageID !== expectedID) ||
    !report.Metadata?.OS?.Family ||
    !report.Metadata.OS.Name ||
    report.Metadata.OS.Eosl ||
    !Array.isArray(report.Results)
  ) {
    throw new Error(
      `Incomplete, unsupported, or mismatched image scan: ${image}`,
    );
  }
  if (
    !report.Results.some(
      (result) =>
        result.Class === 'os-pkgs' &&
        Array.isArray(result.Packages) &&
        result.Packages.length,
    )
  ) {
    throw new Error(`No OS package inventory for ${image}`);
  }
  const findings = new Map();
  for (const result of report.Results) {
    if (
      !['os-pkgs', 'lang-pkgs'].includes(result.Class) ||
      result.Type === 'jar' ||
      typeof result.Type !== 'string' ||
      !result.Type ||
      typeof result.Target !== 'string' ||
      !result.Target ||
      (result.Vulnerabilities !== undefined &&
        !Array.isArray(result.Vulnerabilities))
    ) {
      throw new Error(`Invalid Trivy result for ${image}`);
    }
    for (const vuln of result.Vulnerabilities ?? []) {
      if (
        !/^[A-Za-z0-9_.:-]+$/.test(vuln.VulnerabilityID ?? '') ||
        typeof vuln.PkgName !== 'string' ||
        !vuln.PkgName ||
        typeof vuln.InstalledVersion !== 'string' ||
        !vuln.InstalledVersion ||
        (vuln.FixedVersion !== undefined &&
          typeof vuln.FixedVersion !== 'string') ||
        !['UNKNOWN', 'LOW', 'MEDIUM', 'HIGH', 'CRITICAL'].includes(
          vuln.Severity,
        )
      ) {
        throw new Error(`Invalid Trivy vulnerability for ${image}`);
      }
      // OS targets contain image tags and must not enter the stable identity.
      const location =
        result.Class === 'os-pkgs' ? report.Metadata.OS.Family : result.Target;
      const key = JSON.stringify([
        image,
        result.Class,
        result.Type,
        location,
        vuln.PkgName,
        vuln.VulnerabilityID,
      ]);
      if (!findings.has(key)) {
        findings.set(key, {
          id: vuln.VulnerabilityID,
          name: vuln.PkgName,
          image,
          location,
          exposures: [],
        });
      }
      const exposure = JSON.stringify({
        installed: vuln.InstalledVersion,
        fixed: vuln.FixedVersion ?? '',
        severity: vuln.Severity,
        distro: report.Metadata.OS.Name,
        path: vuln.PkgPath ?? '',
      });
      const exposures = findings.get(key).exposures;
      if (!exposures.includes(exposure)) exposures.push(exposure);
    }
  }
  for (const finding of findings.values()) finding.exposures.sort();
  return findings;
}

export function compareImageAudits(base, head) {
  const normalize = (reports) => {
    if (!reports || Object.keys(reports).length !== imageNames.length) {
      throw new Error('Incomplete production image set');
    }
    const findings = new Map();
    for (const image of imageNames) {
      if (!reports[image])
        throw new Error(`Missing production image: ${image}`);
      for (const [key, value] of normalizeImageAudit(reports[image], image)) {
        findings.set(key, value);
      }
    }
    return findings;
  };
  return compareFindings(normalize(base), normalize(head));
}

export function renderImageComparison(result) {
  const lines = ['## Production image comparison (shadow mode)', ''];
  for (const [category, findings] of Object.entries(result)) {
    lines.push(`### ${category}: ${findings.length}`, '');
    for (const finding of findings) {
      const name = finding.name.replace(/[^@a-zA-Z0-9/_.-]/g, '?');
      lines.push(`- ${finding.image} · ${finding.id} · ${name}`);
    }
    lines.push('');
  }
  lines.push(
    'Both image sets use Trivy 0.70.0 and one frozen vulnerability database.',
    'All severities and unfixed findings remain visible so a severity downgrade or loss of a fix cannot masquerade as remediation.',
    'Changed exposure includes affected versions, severity, fix versions, package paths, or OS versions.',
    'Floating base tags and OS repositories can change between builds. This pilot cannot attribute every difference to the PR.',
    'Existing security gates remain strict; the separate remediation check verifies explicit targets.',
  );
  return `${lines.join('\n')}\n`;
}
