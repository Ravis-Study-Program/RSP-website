import { useQueryClient } from '@tanstack/react-query';
import { apiRequest } from '@/api/client';
import { demoMode, seasonEnrollmentsQueryKey } from '@/api/queries';
import type { EnrollmentPage, InvitationPage } from '@/api/generated/models';
import { NamedConfirmation } from '@/components/Dialogs';
import type { Page, Season } from '@/types';

export function DeleteEmptySeason({ season }: { season: Season }) {
  const client = useQueryClient();
  async function remove() {
    if (demoMode) {
      const members = client.getQueryData<EnrollmentPage>(
        seasonEnrollmentsQueryKey(season.id),
      );
      const invites = client.getQueryData<InvitationPage>([
        'invitations',
        season.id,
      ]);
      if (
        (season.memberCount ?? 0) > 0 ||
        members?.items.length ||
        invites?.items.some((i) => i.status === 'pending')
      )
        throw new Error(
          'This season has enrollment history or pending invitations and cannot be deleted.',
        );
    } else await apiRequest(`/seasons/${season.id}`, { method: 'DELETE' });
    client.setQueryData<Page<Season>>(['seasons'], (current) =>
      current
        ? {
            ...current,
            items: current.items.filter((s) => s.id !== season.id),
            totalCount: Math.max(0, current.totalCount - 1),
          }
        : current,
    );
    if (!demoMode) await client.invalidateQueries();
  }
  return (
    <NamedConfirmation
      name={season.name}
      actionLabel="Delete empty season"
      description="Only a season with no enrollment history, recorded activity or pending invitations can be deleted. Its weeks will also disappear from the website."
      onConfirm={remove}
    />
  );
}
