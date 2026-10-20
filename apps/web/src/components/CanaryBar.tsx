import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { FormDialog } from '@/components/Dialogs';
import { apiRequest } from '@/api/client';
import type {
  CanaryAccess,
  CanaryAccessUpdate,
  CanarySeasonRole,
  GlobalRole,
} from '@/api/generated/models';
import { IconFlask } from '@tabler/icons-react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link } from 'react-router-dom';

/** Set at build time for per-PR canary images; production builds leave it unset. */
export const canaryEnabled = import.meta.env.VITE_RSP_CANARY === 'true';

const globalRoleLabels: Record<GlobalRole, string> = {
  director: 'Director',
  system_admin: 'System Admin',
};

const seasonRoleItems: { value: CanarySeasonRole; label: string }[] = [
  { value: 'student', label: 'Student' },
  { value: 'mentor', label: 'Mentor' },
  { value: 'coordinator', label: 'Coordinator' },
  { value: 'removed', label: 'Removed (kicked / withdrawn)' },
];

const canaryAccessKey = ['canary', 'access'];

/**
 * A canary-only marker, floating over every page including sign-in, that
 * points testers at the emails the canary captures instead of sending.
 */
export function CanaryBar() {
  if (!canaryEnabled) return null;
  return (
    <div
      role="note"
      className="fixed right-3 bottom-3 z-50 flex items-center gap-2 rounded-full border border-amber-300 bg-amber-100 px-3 py-1.5 text-xs text-amber-950 shadow-sm max-[56rem]:bottom-20 dark:border-amber-800 dark:bg-amber-950 dark:text-amber-100"
    >
      <IconFlask size={14} aria-hidden="true" />
      <strong className="font-semibold">Canary</strong>
      <span aria-hidden="true">·</span>
      <span>Fake data</span>
      <span aria-hidden="true">·</span>
      <a className="underline underline-offset-2" href="/mailpit/">
        Captured emails
      </a>
    </div>
  );
}

/** Lets a signed-in canary tester choose their own roles. */
export function CanaryRoleSwitcher() {
  if (!canaryEnabled) return null;
  return <CanaryRoleDialog />;
}

function CanaryRoleDialog() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();
  const access = useQuery({
    queryKey: canaryAccessKey,
    queryFn: () => apiRequest<CanaryAccess>('/canary/access'),
  });
  const save = useMutation({
    mutationFn: (update: CanaryAccessUpdate) =>
      apiRequest<CanaryAccess>('/canary/access', {
        method: 'PUT',
        body: JSON.stringify(update),
      }),
    onSuccess: async (updated) => {
      queryClient.setQueryData(canaryAccessKey, updated);
      // Every permission-dependent screen must re-read the caller's access.
      await queryClient.invalidateQueries();
      setOpen(false);
    },
  });

  return (
    <FormDialog
      title="Switch roles"
      description={`Choose your own access. Season roles apply to ${access.data?.season.name ?? 'the seeded season'}.`}
      trigger="Switch roles"
      triggerVariant="outline"
      triggerSize="sm"
      open={open}
      onOpenChange={(next) => {
        setOpen(next);
        if (!next) save.reset();
      }}
    >
      {access.isLoading ? (
        <p className="text-sm text-muted-foreground">Loading your access…</p>
      ) : access.isError || !access.data ? (
        <p className="text-sm text-destructive">
          {access.error?.message ?? 'Your access could not be loaded.'}
        </p>
      ) : (
        <CanaryRoleForm
          key={JSON.stringify(access.data)}
          access={access.data}
          pending={save.isPending}
          error={save.error?.message}
          onSubmit={(update) => save.mutate(update)}
        />
      )}
    </FormDialog>
  );
}

function CanaryRoleForm({
  access,
  pending,
  error,
  onSubmit,
}: {
  access: CanaryAccess;
  pending: boolean;
  error?: string;
  onSubmit: (update: CanaryAccessUpdate) => void;
}) {
  const [globalRoles, setGlobalRoles] = useState<GlobalRole[]>(
    access.globalRoles.map(({ role }) => role as GlobalRole),
  );
  const [seasonRole, setSeasonRole] = useState<CanarySeasonRole>(
    (access.seasonRole?.role as CanarySeasonRole | undefined) ?? 'student',
  );
  const pendingMfa = [
    ...access.globalRoles,
    ...(access.seasonRole ? [access.seasonRole] : []),
  ].some(({ state }) => state === 'pending_mfa');

  return (
    <form
      className="grid gap-5"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit({ globalRoles, seasonRole });
      }}
    >
      <fieldset className="grid gap-2">
        <legend className="mb-1 text-sm font-medium">Global roles</legend>
        {(Object.keys(globalRoleLabels) as GlobalRole[]).map((role) => (
          <div key={role} className="flex items-center gap-2">
            <Checkbox
              id={`canary-${role}`}
              checked={globalRoles.includes(role)}
              onCheckedChange={(checked) =>
                setGlobalRoles((current) =>
                  checked
                    ? [...current, role]
                    : current.filter((value) => value !== role),
                )
              }
            />
            <Label htmlFor={`canary-${role}`} className="font-normal">
              {globalRoleLabels[role]}
            </Label>
          </div>
        ))}
      </fieldset>
      <div className="grid gap-1.5">
        <Label htmlFor="canary-season-role">Season role</Label>
        <Select
          items={seasonRoleItems}
          value={seasonRole}
          onValueChange={(value) => value && setSeasonRole(value)}
        >
          <SelectTrigger id="canary-season-role" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectGroup>
              {seasonRoleItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </div>
      {pendingMfa || (!access.mfaConfigured && globalRoles.length > 0) ? (
        <p className="text-sm text-muted-foreground">
          Director, System Admin and Coordinator stay pending until you{' '}
          <Link className="underline underline-offset-2" to="/settings">
            set up two-factor authentication
          </Link>
          , exactly as in production.
        </p>
      ) : null}
      {error ? (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      ) : null}
      <Button type="submit" disabled={pending}>
        {pending ? 'Saving…' : 'Save roles'}
      </Button>
    </form>
  );
}
