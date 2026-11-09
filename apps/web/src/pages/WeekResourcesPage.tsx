import { IconArrowLeft, IconEdit } from '@tabler/icons-react';
import { isBlankRichText } from '@/richText';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Link, Navigate, useParams } from 'react-router-dom';

import { ApiError, apiRequest } from '@/api/client';
import type { Week, WeekPage } from '@/api/generated/models';
import {
  demoMode,
  seasonWeeksQueryKey,
  useCurrentUser,
  useSeasons,
  useSeasonWeeks,
} from '@/api/queries';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import { RichTextContent } from '@/components/RichTextContent';
import { RichTextEditor } from '@/components/RichTextEditor';
import { Button, buttonVariants } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { FieldError } from '@/components/ui/field';
import {
  ErrorState,
  InlineNotice,
  PageSkeleton,
} from '@/components/StatusViews';
import { seasonAccess } from '@/seasonAccess';
import { formatDate } from '@/utils';

/**
 * One week's resources page. Members read it; coordinators, Directors and
 * System Admins edit it while the season is open.
 */
export function WeekResourcesPage() {
  const { slug, number } = useParams();
  const seasons = useSeasons();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const user = useCurrentUser();
  const weeks = useSeasonWeeks(season?.id);
  const week = weeks.data?.items.find((item) => String(item.number) === number);
  usePageTitle(week ? `Week ${week.number} Resources` : 'Week Resources');
  const [editing, setEditing] = useState(false);

  if (seasons.isLoading || user.isLoading || weeks.isLoading)
    return <PageSkeleton label="Loading week resources" />;
  if (seasons.isError || user.isError || weeks.isError)
    return (
      <Page>
        <ErrorState
          onRetry={() =>
            void Promise.all([
              seasons.refetch(),
              user.refetch(),
              weeks.refetch(),
            ])
          }
        />
      </Page>
    );
  if (!season || !week)
    return (
      <Page>
        <ErrorState
          title="Week Not Found"
          message="This week may have moved or you may not have access."
        />
      </Page>
    );
  const { hasSeasonAccess, isGlobalManager, isCoordinator } = seasonAccess(
    user.data,
    season.id,
  );
  if (!hasSeasonAccess) return <Navigate to="/forbidden" replace />;
  const canEdit =
    season.status === 'open' && (isGlobalManager || isCoordinator);

  return (
    <Page>
      <PageHeader
        title={`Week ${week.number} Resources`}
        description={`${season.name} · ${formatDate(week.startAt)} – ${formatDate(week.endAt)}`}
        actions={
          <div className="flex flex-wrap items-center gap-2.5">
            <Link
              to={`/seasons/${season.slug}`}
              className={buttonVariants({ variant: 'outline' })}
            >
              <IconArrowLeft size={17} aria-hidden="true" /> Back to Season
            </Link>
            {canEdit && !editing ? (
              <Button type="button" onClick={() => setEditing(true)}>
                <IconEdit size={17} aria-hidden="true" /> Edit Resources
              </Button>
            ) : null}
          </div>
        }
      />
      <div className="flex flex-col gap-6">
        {season.status === 'closed' ? (
          <InlineNotice tone="warning">
            This season is read-only, so the resources can no longer be edited.
          </InlineNotice>
        ) : null}
        <ResourcesPanel
          key={`${week.id}:${week.resourcesHtml}`}
          seasonId={season.id}
          week={week}
          editing={editing}
          onEditingChange={setEditing}
        />
      </div>
    </Page>
  );
}

function ResourcesPanel({
  seasonId,
  week,
  editing,
  onEditingChange: setEditing,
}: {
  seasonId: string;
  week: Week;
  editing: boolean;
  onEditingChange: (editing: boolean) => void;
}) {
  const queryClient = useQueryClient();
  const [draft, setDraft] = useState(week.resourcesHtml);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState('');

  const save = async () => {
    setPending(true);
    setError('');
    try {
      const saved: Week = demoMode
        ? {
            ...week,
            resourcesHtml: isBlankRichText(draft) ? '' : draft,
          }
        : await apiRequest<Week>(
            `/seasons/${encodeURIComponent(seasonId)}/weeks/${encodeURIComponent(week.id)}/resources`,
            {
              method: 'PUT',
              body: JSON.stringify({ resourcesHtml: draft }),
            },
          );
      queryClient.setQueryData<WeekPage>(
        seasonWeeksQueryKey(seasonId),
        (current) =>
          current && {
            ...current,
            items: current.items.map((item) =>
              item.id === saved.id ? saved : item,
            ),
          },
      );
      setEditing(false);
    } catch (requestError) {
      setError(
        requestError instanceof ApiError || requestError instanceof Error
          ? requestError.message
          : 'Unable to save these resources.',
      );
    } finally {
      setPending(false);
    }
  };

  if (editing)
    return (
      <Card>
        <CardContent className="flex flex-col gap-4">
          {error ? <FieldError>{error}</FieldError> : null}
          <RichTextEditor
            id={`week-resources-${week.id}`}
            label="Resources"
            hideLabel
            value={draft}
            onChange={setDraft}
          />
          <div className="flex flex-wrap justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={pending}
              onClick={() => {
                setDraft(week.resourcesHtml);
                setError('');
                setEditing(false);
              }}
            >
              Cancel
            </Button>
            <Button
              type="button"
              disabled={pending}
              onClick={() => void save()}
            >
              {pending ? 'Saving…' : 'Save Resources'}
            </Button>
          </div>
        </CardContent>
      </Card>
    );

  return (
    <Card>
      <CardContent>
        <RichTextContent html={week.resourcesHtml} empty="No resources yet." />
      </CardContent>
    </Card>
  );
}
