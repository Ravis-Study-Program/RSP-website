import type {
  JobApplication,
  JobPositionProgress,
  JobProgressLink,
  JobProgressNode,
} from '@/api/generated/models';
import { isInterviewStage, jobStageKey } from '@/jobStages';

const screen = 'Recruiter screen (confirm info)';
const behavioural = 'Recruiter screen (behavioural)';
const assessment = 'Online assessment';
const leetcode = 'Technical (LeetCode style)';
const systemDesign = 'Technical (system design)';
const oopDesign = 'Technical (OOP design)';
const route = [screen, leetcode, 'Team matching'];

type Path = {
  status: JobApplication['status'];
  stages: readonly string[];
};

// Each row is a group of fictional applicants who reported the same progress.
const cohorts = [
  [6, [], 'active'],
  [7, [screen], 'active'],
  [5, [screen, leetcode], 'active'],
  [3, route, 'active'],
  [2, route, 'offer'],
  [2, [screen], 'inactive'],
  [3, [assessment, oopDesign], 'active'],
  [2, [behavioural, systemDesign], 'active'],
  [2, [assessment], 'inactive'],
] as const;

function fakeReports(positionId: string): Path[] {
  if (positionId !== 'position-1' && positionId !== 'position-2') return [];
  const groups = positionId === 'position-1' ? cohorts : cohorts.slice(5);
  return groups.flatMap(([count, stages, status]) =>
    Array.from({ length: count }, () => ({ status, stages })),
  );
}

export function demoPositionProgress(
  positionId: string,
  applications: Iterable<JobApplication[]>,
): JobPositionProgress {
  const reports = fakeReports(positionId);
  for (const own of applications) {
    reports.push(
      ...own
        .filter((a) => a.shareProgress && a.position.id === positionId)
        .map((a) => ({
          status: a.status,
          stages: a.stages.map((s) => s.title),
        })),
    );
  }
  const nodes = new Map<string, JobProgressNode>();
  const transitions = new Map<string, JobProgressLink>();
  const titles = new Map<string, string>();
  const frequency = new Map<string, number>();
  for (const report of reports) {
    const stages = report.stages.filter(isInterviewStage).map((title) => {
      const key = jobStageKey(title);
      if (!titles.has(key)) titles.set(key, title.trim().replace(/\s+/g, ' '));
      return key;
    });
    for (const key of new Set(stages)) {
      frequency.set(key, (frequency.get(key) ?? 0) + 1);
    }
    const reached = ['Applied', ...stages.map((key) => titles.get(key)!)];
    if (report.status === 'inactive') reached.push('Rejected');
    if (report.status === 'offer') reached.push('Offer');
    let source: number | undefined;
    reached.forEach((title, round) => {
      const key = `${round}:${title}`;
      const node = nodes.get(key) ?? {
        id: nodes.size,
        round,
        title,
        reached: 0,
        current: 0,
      };
      node.reached++;
      if (round === reached.length - 1) node.current++;
      nodes.set(key, node);
      if (source !== undefined) {
        const linkKey = `${source}:${node.id}`;
        const link = transitions.get(linkKey) ?? {
          source,
          target: node.id,
          count: 0,
        };
        link.count++;
        transitions.set(linkKey, link);
      }
      source = node.id;
    });
  }
  return {
    total: reports.length,
    stages: [...nodes.values()],
    transitions: [...transitions.values()],
    suggestions: [...frequency.keys()]
      .sort(
        (a, b) => frequency.get(b)! - frequency.get(a)! || a.localeCompare(b),
      )
      .map((key) => titles.get(key)!),
  };
}
