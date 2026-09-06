import {
  type ColumnDef,
  type ColumnFiltersState,
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  type Row,
  type SortingState,
  type VisibilityState,
  useReactTable,
} from '@tanstack/react-table';
import {
  IconArrowsMaximize,
  IconArrowsMinimize,
  IconChevronDown,
  IconChevronLeft,
  IconChevronRight,
  IconChevronUp,
  IconColumns,
  IconFilter,
  IconSearch,
} from '@tabler/icons-react';
import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from 'react';

import { EmptyState, ErrorState } from '@/components/StatusViews';
import styles from '@/styles/App.module.css';

const SearchHighlightContext = createContext('');

export function HighlightText({ text }: { text: string }) {
  const query = useContext(SearchHighlightContext).trim();
  if (!query) return text;
  const start = text.toLocaleLowerCase().indexOf(query.toLocaleLowerCase());
  if (start < 0) return text;
  return (
    <>
      {text.slice(0, start)}
      <mark>{text.slice(start, start + query.length)}</mark>
      {text.slice(start + query.length)}
    </>
  );
}

export interface DataTableProps<TData> {
  ariaLabel: string;
  data: TData[];
  columns: ColumnDef<TData, any>[];
  loading?: boolean;
  error?: boolean;
  onRetry?: () => void;
  emptyTitle: string;
  emptyMessage: string;
  searchPlaceholder?: string;
  renderCard: (row: Row<TData>) => ReactNode;
  renderExpanded?: (row: Row<TData>) => ReactNode;
  defaultExpanded?: boolean;
  getRowId?: (row: TData) => string;
  renderSummary?: (rows: TData[]) => ReactNode;
}

