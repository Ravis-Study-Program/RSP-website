import { useNavigate, useParams } from 'react-router-dom';

import { accountToPerson } from '@/accountPeople';
import {
  useAdminUsers,
  useCurrentUser,
  usePeople,
  useSeasonEnrollments,
  useSeasons,
  useSeasonPeople,
} from '@/api/queries';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import { PeopleTable } from '@/components/PeopleTable';
import { PersonRowActions } from '@/components/PersonRowActions';
import { Field, FieldLabel } from '@/components/ui/field';
import { useSeasonSwitcher } from '@/components/useSeasonSwitcher';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';

const allSeasons = 'all';

/**
 * The People directory, and the place students and staff are managed. In a
 * season it lists that season's people with their season actions; across
 * every season only account actions are available.
 */
export function PeoplePage() {
  const { slug } = useParams();
  const navigate = useNavigate();
  usePageTitle('People');
  const seasons = useSeasons();
  const currentUser = useCurrentUser();
  const viewer = currentUser.data;
  // System Admins see every account across all seasons, suspended ones and
  // registered nonmembers included, so they can manage them from here.
  const isSystemAdmin = Boolean(viewer?.globalRoles.includes('system_admin'));
  const publicQuery = usePeople(!isSystemAdmin);
  const accountsQuery = useAdminUsers(isSystemAdmin && !slug);
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const seasonQuery = useSeasonPeople(season?.id, season?.name);
  const query = slug
    ? seasonQuery
    : isSystemAdmin
      ? accountsQuery
      : publicQuery;
  const data = slug
    ? (seasonQuery.data?.items ?? [])
    : isSystemAdmin
      ? (accountsQuery.data?.items ?? []).map(accountToPerson)
      : (publicQuery.data?.items ?? []);
  const isManager = Boolean(viewer?.globalRoles.length);
  const membership = viewer?.seasonRoles.find(
    (entry) => entry.seasonId === season?.id && entry.state === 'active',
  );
  const isCoordinator = membership?.role === 'coordinator';
  // Correcting an enrollment needs the enrollment itself.
  const enrollments = useSeasonEnrollments(
    season && (isManager || isCoordinator) ? season.id : undefined,
  );
  const mayAct =
    Boolean(viewer?.globalRoles.includes('system_admin')) ||
    Boolean(season && (isManager || membership));
  // Directors, System Admins and coordinators switch season at the top left.
  const seasonSwitcher = useSeasonSwitcher('people', '/people');
  const seasonItems = [
    { value: allSeasons, label: 'All seasons' },
    ...(seasons.data?.items ?? []).map((item) => ({
      value: item.slug,
      label: item.name,
    })),
  ];

  return (
    <Page>
      <PageHeader title="People" />
      <PeopleTable
        variant="people"
        ariaLabel="People directory"
        toolbarLead={seasonSwitcher}
        data={data}
        seasons={seasons.data?.items}
        seasonId={season?.id}
        showLevel={Boolean(season)}
        extraFilters={{
          content: (
            <Field>
              <FieldLabel htmlFor="people-season">Season</FieldLabel>
              <Select
                items={seasonItems}
                value={slug ?? allSeasons}
                onValueChange={(value) =>
                  navigate(
                    value && value !== allSeasons
                      ? `/seasons/${encodeURIComponent(value)}/people`
                      : '/people',
                  )
                }
              >
                <SelectTrigger id="people-season" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {seasonItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
          ),
          activeCount: Number(Boolean(slug)),
          onClear: () => navigate('/people'),
        }}
        renderActions={
          mayAct
            ? (person) => (
                <PersonRowActions
                  viewer={viewer}
                  person={person}
                  season={season}
                  seasons={seasons.data?.items ?? []}
                  enrollment={enrollments.data?.items.find(
                    (item) => item.id === person.enrollmentId,
                  )}
                  onChanged={() =>
                    void Promise.all([query.refetch(), enrollments.refetch()])
                  }
                />
              )
            : undefined
        }
        loading={query.isLoading || (Boolean(slug) && seasons.isLoading)}
        error={query.isError || (Boolean(slug) && seasons.isError)}
        onRetry={() => void query.refetch()}
        emptyTitle={slug ? 'No People in This Season' : 'No People'}
      />
    </Page>
  );
}
