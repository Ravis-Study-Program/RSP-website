import { useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef } from 'react';

import { adaptCurrentUser } from '@/api/adapters';
import { apiRequest } from '@/api/client';
import type { Me } from '@/api/generated/models';
import { currentUserOptions, demoMode, useCurrentUser } from '@/api/queries';
import { configureDisplayTimezone } from '@/utils';

export function useDisplayTimezone() {
  const userQuery = useCurrentUser();
  const queryClient = useQueryClient();
  const timezoneInitialisation = useRef<string | null>(null);
  if (userQuery.data?.timezone)
    configureDisplayTimezone(userQuery.data.timezone);
  useEffect(() => {
    const user = userQuery.data;
    if (!user || user.timezoneConfigured || demoMode) return;
    const attemptKey = user.id;
    if (timezoneInitialisation.current === attemptKey) return;
    timezoneInitialisation.current = attemptKey;
    const detectedTimezone =
      Intl.DateTimeFormat().resolvedOptions().timeZone || user.timezone;
    void apiRequest<Me>('/me', {
      method: 'PATCH',
      body: JSON.stringify({
        name: user.name,
        slug: user.slug,
        avatarUrl: user.avatarUrl,
        timezone: detectedTimezone,
      }),
    })
      .then((updated) => {
        const adapted = adaptCurrentUser(updated);
        configureDisplayTimezone(adapted.timezone);
        queryClient.setQueryData(currentUserOptions.queryKey, adapted);
      })
      .catch(() => {
        // Settings remains available for a manual retry without overwriting a migrated choice.
      });
  }, [queryClient, userQuery.data]);
}
