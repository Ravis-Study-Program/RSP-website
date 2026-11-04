import { readdir, readFile } from 'node:fs/promises';
import { pathToFileURL } from 'node:url';

export const marker = '<!-- rsp-security-tracker -->';

// Never copy scanner text, secret matches, code excerpts, or errors to issues.
export function imageVulnerabilities(report) {
  return (report.Results ?? []).flatMap((result) =>
    (result.Vulnerabilities ?? []).map((vulnerability) => ({
      id: vulnerability.VulnerabilityID,
      package: vulnerability.PkgName,
      installed: vulnerability.InstalledVersion,
      fixed: vulnerability.FixedVersion ?? '',
      severity: vulnerability.Severity,
    })),
  );
}

export function renderReport({ results, reports, runUrl, sha }) {
  const statuses = Object.entries(results).map(
    ([name, job]) => `| ${name} | ${job.result} |`,
  );
  const sections = reports.map(({ name, text }) => {
    // Escape HTML rather than trusting dependency metadata as issue Markdown.
    const excerpt = text
      .slice(0, 4500)
      .replace(/[&<>]/g, (character) => {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;' }[character];
      })
      .slice(0, 5000);
    return `<details><summary>${name}</summary>\n\n<pre>${excerpt}</pre>\n\n${text.length > 4500 ? 'Truncated; see workflow artifacts.' : ''}\n</details>`;
  });
  return `${marker}
## Latest security scan

[Workflow logs and full reports](${runUrl}) · Commit: \`${sha}\`

| Job | Result |
| --- | --- |
${statuses.join('\n')}

Failure can mean vulnerabilities **or scanner/build errors**. Missing reports mean
incomplete coverage, not a clean scan. Review every scanner in the linked jobs.
Trivy reports fixable HIGH/CRITICAL findings; pnpm's failure threshold is HIGH;
govulncheck reports reachable Go vulnerabilities. Image excerpts contain only
structured vulnerability fields. Go, pnpm, and secret details stay in CI logs/artifacts.

${reports.length ? sections.join('\n\n') : '**No scanner reports available. Inspect the workflow logs.**'}

## Triage

Maintainers: add a comment with each advisory's owner, fixed version or mitigation,
remediation PR, and deadline. Existing triage comments survive each refresh.
See [the remediation policy](../blob/main/SECURITY.md) for deadlines and exceptions.
Renovate PRs appear separately; findings without bot coverage need manual fixes.
`;
}

export async function collectReports(directory) {
  const reports = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = `${directory}/${entry.name}`;
    if (entry.isDirectory()) reports.push(...(await collectReports(path)));
    else if (/^(api|auth|worker|web|migrations|ops)\.json$/.test(entry.name)) {
      reports.push({
        name: entry.name,
        text: JSON.stringify(
          imageVulnerabilities(JSON.parse(await readFile(path, 'utf8'))),
          null,
          2,
        ),
      });
    }
  }
  return reports.sort((a, b) => a.name.localeCompare(b.name));
}

async function main() {
  const {
    GH_TOKEN,
    GITHUB_REPOSITORY,
    GITHUB_RUN_ID,
    GITHUB_SHA,
    SECURITY_JOB_RESULTS,
    GITHUB_API_URL = 'https://api.github.com',
    GITHUB_SERVER_URL = 'https://github.com',
  } = process.env;
  if (!GH_TOKEN || !GITHUB_REPOSITORY || !GITHUB_RUN_ID || !GITHUB_SHA) {
    throw new Error('Missing GitHub workflow environment');
  }
  async function api(path, method = 'GET', body) {
    const response = await fetch(
      `${GITHUB_API_URL}/repos/${GITHUB_REPOSITORY}${path}`,
      {
        method,
        headers: {
          Authorization: `Bearer ${GH_TOKEN}`,
          Accept: 'application/vnd.github+json',
          'Content-Type': 'application/json',
          'X-GitHub-Api-Version': '2022-11-28',
        },
        body: body === undefined ? undefined : JSON.stringify(body),
      },
    );
    if (!response.ok)
      throw new Error(`GitHub API ${method} ${path}: ${response.status}`);
    return response.json();
  }
  // Paginate directly: search indexing can lag and create duplicate trackers.
  let tracker;
  for (let page = 1; ; page++) {
    const issues = await api(`/issues?state=all&per_page=100&page=${page}`);
    tracker = issues.find(
      (issue) =>
        !issue.pull_request &&
        issue.body?.startsWith(marker) &&
        issue.user?.type === 'Bot',
    );
    if (tracker || issues.length < 100) break;
  }
  const body = renderReport({
    results: JSON.parse(SECURITY_JOB_RESULTS),
    reports: await collectReports('security-reports'),
    runUrl: `${GITHUB_SERVER_URL}/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}`,
    sha: GITHUB_SHA,
  });
  const payload = {
    title: 'Security remediation tracker',
    body,
    state: 'open',
  };
  const issue = tracker
    ? await api(`/issues/${tracker.number}`, 'PATCH', payload)
    : await api('/issues', 'POST', payload);
  console.log(`Security tracker: ${issue.html_url}`);
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  await main();
}
