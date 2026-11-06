import { useMemo } from 'react';
import { useLocation, useParams } from 'react-router-dom';

import { usePeople, useSeasons, useSeasonPeople } from '@/api/queries';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import { PeopleTable } from '@/components/PeopleTable';

/**
 * The People directory: a read-only list that looks the same for every
 * role. Managing students happens on All Students and My Students.
 */
export function PeoplePage() {
  const location = useLocation();
  const { slug } = useParams();
  const graduatesOnly = location.pathname === '/graduates';
  usePageTitle(graduatesOnly ? 'Graduates' : 'People');
  const publicQuery = usePeople();
  const seasons = useSeasons();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const seasonQuery = useSeasonPeople(season?.id, season?.name);
  const query = slug ? seasonQuery : publicQuery;
  const data = useMemo(
    () =>
      (query.data?.items ?? []).filter(
        (person) => !graduatesOnly || person.roles.includes('graduate'),
      ),
    [graduatesOnly, query.data],
  );

  return (
    <Page>
      <PageHeader title={graduatesOnly ? 'Graduates' : 'People'} />
      <PeopleTable
        variant="people"
        ariaLabel={graduatesOnly ? 'Graduate directory' : 'People directory'}
        data={data}
        seasons={seasons.data?.items}
        seasonId={season?.id}
        showLevel={Boolean(season)}
        loading={query.isLoading || (Boolean(slug) && seasons.isLoading)}
        error={query.isError || (Boolean(slug) && seasons.isError)}
        onRetry={() => void query.refetch()}
        emptyTitle={graduatesOnly ? 'No Graduates' : 'No People in This Season'}
      />
    </Page>
  );
}
