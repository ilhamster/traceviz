import { ReactNode, useEffect, useMemo, useRef } from "react";
import {
  ConfigurationError,
  Coloring,
  RenderedTraceEdge,
  RenderedTraceSpan,
  RenderedCategoryHierarchy,
  Trace,
  getLabel,
  renderCategoryHierarchyForHorizontalSpans,
  renderHorizontalTraceSpans,
} from "@traceviz/client-core";
import * as d3 from "d3";
import { ContinuousAxisRenderSettings, xAxisRenderSettings } from "../axes/continuous_axis_x.tsx";
import { useAppCore } from "../../core/index.ts";

export type HorizontalTraceYAxisSlotProps<T> = {
  trace: Trace<T>;
  renderedCategories: RenderedCategoryHierarchy;
  traceHeightPx: number;
};

export type HorizontalTraceXAxisSlotProps<T> = {
  trace: Trace<T>;
  renderSettings: ContinuousAxisRenderSettings;
  widthPx: number;
  heightPx: number;
};

export type HorizontalTraceProps<T> = {
  trace: Trace<T>;
  widthPx: number;
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
    renderYAxis,
    renderXAxis,
    transitionDurationMs = 300,
    className,
  } = props;
  const appCore = useAppCore();
  const svgRef = useRef<SVGSVGElement | null>(null);

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

    const spanNodes = svg
      .select(".spans")
      .selectAll<SVGSVGElement, RenderedTraceSpan>("svg")
      .data(renderedTrace.spans, (d) => d.renderID);
    spanNodes.exit().remove();

    const enteredSpans = spanNodes.enter().append("svg");
    enteredSpans.append("rect");
    enteredSpans.append("text");

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
    renderedTrace,
    trace,
    traceAreaWidthPx,
    transitionDurationMs,
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
            })
          : null}
      </div>
      <div style={{ gridColumn: "2", gridRow: "1" }}>
        {traceAreaWidthPx > 0 && renderedTrace !== null ? (
          <svg ref={svgRef} style={{ display: "block" }}>
            <g className="edges" />
            <g className="spans" />
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
            })
          : null}
      </div>
    </div>
  );
}
