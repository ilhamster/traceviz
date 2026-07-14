import { ReactNode, useEffect, useMemo, useReducer, useRef, useState } from "react";
import {
  ConfigurationError,
  Coloring,
  Interactions,
  RenderedCategory,
  RenderedTraceEdge,
  RenderedTraceSpan,
  RenderedCategoryHierarchy,
  Severity,
  StringValue,
  Trace,
  Value,
  ValueMap,
  getLabel,
  renderCategoryHierarchyForHorizontalSpans,
  renderHorizontalTraceSpans,
} from "@traceviz/client-core";
import * as d3 from "d3";
import { distinctUntilChanged, Subject, takeUntil } from "rxjs";
import {
  ContinuousAxisRenderSettings,
  xAxisRenderSettings,
} from "../axes/continuous_axis_x.tsx";
import { useAppCore } from "../../core/index.ts";
import {
  CALLED_OUT_CATEGORY_ID_KEY,
  CATEGORY_ID_KEY,
  UPDATE_CALLED_OUT_CATEGORY_WATCH,
} from "./category_callout.ts";

const SOURCE = "horizontal-trace";

const DETAIL_FORMAT_KEY = "detail_format";
const TOOLTIP_KEY = "tooltip";
const CALLED_OUT_CATEGORY_BAND_COLOR = "#aaa";
const CALLED_OUT_CATEGORY_BAND_OPACITY = 0.35;
export const TRACE_SPANS_TARGET = "trace_spans";
export const TRACE_EDGES_TARGET = "trace_edges";
export const TRACE_SPAN_CLICK_ACTION = "click";
export const TRACE_HIGHLIGHT_REACTION = "highlight";

const supportedActions: Array<[string, string]> = [
  [TRACE_SPANS_TARGET, TRACE_SPAN_CLICK_ACTION],
];
const supportedReactions: Array<[string, string]> = [
  [TRACE_SPANS_TARGET, TRACE_HIGHLIGHT_REACTION],
  [TRACE_EDGES_TARGET, TRACE_HIGHLIGHT_REACTION],
];
const supportedWatches = [UPDATE_CALLED_OUT_CATEGORY_WATCH];

type CalledOutCategoryState = {
  value: Value;
};

function spanTooltip(span: RenderedTraceSpan): string {
  if (span.properties.has(TOOLTIP_KEY)) {
    return span.properties.expectString(TOOLTIP_KEY);
  }
  if (span.properties.has(DETAIL_FORMAT_KEY)) {
    return span.properties.format(span.properties.expectString(DETAIL_FORMAT_KEY));
  }
  return getLabel(span.properties);
}

function calledOutCategory(
  renderedCategories: RenderedCategoryHierarchy,
  calledOutCategoryID: Value | null,
  categoryIDKey: string | null,
): RenderedCategory | null {
  if (calledOutCategoryID === null || categoryIDKey === null) {
    return null;
  }
  for (const renderedCategory of renderedCategories.categories) {
    if (
      renderedCategory.properties.has(categoryIDKey) &&
      calledOutCategoryID.compare(
        renderedCategory.properties.get(categoryIDKey),
      ) === 0
    ) {
      return renderedCategory;
    }
  }
  return null;
}

function stringLikeValue(value: unknown): string | null {
  if (value instanceof StringValue) {
    return value.val;
  }
  if (typeof value === "string") {
    return value;
  }
  if (
    value !== null &&
    typeof value === "object" &&
    "val" in value &&
    typeof (value as { val: unknown }).val === "string"
  ) {
    return (value as { val: string }).val;
  }
  return null;
}

export type HorizontalTraceYAxisSlotProps<T> = {
  trace: Trace<T>;
  renderedCategories: RenderedCategoryHierarchy;
  traceHeightPx: number;
  interactions?: Interactions;
};

export type HorizontalTraceXAxisSlotProps<T> = {
  trace: Trace<T>;
  renderSettings: ContinuousAxisRenderSettings;
  widthPx: number;
  heightPx: number;
  interactions?: Interactions;
};

export type HorizontalTraceProps<T> = {
  trace: Trace<T>;
  widthPx: number;
  interactions?: Interactions;
  yAxisInteractions?: Interactions;
  xAxisInteractions?: Interactions;
  renderYAxis?: (props: HorizontalTraceYAxisSlotProps<T>) => ReactNode;
  renderXAxis?: (props: HorizontalTraceXAxisSlotProps<T>) => ReactNode;
  transitionDurationMs?: number;
  className?: string;
};

