import { useParams } from 'react-router-dom';

import { usePeople, useSeasons, useSeasonPeople } from '@/api/queries';
import { Page, PageHeader, usePageTitle } from '@/components/Common';
import { PeopleTable } from '@/components/PeopleTable';

/**
 * The People directory: a read-only list that looks the same for every
 * role. In a season it lists that season's people; outside one it lists everyone, every season.
 * Managing students happens on All Students and My Students.
 */
export function PeoplePage() {
  const { slug } = useParams();
  usePageTitle('People');
  const publicQuery = usePeople();
  const seasons = useSeasons();
  const season = seasons.data?.items.find((item) => item.slug === slug);
  const seasonQuery = useSeasonPeople(season?.id, season?.name);
  const query = slug ? seasonQuery : publicQuery;
  const data = query.data?.items ?? [];

  return (
    <Page>
      <PageHeader title="People" />
      <PeopleTable
        variant="people"
        ariaLabel="People directory"
        data={data}
        seasons={seasons.data?.items}
        seasonId={season?.id}
        showLevel={Boolean(season)}
        loading={query.isLoading || (Boolean(slug) && seasons.isLoading)}
        error={query.isError || (Boolean(slug) && seasons.isError)}
        onRetry={() => void query.refetch()}
        emptyTitle={slug ? 'No People in This Season' : 'No People'}
      />
    </Page>
  );
}
