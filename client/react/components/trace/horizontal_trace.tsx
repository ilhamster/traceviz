import { ReactNode, useEffect, useMemo, useRef, useState } from "react";
import {
  AxisType,
  ConfigurationError,
  Coloring,
  DoubleValue,
  Duration,
  DurationValue,
  IntegerValue,
  Interactions,
  RenderedCategory,
  RenderedTraceEdge,
  RenderedTraceSpan,
  RenderedCategoryHierarchy,
  Severity,
  StringValue,
  Timestamp,
  TimestampValue,
  Trace,
  Value,
  ValueMap,
  getLabel,
  renderCategoryHierarchyForHorizontalSpans,
  renderHorizontalTraceSpans,
} from "@traceviz/client-core";
import * as d3 from "d3";
import { Subject } from "rxjs";
import {
  ContinuousAxisRenderSettings,
  domainFromAxis,
  xAxisRenderSettings,
} from "../axes/continuous_axis_x.tsx";
import { useAppCore } from "../../core/index.ts";
import {
  CALLED_OUT_CATEGORY_ID_KEY,
  CATEGORY_ID_KEY,
  UPDATE_CALLED_OUT_CATEGORY_WATCH,
} from "./category_callout.ts";

const SOURCE = "horizontal-trace";

const OPERATION_NAME_KEY = "operation_name";
const PROCESS_ID_KEY = "process_id";
const SERVICE_NAME_KEY = "service_name";
const SPAN_ID_KEY = "span_id";
const SPAN_KIND_KEY = "span_kind";
const SPAN_NAME_KEY = "span_name";
const SUBSPAN_KIND_KEY = "subspan_kind";
const TRACE_START_KEY = "trace_start";
const TRACE_END_KEY = "trace_end";
const CALLED_OUT_CATEGORY_BAND_COLOR = "#aaa";
const CALLED_OUT_CATEGORY_BAND_OPACITY = 0.35;
const CAUSAL_EVENT_COUNT_KEY = "causal_event_count";
const SUSPEND_COUNT_KEY = "suspend_count";
const CONCURRENCY_AVG_KEY = "concurrency_avg";
const CONCURRENCY_PEAK_KEY = "concurrency_peak";
const EVENT_DEPENDENCY_KEY = "event_dependency_key";
const EVENT_DEPENDENCY_TYPE_KEY = "event_dependency_type";
const EVENT_DETAIL_KEY = "event_detail";
const EVENT_DISPLAY_NAME_KEY = "event_display_name";
const EVENT_LABEL_KEY = "event_label";
const EVENT_TIME_KEY = "event_time";
const EVENT_TYPE_KEY = "event_type";
export const TRACE_SPANS_TARGET = "trace_spans";
export const TRACE_SPAN_CLICK_ACTION = "click";
export const TRACE_CHART_TARGET = "chart";
export const TRACE_BRUSH_ACTION = "brush";
export const TRACE_RESET_ZOOM_ACTION = "reset_zoom";
export const TRACE_ZOOM_START_KEY = "zoom_start";
export const TRACE_ZOOM_END_KEY = "zoom_end";

const supportedActions: Array<[string, string]> = [
  [TRACE_SPANS_TARGET, TRACE_SPAN_CLICK_ACTION],
  [TRACE_CHART_TARGET, TRACE_BRUSH_ACTION],
  [TRACE_CHART_TARGET, TRACE_RESET_ZOOM_ACTION],
];
const supportedReactions: Array<[string, string]> = [];
const supportedWatches = [UPDATE_CALLED_OUT_CATEGORY_WATCH];

type CalledOutCategoryState = {
  value: Value;
};

function optionalString(properties: RenderedTraceSpan["properties"], key: string): string | null {
	if (!properties.has(key)) {
		return null;
	}
	return properties.expectString(key);
}

function optionalInteger(properties: RenderedTraceSpan["properties"], key: string): number | null {
	if (!properties.has(key)) {
		return null;
	}
	const value = properties.get(key);
	if (value instanceof IntegerValue) {
		return value.val;
	}
	return null;
}

function optionalDouble(properties: RenderedTraceSpan["properties"], key: string): number | null {
	if (!properties.has(key)) {
		return null;
	}
	const value = properties.get(key);
	if (value instanceof DoubleValue) {
		return value.val;
	}
	return null;
}

