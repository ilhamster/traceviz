import {
  AxisType,
  Coloring,
  ConfigurationError,
  DoubleValue,
  Duration,
  DurationValue,
  Interactions,
  RenderedTraceSpan,
  Severity,
  Timestamp,
  TimestampValue,
  Trace,
  Value,
  ValueMap,
  hex,
  renderHorizontalTraceSpans,
} from "@traceviz/client-core";
import {
  PointerEvent as ReactPointerEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { Subject } from "rxjs";
import { domainFromAxis } from "../axes/continuous_axis_x.tsx";
import { useAppCore } from "../../core/index.ts";

const SOURCE = "trace-minimap";
const MIN_BRUSH_WIDTH_PX = 2;
const MIN_DOMAIN_WIDTH = 1;
const WHEEL_SENSITIVITY = 0.0015;
const DEFAULT_SPAN_COLOR = "#94a3b8";
const MAX_SPAN_HEIGHT_PX = 10;
const MATCH_MARKER_BUCKET_WIDTH_PX = 11;
const MATCH_MARKER_BUCKET_HEIGHT_PX = 10;
const EXPANSION_MARKER_WIDTH_PX = 11;
const EXPANSION_MARKER_HEIGHT_PX = 10;
const DIRECT_MARKER_SIZE_PX = 5;

// TraceViz response property used to preserve important spans during minimap
// decimation. Its value is the color of the fixed-size semantic marker.
export const TRACE_MINIMAP_HIGHLIGHT_COLOR_KEY =
  "trace_minimap_highlight_color";
export const TRACE_MINIMAP_MATCH_KIND_KEY = "trace_minimap_match_kind";
export const TRACE_MINIMAP_DIRECT_MATCH = "direct";
export const TRACE_MINIMAP_REQUIRES_EXPANSION = "requires_expansion";
export const TRACE_MINIMAP_TARGET = "trace_minimap";
export const TRACE_MINIMAP_SET_DOMAIN_ACTION = "set_domain";
export const TRACE_MINIMAP_RESET_DOMAIN_ACTION = "reset_domain";
export const TRACE_MINIMAP_VIEWPORT_WATCH = "trace_minimap_viewport";
export const TRACE_ZOOM_START_KEY = "zoom_start";
export const TRACE_ZOOM_END_KEY = "zoom_end";

const supportedActions: Array<[string, string]> = [
  [TRACE_MINIMAP_TARGET, TRACE_MINIMAP_SET_DOMAIN_ACTION],
  [TRACE_MINIMAP_TARGET, TRACE_MINIMAP_RESET_DOMAIN_ACTION],
];
const supportedWatches = [TRACE_MINIMAP_VIEWPORT_WATCH];

type NumericDomain = [number, number];
type MatchMarker = {
  centerXPx: number;
  centerYPx: number;
  color: string;
  direct: boolean;
  requiresExpansion: boolean;
};
type RGBColor = { red: number; green: number; blue: number };
type SpanRow = {
  y0Px: number;
  y1Px: number;
  spans: RenderedTraceSpan[];
};
type DragState =
  | {
      kind: "brush";
      pointerID: number;
      startPx: number;
    }
  | {
      kind: "pan";
      pointerID: number;
      pointerOffsetPx: number;
      width: number;
    };

export type TraceMinimapProps<T> = {
  trace: Trace<T>;
  widthPx: number;
  heightPx?: number;
  interactions?: Interactions;
  className?: string;
};

function clamp(value: number, min: number, max: number): number {
  return Math.min(Math.max(value, min), max);
}

function normalizedDomain(
  domain: NumericDomain,
  fullDomain: NumericDomain,
): NumericDomain {
  const [fullStart, fullEnd] = fullDomain;
  let [start, end] = domain;
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) {
    return fullDomain;
  }
  const width = Math.min(end - start, fullEnd - fullStart);
  start = clamp(start, fullStart, fullEnd - width);
  end = start + width;
  return [start, end];
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

function axisOffsetFromValue(trace: Trace<unknown>, value: Value): number {
  if (value instanceof DurationValue) {
    return value.val.nanos;
  }
  if (value instanceof DoubleValue) {
    return value.val;
  }
  if (
    value instanceof TimestampValue &&
    trace.axis.type === AxisType.TIMESTAMP
  ) {
    return trace.axis.dist(trace.axis.min, value.val);
  }
  throw new ConfigurationError("minimap viewport value does not match its axis")
    .from(SOURCE)
    .at(Severity.ERROR);
}

function parseColor(color: string): RGBColor {
  const normalized = hex(color);
  if (!/^#[0-9a-f]{6}$/i.test(normalized)) {
    return parseColor(DEFAULT_SPAN_COLOR);
  }
  return {
    red: Number.parseInt(normalized.slice(1, 3), 16),
    green: Number.parseInt(normalized.slice(3, 5), 16),
    blue: Number.parseInt(normalized.slice(5, 7), 16),
  };
}

function drawExpansionMarker(
  context: CanvasRenderingContext2D,
  marker: MatchMarker,
  haloColor: string,
): void {
  const halfWidth = EXPANSION_MARKER_WIDTH_PX / 2;
  const halfHeight = EXPANSION_MARKER_HEIGHT_PX / 2;
  const drawTriangle = (): void => {
    context.beginPath();
    context.moveTo(marker.centerXPx - halfWidth, marker.centerYPx - halfHeight);
    context.lineTo(marker.centerXPx + halfWidth, marker.centerYPx - halfHeight);
    context.lineTo(marker.centerXPx, marker.centerYPx + halfHeight);
    context.closePath();
    context.stroke();
  };
  context.save();
  context.lineJoin = "round";
  context.strokeStyle = haloColor;
  context.lineWidth = 4;
  drawTriangle();
  context.strokeStyle = marker.color;
  context.lineWidth = 2;
  drawTriangle();
  context.restore();
}

function drawDirectMarker(
  context: CanvasRenderingContext2D,
  marker: MatchMarker,
  haloColor: string,
): void {
  const drawDiamond = (sizePx: number): void => {
    const halfSize = sizePx / 2;
    context.beginPath();
    context.moveTo(marker.centerXPx, marker.centerYPx - halfSize);
    context.lineTo(marker.centerXPx + halfSize, marker.centerYPx);
    context.lineTo(marker.centerXPx, marker.centerYPx + halfSize);
    context.lineTo(marker.centerXPx - halfSize, marker.centerYPx);
    context.closePath();
    context.fill();
  };
  context.save();
  context.fillStyle = haloColor;
  drawDiamond(DIRECT_MARKER_SIZE_PX + 3);
  context.fillStyle = marker.color;
  drawDiamond(DIRECT_MARKER_SIZE_PX);
  context.restore();
}

function verticalLayout(
  spans: RenderedTraceSpan[],
  heightPx: number,
): { scale: number; offsetPx: number } {
  const sourceHeightPx = Math.max(1, ...spans.map((span) => span.y1Px));
  const tallestSpanPx = Math.max(1, ...spans.map((span) => span.height));
  const scale = Math.min(
    1,
    MAX_SPAN_HEIGHT_PX / tallestSpanPx,
    heightPx / sourceHeightPx,
  );
  return {
    scale,
    offsetPx: Math.max(0, (heightPx - sourceHeightPx * scale) / 2),
  };
}

function addRange(
  values: Float64Array,
  rowOffset: number,
  startXPx: number,
  endXPx: number,
  value: number,
): void {
  values[rowOffset + startXPx] += value;
  values[rowOffset + endXPx] -= value;
}

/**
 * Rasterizes spans into a fixed-height overview. Each source row is resolved
 * in painter order before it contributes to output buckets, so a subspan
 * replaces its parent rather than being counted as additional work.
 */
function drawSpanOccupancy(
  context: CanvasRenderingContext2D,
  spans: RenderedTraceSpan[],
  coloring: Coloring,
  widthPx: number,
  heightPx: number,
  scale: number,
  offsetPx: number,
): void {
  const rasterWidth = Math.max(1, Math.round(widthPx));
  const rasterHeight = Math.max(1, Math.round(heightPx));
  const rowStride = rasterWidth + 1;
  const redDiff = new Float64Array(rowStride * rasterHeight);
  const greenDiff = new Float64Array(rowStride * rasterHeight);
  const blueDiff = new Float64Array(rowStride * rasterHeight);
  const weightDiff = new Float64Array(rowStride * rasterHeight);
  const colors: RGBColor[] = [];
  const colorIndices = new Map<string, number>();
  const rows = new Map<string, SpanRow>();

  for (const span of spans) {
    if (
      span.width <= 0 ||
      span.properties.has(TRACE_MINIMAP_HIGHLIGHT_COLOR_KEY)
    ) {
      continue;
    }
    const key = `${span.y0Px}:${span.y1Px}`;
    let row = rows.get(key);
    if (!row) {
      row = { y0Px: span.y0Px, y1Px: span.y1Px, spans: [] };
      rows.set(key, row);
    }
    row.spans.push(span);
  }

  const rowColors = new Int32Array(rasterWidth);
  const touchedColumns: number[] = [];
  for (const row of rows.values()) {
    let minimumTouchedX = rasterWidth;
    let maximumTouchedX = -1;
    for (const span of row.spans) {
      const color =
        coloring.colors(span.properties).primary || DEFAULT_SPAN_COLOR;
      let colorIndex = colorIndices.get(color);
      if (colorIndex === undefined) {
        colorIndex = colors.length;
        colors.push(parseColor(color));
        colorIndices.set(color, colorIndex);
      }
      const startXPx = clamp(Math.floor(span.x0Px), 0, rasterWidth);
      const endXPx = clamp(Math.ceil(span.x1Px), 0, rasterWidth);
      for (let x = startXPx; x < endXPx; x++) {
        if (rowColors[x] === 0) {
          touchedColumns.push(x);
        }
        rowColors[x] = colorIndex + 1;
      }
      minimumTouchedX = Math.min(minimumTouchedX, startXPx);
      maximumTouchedX = Math.max(maximumTouchedX, endXPx - 1);
    }
    if (maximumTouchedX < minimumTouchedX) {
      continue;
    }

    const transformedY0 = offsetPx + row.y0Px * scale;
    const transformedY1 = offsetPx + row.y1Px * scale;
    const startYPx = clamp(Math.floor(transformedY0), 0, rasterHeight);
    const endYPx = clamp(Math.ceil(transformedY1), 0, rasterHeight);
    for (let y = startYPx; y < endYPx; y++) {
      const verticalCoverage = Math.max(
        0,
        Math.min(transformedY1, y + 1) - Math.max(transformedY0, y),
      );
      if (verticalCoverage === 0) {
        continue;
      }
      const rowOffset = y * rowStride;
      let runStart = minimumTouchedX;
      let runColorIndex = rowColors[runStart];
      for (let x = minimumTouchedX + 1; x <= maximumTouchedX + 1; x++) {
        const nextColorIndex = x <= maximumTouchedX ? rowColors[x] : 0;
        if (nextColorIndex === runColorIndex) {
          continue;
        }
        if (runColorIndex > 0) {
          const rgb = colors[runColorIndex - 1];
          addRange(
            redDiff,
            rowOffset,
            runStart,
            x,
            rgb.red * verticalCoverage,
          );
          addRange(
            greenDiff,
            rowOffset,
            runStart,
            x,
            rgb.green * verticalCoverage,
          );
          addRange(
            blueDiff,
            rowOffset,
            runStart,
            x,
            rgb.blue * verticalCoverage,
          );
          addRange(
            weightDiff,
            rowOffset,
            runStart,
            x,
            verticalCoverage,
          );
        }
        runStart = x;
        runColorIndex = nextColorIndex;
      }
    }
    for (const x of touchedColumns) {
      rowColors[x] = 0;
    }
    touchedColumns.length = 0;
  }

  for (let y = 0; y < rasterHeight; y++) {
    const rowOffset = y * rowStride;
    let red = 0;
    let green = 0;
    let blue = 0;
    let weight = 0;
    let runStart = 0;
    let runStyle = "";
    let runAlpha = 0;
    const flushRun = (endXPx: number): void => {
      if (runStyle === "" || endXPx <= runStart) {
        return;
      }
      context.fillStyle = runStyle;
      context.globalAlpha = runAlpha;
      context.fillRect(runStart, y, endXPx - runStart, 1);
    };
    for (let x = 0; x <= rasterWidth; x++) {
      red += redDiff[rowOffset + x];
      green += greenDiff[rowOffset + x];
      blue += blueDiff[rowOffset + x];
      weight += weightDiff[rowOffset + x];
      let style = "";
      if (weight > 0) {
        const renderedRed = Math.round(red / weight);
        const renderedGreen = Math.round(green / weight);
        const renderedBlue = Math.round(blue / weight);
        style = `rgb(${renderedRed} ${renderedGreen} ${renderedBlue})`;
      }
      const alpha = Math.round(Math.min(1, weight) * 1000) / 1000;
      if (x === 0) {
        runStyle = style;
        runAlpha = alpha;
        continue;
      }
      if (style !== runStyle || alpha !== runAlpha || x === rasterWidth) {
        flushRun(x);
        runStart = x;
        runStyle = style;
        runAlpha = alpha;
      }
    }
  }
  context.globalAlpha = 1;
}

export function TraceMinimap<T>(props: TraceMinimapProps<T>): JSX.Element {
  const {
    trace,
    widthPx,
    heightPx = 60,
    interactions,
    className,
  } = props;
  const appCore = useAppCore();
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const overlayRef = useRef<SVGSVGElement | null>(null);
  const fullDomain = useMemo<NumericDomain>(
    () => domainFromAxis(trace.axis),
    [trace],
  );
  const dragRef = useRef<DragState | null>(null);
  const viewportDomainRef = useRef<NumericDomain>(fullDomain);
  const previewDomainRef = useRef<NumericDomain | null>(null);
  const [viewportDomain, setViewportDomain] =
    useState<NumericDomain>(fullDomain);
  const [previewDomain, setPreviewDomain] =
    useState<NumericDomain | null>(null);

  const renderedTrace = useMemo(
    () =>
      renderHorizontalTraceSpans(trace, Math.max(0, widthPx), {
        includeCategoryHeaderSpace: false,
      }),
    [trace, widthPx],
  );

  // A new minimap trace defines a new complete domain; watches subsequently
  // replace this with the tool's committed viewport.
  useEffect(() => {
    setViewportDomain(fullDomain);
    viewportDomainRef.current = fullDomain;
    setPreviewDomain(null);
    previewDomainRef.current = null;
  }, [fullDomain]);

  // Watches the tool-owned temporal Values so the viewport overlay moves
  // immediately and independently from minimap data refetches.
  useEffect(() => {
    if (!interactions) {
      return;
    }
    try {
      interactions.checkForSupportedActions(supportedActions);
      interactions.checkForSupportedWatches(supportedWatches);
    } catch (err: unknown) {
      appCore.err(
        err instanceof Error ? err : new ConfigurationError(String(err)),
      );
    }
    const unsubscribe = new Subject<void>();
    const errors = interactions.watch(
      TRACE_MINIMAP_VIEWPORT_WATCH,
      (values: ValueMap) => {
        if (
          !values.has(TRACE_ZOOM_START_KEY) ||
          !values.has(TRACE_ZOOM_END_KEY)
        ) {
          throw new ConfigurationError(
            `watch '${TRACE_MINIMAP_VIEWPORT_WATCH}' requires both temporal bounds`,
          )
            .from(SOURCE)
            .at(Severity.ERROR);
        }
        const domain = normalizedDomain(
          [
            axisOffsetFromValue(trace, values.get(TRACE_ZOOM_START_KEY)),
            axisOffsetFromValue(trace, values.get(TRACE_ZOOM_END_KEY)),
          ],
          fullDomain,
        );
        viewportDomainRef.current = domain;
        setViewportDomain(domain);
        setPreviewDomain(null);
        previewDomainRef.current = null;
      },
      unsubscribe,
    );
    const errorSubscription = errors.subscribe((err) => appCore.err(err));
    return () => {
      unsubscribe.next();
      unsubscribe.complete();
      errorSubscription.unsubscribe();
    };
  }, [appCore, fullDomain, interactions, trace]);

  // Renders ordinary work as a coverage raster and search results as semantic
  // fixed-size glyphs that remain legible after temporal and vertical thinning.
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || widthPx <= 0 || heightPx <= 0) {
      return;
    }
    const context = canvas.getContext("2d");
    if (!context) {
      return;
    }
    const deviceScale = window.devicePixelRatio || 1;
    canvas.width = Math.max(1, Math.round(widthPx * deviceScale));
    canvas.height = Math.max(1, Math.round(heightPx * deviceScale));
    canvas.style.width = `${widthPx}px`;
    canvas.style.height = `${heightPx}px`;
    context.setTransform(deviceScale, 0, 0, deviceScale, 0, 0);
    context.clearRect(0, 0, widthPx, heightPx);

    const layout = verticalLayout(renderedTrace.spans, heightPx);
    const coloring = new Coloring(trace.properties);
    drawSpanOccupancy(
      context,
      renderedTrace.spans,
      coloring,
      widthPx,
      heightPx,
      layout.scale,
      layout.offsetPx,
    );

    const matchMarkers = new Map<string, MatchMarker>();
    for (const span of renderedTrace.spans) {
      const highlightColor = span.properties.has(
        TRACE_MINIMAP_HIGHLIGHT_COLOR_KEY,
      )
        ? span.properties.expectString(TRACE_MINIMAP_HIGHLIGHT_COLOR_KEY)
        : "";
      if (!highlightColor) {
        continue;
      }
      const matchKind = span.properties.has(TRACE_MINIMAP_MATCH_KIND_KEY)
        ? span.properties.expectString(TRACE_MINIMAP_MATCH_KIND_KEY)
        : TRACE_MINIMAP_DIRECT_MATCH;
      const centerXPx = clamp(
        span.x0Px + span.width / 2,
        MATCH_MARKER_BUCKET_WIDTH_PX / 2,
        Math.max(
          MATCH_MARKER_BUCKET_WIDTH_PX / 2,
          widthPx - MATCH_MARKER_BUCKET_WIDTH_PX / 2,
        ),
      );
      const centerYPx = clamp(
        layout.offsetPx + ((span.y0Px + span.y1Px) / 2) * layout.scale,
        MATCH_MARKER_BUCKET_HEIGHT_PX / 2,
        Math.max(
          MATCH_MARKER_BUCKET_HEIGHT_PX / 2,
          heightPx - MATCH_MARKER_BUCKET_HEIGHT_PX / 2,
        ),
      );
      const column = Math.round(centerXPx / MATCH_MARKER_BUCKET_WIDTH_PX);
      const row = Math.round(centerYPx / MATCH_MARKER_BUCKET_HEIGHT_PX);
      const key = `${row}:${column}`;
      const marker = matchMarkers.get(key) ?? {
        centerXPx,
        centerYPx,
        color: highlightColor,
        direct: false,
        requiresExpansion: false,
      };
      marker.direct ||= matchKind === TRACE_MINIMAP_DIRECT_MATCH;
      marker.requiresExpansion ||=
        matchKind === TRACE_MINIMAP_REQUIRES_EXPANSION;
      matchMarkers.set(key, marker);
    }
    const markerHaloColor =
      window
        .getComputedStyle(canvas)
        .getPropertyValue("--minimap-marker-halo")
        .trim() || "#ffffff";
    for (const marker of matchMarkers.values()) {
      if (marker.direct) {
        drawDirectMarker(context, marker, markerHaloColor);
      }
      if (marker.requiresExpansion) {
        drawExpansionMarker(context, marker, markerHaloColor);
      }
    }
  }, [heightPx, renderedTrace, trace, widthPx]);

  const activeDomain = previewDomain ?? viewportDomain;
  const fullWidth = Math.max(MIN_DOMAIN_WIDTH, fullDomain[1] - fullDomain[0]);
  const domainToPixel = (moment: number): number =>
    ((moment - fullDomain[0]) / fullWidth) * widthPx;
  const pixelToDomain = useCallback(
    (pixel: number): number =>
      fullDomain[0] + (clamp(pixel, 0, widthPx) / widthPx) * fullWidth,
    [fullDomain, fullWidth, widthPx],
  );
  const viewportStartPx = domainToPixel(activeDomain[0]);
  const viewportEndPx = domainToPixel(activeDomain[1]);

  const pointerX = (event: ReactPointerEvent<SVGSVGElement>): number => {
    const bounds = event.currentTarget.getBoundingClientRect();
    return clamp(event.clientX - bounds.left, 0, widthPx);
  };

  const publishDomain = useCallback(
    (domain: NumericDomain): void => {
      if (!interactions) {
        return;
      }
      const normalized = normalizedDomain(domain, fullDomain);
      interactions.update(
        TRACE_MINIMAP_TARGET,
        TRACE_MINIMAP_SET_DOMAIN_ACTION,
        new ValueMap(
          new Map<string, Value>([
            [TRACE_ZOOM_START_KEY, axisOffsetValue(trace, normalized[0])],
            [TRACE_ZOOM_END_KEY, axisOffsetValue(trace, normalized[1])],
          ]),
        ),
      );
    },
    [fullDomain, interactions, trace],
  );

  const onPointerDown = (event: ReactPointerEvent<SVGSVGElement>): void => {
    if (!interactions || event.button !== 0 || widthPx <= 0) {
      return;
    }
    const x = pointerX(event);
    const viewportIsFull = viewportEndPx - viewportStartPx >= widthPx - 1;
    if (!viewportIsFull && x >= viewportStartPx && x <= viewportEndPx) {
      dragRef.current = {
        kind: "pan",
        pointerID: event.pointerId,
        pointerOffsetPx: x - viewportStartPx,
        width: viewportDomainRef.current[1] - viewportDomainRef.current[0],
      };
    } else {
      dragRef.current = {
        kind: "brush",
        pointerID: event.pointerId,
        startPx: x,
      };
    }
    event.currentTarget.setPointerCapture(event.pointerId);
    event.preventDefault();
  };

  const onPointerMove = (event: ReactPointerEvent<SVGSVGElement>): void => {
    const drag = dragRef.current;
    if (!drag || drag.pointerID !== event.pointerId) {
      return;
    }
    const x = pointerX(event);
    if (drag.kind === "brush") {
      if (Math.abs(x - drag.startPx) >= MIN_BRUSH_WIDTH_PX) {
        const domain = normalizedDomain(
          [
            pixelToDomain(Math.min(x, drag.startPx)),
            pixelToDomain(Math.max(x, drag.startPx)),
          ],
          fullDomain,
        );
        previewDomainRef.current = domain;
        setPreviewDomain(domain);
      }
    } else {
      const desiredStart = pixelToDomain(x - drag.pointerOffsetPx);
      const domain = normalizedDomain(
        [desiredStart, desiredStart + drag.width],
        fullDomain,
      );
      previewDomainRef.current = domain;
      setPreviewDomain(domain);
    }
    event.preventDefault();
  };

  const finishPointer = (event: ReactPointerEvent<SVGSVGElement>): void => {
    const drag = dragRef.current;
    if (!drag || drag.pointerID !== event.pointerId) {
      return;
    }
    dragRef.current = null;
    const committedPreview = previewDomainRef.current;
    if (committedPreview !== null) {
      try {
        publishDomain(committedPreview);
      } catch (err: unknown) {
        appCore.err(
          err instanceof Error ? err : new ConfigurationError(String(err)),
        );
      }
    }
    previewDomainRef.current = null;
    setPreviewDomain(null);
    try {
      event.currentTarget.releasePointerCapture(event.pointerId);
    } catch {
      // Pointer capture may already have been released after cancellation.
    }
    event.preventDefault();
  };

  // React delegates wheel events through a passive listener, which prevents a
  // synthetic onWheel handler from reliably suppressing page scrolling. Own
  // the minimap wheel gesture with an explicitly non-passive native listener.
  useEffect(() => {
    const overlay = overlayRef.current;
    if (!overlay || !interactions || widthPx <= 0) {
      return;
    }
    const onWheel = (event: WheelEvent): void => {
      event.preventDefault();
      const bounds = overlay.getBoundingClientRect();
      const mouseX = clamp(event.clientX - bounds.left, 0, widthPx);
      const anchor = pixelToDomain(mouseX);
      const factor = clamp(
        Math.exp(event.deltaY * WHEEL_SENSITIVITY),
        0.5,
        2,
      );
      const currentDomain = viewportDomainRef.current;
      const start = anchor + (currentDomain[0] - anchor) * factor;
      const end = anchor + (currentDomain[1] - anchor) * factor;
      try {
        publishDomain([start, end]);
      } catch (err: unknown) {
        appCore.err(
          err instanceof Error ? err : new ConfigurationError(String(err)),
        );
      }
    };
    overlay.addEventListener("wheel", onWheel, { passive: false });
    return () => overlay.removeEventListener("wheel", onWheel);
  }, [appCore, interactions, pixelToDomain, publishDomain, widthPx]);

  return (
    <div
      className={className}
      style={{
        position: "relative",
        width: `${widthPx}px`,
        height: `${heightPx}px`,
      }}
      aria-label="Trace minimap"
    >
      <canvas ref={canvasRef} aria-hidden="true" />
      <svg
        ref={overlayRef}
        width={widthPx}
        height={heightPx}
        viewBox={`0 0 ${widthPx} ${heightPx}`}
        style={{ position: "absolute", inset: 0, touchAction: "none" }}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={finishPointer}
        onPointerCancel={finishPointer}
        onDoubleClick={(event) => {
          try {
            interactions?.update(
              TRACE_MINIMAP_TARGET,
              TRACE_MINIMAP_RESET_DOMAIN_ACTION,
            );
          } catch (err: unknown) {
            appCore.err(
              err instanceof Error ? err : new ConfigurationError(String(err)),
            );
          }
          event.preventDefault();
        }}
      >
        <path
          className="traceviz-minimap-shade"
          d={`M0 0H${widthPx}V${heightPx}H0Z M${viewportStartPx} 0V${heightPx}H${viewportEndPx}V0Z`}
          fillRule="evenodd"
          fill="rgba(32, 33, 36, 0.34)"
        />
        <rect
          className="traceviz-minimap-viewport"
          x={viewportStartPx}
          y={0.5}
          width={Math.max(1, viewportEndPx - viewportStartPx)}
          height={Math.max(0, heightPx - 1)}
          fill="transparent"
          stroke="#315f9d"
        />
      </svg>
    </div>
  );
}
