// Development/demo only: production uses the Go API and isolated Jobs tables.
import seed from '@/data/jobPositions.json';
import type {
  JobApplication,
  JobApplicationStatusInput,
  JobCompany,
  JobPosition,
  JobPositionInput,
  JobStage,
  JobStageInput,
} from '@/api/generated/models';

let companies: JobCompany[] = [
  ...new Map(
    seed.map((v) => [
      v.companyName,
      {
        id: `company-${v.companyName}`,
        name: v.companyName,
        category: v.category as JobCompany['category'],
      },
    ]),
  ).values(),
];
let positions: JobPosition[] = seed.map((v, i) => ({
  id: `position-${i + 1}`,
  company: companies.find((c) => c.name === v.companyName)!,
  title: v.title,
  jobType: v.jobType as JobPosition['jobType'],
  eligibility: v.eligibility,
  applicationUrl: v.applicationUrl,
  locations: v.locations,
  notes: v.notes,
  status: 'open',
}));
const applications = new Map<string, JobApplication[]>();
const aliases = new Map<string, string>();
const canonical = (id: string): string =>
  aliases.has(id) ? canonical(aliases.get(id)!) : id;
const page = <T>(items: T[]) => ({
  items: structuredClone(items),
  totalCount: items.length,
  pageInfo: { hasMore: false, nextCursor: null, previousCursor: null },
});

export function demoJobRequest(
  path: string,
  method: string,
  body: unknown,
  userId?: string,
): unknown {
  const parts = path.split('?')[0].split('/').filter(Boolean);
  if (method === 'GET' && parts[0] === 'jobs') {
    if (parts[1] === 'companies') return { items: structuredClone(companies) };
    if (parts.length === 2) return page(positions);
    return structuredClone(positions.find((p) => p.id === canonical(parts[2])));
  }
  if (!userId) throw new Error('Sign in to continue.');
  const own = applications.get(userId) ?? [];
  applications.set(userId, own);
  if (parts[0] === 'me') {
    if (parts.length === 2 && method === 'GET') return page(own);
    if (parts.length === 2 && method === 'POST') {
      const input = body as { positionId: string; date: string };
      const position = positions.find(
        (p) => p.id === canonical(input.positionId),
      )!;
      let app = own.find((a) => a.position.id === position.id);
      if (!app) {
        app = {
          id: crypto.randomUUID(),
          status: 'active',
          position,
          stages: [
            {
              id: crypto.randomUUID(),
              title: 'Applied',
              date: input.date,
              notes: '',
              initial: true,
            },
          ],
        };
        own.push(app);
      }
      return { id: app.id };
    }
    const application = own.find((a) => a.id === parts[2]);
    if (!application) throw new Error('Application not found.');
    if (parts.length === 3 && method === 'PATCH') {
      application.status = (body as JobApplicationStatusInput).status;
      return;
    }
    if (parts.length === 3 && method === 'DELETE') {
      applications.set(
        userId,
        own.filter((a) => a !== application),
      );
      return;
    }
    if (parts[3] === 'stages') {
      const stage = application.stages.find((s) => s.id === parts[4]);
      if (method === 'DELETE') {
        if (stage?.initial) throw new Error('Applied must stay first.');
        application.stages = application.stages.filter((s) => s !== stage);
        return;
      }
      const input = body as JobStageInput;
      const saved: JobStage = {
        ...input,
        id: stage?.id ?? crypto.randomUUID(),
        initial: stage?.initial ?? false,
      };
      if (stage) application.stages[application.stages.indexOf(stage)] = saved;
      else application.stages.push(saved);
      application.stages.sort(
        (a, b) =>
          Number(b.initial) - Number(a.initial) || a.date.localeCompare(b.date),
      );
      return structuredClone(saved);
    }
  }
  if (parts[1] === 'companies') {
    const input = body as Pick<JobCompany, 'name' | 'category'>;
    companies = companies.map((c) =>
      c.id === parts[2] ? { ...c, ...input } : c,
    );
    positions.forEach((p) => {
      p.company = companies.find((c) => c.id === p.company.id)!;
    });
    return structuredClone(companies.find((c) => c.id === parts[2]));
  }
  if (parts[3] === 'merge') {
    const source = positions.find((p) => p.id === parts[2])!;
    const target = positions.find(
      (p) => p.id === (body as { targetId: string }).targetId,
    )!;
    target.locations = [...new Set([...target.locations, ...source.locations])];
    for (const [owner, records] of applications) {
      const from = records.find((a) => a.position.id === source.id);
      const to = records.find((a) => a.position.id === target.id);
      if (from && to) {
        to.stages.push(
          ...from.stages.map((s) =>
            s.initial
              ? { ...s, initial: false, title: 'Applied (merged listing)' }
              : s,
          ),
        );
        to.stages.sort(
          (a, b) =>
            Number(b.initial) - Number(a.initial) ||
            a.date.localeCompare(b.date),
        );
        applications.set(
          owner,
          records.filter((a) => a !== from),
        );
      } else if (from) from.position = target;
    }
    aliases.set(source.id, target.id);
    positions = positions.filter((p) => p !== source);
    return structuredClone(target);
  }
  const input = body as JobPositionInput;
  let company = companies.find(
    (c) => c.name.toLowerCase() === input.companyName.trim().toLowerCase(),
  );
  if (!company) {
    company = {
      id: crypto.randomUUID(),
      name: input.companyName,
      category: input.category,
    };
    companies.push(company);
  }
  const saved: JobPosition = {
    id: parts[2] ?? crypto.randomUUID(),
    company,
    title: input.title,
    jobType: input.jobType,
    eligibility: input.eligibility,
    applicationUrl: input.applicationUrl,
    locations: input.locations,
    notes: input.notes,
    status: input.status,
  };
  const existing = positions.find((p) => p.id === parts[2]);
  if (existing) Object.assign(existing, saved);
  else positions.push(saved);
  return structuredClone(saved);
}
