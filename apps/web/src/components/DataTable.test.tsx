import type { ColumnDef } from '@tanstack/react-table';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { DataTable } from '@/components/DataTable';

interface RowData {
  id: string;
  name: string;
  status: string;
}
const columns: ColumnDef<RowData, any>[] = [
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'status', header: 'Status' },
];

describe('DataTable', () => {
  it('filters data and announces result count', async () => {
    const user = userEvent.setup();
    render(
      <DataTable
        ariaLabel="Members"
        data={[
          { id: '1', name: 'Amelia', status: 'active' },
          { id: '2', name: 'Noah', status: 'completed' },
        ]}
        columns={columns}
        emptyTitle="No members"
        emptyMessage="No members yet."
        getRowId={(row) => row.id}
        renderCard={(row) => row.original.name}
      />,
    );
    await user.type(screen.getByLabelText('Search members'), 'Amelia');
    expect(screen.getByText('1 results')).toBeInTheDocument();
    expect(screen.getAllByText('Amelia').length).toBeGreaterThan(0);
    expect(screen.queryByText('Noah')).not.toBeInTheDocument();
  });

  it('distinguishes an empty collection from a filtered empty state', async () => {
    render(
      <DataTable
        ariaLabel="Members"
        data={[]}
        columns={columns}
        emptyTitle="No members"
        emptyMessage="No members yet."
        renderCard={(row) => row.original.name}
      />,
    );
    expect(
      screen.getByRole('heading', { name: 'No members' }),
    ).toBeInTheDocument();
    expect(screen.getByText('No members yet.')).toBeInTheDocument();
  });

  it('can clear a column-filter-only empty state', async () => {
    const user = userEvent.setup();
    render(
      <DataTable
        ariaLabel="Members"
        data={[{ id: '1', name: 'Amelia', status: 'active' }]}
        columns={columns}
        emptyTitle="No members"
        emptyMessage="No members yet."
        getRowId={(row) => row.id}
        renderCard={(row) => row.original.name}
      />,
    );
    await user.click(screen.getByText('Filters'));
    await user.type(screen.getByPlaceholderText('Filter status'), 'completed');
    expect(
      screen.getByText('Try a different search or clear the current filters.'),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Clear filters' }));
    expect(screen.getAllByText('Amelia').length).toBeGreaterThan(0);
  });
});

it('summarises all filtered records independently of pagination', async () => {
  const user = userEvent.setup();
  render(
    <DataTable
      ariaLabel="Summary members"
      data={Array.from({ length: 23 }, (_, i) => ({
        id: String(i),
        name: i < 12 ? 'Amelia' : 'Noah',
        status: 'active',
      }))}
      columns={columns}
      emptyTitle="None"
      emptyMessage="None"
      renderCard={(row) => row.original.name}
      renderSummary={(rows) => (
        <output aria-label="Summary count">{rows.length}</output>
      )}
    />,
  );
  expect(screen.getByLabelText('Summary count')).toHaveTextContent('23');
  await user.click(screen.getByRole('button', { name: 'Next page' }));
  expect(screen.getByLabelText('Summary count')).toHaveTextContent('23');
  await user.type(screen.getByLabelText('Search summary members'), 'Amelia');
  expect(screen.getByLabelText('Summary count')).toHaveTextContent('12');
});
