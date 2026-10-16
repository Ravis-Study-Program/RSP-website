import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
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
import { cn } from '@/lib/utils';
import { useMediaQuery } from '@/lib/useMediaQuery';

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

const toolbarClass =
  'flex flex-wrap items-center justify-between gap-3 border-b p-3 max-[56rem]:flex-col max-[56rem]:items-stretch';
const paginationClass =
  'flex flex-wrap items-center justify-between gap-3 border-t p-3 max-[56rem]:flex-col max-[56rem]:items-stretch';

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

  const compact = useMediaQuery('(max-width: 56rem)');
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
      <div
        className="overflow-hidden rounded-lg border bg-card"
        aria-busy="true"
      >
        <div className={toolbarClass}>
          <Skeleton className="h-9 w-full max-w-96 text-transparent">
            Loading
          </Skeleton>
        </div>
        <div className="grid min-h-60 place-items-center p-8 text-center">
          <span role="status">Loading {ariaLabel.toLowerCase()}…</span>
        </div>
      </div>
    );
  }

  if (error)
    return (
      <div className="overflow-hidden rounded-lg border bg-card">
        <ErrorState onRetry={onRetry} />
      </div>
    );

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
      <section
        className={cn(
          'overflow-hidden rounded-lg border bg-card',
          dense && '[&_tbody_td]:py-2 [&_thead_th]:h-9 [&_tbody_td]:text-sm',
          fullscreen && 'fixed inset-0 z-60 overflow-auto rounded-none',
        )}
        aria-label={ariaLabel}
      >
        <div className={toolbarClass}>
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
                {columnFilters.length ? ` (${columnFilters.length})` : ''}
              </PopoverTrigger>
              <PopoverContent align="end" className="grid w-72 gap-3 p-3">
                {filterableColumns.map((column) => {
                  const label =
                    typeof column.columnDef.header === 'string'
                      ? column.columnDef.header
                      : column.id;
                  const inputId = `${ariaLabel}-filter-${column.id}`.replace(
                    /\s+/g,
                    '-',
                  );
                  return (
                    <Field key={column.id}>
                      <FieldLabel htmlFor={inputId}>{label}</FieldLabel>
                      <Input
                        id={inputId}
                        value={
                          (column.getFilterValue() as string | undefined) ?? ''
                        }
                        onChange={(event) =>
                          column.setFilterValue(event.target.value || undefined)
                        }
                        placeholder={`Filter ${label.toLowerCase()}`}
                      />
                    </Field>
                  );
                })}
                {columnFilters.length ? (
                  <Button
                    variant="outline"
                    type="button"
                    onClick={() => setColumnFilters([])}
                  >
                    Clear column filters
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
                fullscreen ? 'Exit full screen table' : 'Open full screen table'
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
                <Button
                  variant="outline"
                  type="button"
                  onClick={() => {
                    setGlobalFilter('');
                    setColumnFilters([]);
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
                <Table>
                  <caption className="sr-only">
                    {ariaLabel}. Sortable and filterable data table.
                  </caption>
                  <TableHeader>
                    {table.getHeaderGroups().map((headerGroup) => (
                      <TableRow key={headerGroup.id}>
                        {headerGroup.headers.map((header) => (
                          <TableHead
                            key={header.id}
                            scope="col"
                            className="px-3.5 text-xs font-semibold text-muted-foreground"
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
                                  <IconChevronUp size={15} aria-hidden="true" />
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
                <NativeSelect
                  className="w-20"
                  value={table.getState().pagination.pageSize}
                  onChange={(event) => {
                    const value = Number(event.target.value);
                    table.setPageSize(value);
                    localStorage.setItem('rsp-table-page-size', String(value));
                  }}
                >
                  {[10, 25, 50, 100].map((value) => (
                    <NativeSelectOption key={value} value={value}>
                      {value}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
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
      </section>
      {renderSummary?.(filteredRows.map((row) => row.original))}
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
          <TableCell className="px-3.5 py-3" key={cell.id}>
            {flexRender(cell.column.columnDef.cell, cell.getContext())}
          </TableCell>
        ))}
      </TableRow>
      {row.getIsExpanded() && renderExpanded ? (
        <TableRow className="hover:bg-transparent">
          <TableCell
            className="whitespace-normal bg-muted/50 px-4 py-4"
            colSpan={row.getVisibleCells().length}
          >
            {renderExpanded(row)}
          </TableCell>
        </TableRow>
      ) : null}
    </>
  );
}
