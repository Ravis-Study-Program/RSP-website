import type {
  JobCompany,
  JobPosition,
  JobReferralCandidate,
  JobReferralProfile,
} from '@/api/generated/models';
import { selectedDemoUser } from '@/api/queries';
import { people } from '@/data/demo';
import { isPublicResumeURL, maxReferralRequests } from '@/jobs';

const profiles = new Map<string, JobReferralProfile>([
  [
    people[1].id,
    {
      reason:
        'I built a scheduling tool for my university club and wrote integration tests for it. I’m looking for an internship where I can work on backend systems.',
      resumeUrl: 'https://drive.google.com/file/d/rsp-demo-noah-williams/view',
    },
  ],
  [
    people[2].id,
    {
      reason:
        'I maintained the frontend for a student events platform and improved its keyboard navigation. I’d like to work on accessible web tools.',
      resumeUrl: 'https://drive.google.com/file/d/rsp-demo-zara-ahmed/view',
    },
  ],
]);
const requests = new Map<string, Set<string>>([
  [people[1].id, new Set(['position-1'])],
  [people[2].id, new Set(['position-1', 'position-4'])],
]);
const offers = new Map<string, Set<string>>([
  ['sample-referrer-1', new Set(['company-Google'])],
  ['sample-referrer-2', new Set(['company-Canva'])],
  ['sample-referrer-3', new Set(['company-IMC'])],
]);

export function moveDemoReferralRequests(
  source: JobPosition,
  target?: JobPosition,
) {
  for (const selected of requests.values()) {
    if (
      selected.delete(source.id) &&
      target?.status === 'open' &&
      target.company.id === source.company.id
    )
      selected.add(target.id);
  }
}

export function demoJobReferralRequest(
  parts: string[],
  method: string,
  body: unknown,
  userId: string,
  positions: JobPosition[],
  companies: JobCompany[],
) {
  const canRefer = selectedDemoUser().alumni;
  const ownOffers = offers.get(userId) ?? new Set<string>();
  const ownRequests = requests.get(userId) ?? new Set<string>();
  const profile = profiles.get(userId);
  if (!parts.length && method === 'GET') {
    return {
      profile: profile ?? { reason: '', resumeUrl: '' },
      requests: [...ownRequests],
      companies: canRefer
        ? companies.filter(({ id }) => ownOffers.has(id))
        : [],
    };
  }
  if (parts[0] === 'counts' && method === 'GET') {
    const counts = new Map<string, number>();
    for (const [referrerId, companyIds] of offers) {
      if (referrerId === userId && !canRefer) continue;
      for (const id of companyIds) counts.set(id, (counts.get(id) ?? 0) + 1);
    }
    return {
      items: [...counts].map(([companyId, count]) => ({ companyId, count })),
    };
  }
  if (parts[0] === 'profile') {
    if (method === 'DELETE') {
      profiles.delete(userId);
      requests.delete(userId);
      return;
    }
    if (method === 'PUT') {
      const input = body as JobReferralProfile;
      const reason = input.reason.trim();
      const resumeUrl = input.resumeUrl.trim();
      if (!reason || reason.length > 4000)
        throw new Error(
          'Enter why you would be a good referral, up to 4,000 characters.',
        );
      if (!isPublicResumeURL(resumeUrl))
        throw new Error('Enter a public link to your resume.');
      profiles.set(userId, { reason, resumeUrl });
      return;
    }
  }
  if (parts[0] === 'requests') {
    if (method === 'DELETE') {
      ownRequests.delete(parts[1]);
      return;
    }
    if (!profile)
      throw new Error(
        'Complete your referral profile before requesting a referral.',
      );
    if (
      !positions.some(({ id, status }) => id === parts[1] && status === 'open')
    )
      throw new Error('This position is no longer open for referrals.');
    if (!ownRequests.has(parts[1]) && ownRequests.size >= maxReferralRequests)
      throw new Error(
        'You can request referrals for up to five open positions.',
      );
    ownRequests.add(parts[1]);
    requests.set(userId, ownRequests);
    return;
  }
  if (parts[0] === 'offers') {
    if (!canRefer) throw new Error('Only RSP graduates can refer candidates.');
    if (method === 'DELETE') ownOffers.delete(parts[1]);
    else if (companies.some(({ id }) => id === parts[1]))
      ownOffers.add(parts[1]);
    else throw new Error('Company not found.');
    offers.set(userId, ownOffers);
    return;
  }
  if (parts[0] === 'companies' && method === 'GET') {
    if (!canRefer) throw new Error('Only RSP graduates can refer candidates.');
    const companyId = parts[1];
    const candidates: JobReferralCandidate[] = [];
    if (ownOffers.has(companyId)) {
      for (const person of people) {
        const candidateProfile = profiles.get(person.id);
        if (person.id === userId || !candidateProfile) continue;
        const selected = requests.get(person.id);
        const matching = positions.filter(
          (p) =>
            p.company.id === companyId &&
            p.status === 'open' &&
            selected?.has(p.id),
        );
        if (matching.length)
          candidates.push({
            id: person.id,
            name: person.name,
            reason: candidateProfile.reason,
            resumeUrl: candidateProfile.resumeUrl,
            positions: matching.map(({ id, title }) => ({ id, title })),
          });
      }
    }
    return {
      items: candidates,
      totalCount: candidates.length,
      pageInfo: { hasMore: false, nextCursor: null, previousCursor: null },
    };
  }
  throw new Error('This referral information is unavailable.');
}
