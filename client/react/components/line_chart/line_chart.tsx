import { useEffect, useMemo, useRef, useState } from "react";
import {
  Axis,
  AxisType,
  Coloring,
  ConfigurationError,
  DoubleValue,
  Duration,
  Interactions,
  Point,
  Severity,
  Timestamp,
  TimestampValue,
  Value,
  ValueMap,
  XYChart,
  type ResponseNode,
} from "@traceviz/client-core";
import { Loader } from "@mantine/core";
import { brushX } from "d3-brush";
import * as d3 from "d3";
import { curveLinear, line } from "d3-shape";
import { Subject } from "rxjs";

import { useAppCore } from "../../core/index.ts";

const SOURCE = "react.components.line_chart";

const CHART = "chart";
const ACTION_BRUSH = "brush";
const ZOOM_START_KEY = "zoom_start";
const ZOOM_END_KEY = "zoom_end";

const WATCH_TYPE_UPDATE_X_AXIS_MARKER = "update_x_axis_marker";
const X_AXIS_MARKER_POSITION_KEY = "x_axis_marker_position";

enum Keys {
  X_AXIS_RENDER_LABEL_HEIGHT_PX = "x_axis_render_label_height_px",
  X_AXIS_RENDER_MARKERS_HEIGHT_PX = "x_axis_render_markers_height_px",
  Y_AXIS_RENDER_LABEL_WIDTH_PX = "y_axis_render_label_width_px",
  Y_AXIS_RENDER_MARKERS_WIDTH_PX = "y_axis_render_markers_width_px",
}

const supportedActions: Array<[string, string]> = [[CHART, ACTION_BRUSH]];
const supportedWatches = [WATCH_TYPE_UPDATE_X_AXIS_MARKER];

type ContinuousAxisRenderSettings = {
  axisMarkersDepthPx: number;
  axisLabelDepthPx: number;
};

function xAxisRenderSettings(properties: ValueMap): ContinuousAxisRenderSettings {
  return {
    axisMarkersDepthPx: properties.expectNumber(Keys.X_AXIS_RENDER_MARKERS_HEIGHT_PX),
    axisLabelDepthPx: properties.expectNumber(Keys.X_AXIS_RENDER_LABEL_HEIGHT_PX),
  };
}

function yAxisRenderSettings(properties: ValueMap): ContinuousAxisRenderSettings {
  return {
    axisMarkersDepthPx: properties.expectNumber(Keys.Y_AXIS_RENDER_MARKERS_WIDTH_PX),
    axisLabelDepthPx: properties.expectNumber(Keys.Y_AXIS_RENDER_LABEL_WIDTH_PX),
  };
}

function axisValue(val: unknown): Date | number {
  if (val instanceof Timestamp) {
    return val.toDate();
  }
  if (val instanceof Duration) {
    return val.nanos;
  }
  if (typeof val === "number") {
    return val;
  }
  throw new ConfigurationError("axis value must be number, Duration, or Timestamp")
    .from(SOURCE)
    .at(Severity.ERROR);
}

function scaleFromAxis<T>(axis: Axis<T>, rangeLowPx: number, rangeHighPx: number) {
  if (axis.type === AxisType.TIMESTAMP) {
    return d3
      .scaleTime<Date, number>()
      .domain([axisValue(axis.min) as Date, axisValue(axis.max) as Date])
      .range([rangeLowPx, rangeHighPx]);
  }
  return d3
    .scaleLinear<number, number>()
    .domain([axisValue(axis.min) as number, axisValue(axis.max) as number])
    .range([rangeLowPx, rangeHighPx]);
}

function axisDepthPx(settings: ContinuousAxisRenderSettings): number {
  return settings.axisMarkersDepthPx + settings.axisLabelDepthPx;
}

export type LineChartProps = {
  data?: ResponseNode;
  interactions?: Interactions;
  loading?: boolean;
  className?: string;
  svgMargin?: number;
};