export function HorizontalTrace<T>(
  props: HorizontalTraceProps<T>,
): JSX.Element | null {
  const {
    trace,
    widthPx,
    interactions,
    yAxisInteractions,
    xAxisInteractions,
    renderYAxis,
    renderXAxis,
    transitionDurationMs = 300,
    className,
  } = props;
  const appCore = useAppCore();
  const svgRef = useRef<SVGSVGElement | null>(null);
  const [calledOutCategoryID, setCalledOutCategoryID] =
    useState<CalledOutCategoryState | null>(null);
  const [calledOutCategoryIDKey, setCalledOutCategoryIDKey] =
    useState<string | null>(null);
  const [highlightRevision, redrawHighlights] = useReducer(
    (revision: number) => revision + 1,
    0,
  );

  const renderedCategories: RenderedCategoryHierarchy | null = useMemo(() => {
    try {
      return renderCategoryHierarchyForHorizontalSpans(trace);
    } catch (err: unknown) {
      appCore.err(
        err instanceof Error ? err : new ConfigurationError(String(err)),
      );
      return null;
    }
  }, [appCore, trace]);

  const traceAreaWidthPx =
    renderedCategories === null
      ? 0
      : Math.max(0, widthPx - renderedCategories.widthPx);

  const renderedTrace = useMemo(() => {
    if (renderedCategories === null || traceAreaWidthPx <= 0) {
      return null;
    }
    try {
      return renderHorizontalTraceSpans(trace, traceAreaWidthPx);
    } catch (err: unknown) {
      appCore.err(
        err instanceof Error ? err : new ConfigurationError(String(err)),
      );
      return null;
    }
  }, [appCore, renderedCategories, trace, traceAreaWidthPx]);

  const axisRenderSettings: ContinuousAxisRenderSettings | null = useMemo(() => {
    try {
      return xAxisRenderSettings(trace.properties);
    } catch (err: unknown) {
      appCore.err(
        err instanceof Error ? err : new ConfigurationError(String(err)),
      );
      return null;
    }
  }, [appCore, trace]);
  const axisDepthPx = axisRenderSettings?.axisDepthPx() ?? 0;
  const traceHeightPx = renderedCategories.heightPx;

  useEffect(() => {
    if (!interactions) {
      return;
    }
    try {
      interactions.checkForSupportedActions(supportedActions);
      interactions.checkForSupportedReactions(supportedReactions);
      interactions.checkForSupportedWatches(supportedWatches);
    } catch (err: unknown) {
      appCore.err(
        err instanceof Error ? err : new ConfigurationError(String(err)),
      );
    }

    const unsubscribe = new Subject<void>();
    const watchErrors = interactions.watch(
      UPDATE_CALLED_OUT_CATEGORY_WATCH,
      (vm: ValueMap) => {
        if (!vm.has(CALLED_OUT_CATEGORY_ID_KEY)) {
          throw new ConfigurationError(
            `watch '${UPDATE_CALLED_OUT_CATEGORY_WATCH}' requires '${CALLED_OUT_CATEGORY_ID_KEY}'`,
          )
            .from(SOURCE)
            .at(Severity.ERROR);
        }
        if (!vm.has(CATEGORY_ID_KEY)) {
          throw new ConfigurationError(
            `watch '${UPDATE_CALLED_OUT_CATEGORY_WATCH}' requires '${CATEGORY_ID_KEY}'`,
          )
            .from(SOURCE)
            .at(Severity.ERROR);
        }
        const nextCategoryID = vm.get(CALLED_OUT_CATEGORY_ID_KEY);
        const nextCategoryIDKey = stringLikeValue(vm.get(CATEGORY_ID_KEY));
        if (nextCategoryIDKey === null) {
          throw new ConfigurationError(
            `'${CATEGORY_ID_KEY}' on watch '${UPDATE_CALLED_OUT_CATEGORY_WATCH}' must be a string`,
          )
            .from(SOURCE)
            .at(Severity.ERROR);
        }
        setCalledOutCategoryID({ value: nextCategoryID });
        setCalledOutCategoryIDKey(nextCategoryIDKey);
      },
      unsubscribe,
    );
    const errSub = watchErrors.subscribe((err) => appCore.err(err));
    return () => {
      unsubscribe.next();
      unsubscribe.complete();
      errSub.unsubscribe();
    };
  }, [appCore, interactions]);

  // Applies generic TraceViz highlight reactions to rendered spans, subspans,
  // and edges. Predicates observe only each feature's properties and Values
  // owned by the embedding tool; this component does not interpret semantics.
  useEffect(() => {
    if (!interactions || renderedTrace === null) {
      return;
    }
    const unsubscribe = new Subject<void>();
    const subscribe = (
      target: string,
      renderedItem: RenderedTraceSpan | RenderedTraceEdge,
    ): void => {
      interactions
        .match(target, TRACE_HIGHLIGHT_REACTION)(renderedItem.properties)
        .pipe(distinctUntilChanged(), takeUntil(unsubscribe))
        .subscribe({
          next: (highlighted: boolean) => {
            if (renderedItem.highlighted !== highlighted) {
              renderedItem.highlighted = highlighted;
              redrawHighlights();
            }
          },
          error: (err: unknown) => appCore.err(err),
        });
    };
    renderedTrace.spans.forEach((span) => subscribe(TRACE_SPANS_TARGET, span));
    renderedTrace.edges.forEach((edge) => subscribe(TRACE_EDGES_TARGET, edge));
    return () => {
      unsubscribe.next();
      unsubscribe.complete();
    };
  }, [appCore, interactions, renderedTrace]);

  useEffect(() => {
    if (
      !svgRef.current ||
      traceAreaWidthPx <= 0 ||
      renderedCategories === null ||
      renderedTrace === null
    ) {
      return;
    }

    const heightPx = renderedCategories.heightPx;
    const svg = d3.select(svgRef.current);
    svg.attr("width", traceAreaWidthPx).attr("height", heightPx);

    const coloring = new Coloring(trace.properties);
    const primaryOrSecondary = (
      properties: RenderedTraceSpan["properties"],
      highlighted: boolean,
    ) =>
      highlighted
        ? coloring.colors(properties).secondary ||
          coloring.colors(properties).primary ||
          ""
        : coloring.colors(properties).primary || "";
    const strokeOrSecondary = (
      properties: RenderedTraceSpan["properties"],
      highlighted: boolean,
    ) =>
      highlighted
        ? coloring.colors(properties).secondary ||
          coloring.colors(properties).stroke ||
          ""
        : coloring.colors(properties).stroke || "";
    const highlightGlow = (
      properties: RenderedTraceSpan["properties"],
      highlighted: boolean,
    ): string | null => {
      if (!highlighted) {
        return null;
      }
      const colors = coloring.colors(properties);
      const glowColor = colors.secondary || colors.primary || colors.stroke;
      return glowColor
        ? `drop-shadow(0 0 3px ${glowColor}) drop-shadow(0 0 1px ${glowColor})`
        : null;
    };

    svg
      .select<SVGGElement>(".spans")
      .attr("width", traceAreaWidthPx)
      .attr("height", heightPx);
    svg
      .select<SVGGElement>(".edges")
      .attr("width", traceAreaWidthPx)
      .attr("height", heightPx)
      .style("pointer-events", "none");
    const categoryBand = calledOutCategory(
      renderedCategories,
      calledOutCategoryID?.value ?? null,
      calledOutCategoryIDKey,
    );
    const categoryBandNodes = svg
      .select(".called-out-category-band")
      .selectAll<SVGRectElement, RenderedCategory>("rect")
      .data(categoryBand ? [categoryBand] : [], (d) => d.category.id);
    categoryBandNodes.exit().remove();
    const enteredCategoryBands = categoryBandNodes.enter().append("rect");
    enteredCategoryBands
      .merge(categoryBandNodes)
      .attr("x", 0)
      .attr("y", (category) => category.y0Px)
      .attr("width", traceAreaWidthPx)
      .attr("height", (category) => category.height)
      .attr("fill", CALLED_OUT_CATEGORY_BAND_COLOR)
      .attr("opacity", CALLED_OUT_CATEGORY_BAND_OPACITY);

    const spanNodes = svg
      .select(".spans")
      .selectAll<SVGSVGElement, RenderedTraceSpan>("svg")
      .data(renderedTrace.spans, (d) => d.renderID);
    spanNodes.exit().remove();

    const enteredSpans = spanNodes.enter().append("svg");
    enteredSpans.append("rect");
    enteredSpans.append("text");
    enteredSpans.append("title");

    const mergedSpans = enteredSpans.merge(spanNodes);
    mergedSpans
      .attr("x", (rs) => rs.x0Px)
      .attr("y", (rs) => rs.y0Px)
      .attr("width", (rs) => (rs.width === 0 ? 1 : rs.width))
      .attr("height", (rs) => rs.height)
      .style("overflow", "visible");

    mergedSpans
      .select("rect")
      .attr("width", (rs) => (rs.width === 0 ? 1 : rs.width))
      .attr("height", (rs) => rs.height)
      .attr("fill", (rs) => primaryOrSecondary(rs.properties, rs.highlighted))
      .style("filter", (rs) => highlightGlow(rs.properties, rs.highlighted));

    mergedSpans
      .select("text")
      .attr("dominant-baseline", "hanging")
      .attr("y", 1)
      .attr("fill", (rs) => strokeOrSecondary(rs.properties, rs.highlighted))
      .text((rs) => getLabel(rs.properties));

    mergedSpans.select("title").text(spanTooltip);
    mergedSpans
      .style("cursor", interactions ? "pointer" : null)
      .on("click", (event: MouseEvent, span: RenderedTraceSpan) => {
        if (!interactions) {
          return;
        }
        try {
          interactions.update(TRACE_SPANS_TARGET, TRACE_SPAN_CLICK_ACTION, span.properties);
        } catch (err: unknown) {
          appCore.err(
            err instanceof Error ? err : new ConfigurationError(String(err)),
          );
        }
        event.stopPropagation();
      });

    if (transitionDurationMs > 0) {
      mergedSpans
        .transition()
        .duration(transitionDurationMs)
        .attr("x", (rs) => rs.x0Px)
        .attr("y", (rs) => rs.y0Px)
        .attr("width", (rs) => (rs.width === 0 ? 1 : rs.width))
        .attr("height", (rs) => rs.height);

      mergedSpans
        .select("rect")
        .transition()
        .duration(transitionDurationMs)
        .attr("width", (rs) => (rs.width === 0 ? 1 : rs.width))
        .attr("height", (rs) => rs.height)
        .attr("fill", (rs) =>
          primaryOrSecondary(rs.properties, rs.highlighted),
        );
    }

    const edgeNodes = svg
      .select(".edges")
      .selectAll<SVGLineElement, RenderedTraceEdge>("line")
      .data(renderedTrace.edges, (d) => d.renderID);
    edgeNodes.exit().remove();
    const enteredEdges = edgeNodes.enter().append("line");
    const mergedEdges = enteredEdges.merge(edgeNodes);
    mergedEdges
      .attr("x1", (re) => re.x0Px)
      .attr("y1", (re) => re.y0Px)
      .attr("x2", (re) => re.x1Px)
      .attr("y2", (re) => re.y1Px)
      .attr("stroke", (re) => strokeOrSecondary(re.properties, re.highlighted))
      .style("filter", (re) => highlightGlow(re.properties, re.highlighted));

    if (transitionDurationMs > 0) {
      mergedEdges
        .transition()
        .duration(transitionDurationMs)
        .attr("x1", (re) => re.x0Px)
        .attr("y1", (re) => re.y0Px)
        .attr("x2", (re) => re.x1Px)
        .attr("y2", (re) => re.y1Px)
        .attr("stroke", (re) =>
          strokeOrSecondary(re.properties, re.highlighted),
        );
    }
  }, [
    renderedCategories.heightPx,
    calledOutCategoryID,
    calledOutCategoryIDKey,
    renderedTrace,
    trace,
    traceAreaWidthPx,
    transitionDurationMs,
    interactions,
    appCore,
    highlightRevision,
  ]);

  if (renderedCategories === null) {
    return null;
  }

  return (
    <div
      className={className}
      style={{
        display: "grid",
        gridTemplateColumns: `${renderedCategories.widthPx}px 1fr`,
        gridTemplateRows: `${traceHeightPx}px ${axisDepthPx}px`,
        alignItems: "start",
      }}
    >
      <div style={{ gridColumn: "1", gridRow: "1" }}>
        {renderYAxis
          ? renderYAxis({
              trace,
              renderedCategories,
              traceHeightPx,
              interactions: yAxisInteractions,
            })
          : null}
      </div>
      <div style={{ gridColumn: "2", gridRow: "1" }}>
        {traceAreaWidthPx > 0 && renderedTrace !== null ? (
          <svg ref={svgRef} style={{ display: "block" }}>
            <g className="called-out-category-band" />
            <g className="spans" />
            <g className="edges" />
          </svg>
        ) : null}
      </div>
      <div style={{ gridColumn: "1", gridRow: "2" }} />
      <div style={{ gridColumn: "2", gridRow: "2" }}>
        {traceAreaWidthPx > 0 && axisRenderSettings !== null && renderXAxis
          ? renderXAxis({
              trace,
              renderSettings: axisRenderSettings,
              widthPx: traceAreaWidthPx,
              heightPx: axisDepthPx,
              interactions: xAxisInteractions,
            })
          : null}
      </div>
    </div>
  );
}
