import { getAccessToken, problemFromResponse } from '@/api/client';

export async function orvalRequest<T>(
  url: string,
  options: RequestInit,
): Promise<T> {
  const apiUrl = url.startsWith('/api/v2') ? url : `/api/v2${url}`;
  const isPublicHealthCheck =
    url.endsWith('/health/live') || url.endsWith('/health/ready');
  const response = await fetch(apiUrl, {
    ...options,
    credentials: 'same-origin',
    headers: {
      Accept: 'application/json, application/problem+json',
      ...options.headers,
      ...(isPublicHealthCheck
        ? {}
        : { Authorization: `Bearer ${await getAccessToken()}` }),
    },
  });
  if (!response.ok) {
    throw await problemFromResponse(response);
  }
  const data = response.status === 204 ? undefined : await response.json();
  return { data, status: response.status, headers: response.headers } as T;
}
