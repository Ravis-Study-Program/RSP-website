import { ApiProblem } from '@/api/client';
import type { ProblemDetails } from '@/types';

let token: { value: string; expiresAt: number } | null = null;

async function accessToken() {
  if (token && token.expiresAt > Date.now() + 15_000) return token.value;
  const response = await fetch('/api/auth/token', { method: 'POST', credentials: 'include' });
  if (!response.ok) throw new Error('Unable to establish an application session.');
  const payload = (await response.json()) as { token: string; expiresAt: string };
  token = { value: payload.token, expiresAt: Date.parse(payload.expiresAt) };
  return token.value;
}

export async function orvalRequest<T>(url: string, options: RequestInit): Promise<T> {
  const apiUrl = url.startsWith('/api/v2') ? url : `/api/v2${url}`;
  const isPublicHealthCheck = url.endsWith('/health/live') || url.endsWith('/health/ready');
  const response = await fetch(apiUrl, {
    ...options,
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json, application/problem+json',
      ...options.headers,
      ...(isPublicHealthCheck ? {} : { Authorization: `Bearer ${await accessToken()}` }),
    },
  });
  if (!response.ok) {
    const problem = (await response.json()) as ProblemDetails;
    throw new ApiProblem(problem);
  }
  const data = response.status === 204 ? undefined : await response.json();
  return { data, status: response.status, headers: response.headers } as T;
}
