import type { ProblemDetails } from '@/types';

let accessToken: { value: string; expiresAt: number } | null = null;

export class ApiProblem extends Error {
  constructor(public readonly problem: ProblemDetails) {
    super(problem.detail ?? problem.title);
    this.name = 'ApiProblem';
  }
}

async function getAccessToken(): Promise<string> {
  if (accessToken && accessToken.expiresAt > Date.now() + 15_000) return accessToken.value;

  const response = await fetch('/api/auth/token', {
    method: 'POST',
    credentials: 'include',
    headers: { Accept: 'application/json' },
  });

  if (!response.ok) throw await toProblem(response);
  const payload = (await response.json()) as { token: string; expiresAt: string };
  accessToken = { value: payload.token, expiresAt: Date.parse(payload.expiresAt) };
  return accessToken.value;
}

export function clearAccessToken() {
  accessToken = null;
}

export async function apiRequest<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = await getAccessToken();
  const response = await fetch(`/api/v2${path}`, {
    ...init,
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json, application/problem+json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
      Authorization: `Bearer ${token}`,
    },
  });

  if (!response.ok) throw await toProblem(response);
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

async function toProblem(response: Response): Promise<ApiProblem> {
  let problem: ProblemDetails;
  try {
    problem = (await response.json()) as ProblemDetails;
  } catch {
    problem = {
      type: 'about:blank',
      title: 'Request failed',
      status: response.status,
      code: 'unexpected_response',
      requestId: response.headers.get('x-request-id') ?? 'unknown',
    };
  }
  return new ApiProblem(problem);
}
