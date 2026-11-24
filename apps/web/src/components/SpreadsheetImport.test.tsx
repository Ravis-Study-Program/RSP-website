import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { SpreadsheetImport } from '@/components/SpreadsheetImport';
import type { Season } from '@/types';

const season = {
  id: 's1',
  name: 'Summer 2025/26',
  slug: 'summer-2025-26',
  status: 'open',
} as Season;

function renderImport(seasons: Season[] | undefined) {
  const wrap = (children: ReactNode) => (
    <QueryClientProvider client={new QueryClient()}>
      {children}
    </QueryClientProvider>
  );
  return render(
    wrap(
      <SpreadsheetImport
        kinds={['seasons', 'invitations']}
        title="Mass Import"
        trigger="Mass Import"
        seasons={seasons}
      />,
    ),
  );
}

it('waits for the seasons before a spreadsheet can be read', () => {
  const { rerender } = renderImport(undefined);
  // A file read before the seasons load would find none of the seasons it
  // names, so the import stays closed until they arrive.
  expect(screen.getByRole('button', { name: 'Mass Import' })).toBeDisabled();
  rerender(
    <QueryClientProvider client={new QueryClient()}>
      <SpreadsheetImport
        kinds={['seasons', 'invitations']}
        title="Mass Import"
        trigger="Mass Import"
        seasons={[season]}
      />
    </QueryClientProvider>,
  );
  expect(screen.getByRole('button', { name: 'Mass Import' })).toBeEnabled();
});