function valueDifferenceString(start: Value, end: Value): string | null {
  if (start instanceof DurationValue && end instanceof DurationValue) {
    return end.val.sub(start.val).toString();
  }
  if (start instanceof TimestampValue && end instanceof TimestampValue) {
    return end.val.sub(start.val).toString();
  }
  if (
    (start instanceof IntegerValue || start instanceof DoubleValue) &&
    (end instanceof IntegerValue || end instanceof DoubleValue)
  ) {
    return (end.val - start.val).toString();
  }
  return null;
}

function spanTooltip(span: RenderedTraceSpan): string {
  const name =
    optionalString(span.properties, SPAN_NAME_KEY) || getLabel(span.properties);
  const serviceName = optionalString(span.properties, SERVICE_NAME_KEY);
  const operationName = optionalString(span.properties, OPERATION_NAME_KEY);
	const spanID = optionalString(span.properties, SPAN_ID_KEY);
	const spanKind = optionalString(span.properties, SPAN_KIND_KEY);
	const processID = optionalString(span.properties, PROCESS_ID_KEY);
	const subspanKind = optionalString(span.properties, SUBSPAN_KIND_KEY);
	const suspendCount = optionalInteger(span.properties, SUSPEND_COUNT_KEY);
	const causalEventCount = optionalInteger(span.properties, CAUSAL_EVENT_COUNT_KEY);
	const concurrencyAvg = optionalDouble(span.properties, CONCURRENCY_AVG_KEY);
	const concurrencyPeak = optionalInteger(span.properties, CONCURRENCY_PEAK_KEY);
	const eventDisplayName = optionalString(span.properties, EVENT_DISPLAY_NAME_KEY);
	const eventType = optionalString(span.properties, EVENT_TYPE_KEY);
	const eventTime = optionalString(span.properties, EVENT_TIME_KEY);
	const eventLabel = optionalString(span.properties, EVENT_LABEL_KEY);
	const eventDependencyType = optionalString(span.properties, EVENT_DEPENDENCY_TYPE_KEY);
	const eventDependencyKey = optionalString(span.properties, EVENT_DEPENDENCY_KEY);
	const eventDetail = optionalString(span.properties, EVENT_DETAIL_KEY);
	const lines: string[] = [];
  if (name) {
    lines.push(name);
  }
  if (spanKind || subspanKind) {
    lines.push(`Kind: ${subspanKind || spanKind}`);
  }
	if (eventDisplayName) {
		lines.push(`Event: ${eventDisplayName}`);
	}
	if (eventType) {
		lines.push(`Event type: ${eventType}`);
	}
	if (eventTime) {
		lines.push(`Event time: ${eventTime}`);
	}
	if (eventLabel) {
		lines.push(`Label: ${eventLabel}`);
	}
	if (eventDependencyType) {
		lines.push(`Dependency: ${eventDependencyType}`);
	}
	if (eventDependencyKey) {
		lines.push(`Dependency key: ${eventDependencyKey}`);
	}
	if (eventDetail) {
		lines.push(eventDetail);
	}
  if (spanID) {
    lines.push(`Span ID: ${spanID}`);
  }
  if (serviceName) {
    lines.push(`Service: ${serviceName}`);
  }
  if (processID) {
    lines.push(`Process: ${processID}`);
  }
	if (operationName && operationName !== name) {
		lines.push(`Operation: ${operationName}`);
	}
	if (suspendCount !== null) {
		lines.push(`Suspends: ${suspendCount}`);
	}
	if (causalEventCount !== null) {
		lines.push(`Causal events: ${causalEventCount}`);
	}
	if (concurrencyAvg !== null) {
		lines.push(`Avg concurrency: ${concurrencyAvg.toFixed(2)}`);
	}
	if (concurrencyPeak !== null) {
		lines.push(`Peak concurrency: ${concurrencyPeak}`);
	}
	if (
    span.properties.has(TRACE_START_KEY) &&
    span.properties.has(TRACE_END_KEY)
  ) {
    const start = span.properties.get(TRACE_START_KEY);
    const end = span.properties.get(TRACE_END_KEY);
    const duration = valueDifferenceString(start, end);
    if (duration) {
      lines.push(`Duration: ${duration}`);
    } else {
      lines.push(`Start: ${start.toString()}`);
      lines.push(`End: ${end.toString()}`);
    }
  }
  return lines.join("\n");
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

function axisOffsetValue(trace: Trace<unknown>, offset: number): Value {
  switch (trace.axis.type) {
    case AxisType.DURATION:
      return new DurationValue(new Duration(offset));
    case AxisType.DOUBLE:
      return new DoubleValue(offset);
    case AxisType.TIMESTAMP:
      return new TimestampValue(
        (trace.axis.min as Timestamp).add(new Duration(offset)),
      );
    default:
      throw new ConfigurationError("unsupported trace x-axis type")
        .from(SOURCE)
        .at(Severity.ERROR);
  }
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
        const nextCategoryIDKey = vm.get(CATEGORY_ID_KEY);
        if (!(nextCategoryIDKey instanceof StringValue)) {
          throw new ConfigurationError(
            `'${CATEGORY_ID_KEY}' on watch '${UPDATE_CALLED_OUT_CATEGORY_WATCH}' must be a string`,
          )
            .from(SOURCE)
            .at(Severity.ERROR);
        }
        setCalledOutCategoryID({ value: nextCategoryID });
        setCalledOutCategoryIDKey(nextCategoryIDKey.val);
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

  useEffect((): void => {
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
        ? coloring.colors(properties).secondary || ""
        : coloring.colors(properties).primary || "";
    const strokeOrSecondary = (
      properties: RenderedTraceSpan["properties"],
      highlighted: boolean,
    ) =>
      highlighted
        ? coloring.colors(properties).secondary || ""
        : coloring.colors(properties).stroke || "";

    svg
      .select<SVGGElement>(".spans")
      .attr("width", traceAreaWidthPx)
      .attr("height", heightPx);
    svg
      .select<SVGGElement>(".edges")
      .attr("width", traceAreaWidthPx)
      .attr("height", heightPx);
    svg
      .select<SVGGElement>(".brush")
      .attr("width", traceAreaWidthPx)
      .attr("height", heightPx);

    const brushLayer = svg.select<SVGGElement>(".brush");
    const brush = d3
      .brushX<unknown>()
      .extent([
        [0, 0],
        [traceAreaWidthPx, heightPx],
      ])
      .on("end", (event: any) => {
        if (!event?.sourceEvent || !interactions) {
          return;
        }
        try {
          const extent = event.selection as [number, number] | null;
          if (!extent) {
            return;
          }
          const [leftPx, rightPx] = extent;
          if (Math.abs(rightPx - leftPx) < 2) {
            brushLayer.call(brush.move as any, null);
            return;
          }
          const [domainStart, domainEnd] = domainFromAxis(trace.axis);
          const scale = d3
            .scaleLinear()
            .domain([0, traceAreaWidthPx])
            .range([domainStart, domainEnd]);
          interactions.update(
            TRACE_CHART_TARGET,
            TRACE_BRUSH_ACTION,
            new ValueMap(
              new Map([
                [TRACE_ZOOM_START_KEY, axisOffsetValue(trace, scale(leftPx))],
                [TRACE_ZOOM_END_KEY, axisOffsetValue(trace, scale(rightPx))],
              ]),
            ),
          );
          brushLayer.call(brush.move as any, null);
        } catch (err: unknown) {
          appCore.err(
            err instanceof Error ? err : new ConfigurationError(String(err)),
          );
        }
      });
    brushLayer.call(brush as any);
    svg.on("dblclick", (event: MouseEvent) => {
      if (!interactions) {
        return;
      }
      try {
        interactions.update(TRACE_CHART_TARGET, TRACE_RESET_ZOOM_ACTION);
      } catch (err: unknown) {
        appCore.err(
          err instanceof Error ? err : new ConfigurationError(String(err)),
        );
      }
      event.stopPropagation();
    });

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
      .attr("height", (rs) => rs.height);

    mergedSpans
      .select("rect")
      .attr("width", (rs) => (rs.width === 0 ? 1 : rs.width))
      .attr("height", (rs) => rs.height)
      .attr("fill", (rs) => primaryOrSecondary(rs.properties, rs.highlighted));

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
      .attr("stroke", (re) => strokeOrSecondary(re.properties, re.highlighted));

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
            <g className="brush" />
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
