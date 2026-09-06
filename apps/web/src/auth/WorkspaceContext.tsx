import { useQueryClient } from '@tanstack/react-query';
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type PropsWithChildren,
} from 'react';

import { adaptCurrentUser } from '@/api/adapters';
import { apiRequest } from '@/api/client';
import type { Me } from '@/api/generated/models';
import { currentUserOptions, demoMode, useCurrentUser } from '@/api/queries';
import type { Role } from '@/types';
import { configureDisplayTimezone } from '@/utils';
import { useLocation, useNavigate } from 'react-router-dom';

export interface WorkspaceOption {
  id: string;
  label: string;
  role: Role;
  seasonSlug?: string;
}

interface WorkspaceValue {
  workspaces: WorkspaceOption[];
  activeWorkspace: WorkspaceOption | null;
  setActiveWorkspaceId: (id: string) => void;
}

const WorkspaceContext = createContext<WorkspaceValue | null>(null);

export function WorkspaceProvider({ children }: PropsWithChildren) {
  const userQuery = useCurrentUser();
  const location = useLocation();
  const navigate = useNavigate();
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
  const workspaces = useMemo<WorkspaceOption[]>(() => {
    if (!userQuery.data) return [];
    const options: WorkspaceOption[] = [];
    if (userQuery.data.globalRoles.includes('system_admin'))
      options.push({
        id: 'system-admin',
        label: 'System administration',
        role: 'system_admin',
      });
    if (userQuery.data.globalRoles.includes('director'))
      options.push({
        id: 'director',
        label: 'Director workspace',
        role: 'director',
      });
    options.push(
      ...userQuery.data.seasonRoles
        .filter(
          (membership) =>
            membership.state === 'active' || membership.state === 'completed',
        )
        .map((membership) => ({
          id: membership.seasonId,
          label: membership.seasonSlug
            .split('-')
            .map((part) => part[0]?.toUpperCase() + part.slice(1))
            .join(' '),
          role: membership.role,
          seasonSlug: membership.seasonSlug,
        })),
    );
    if (userQuery.data.alumni)
      options.push({
        id: 'alumni',
        label: 'Graduate workspace',
        role: 'graduate',
      });
    else if (
      userQuery.data.seasonRoles.length > 0 &&
      !userQuery.data.seasonRoles.some(
        (membership) => membership.state === 'active',
      )
    )
      options.push({
        id: 'former-member',
        label: 'Former member workspace',
        role: 'former_member',
      });
    return options;
  }, [userQuery.data]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const routeSeasonSlug = location.pathname.match(/^\/seasons\/([^/]+)/)?.[1];
  const activeWorkspace =
    workspaces.find(
      (item) => item.seasonSlug === routeSeasonSlug && routeSeasonSlug,
    ) ??
    workspaces.find((item) => item.id === selectedId) ??
    workspaces[0] ??
    null;

  return (
    <WorkspaceContext.Provider
      value={{
        workspaces,
        activeWorkspace,
        setActiveWorkspaceId: (id) => {
          setSelectedId(id);
          const chosen = workspaces.find((item) => item.id === id);
          const section = location.pathname.match(
            /^\/seasons\/[^/]+\/(people|practice|mock-interviews|mentees)$/,
          )?.[1];
          navigate(
            chosen?.seasonSlug
              ? `/seasons/${chosen.seasonSlug}${section ? `/${section}` : ''}`
              : '/dashboard',
          );
        },
      }}
    >
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspace() {
  const context = useContext(WorkspaceContext);
  if (!context)
    throw new Error('useWorkspace must be used inside WorkspaceProvider');
  return context;
}
