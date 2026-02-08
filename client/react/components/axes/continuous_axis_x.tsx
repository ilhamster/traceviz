import { useEffect, useMemo, useRef } from "react";
import {
  Axis,
  AxisType,
  ConfigurationError,
  Duration,
  Severity,
  Timestamp,
  ValueMap,
} from "@traceviz/client-core";
import * as d3 from "d3";
import { useAppCore } from "../../core/index.ts";

const SOURCE = "react.components.axes.continuous_axis_x";

enum Keys {
  X_AXIS_RENDER_LABEL_HEIGHT_PX = "x_axis_render_label_height_px",
  X_AXIS_RENDER_MARKERS_HEIGHT_PX = "x_axis_render_markers_height_px",
}

export class ContinuousAxisRenderSettings {
  axisMarkersDepthPx: number;
  axisLabelDepthPx: number;

  constructor() {
    this.axisMarkersDepthPx = 0;
    this.axisLabelDepthPx = 0;
  }

  axisDepthPx(): number {
    return this.axisMarkersDepthPx + this.axisLabelDepthPx;
  }
}

export function xAxisRenderSettings(
  properties: ValueMap,
): ContinuousAxisRenderSettings {
  const ret = new ContinuousAxisRenderSettings();
  ret.axisMarkersDepthPx = properties.expectNumber(
    Keys.X_AXIS_RENDER_MARKERS_HEIGHT_PX,
  );
  ret.axisLabelDepthPx = properties.expectNumber(
    Keys.X_AXIS_RENDER_LABEL_HEIGHT_PX,
  );
  return ret;
}

export function axisValue(val: unknown): number {
  if (val instanceof Duration) {
    return val.nanos;
  }
  if (typeof val === "number") {
    return val;
  }
  throw new ConfigurationError(
    "axis value must be number, Duration, or Timestamp",
  )
    .from(SOURCE)
    .at(Severity.ERROR);
}

export function domainFromAxis(axis: Axis<unknown>): [number, number] {
  if (axis.type === AxisType.TIMESTAMP) {
    const minTs = axis.min as Timestamp;
    const maxTs = axis.max as Timestamp;
    return [0, axis.dist(minTs, maxTs)];
  }
  return [axisValue(axis.min), axisValue(axis.max)];
}

function scaleDomainFromAxis(axis: Axis<unknown>): [number, number] {
  const [min, max] = domainFromAxis(axis);
  return [min, max];
}

function tickValuesForAxis(
  axis: Axis<unknown>,
  scale: d3.ScaleLinear<number, number>,
  domain: [number, number],
): number[] | undefined {
  if (axis.type === AxisType.DOUBLE) {
    return undefined;
  }
  const [min, max] = domain;
  const raw = [min, ...scale.ticks(9), max].filter((v) => v >= min && v <= max);
  const uniq = new Map<string, number>();
  for (const v of raw) {
    uniq.set(v.toString(), v);
  }
  return Array.from(uniq.values()).sort((a, b) => a - b);
}

export function tickFormatterForAxis(
  axis: Axis<unknown>,
): ((value: number) => string) | undefined {
  if (axis.type === AxisType.DOUBLE) {
    return undefined;
  }
  if (axis.type === AxisType.TIMESTAMP) {
    return (value: number) => new Duration(value).toString();
  }
  return (value: number) => new Duration(value).toString();
}

function startLabelForAxis(axis: Axis<unknown>): string | null {
  if (axis.type !== AxisType.TIMESTAMP) {
    return null;
  }
  const start = axis.min as Timestamp;
  return `start ${start.toDate().toISOString()}`;
}

export type ContinuousXAxisProps = {
  axis: Axis<unknown>;
  widthPx: number;
  renderSettings: ContinuousAxisRenderSettings;
  className?: string;
};

export function StandardContinuousXAxis(
  props: ContinuousXAxisProps,
): JSX.Element | null {
  const { axis, widthPx, renderSettings, className } = props;
  const appCore = useAppCore();
  const svgRef = useRef<SVGSVGElement | null>(null);
  let domain: [number, number] = [0, 0];
  let tickFormatter: ((value: number) => string) | undefined;
  let startLabel = "";

  appCore.onPublish(() => {
    domain = useMemo(() => {
      try {
        return scaleDomainFromAxis(axis);
      } catch (err: unknown) {
        appCore.err(
          err instanceof Error ? err : new ConfigurationError(String(err)),
        );
        return null;
      }
    }, [appCore, axis]);
    tickFormatter = useMemo(() => tickFormatterForAxis(axis), [axis]);
    startLabel = useMemo(() => startLabelForAxis(axis), [axis]);
  });

  useEffect((): void => {
    if (!svgRef.current || domain === null) {
      return;
    }

    try {
      const svg = d3.select(svgRef.current);
      svg.attr("width", widthPx).attr("height", renderSettings.axisDepthPx());

      const scale = d3.scaleLinear().domain(domain).range([0, widthPx]);
      const axisBottom = d3.axisBottom(scale);
      if (tickFormatter) {
        axisBottom.tickFormat((value: d3.NumberValue) =>
          tickFormatter(Number(value)),
        );
      }
      const tickValues = tickValuesForAxis(axis, scale, domain);
      if (tickValues) {
        axisBottom.tickValues(tickValues);
      }

      svg.select<SVGGElement>(".x-axis").call(axisBottom);

      // Keep the edge tick labels inside the axis bounds without padding.
      const ticks = svg.selectAll<SVGGElement, unknown>(".x-axis .tick");
      const lastIndex = ticks.size() - 1;
      ticks.select<SVGTextElement>("text").attr("text-anchor", (_d, i) => {
        if (i === 0) return "start";
        if (i === lastIndex) return "end";
        return "middle";
      });

      const labelY =
        renderSettings.axisMarkersDepthPx + renderSettings.axisLabelDepthPx;

      svg.select(".x-axis-label").selectAll("*").remove();
      svg
        .select(".x-axis-label")
        .append("text")
        .attr("x", widthPx / 2)
        .attr("y", labelY)
        .attr("text-anchor", "middle")
        .attr("font-size", 10)
        .attr("fill", "currentColor")
        .attr("font-family", "sans-serif")
        .text(axis.category.displayName);

      svg.select(".x-axis-start-label").selectAll("*").remove();
      if (startLabel) {
        svg
          .select(".x-axis-start-label")
          .append("text")
          .attr("x", 0)
          .attr("y", renderSettings.axisMarkersDepthPx + 6)
          .attr("text-anchor", "start")
          .attr("font-size", 9)
          .attr("fill", "currentColor")
          .attr("font-family", "sans-serif")
          .text(startLabel);
      }
    } catch (err: unknown) {
      appCore.err(
        err instanceof Error ? err : new ConfigurationError(String(err)),
      );
    }
  }, [
    appCore,
    axis,
    domain,
    tickFormatter,
    startLabel,
    widthPx,
    renderSettings,
  ]);

  if (domain === null) {
    return null;
  }

  return (
    <svg ref={svgRef} className={className} style={{ display: "block" }}>
      <g className="x-axis" />
      <g className="x-axis-label" />
      <g className="x-axis-start-label" />
    </svg>
  );
}
