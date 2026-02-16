import {
  CanonicalTable,
  Cell,
  ConfigurationError,
  getLabel,
  getStyles,
  Header,
  Interactions,
  Row,
  Severity,
  StringValue,
  type ResponseNode,
} from "@traceviz/client-core";
import { Loader, Pagination, ScrollArea, Table, Text } from "@mantine/core";
import * as d3 from "d3";
import { Subject, takeUntil } from "rxjs";
import { useEffect, useReducer, useRef, useState } from "react";

import { useAppCore } from "../../core/index.ts";

const SOURCE = "data-table";

// Valid interactions targets
const ROW = "rows";
const COLUMN = "columns";
const TABLE = "table";

// Valid action types
const CLICK = "click";
const SHIFTCLICK = "shift-click";
const MOUSEOVER = "mouseover";
const MOUSEOUT = "mouseout";

// Valid reaction types
const HIGHLIGHT = "highlight";
const REDRAW = "redraw";

// Valid watch types
const UPDATE_SORT_DIRECTION = "update_sort_direction";
const UPDATE_SORT_COLUMN = "update_sort_column";

// Sort watch keys
const SORT_DIRECTION = "sort_direction";
const SORT_COLUMN = "sort_column";

// Valid sort directions
const SORT_ASC = "asc";
const SORT_DESC = "desc";
const SORT_NONE = "";

const requestAnimationFrameSafe = (cb: FrameRequestCallback): number => {
  if (
    typeof window !== "undefined" &&
    typeof window.requestAnimationFrame === "function"
  ) {
    return window.requestAnimationFrame(cb);
  }
  return window.setTimeout(() => cb(Date.now()), 0);
};

const cancelAnimationFrameSafe = (handle: number): void => {
  if (
    typeof window !== "undefined" &&
    typeof window.cancelAnimationFrame === "function"
  ) {
    window.cancelAnimationFrame(handle);
    return;
  }
  window.clearTimeout(handle);
};

const supportedActions: Array<[string, string]> = [
  [ROW, CLICK],
  [ROW, SHIFTCLICK],
  [ROW, MOUSEOVER],
  [ROW, MOUSEOUT],
  [COLUMN, CLICK],
];

const supportedReactions: Array<[string, string]> = [
  [ROW, HIGHLIGHT],
  [TABLE, REDRAW],
];

const supportedWatches = [UPDATE_SORT_DIRECTION, UPDATE_SORT_COLUMN];

type SortState = {
  active: string;
  direction: string;
};

export type DataTableProps = {
  data?: ResponseNode;
  interactions?: Interactions;
  loading?: boolean;
  className?: string;
  withPagination?: boolean;
  scrollable?: boolean;
  rowHeightPxOverride?: number;
  fontSizePxOverride?: number;
};

