import type { ColumnDef } from '@tanstack/react-table';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';

import { DataTable } from '@/components/DataTable';

interface RowData { id: string; name: string; status: string }
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
        data={[{ id: '1', name: 'Amelia', status: 'active' }, { id: '2', name: 'Noah', status: 'completed' }]}
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
    render(<DataTable ariaLabel="Members" data={[]} columns={columns} emptyTitle="No members" emptyMessage="No members yet." renderCard={(row) => row.original.name} />);
    expect(screen.getByRole('heading', { name: 'No members' })).toBeInTheDocument();
    expect(screen.getByText('No members yet.')).toBeInTheDocument();
  });
});