export function DataTable<TData>({
  ariaLabel,
  data,
  columns,
  loading = false,
  error = false,
  onRetry,
  emptyTitle,
  emptyMessage,
  searchPlaceholder = 'Search all columns',
  renderCard,
  renderExpanded,
  defaultExpanded = false,
  getRowId,
  renderSummary,
}: DataTableProps<TData>) {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([]);
  const [globalFilter, setGlobalFilter] = useState('');
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
  const [dense, setDense] = useState(
    () => localStorage.getItem('rsp-table-density') === 'compact',
  );
  const [fullscreen, setFullscreen] = useState(false);
  const storedPageSize = Number(
    localStorage.getItem('rsp-table-page-size') ?? '10',
  );
  const initialPageSize = [10, 25, 50, 100].includes(storedPageSize)
    ? storedPageSize
    : 10;

  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnFilters, globalFilter, columnVisibility },
    onSortingChange: setSorting,
    onColumnFiltersChange: setColumnFilters,
    onGlobalFilterChange: setGlobalFilter,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getExpandedRowModel: getExpandedRowModel(),
    getRowCanExpand: () => Boolean(renderExpanded),
    getRowId,
    initialState: {
      pagination: { pageSize: initialPageSize },
      ...(defaultExpanded ? { expanded: true } : {}),
    },
  });

  useEffect(() => {
    localStorage.setItem(
      'rsp-table-density',
      dense ? 'compact' : 'comfortable',
    );
  }, [dense]);

  useEffect(() => {
    table.setPageIndex(0);
  }, [columnFilters, globalFilter, table]);

  useEffect(() => {
    if (!fullscreen) return;
    const onKeyDown = (event: KeyboardEvent) =>
      event.key === 'Escape' && setFullscreen(false);
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [fullscreen]);

  if (loading) {
    return (
      <div className={styles.tableShell} aria-busy="true">
        <div className={styles.tableToolbar}>
          <span className={`${styles.skeleton} ${styles.tableSkeletonSearch}`}>
            Loading
          </span>
        </div>
        <div className={styles.tableState}>
          <span role="status">Loading {ariaLabel.toLowerCase()}…</span>
        </div>
      </div>
    );
  }

  if (error)
    return (
      <div className={styles.tableShell}>
        <ErrorState onRetry={onRetry} />
      </div>
    );

  const filteredRows = table.getFilteredRowModel().rows;
  const pageRows = table.getRowModel().rows;

  return (
    <SearchHighlightContext.Provider value={globalFilter}>
      <section
        className={`${styles.tableShell} ${dense ? styles.compactTable : ''} ${fullscreen ? styles.tableShellFullscreen : ''}`}
        aria-label={ariaLabel}
      >
        <div className={styles.tableToolbar}>
          <div className={styles.searchWrap}>
            <IconSearch size={18} aria-hidden="true" />
            <label
              className={styles.visuallyHidden}
              htmlFor={`${ariaLabel}-search`}
            >
              Search {ariaLabel.toLowerCase()}
            </label>
            <input
              id={`${ariaLabel}-search`}
              className={styles.searchInput}
              type="search"
              placeholder={searchPlaceholder}
              value={globalFilter}
              onChange={(event) => setGlobalFilter(event.target.value)}
            />
          </div>
          <div className={styles.toolbarActions}>
            <button
              className={styles.buttonQuiet}
              type="button"
              onClick={() => setDense((value) => !value)}
            >
              {dense ? 'Comfortable rows' : 'Compact rows'}
            </button>
            <details className={styles.columnMenu}>
              <summary
                className={styles.iconButton}
                aria-label="Choose visible columns"
              >
                <IconColumns size={19} aria-hidden="true" />
              </summary>
              <div className={styles.columnMenuPanel}>
                {table
                  .getAllLeafColumns()
                  .filter((column) => column.getCanHide())
                  .map((column) => (
                    <label className={styles.checkLabel} key={column.id}>
                      <input
                        type="checkbox"
                        checked={column.getIsVisible()}
                        onChange={column.getToggleVisibilityHandler()}
                      />
                      {typeof column.columnDef.header === 'string'
                        ? column.columnDef.header
                        : column.id}
                    </label>
                  ))}
              </div>
            </details>
            <details className={styles.columnMenu}>
              <summary className={styles.buttonQuiet}>
                <IconFilter size={18} aria-hidden="true" /> Filters
                {columnFilters.length ? ` (${columnFilters.length})` : ''}
              </summary>
              <div className={styles.columnMenuPanel}>
                {table
                  .getAllLeafColumns()
                  .filter(
                    (column) =>
                      column.getCanFilter() &&
                      typeof table
                        .getPreFilteredRowModel()
                        .flatRows[0]?.getValue(column.id) === 'string',
                  )
                  .map((column) => {
                    const label =
                      typeof column.columnDef.header === 'string'
                        ? column.columnDef.header
                        : column.id;
                    return (
                      <label className={styles.field} key={column.id}>
                        <span>{label}</span>
                        <input
                          className={styles.input}
                          value={
                            (column.getFilterValue() as string | undefined) ??
                            ''
                          }
                          onChange={(event) =>
                            column.setFilterValue(
                              event.target.value || undefined,
                            )
                          }
                          placeholder={`Filter ${label.toLowerCase()}`}
                        />
                      </label>
                    );
                  })}
                {columnFilters.length ? (
                  <button
                    className={styles.buttonSecondary}
                    type="button"
                    onClick={() => setColumnFilters([])}
                  >
                    Clear column filters
                  </button>
                ) : null}
              </div>
            </details>
            <button
              className={styles.iconButton}
              type="button"
              onClick={() => setFullscreen((value) => !value)}
              aria-label={
                fullscreen ? 'Exit full screen table' : 'Open full screen table'
              }
            >
              {fullscreen ? (
                <IconArrowsMinimize size={19} aria-hidden="true" />
              ) : (
                <IconArrowsMaximize size={19} aria-hidden="true" />
              )}
            </button>
          </div>
        </div>
        <div className={styles.visuallyHidden} aria-live="polite">
          {filteredRows.length} results
        </div>
        {!data.length || !filteredRows.length ? (
          <EmptyState
            title={emptyTitle}
            message={
              !data.length
                ? emptyMessage
                : 'Try a different search or clear the current filters.'
            }
            filtered={Boolean(
              data.length && (globalFilter || columnFilters.length),
            )}
            action={
              globalFilter || columnFilters.length ? (
                <button
                  className={styles.buttonSecondary}
                  type="button"
                  onClick={() => {
                    setGlobalFilter('');
                    setColumnFilters([]);
                  }}
                >
                  Clear filters
                </button>
              ) : undefined
            }
          />
        ) : (
          <>
            <div className={styles.desktopTable}>
              <table className={styles.dataTable}>
                <caption className={styles.visuallyHidden}>
                  {ariaLabel}. Sortable and filterable data table.
                </caption>
                <thead>
                  {table.getHeaderGroups().map((headerGroup) => (
                    <tr key={headerGroup.id}>
                      {headerGroup.headers.map((header) => (
                        <th
                          key={header.id}
                          scope="col"
                          aria-sort={
                            header.column.getIsSorted() === 'asc'
                              ? 'ascending'
                              : header.column.getIsSorted() === 'desc'
                                ? 'descending'
                                : 'none'
                          }
                        >
                          {header.isPlaceholder ? null : header.column.getCanSort() ? (
                            <button
                              className={styles.sortButton}
                              type="button"
                              onClick={header.column.getToggleSortingHandler()}
                            >
                              {flexRender(
                                header.column.columnDef.header,
                                header.getContext(),
                              )}
                              {header.column.getIsSorted() === 'asc' ? (
                                <IconChevronUp size={15} aria-hidden="true" />
                              ) : header.column.getIsSorted() === 'desc' ? (
                                <IconChevronDown size={15} aria-hidden="true" />
                              ) : null}
                            </button>
                          ) : (
                            flexRender(
                              header.column.columnDef.header,
                              header.getContext(),
                            )
                          )}
                        </th>
                      ))}
                    </tr>
                  ))}
                </thead>
                <tbody>
                  {pageRows.map((row) => (
                    <TableRow
                      key={row.id}
                      row={row}
                      renderExpanded={renderExpanded}
                    />
                  ))}
                </tbody>
              </table>
            </div>
            <div className={styles.mobileCards}>
              {pageRows.map((row) => (
                <article className={styles.mobileCard} key={row.id}>
                  {renderCard(row)}
                </article>
              ))}
            </div>
            <div className={styles.tablePagination}>
              <label className={styles.inline}>
                <span className={styles.metricLabel}>Rows per page</span>
                <select
                  className={styles.select}
                  value={table.getState().pagination.pageSize}
                  onChange={(event) => {
                    const value = Number(event.target.value);
                    table.setPageSize(value);
                    localStorage.setItem('rsp-table-page-size', String(value));
                  }}
                >
                  {[10, 25, 50, 100].map((value) => (
                    <option key={value} value={value}>
                      {value}
                    </option>
                  ))}
                </select>
              </label>
              <span className={styles.muted}>
                Page {table.getState().pagination.pageIndex + 1} of{' '}
                {Math.max(table.getPageCount(), 1)}
              </span>
              <div className={styles.paginationControls}>
                <button
                  className={styles.iconButton}
                  type="button"
                  disabled={!table.getCanPreviousPage()}
                  onClick={() => table.previousPage()}
                  aria-label="Previous page"
                >
                  <IconChevronLeft size={19} aria-hidden="true" />
                </button>
                <button
                  className={styles.iconButton}
                  type="button"
                  disabled={!table.getCanNextPage()}
                  onClick={() => table.nextPage()}
                  aria-label="Next page"
                >
                  <IconChevronRight size={19} aria-hidden="true" />
                </button>
              </div>
            </div>
          </>
        )}
      </section>
      {renderSummary?.(filteredRows.map((row) => row.original))}
    </SearchHighlightContext.Provider>
  );
}

function TableRow<TData>({
  row,
  renderExpanded,
}: {
  row: Row<TData>;
  renderExpanded?: (row: Row<TData>) => ReactNode;
}) {
  return (
    <>
      <tr>
        {row.getVisibleCells().map((cell) => (
          <td key={cell.id}>
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </td>
        ))}
      </tr>
      {row.getIsExpanded() && renderExpanded ? (
        <tr className={styles.expandedRow}>
          <td colSpan={row.getVisibleCells().length}>{renderExpanded(row)}</td>
        </tr>
      ) : null}
    </>
  );
}