export function DataTable({
  data,
  interactions,
  loading = false,
  className,
  withPagination = true,
  scrollable = true,
  rowHeightPxOverride,
  fontSizePxOverride,
}: DataTableProps): JSX.Element {
  const appCore = useAppCore();
  const componentRef = useRef<HTMLDivElement | null>(null);
  const paginationRef = useRef<HTMLDivElement | null>(null);
  const headerRef = useRef<HTMLTableRowElement | null>(null);
  const sampleRowRef = useRef<HTMLTableRowElement | null>(null);
  const [table, setTable] = useState<CanonicalTable | null>(null);
  const [columns, setColumns] = useState<Header[]>([]);
  const [rows, setRows] = useState<Row[]>([]);
  const [pageIndex, setPageIndex] = useState<number>(0);
  const [pageSize, setPageSize] = useState<number>(0);
  const [sort, setSort] = useState<SortState>({
    active: "",
    direction: "",
  });
  const [, forceRender] = useReducer((n: number) => n + 1, 0);

  const rowHeightPx =
    rowHeightPxOverride ?? table?.renderProperties.rowHeightPx ?? 20;
  const fontSizePx =
    fontSizePxOverride ??
    table?.renderProperties.fontSizePx ??
    Math.round(rowHeightPx * 0.66);

  // Update the visible rows based on the current page index and size.
  const updateRows = (): void => {
    if (!table) {
      return;
    }
    rows.forEach((row) => row.dispose());
    let start = 0;
    let end = table.rowCount;
    if (withPagination && pageSize > 0) {
      start = pageSize * pageIndex;
      end = start + pageSize;
    }
    setRows(table.rowSlice(start, end));
  };

  const redraw = (): void => {
    if (!table || !componentRef.current || !withPagination) {
      updateRows();
      return;
    }
    const container = componentRef.current;
    const height = container.offsetHeight;
    const footerHeight = paginationRef.current?.offsetHeight ?? 0;
    const headerHeight = headerRef.current?.offsetHeight ?? rowHeightPx;
    const measuredRowHeight =
      sampleRowRef.current?.offsetHeight ?? rowHeightPx;
    const computedStyle = window.getComputedStyle(container);
    const paddingTop = parseFloat(computedStyle.paddingTop) || 0;
    const paddingBottom = parseFloat(computedStyle.paddingBottom) || 0;
    const availableHeight = Math.max(
      0,
      height - footerHeight - headerHeight - paddingTop - paddingBottom,
    );
    const rowsPerPage = Math.max(
      1,
      Math.floor(availableHeight / Math.max(measuredRowHeight, 1)),
    );
    console.log("[DataTable] sizing", {
      height,
      footerHeight,
      headerHeight,
      measuredRowHeight,
      paddingTop,
      paddingBottom,
      availableHeight,
      rowsPerPage,
    });
    setPageSize(rowsPerPage);
    setPageIndex(0);
  };

  // Initialize 'table', 'column', and 'rows' when new data is available.
  useEffect(() => {
    if (!data) {
      setTable(null);
      setColumns([]);
      setRows([]);
      return;
    }
    try {
      const rowMatch = interactions?.match(ROW, HIGHLIGHT);
      const nextTable = new CanonicalTable(data, rowMatch, undefined, () => {
        forceRender();
      });
      setTable(nextTable);
      setColumns(nextTable.columns());
      setPageIndex(0);
      setRows(nextTable.rowSlice(0, nextTable.rowCount));
    } catch (err: unknown) {
      appCore.err(err);
    }
  }, [appCore, data, interactions]);

  useEffect(() => {
    updateRows();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pageIndex, pageSize, table]);

  useEffect(() => {
    if (!componentRef.current) {
      return;
    }
    const handle = requestAnimationFrameSafe(() => {
      redraw();
    });
    return () => cancelAnimationFrameSafe(handle);
  }, [table, withPagination, rowHeightPx]);

  useEffect(() => {
    if (!withPagination) {
      return;
    }
    const ro = new ResizeObserver(() => {
      redraw();
    });
    const nodes: Array<Element> = [];
    if (componentRef.current) {
      nodes.push(componentRef.current);
    }
    if (paginationRef.current) {
      nodes.push(paginationRef.current);
    }
    if (headerRef.current) {
      nodes.push(headerRef.current);
    }
    if (sampleRowRef.current) {
      nodes.push(sampleRowRef.current);
    }
    nodes.forEach((node) => ro.observe(node));
    return () => ro.disconnect();
  }, [withPagination, rowHeightPx, rows.length]);

  useEffect(() => {
    if (!interactions) {
      return;
    }
    try {
      interactions.checkForSupportedActions(supportedActions);
      interactions.checkForSupportedReactions(supportedReactions);
      interactions.checkForSupportedWatches(supportedWatches);
    } catch (err: unknown) {
      appCore.err(err);
    }
  }, [appCore, interactions]);

  useEffect(() => {
    if (!interactions) {
      return;
    }
    const unsubscribe = new Subject<void>();
    const watchErrors = interactions.watchAll(
      new Map<string, (vm) => void>([
        [
          UPDATE_SORT_DIRECTION,
          (vm) => {
            const sortDirectionVal = vm.get(SORT_DIRECTION);
            if (sortDirectionVal instanceof StringValue) {
              if (
                sortDirectionVal.val === SORT_ASC ||
                sortDirectionVal.val === SORT_DESC ||
                sortDirectionVal.val === SORT_NONE
              ) {
                setSort((prev) => ({
                  active: prev.active,
                  direction: sortDirectionVal.val,
                }));
              } else {
                appCore.err(
                  new ConfigurationError(
                    `${SORT_DIRECTION} on the ${UPDATE_SORT_DIRECTION} watch can only be 'asc', 'desc', or ''`,
                  )
                    .from(SOURCE)
                    .at(Severity.ERROR),
                );
              }
            } else {
              appCore.err(
                new ConfigurationError(
                  `${SORT_DIRECTION} on the ${UPDATE_SORT_DIRECTION} watch only supports string contents`,
                )
                  .from(SOURCE)
                  .at(Severity.ERROR),
              );
            }
          },
        ],
        [
          UPDATE_SORT_COLUMN,
          (vm) => {
            const sortColumnVal = vm.get(SORT_COLUMN);
            if (sortColumnVal instanceof StringValue) {
              setSort((prev) => ({
                active: sortColumnVal.val,
                direction: prev.direction,
              }));
            } else {
              appCore.err(
                new ConfigurationError(
                  `${SORT_COLUMN} on the ${UPDATE_SORT_COLUMN} watch only supports string contents`,
                )
                  .from(SOURCE)
                  .at(Severity.ERROR),
              );
            }
          },
        ],
      ]),
      unsubscribe,
    );
    const redrawSub = interactions
      .match(
        TABLE,
        REDRAW,
      )(undefined)
      .pipe(takeUntil(unsubscribe))
      .subscribe((changed: boolean) => {
        if (changed) {
          requestAnimationFrameSafe(() => {
            redraw();
          });
        }
      });
    const errSub = watchErrors.pipe(takeUntil(unsubscribe)).subscribe((err) => {
      appCore.err(err);
    });
    return () => {
      unsubscribe.next();
      unsubscribe.complete();
      errSub.unsubscribe();
      redrawSub.unsubscribe();
    };
  }, [appCore, interactions]);

  const pageCount =
    table && pageSize > 0 ? Math.ceil(table.rowCount / pageSize) : 1;

  const handleSort = (column: Header): void => {
    let nextDirection = SORT_ASC;
    if (sort.active === column.category.id) {
      nextDirection =
        sort.direction === SORT_ASC
          ? SORT_DESC
          : sort.direction === SORT_DESC
            ? SORT_NONE
            : SORT_ASC;
    }
    const properties = column.properties.with([
      SORT_DIRECTION,
      new StringValue(nextDirection),
    ]);
    interactions?.update(COLUMN, CLICK, properties);
    setSort({ active: column.category.id, direction: nextDirection });
  };

  const rowClick = (row: Row, shiftDepressed: boolean): void => {
    if (!shiftDepressed) {
      interactions?.update(ROW, CLICK, row.properties);
    } else {
      interactions?.update(ROW, SHIFTCLICK, row.properties);
    }
  };

  const rowMouseover = (row: Row): void => {
    interactions?.update(ROW, MOUSEOVER, row.properties);
  };

  const rowMouseout = (row: Row): void => {
    interactions?.update(ROW, MOUSEOUT, row.properties);
  };

  const rowStyle = (row: Row): React.CSSProperties => {
    const style: React.CSSProperties = {};
    if (!table) {
      return style;
    }
    try {
      const rowColors = table.coloring.colors(row.properties);
      if (row.highlighted) {
        if (rowColors.primary) {
          const d3Color = d3.color(rowColors.primary);
          if (d3Color) {
            style.backgroundColor = d3Color.brighter(2).toString();
          }
        } else if (rowColors.secondary) {
          style.backgroundColor = rowColors.secondary;
        }
      } else if (rowColors.primary) {
        style.backgroundColor = rowColors.primary;
      }
      if (rowColors.stroke) {
        style.color = rowColors.stroke;
      }
    } catch (err: unknown) {
      appCore.err(err);
    }
    return style;
  };

  const cellStyle = (cell: Cell, row: Row): React.CSSProperties => {
    const style = rowStyle(row);
    if (!table) {
      return style;
    }
    const cellStyles = getStyles(cell.properties);
    for (const key of Object.keys(cellStyles)) {
      style[key] = cellStyles[key];
    }
    try {
      const cellColors = table.coloring.colors(cell.properties);
      if (cellColors.primary) {
        style.backgroundColor = cellColors.primary;
      }
      if (cellColors.stroke) {
        style.color = cellColors.stroke;
      }
    } catch (err: unknown) {
      appCore.err(err);
    }
    return style;
  };

  const cellLabel = (cell: Cell): string => getLabel(cell.properties);

  return (
    <div ref={componentRef} className={className}>
      {loading ? <Loader size="sm" /> : null}
      {table === null ? (
        <Text size="sm" c="dimmed">
          No data.
        </Text>
      ) : scrollable ? (
        <ScrollArea className="table-scroll">
          <Table
            striped
            highlightOnHover
            withTableBorder
            withColumnBorders
            style={{ fontSize: `${fontSizePx}px` }}
          >
            <Table.Thead>
            <Table.Tr ref={headerRef} style={{ height: rowHeightPx }}>
              {columns.map((column) => (
                  <Table.Th
                    key={column.category.id}
                    title={column.category.description}
                    onClick={() => handleSort(column)}
                    style={{ textAlign: "left", fontSize: "75%" }}
                  >
                    {column.category.displayName}
                  </Table.Th>
                ))}
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rows.map((row, idx) => (
                <Table.Tr
                  key={idx}
                  ref={idx === 0 ? sampleRowRef : undefined}
                  style={{ height: rowHeightPx, ...rowStyle(row) }}
                  onMouseOver={() => rowMouseover(row)}
                  onMouseOut={() => rowMouseout(row)}
                  onClick={(ev) => rowClick(row, ev.shiftKey)}
                >
                  {row.cells(columns).map((cell, cellIdx) => (
                    <Table.Td
                      key={`${idx}-${cellIdx}`}
                      title={cellLabel(cell) || cell.value.toString()}
                    >
                      <div style={cellStyle(cell, row)}>
                        {cell.value.toString()}
                      </div>
                    </Table.Td>
                  ))}
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </ScrollArea>
      ) : (
        <div className="table-content">
          <Table
            striped
            highlightOnHover
            withTableBorder
            withColumnBorders
            style={{ fontSize: `${fontSizePx}px` }}
          >
            <Table.Thead>
            <Table.Tr ref={headerRef} style={{ height: rowHeightPx }}>
              {columns.map((column) => (
                  <Table.Th
                    key={column.category.id}
                    title={column.category.description}
                    onClick={() => handleSort(column)}
                    style={{ textAlign: "left", fontSize: "75%" }}
                  >
                    {column.category.displayName}
                  </Table.Th>
                ))}
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
            {rows.map((row, idx) => (
              <Table.Tr
                key={idx}
                ref={idx === 0 ? sampleRowRef : undefined}
                style={{ height: rowHeightPx, ...rowStyle(row) }}
                onMouseOver={() => rowMouseover(row)}
                onMouseOut={() => rowMouseout(row)}
                onClick={(ev) => rowClick(row, ev.shiftKey)}
              >
                  {row.cells(columns).map((cell, cellIdx) => (
                    <Table.Td
                      key={`${idx}-${cellIdx}`}
                      title={cellLabel(cell) || cell.value.toString()}
                    >
                      <div style={cellStyle(cell, row)}>
                        {cell.value.toString()}
                      </div>
                    </Table.Td>
                  ))}
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </div>
      )}
      {withPagination ? (
        <div ref={paginationRef} className="table-pagination">
          <Pagination
            value={pageIndex + 1}
            onChange={(page) => setPageIndex(page - 1)}
            total={pageCount}
            mt="sm"
          />
        </div>
      ) : null}
    </div>
  );
}
