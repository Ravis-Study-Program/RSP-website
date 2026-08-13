import { createContext, useContext, useMemo, useState, type PropsWithChildren } from 'react';

import { useCurrentUser } from '@/api/queries';
import type { Role } from '@/types';

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
  const workspaces = useMemo<WorkspaceOption[]>(() => {
    if (!userQuery.data) return [];
    const options: WorkspaceOption[] = userQuery.data.seasonRoles.map((membership) => ({
      id: membership.seasonId,
      label: membership.seasonSlug
        .split('-')
        .map((part) => part[0]?.toUpperCase() + part.slice(1))
        .join(' '),
      role: membership.role,
      seasonSlug: membership.seasonSlug,
    }));
    if (userQuery.data.alumni) options.push({ id: 'alumni', label: 'Graduate workspace', role: 'graduate' });
    if (userQuery.data.globalRoles.includes('director')) options.push({ id: 'director', label: 'Director workspace', role: 'director' });
    if (userQuery.data.globalRoles.includes('system_admin'))
      options.push({ id: 'system-admin', label: 'System administration', role: 'system_admin' });
    return options;
  }, [userQuery.data]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const activeWorkspace = workspaces.find((item) => item.id === selectedId) ?? workspaces[0] ?? null;

  return (
    <WorkspaceContext.Provider value={{ workspaces, activeWorkspace, setActiveWorkspaceId: setSelectedId }}>
      {children}
    </WorkspaceContext.Provider>
  );
}

export function useWorkspace() {
  const context = useContext(WorkspaceContext);
  if (!context) throw new Error('useWorkspace must be used inside WorkspaceProvider');
  return context;
}