export function LineChart({
  data,
  interactions,
  loading = false,
  className,
  svgMargin = 4,
}: LineChartProps): JSX.Element {
  const appCore = useAppCore();
  const componentRef = useRef<HTMLDivElement | null>(null);
  const chartSvgRef = useRef<SVGSVGElement | null>(null);
  const xAxisSvgRef = useRef<SVGSVGElement | null>(null);
  const yAxisSvgRef = useRef<SVGSVGElement | null>(null);

  const [size, setSize] = useState({ width: 0, height: 0 });
  const [chartData, setChartData] = useState<XYChart | null>(null);
  const [xAxisMarkerVal, setXAxisMarkerVal] = useState<Date | number | undefined>(undefined);

  useEffect(() => {
    if (!data) {
      setChartData(null);
      return;
    }
    try {
      setChartData(XYChart.fromNode(data));
    } catch (err: unknown) {
      appCore.err(err);
      setChartData(null);
    }
  }, [appCore, data]);

  useEffect(() => {
    if (!componentRef.current) {
      return;
    }
    const node = componentRef.current;
    const updateSize = (): void => {
      setSize({
        width: node.offsetWidth,
        height: node.offsetHeight,
      });
    };
    updateSize();
    const ro = new ResizeObserver(() => {
      updateSize();
    });
    ro.observe(node);
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    if (!interactions) {
      return;
    }
    try {
      interactions.checkForSupportedActions(supportedActions);
      interactions.checkForSupportedReactions([]);
      interactions.checkForSupportedWatches(supportedWatches);
    } catch (err: unknown) {
      appCore.err(err);
    }

    const unsubscribe = new Subject<void>();
    const watchErrors = interactions.watch(
      WATCH_TYPE_UPDATE_X_AXIS_MARKER,
      (vm: ValueMap) => {
        const markerVal = vm.get(X_AXIS_MARKER_POSITION_KEY);
        if (markerVal instanceof TimestampValue) {
          setXAxisMarkerVal(markerVal.val.toDate());
        } else if (markerVal instanceof DoubleValue) {
          setXAxisMarkerVal(markerVal.val);
        } else {
          setXAxisMarkerVal(undefined);
        }
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

  const layout = useMemo(() => {
    if (!chartData) {
      return null;
    }
    try {
      const xSettings = xAxisRenderSettings(chartData.properties);
      const ySettings = yAxisRenderSettings(chartData.properties);
      const xDepth = axisDepthPx(xSettings);
      const yDepth = axisDepthPx(ySettings);
      const chartWidth = Math.max(0, size.width - yDepth);
      const chartHeight = Math.max(0, size.height - xDepth);
      return {
        chartWidth,
        chartHeight,
        xSettings,
        ySettings,
        xDepth,
        yDepth,
      };
    } catch (err: unknown) {
      appCore.err(err);
      return null;
    }
  }, [appCore, chartData, size.height, size.width]);

  useEffect(() => {
    if (!chartData || !layout) {
      return;
    }
    if (
      !chartSvgRef.current ||
      !xAxisSvgRef.current ||
      !yAxisSvgRef.current ||
      layout.chartHeight <= 0 ||
      layout.chartWidth <= 0
    ) {
      return;
    }

    try {
      const xScale = scaleFromAxis(chartData.xAxis, svgMargin, layout.chartWidth - svgMargin);
      const yScale = scaleFromAxis(chartData.yAxis, layout.chartHeight - svgMargin, svgMargin);

      const xAxis = d3.axisBottom(xScale as any);
      const yAxis = d3.axisLeft(yScale as any);

      const xAxisSvg = d3.select(xAxisSvgRef.current);
      xAxisSvg.attr("width", layout.chartWidth).attr("height", layout.xDepth);
      xAxisSvg.select<SVGGElement>(".x-axis").call(xAxis);
      xAxisSvg.select<SVGGElement>(".x-axis-label").selectAll("*").remove();
      xAxisSvg
        .select<SVGGElement>(".x-axis-label")
        .append("text")
        .attr("x", layout.chartWidth / 2)
        .attr("y", layout.xSettings.axisMarkersDepthPx + layout.xSettings.axisLabelDepthPx)
        .attr("text-anchor", "middle")
        .attr("font-size", 10)
        .attr("fill", "currentColor")
        .text(chartData.xAxis.category.displayName);

      const yAxisSvg = d3.select(yAxisSvgRef.current);
      yAxisSvg.attr("width", layout.yDepth).attr("height", layout.chartHeight);
      yAxisSvg
        .select<SVGGElement>(".y-axis")
        .attr(
          "transform",
          `translate(${layout.ySettings.axisMarkersDepthPx + layout.ySettings.axisLabelDepthPx}, 0)`,
        )
        .call(yAxis);
      yAxisSvg.select<SVGGElement>(".y-axis-label").selectAll("*").remove();
      yAxisSvg
        .select<SVGGElement>(".y-axis-label")
        .append("text")
        .attr("x", -layout.chartHeight / 2)
        .attr("y", layout.ySettings.axisLabelDepthPx)
        .attr("transform", "rotate(-90)")
        .attr("text-anchor", "middle")
        .attr("font-size", 10)
        .attr("fill", "currentColor")
        .text(chartData.yAxis.category.displayName);

      const chartSvg = d3.select(chartSvgRef.current);
      chartSvg.attr("width", layout.chartWidth).attr("height", layout.chartHeight);

      chartSvg
        .select<SVGRectElement>(".chart-hitbox")
        .attr("x", 0)
        .attr("y", 0)
        .attr("width", layout.chartWidth)
        .attr("height", layout.chartHeight)
        .attr("fill", "transparent");

      const chartArea = chartSvg.select<SVGGElement>(".chart-area");
      chartArea.selectAll("*").remove();

      const coloring = new Coloring(chartData.properties);
      chartArea
        .selectAll<SVGPathElement, Point[]>(".line")
        .data(chartData.series.map((series) => series.points))
        .enter()
        .append("path")
        .attr("stroke", (_points, index) => {
          const series = chartData.series[index];
          return coloring.colors(series.properties).primary || "#5b8cff";
        })
        .style("stroke-width", 1)
        .style("fill", "none")
        .attr("d", (points: Point[]) => {
          return line<Point>()
            .curve(curveLinear)
            .x((point: Point) => {
              return xScale(axisValue(chartData.xAxis.value(point.properties, chartData.xAxis.category.id)) as any) as number;
            })
            .y((point: Point) => {
              return yScale(axisValue(chartData.yAxis.value(point.properties, chartData.yAxis.category.id)) as any) as number;
            })(points);
        });

      const markerLayer = chartSvg.select<SVGGElement>(".x-axis-marker");
      markerLayer.selectAll("*").remove();
      if (xAxisMarkerVal !== undefined) {
        const markerPos = xScale(xAxisMarkerVal as any) as number;
        if (Number.isFinite(markerPos) && markerPos >= 0 && markerPos <= layout.chartWidth) {
          markerLayer
            .append("line")
            .attr("x1", markerPos)
            .attr("y1", 0)
            .attr("x2", markerPos)
            .attr("y2", layout.chartHeight)
            .style("stroke-width", 1)
            .style("stroke", "#f2b316")
            .style("fill", "none");
        }
      }

      const brushLayer = chartSvg.select<SVGGElement>(".brush");
      const brush = brushX()
        .extent([
          [0, 0],
          [layout.chartWidth, layout.chartHeight],
        ])
        .on("end", (event: any) => {
          // Ignore programmatic brush events (e.g. brush.move(null) clears).
          if (!event?.sourceEvent) {
            return;
          }
          if (!interactions) {
            return;
          }
          try {
            const extent = event.selection as [number, number] | null;
            let minValue: Value | undefined;
            let maxValue: Value | undefined;
            if (!extent) {
              if (chartData.xAxis.type === AxisType.TIMESTAMP) {
                minValue = new TimestampValue(new Timestamp(0, 0));
                maxValue = new TimestampValue(new Timestamp(0, 0));
              } else {
                minValue = new DoubleValue(0);
                maxValue = new DoubleValue(0);
              }
            } else {
              const zoomDomainMin = xScale.invert(extent[0]);
              const zoomDomainMax = xScale.invert(extent[1]);
              if (zoomDomainMin instanceof Date && zoomDomainMax instanceof Date) {
                minValue = new TimestampValue(Timestamp.fromDate(zoomDomainMin));
                maxValue = new TimestampValue(Timestamp.fromDate(zoomDomainMax));
              } else if (
                typeof zoomDomainMin === "number" &&
                typeof zoomDomainMax === "number"
              ) {
                minValue = new DoubleValue(zoomDomainMin);
                maxValue = new DoubleValue(zoomDomainMax);
              }
            }
            if (!minValue || !maxValue) {
              throw new ConfigurationError(
                "x-axis extents should either both be numbers, or both be timestamps",
              )
                .from(SOURCE)
                .at(Severity.ERROR);
            }
            interactions.update(
              CHART,
              ACTION_BRUSH,
              new ValueMap(
                new Map([
                  [ZOOM_START_KEY, minValue],
                  [ZOOM_END_KEY, maxValue],
                ]),
              ),
            );
            // Clear the visual brush selection after applying zoom.
            brushLayer.call(brush.move as any, null);
          } catch (err: unknown) {
            appCore.err(err);
          }
        });

      brushLayer.call(brush as any);
    } catch (err: unknown) {
      appCore.err(err);
    }
  }, [appCore, chartData, interactions, layout, svgMargin, xAxisMarkerVal]);

  return (
    <div className={className}>
      {loading ? <Loader size="sm" /> : null}
      <div ref={componentRef} className="line-chart-content" style={{ minHeight: 0, flex: 1 }}>
        {chartData && layout && layout.chartWidth > 0 && layout.chartHeight > 0 ? (
          <div
            style={{
              display: "grid",
              gridTemplateColumns: `${layout.yDepth}px 1fr`,
              gridTemplateRows: `${layout.chartHeight}px ${layout.xDepth}px`,
              alignItems: "start",
              minHeight: 0,
              minWidth: 0,
            }}
          >
            <div style={{ gridColumn: "1", gridRow: "1" }}>
              <svg ref={yAxisSvgRef} style={{ display: "block" }}>
                <g className="y-axis" />
                <g className="y-axis-label" />
              </svg>
            </div>
            <div style={{ gridColumn: "2", gridRow: "1" }}>
              <svg ref={chartSvgRef} style={{ display: "block" }}>
                <rect className="chart-hitbox" />
                <g className="chart-area" />
                <g className="x-axis-marker" />
                <g className="brush" />
              </svg>
            </div>
            <div style={{ gridColumn: "1", gridRow: "2" }} />
            <div style={{ gridColumn: "2", gridRow: "2" }}>
              <svg ref={xAxisSvgRef} style={{ display: "block" }}>
                <g className="x-axis" />
                <g className="x-axis-label" />
              </svg>
            </div>
          </div>
        ) : (
          <div style={{ color: "#5b6676", fontSize: 12 }}>No chart data.</div>
        )}
      </div>
    </div>
  );
}
