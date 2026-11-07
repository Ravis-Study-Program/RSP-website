import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableHeader,
  TableRow,
  TableHead,
  TableBody,
  TableCell,
} from '@/components/ui/table';
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
  type RowData,
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
import { renderCell } from '@/lib/renderCell';
import { cn } from '@/lib/utils';
import { useMediaQuery } from '@/lib/useMediaQuery';

declare module '@tanstack/react-table' {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  interface ColumnMeta<TData extends RowData, TValue> {
    /** Fixed width in rem. Columns without one share the remaining space. */
    width?: number;
    /** Let long content wrap instead of truncating with an ellipsis. */
    wrap?: boolean;
    /** Align the header and cell content to the end, e.g. for actions. */
    align?: 'end';
    /** Filter by choosing one of these values instead of typing. */
    filterOptions?: { label: string; value: string }[];
  }
}

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

/**
 * The chevron that opens and closes a row's details, in a table's first
 * column. `subject` finishes its label: "Show notes for Two Sum".
 */
export function ExpandRowButton<TData>({
  row,
  subject,
}: {
  row: Row<TData>;
  subject: string;
}) {
  const expanded = row.getIsExpanded();
  return (
    <Button
      variant="ghost"
      size="icon"
      type="button"
      aria-label={`${expanded ? 'Hide' : 'Show'} ${subject}`}
      aria-expanded={expanded}
      onClick={row.getToggleExpandedHandler()}
    >
      {expanded ? (
        <IconChevronUp size={18} aria-hidden="true" />
      ) : (
        <IconChevronDown size={18} aria-hidden="true" />
      )}
    </Button>
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
  emptyMessage?: string;
  searchPlaceholder?: string;
  renderCard: (row: Row<TData>) => ReactNode;
  renderExpanded?: (row: Row<TData>) => ReactNode;
  defaultExpanded?: boolean;
  getRowId?: (row: TData) => string;
  renderSummary?: (rows: TData[]) => ReactNode;
  /** Filters beyond the column text filters, shown in the Filters popover. */
  extraFilters?: ExtraFilters;
  /** Controls shown beside the search box, such as a view switch. */
  toolbarStart?: ReactNode;
  /**
   * Controls shown before the search box, such as tabs switching between
   * tables. They stay while the table loads or fails.
   */
  toolbarLead?: ReactNode;
  /** `data` holds the loaded server pages; more rows load on demand. */
  serverPagination?: ServerPagination;
}

export interface ExtraFilters {
  content: ReactNode;
  /** How many of these filters are set, added to the Filters count. */
  activeCount: number;
  onClear: () => void;
}

export interface ServerPagination {
  totalCount: number;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore: () => void;
}

const toolbarClass =
  'flex flex-wrap items-center justify-between gap-3 border-b p-4 max-[56rem]:flex-col max-[56rem]:items-stretch';
const paginationClass =
  'flex flex-wrap items-center justify-between gap-3 border-t p-4 max-[56rem]:flex-col max-[56rem]:items-stretch';
// Fixed layout keeps every page's columns the same width; anything that does
// not fit is truncated. Flex rows (actions, badge groups) still wrap.
const flexibleColumnMinWidth = 8;
const truncateCellClass =
  'overflow-hidden text-ellipsis [&_:where(div,p):not(.flex,.grid)]:truncate';
const pageSizeItems = [10, 25, 50, 100].map((value) => ({
  label: String(value),
  value,
}));

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
  extraFilters,
  toolbarStart,
  toolbarLead,
  serverPagination,
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

  const compact = useMediaQuery('(max-width: 56rem)');
  const table = useReactTable({
    data,
    columns,
    state: { sorting, columnFilters, globalFilter, columnVisibility },
    // Sorts chain: a click sorts by that column first, and the columns sorted
    // before it still order its ties, newest first. Ties on every sort keep
    // the rows' original order.
    isMultiSortEvent: () => true,
    onSortingChange: (updater) =>
      setSorting((previous) => {
        const next =
          typeof updater === 'function' ? updater(previous) : updater;
        const changed = next.find(
          (sort) =>
            !previous.some(
              (item) => item.id === sort.id && item.desc === sort.desc,
            ),
        );
        return changed
          ? [changed, ...next.filter((sort) => sort.id !== changed.id)]
          : next;
      }),
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
    // Appended server pages must not jump the reader back to page one.
    autoResetPageIndex: !serverPagination,
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

  // A toolbar with its own controls stays mounted while loading, so an open
  // picker in it is not torn down when the rows arrive.
  if (loading && !extraFilters && !toolbarLead && !toolbarStart) {
    return (
      <div
        className="overflow-hidden rounded-lg border bg-card"
        aria-busy="true"
      >
        <div className={toolbarClass}>
          <div className="flex min-w-0 flex-1 flex-wrap items-center gap-3">
            {toolbarLead}
            <Skeleton className="h-9 w-full max-w-96 text-transparent">
              Loading
            </Skeleton>
          </div>
        </div>
        <div className="grid min-h-60 place-items-center p-8 text-center">
          <span role="status">Loading {ariaLabel.toLowerCase()}…</span>
        </div>
      </div>
    );
  }

  if (error && !extraFilters)
    return (
      <div className="overflow-hidden rounded-lg border bg-card">
        {toolbarLead ? <div className={toolbarClass}>{toolbarLead}</div> : null}
        <ErrorState onRetry={onRetry} />
      </div>
    );

  const visibleColumns = table.getVisibleLeafColumns();
  const tableMinWidth = visibleColumns.reduce(
    (total, column) =>
      total + (column.columnDef.meta?.width ?? flexibleColumnMinWidth),
    0,
  );
  const activeFilterCount =
    columnFilters.length + (extraFilters?.activeCount ?? 0);
  const filteredRows = table.getFilteredRowModel().rows;
  const pageRows = table.getRowModel().rows;
  const filterableColumns = table
    .getAllLeafColumns()
    .filter(
      (column) =>
        column.getCanFilter() &&
        typeof table
          .getPreFilteredRowModel()
          .flatRows[0]?.getValue(column.id) === 'string',
    );

  return (
    <SearchHighlightContext.Provider value={globalFilter}>
      <div className="flex flex-col gap-6">
        <section
          className={cn(
            'overflow-hidden rounded-lg border bg-card',
            dense &&
              '[&_tbody_td]:h-12 [&_tbody_td]:py-2 [&_thead_th]:h-9 [&_tbody_td]:text-sm',
            fullscreen && 'fixed inset-0 z-60 overflow-auto rounded-none',
          )}
          aria-label={ariaLabel}
        >
          <div className={toolbarClass}>
            <div className="flex min-w-0 flex-1 flex-wrap items-center gap-3">
              {toolbarLead}
              <div className="relative w-full max-w-96">
                <IconSearch
                  className="pointer-events-none absolute top-1/2 left-2.5 -translate-y-1/2 text-muted-foreground"
                  size={18}
                  aria-hidden="true"
                />
                <label className="sr-only" htmlFor={`${ariaLabel}-search`}>
                  Search {ariaLabel.toLowerCase()}
                </label>
                <Input
                  id={`${ariaLabel}-search`}
                  className="h-9 pl-9"
                  type="search"
                  placeholder={searchPlaceholder}
                  value={globalFilter}
                  onChange={(event) => setGlobalFilter(event.target.value)}
                />
              </div>
              {toolbarStart}
            </div>
            <div className="flex flex-wrap items-center gap-2 max-[56rem]:justify-between">
              <Button
                variant="ghost"
                type="button"
                onClick={() => setDense((value) => !value)}
              >
                {dense ? 'Comfortable rows' : 'Compact rows'}
              </Button>
              <Popover>
                <PopoverTrigger
                  render={
                    <Button
                      variant="outline"
                      size="icon-lg"
                      aria-label="Choose visible columns"
                    />
                  }
                >
                  <IconColumns size={19} aria-hidden="true" />
                </PopoverTrigger>
                <PopoverContent align="end" className="w-56 p-2">
                  <div className="grid gap-1">
                    {table
                      .getAllLeafColumns()
                      .filter((column) => column.getCanHide())
                      .map((column) => (
                        <Label
                          className="min-h-9 rounded-md px-2 font-normal hover:bg-muted"
                          key={column.id}
                        >
                          <Checkbox
                            checked={column.getIsVisible()}
                            onCheckedChange={(checked) =>
                              column.toggleVisibility(Boolean(checked))
                            }
                          />
                          {typeof column.columnDef.header === 'string'
                            ? column.columnDef.header
                            : column.id}
                        </Label>
                      ))}
                  </div>
                </PopoverContent>
              </Popover>
              <Popover>
                <PopoverTrigger
                  render={
                    <Button variant="outline" className="h-9" type="button" />
                  }
                >
                  <IconFilter size={18} aria-hidden="true" />
                  Filters
                  {activeFilterCount ? ` (${activeFilterCount})` : ''}
                </PopoverTrigger>
                <PopoverContent
                  align="end"
                  className="grid max-h-(--available-height) w-72 gap-3 overflow-y-auto p-3"
                >
                  {extraFilters?.content}
                  {filterableColumns.map((column) => {
                    const label =
                      typeof column.columnDef.header === 'string'
                        ? column.columnDef.header
                        : column.id;
                    const inputId = `${ariaLabel}-filter-${column.id}`.replace(
                      /\s+/g,
                      '-',
                    );
                    const options = column.columnDef.meta?.filterOptions;
                    if (options) {
                      const items = [{ label: 'All', value: '' }, ...options];
                      return (
                        <Field key={column.id}>
                          <FieldLabel htmlFor={inputId}>{label}</FieldLabel>
                          <Select
                            items={items}
                            value={
                              (column.getFilterValue() as string | undefined) ??
                              ''
                            }
                            onValueChange={(value) =>
                              column.setFilterValue(value || undefined)
                            }
                          >
                            <SelectTrigger id={inputId} className="w-full">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectGroup>
                                {items.map((item) => (
                                  <SelectItem
                                    key={item.value}
                                    value={item.value}
                                  >
                                    {item.label}
                                  </SelectItem>
                                ))}
                              </SelectGroup>
                            </SelectContent>
                          </Select>
                        </Field>
                      );
                    }
                    return (
                      <Field key={column.id}>
                        <FieldLabel htmlFor={inputId}>{label}</FieldLabel>
                        <Input
                          id={inputId}
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
                      </Field>
                    );
                  })}
                  {activeFilterCount ? (
                    <Button
                      variant="outline"
                      type="button"
                      onClick={() => {
                        setColumnFilters([]);
                        extraFilters?.onClear();
                      }}
                    >
                      Clear table filters
                    </Button>
                  ) : null}
                </PopoverContent>
              </Popover>
              <Button
                variant="ghost"
                size="icon-lg"
                type="button"
                onClick={() => setFullscreen((value) => !value)}
                aria-label={
                  fullscreen
                    ? 'Exit full screen table'
                    : 'Open full screen table'
                }
              >
                {fullscreen ? (
                  <IconArrowsMinimize size={19} aria-hidden="true" />
                ) : (
                  <IconArrowsMaximize size={19} aria-hidden="true" />
                )}
              </Button>
            </div>
          </div>
          <div className="sr-only" aria-live="polite">
            {loading ? 'Loading results…' : `${filteredRows.length} results`}
          </div>
          {loading ? (
            <div
              className="grid min-h-60 place-items-center p-8 text-center"
              aria-busy="true"
            >
              <span role="status">Loading {ariaLabel.toLowerCase()}…</span>
            </div>
          ) : error ? (
            <ErrorState onRetry={onRetry} />
          ) : !data.length || !filteredRows.length ? (
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
                globalFilter || activeFilterCount ? (
                  <Button
                    variant="outline"
                    type="button"
                    onClick={() => {
                      setGlobalFilter('');
                      setColumnFilters([]);
                      extraFilters?.onClear();
                    }}
                  >
                    Clear filters
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <>
              {compact ? null : (
                <div className="overflow-x-auto">
                  <Table
                    className="table-fixed"
                    style={{ minWidth: `${tableMinWidth}rem` }}
                  >
                    <caption className="sr-only">
                      {ariaLabel}. Sortable and filterable data table.
                    </caption>
                    <colgroup>
                      {visibleColumns.map((column) => {
                        const width = column.columnDef.meta?.width;
                        return (
                          <col
                            key={column.id}
                            style={width ? { width: `${width}rem` } : undefined}
                          />
                        );
                      })}
                    </colgroup>
                    <TableHeader>
                      {table.getHeaderGroups().map((headerGroup) => (
                        <TableRow key={headerGroup.id}>
                          {headerGroup.headers.map((header) => (
                            <TableHead
                              key={header.id}
                              scope="col"
                              className={cn(
                                'truncate px-4 text-xs font-semibold text-muted-foreground',
                                header.column.columnDef.meta?.align === 'end' &&
                                  'text-right',
                              )}
                              aria-sort={
                                header.column.getIsSorted() === 'asc'
                                  ? 'ascending'
                                  : header.column.getIsSorted() === 'desc'
                                    ? 'descending'
                                    : 'none'
                              }
                            >
                              {header.isPlaceholder ? null : header.column.getCanSort() ? (
                                <Button
                                  variant="ghost"
                                  className="h-auto justify-start p-0 text-xs font-semibold hover:bg-transparent"
                                  type="button"
                                  onClick={header.column.getToggleSortingHandler()}
                                >
                                  {flexRender(
                                    header.column.columnDef.header,
                                    header.getContext(),
                                  )}
                                  {header.column.getIsSorted() === 'asc' ? (
                                    <IconChevronUp
                                      size={15}
                                      aria-hidden="true"
                                    />
                                  ) : header.column.getIsSorted() === 'desc' ? (
                                    <IconChevronDown
                                      size={15}
                                      aria-hidden="true"
                                    />
                                  ) : null}
                                </Button>
                              ) : (
                                flexRender(
                                  header.column.columnDef.header,
                                  header.getContext(),
                                )
                              )}
                            </TableHead>
                          ))}
                        </TableRow>
                      ))}
                    </TableHeader>
                    <TableBody>
                      {pageRows.map((row) => (
                        <DataTableRecord
                          key={row.id}
                          row={row}
                          renderExpanded={renderExpanded}
                        />
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
              {compact ? (
                <div className="divide-y px-4">
                  {pageRows.map((row) => (
                    <article className="py-4" key={row.id}>
                      {renderCard(row)}
                    </article>
                  ))}
                </div>
              ) : null}
              <div className={paginationClass}>
                <Label className="gap-2">
                  <span className="text-xs font-semibold">Rows per page</span>
                  <Select
                    items={pageSizeItems}
                    value={table.getState().pagination.pageSize}
                    onValueChange={(value) => {
                      if (value == null) return;
                      const next = Number(value);
                      table.setPageSize(next);
                      localStorage.setItem('rsp-table-page-size', String(next));
                    }}
                  >
                    <SelectTrigger aria-label="Rows per page" className="w-20">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {pageSizeItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </Label>
                <span className="text-sm text-muted-foreground">
                  Page {table.getState().pagination.pageIndex + 1} of{' '}
                  {Math.max(table.getPageCount(), 1)}
                </span>
                <div className="flex items-center gap-1">
                  <Button
                    variant="ghost"
                    size="icon-lg"
                    type="button"
                    disabled={!table.getCanPreviousPage()}
                    onClick={() => table.previousPage()}
                    aria-label="Previous page"
                  >
                    <IconChevronLeft size={19} aria-hidden="true" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="icon-lg"
                    type="button"
                    disabled={!table.getCanNextPage()}
                    onClick={() => table.nextPage()}
                    aria-label="Next page"
                  >
                    <IconChevronRight size={19} aria-hidden="true" />
                  </Button>
                </div>
              </div>
            </>
          )}
          {serverPagination && !loading && !error && data.length > 0 ? (
            <>
              <span className="sr-only" aria-live="polite">
                Loaded {data.length} of {serverPagination.totalCount}
              </span>
              {serverPagination.hasMore ? (
                <div className={paginationClass}>
                  <Button
                    variant="outline"
                    type="button"
                    disabled={serverPagination.loadingMore}
                    onClick={serverPagination.onLoadMore}
                  >
                    {serverPagination.loadingMore ? 'Loading…' : 'Load more'}
                  </Button>
                </div>
              ) : null}
            </>
          ) : null}
        </section>
        {renderSummary?.(filteredRows.map((row) => row.original))}
      </div>
    </SearchHighlightContext.Provider>
  );
}

function DataTableRecord<TData>({
  row,
  renderExpanded,
}: {
  row: Row<TData>;
  renderExpanded?: (row: Row<TData>) => ReactNode;
}) {
  return (
    <>
      <TableRow>
        {row.getVisibleCells().map((cell) => (
          <TableCell
            className={cn(
              // TableCell drops the right padding around checkboxes, which suits
              // selection columns; these tables have none, so keep the inset.
              // Every row is as tall as one holding a row button (a cell's
              // height is its minimum), with or without one.
              'h-14 px-4 py-3 [&:has([role=checkbox])]:pr-4',
              cell.column.columnDef.meta?.wrap
                ? 'whitespace-normal'
                : truncateCellClass,
              cell.column.columnDef.meta?.align === 'end' &&
                'text-right [&>.flex]:justify-end',
            )}
            key={cell.id}
          >
            {renderCell(cell.column.columnDef.cell, cell.getContext())}
          </TableCell>
        ))}
      </TableRow>
      {row.getIsExpanded() && renderExpanded ? (
        <TableRow className="hover:bg-transparent">
          <TableCell
            className="whitespace-normal bg-muted/50 px-4 py-4 [&:has([role=checkbox])]:pr-4"
            colSpan={row.getVisibleCells().length}
          >
            {renderExpanded(row)}
          </TableCell>
        </TableRow>
      ) : null}
    </>
  );
}
