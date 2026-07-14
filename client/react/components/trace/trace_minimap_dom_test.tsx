import { fireEvent, render } from "@testing-library/react";
import { JSDOM } from "jsdom";
import {
  Action,
  AppCore,
  Trace,
  Interactions,
  TimestampValue,
  Update,
  ValueMap,
  dbl,
  int,
  node,
  sec,
  str,
  ts,
  valueMap,
} from "@traceviz/client-core";
import { AppCoreContext } from "../../core/index.ts";
import { buildTestTrace } from "../../testcases/trace.ts";
import {
  TraceMinimap,
  TRACE_MINIMAP_DIRECT_MATCH,
  TRACE_MINIMAP_REQUIRES_EXPANSION,
  TRACE_MINIMAP_SET_DOMAIN_ACTION,
  TRACE_MINIMAP_TARGET,
  TRACE_ZOOM_END_KEY,
  TRACE_ZOOM_START_KEY,
} from "./trace_minimap.tsx";

const dom = new JSDOM("<!doctype html><html><body></body></html>");
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).window = dom.window;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).document = dom.window.document;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).HTMLElement = dom.window.HTMLElement;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).HTMLCanvasElement = dom.window.HTMLCanvasElement;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).SVGElement = dom.window.SVGElement;
class TestPointerEvent extends dom.window.MouseEvent {
  readonly pointerId: number;

  constructor(type: string, init: MouseEventInit & { pointerId?: number }) {
    super(type, init);
    this.pointerId = init.pointerId ?? 0;
  }
}
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).PointerEvent = TestPointerEvent;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).__tracevizApplyWindow?.(dom.window);

function buildMatchTrace(matchKinds: string[]): Trace<unknown> {
  const matchNodes = matchKinds.map((matchKind) =>
    node(
      valueMap(
        { key: "trace_node_type", val: int(2) },
        { key: "trace_start", val: ts(sec(40)) },
        { key: "trace_end", val: ts(sec(70)) },
        {
          key: "trace_minimap_highlight_color",
          val: str("#f97316"),
        },
        {
          key: "trace_minimap_match_kind",
          val: str(matchKind),
        },
      ),
    ),
  );
  return Trace.fromNode(
    node(
      valueMap(
        { key: "category_defined_id", val: str("x_axis") },
        { key: "category_display_name", val: str("Time") },
        { key: "category_description", val: str("Time") },
        { key: "axis_type", val: str("timestamp") },
        { key: "axis_min", val: ts(sec(0)) },
        { key: "axis_max", val: ts(sec(100)) },
        { key: "span_width_cat_px", val: int(15) },
        { key: "span_padding_cat_px", val: int(0) },
        { key: "category_header_cat_px", val: int(10) },
        { key: "category_handle_val_px", val: int(10) },
        { key: "category_padding_cat_px", val: int(3) },
        { key: "category_margin_val_px", val: int(5) },
        { key: "category_min_width_cat_px", val: int(15) },
        { key: "category_base_width_val_px", val: int(40) },
        { key: "x_axis_render_markers_height_px", val: dbl(24) },
        { key: "x_axis_render_label_height_px", val: dbl(16) },
      ),
      node(
        valueMap(
          { key: "trace_node_type", val: int(0) },
          { key: "category_defined_id", val: str("category") },
          { key: "category_display_name", val: str("Category") },
          { key: "category_description", val: str("Category") },
        ),
        node(
          valueMap(
            { key: "trace_node_type", val: int(1) },
            { key: "trace_start", val: ts(sec(0)) },
            { key: "trace_end", val: ts(sec(100)) },
            { key: "primary_color", val: str("#94a3b8") },
          ),
          ...matchNodes,
        ),
      ),
    ),
  );
}

function buildDenseTrace(rowCount: number): Trace<unknown> {
  const categories = Array.from({ length: rowCount }, (_, index) =>
    node(
      valueMap(
        { key: "trace_node_type", val: int(0) },
        { key: "category_defined_id", val: str(`category-${index}`) },
        { key: "category_display_name", val: str(`Category ${index}`) },
        { key: "category_description", val: str(`Category ${index}`) },
      ),
      node(
        valueMap(
          { key: "trace_node_type", val: int(1) },
          { key: "trace_start", val: ts(sec(0)) },
          { key: "trace_end", val: ts(sec(100)) },
          { key: "primary_color", val: str("#94a3b8") },
        ),
      ),
    ),
  );
  return Trace.fromNode(
    node(
      valueMap(
        { key: "category_defined_id", val: str("x_axis") },
        { key: "category_display_name", val: str("Time") },
        { key: "category_description", val: str("Time") },
        { key: "axis_type", val: str("timestamp") },
        { key: "axis_min", val: ts(sec(0)) },
        { key: "axis_max", val: ts(sec(100)) },
        { key: "span_width_cat_px", val: int(15) },
        { key: "span_padding_cat_px", val: int(0) },
        { key: "category_header_cat_px", val: int(10) },
        { key: "category_handle_val_px", val: int(10) },
        { key: "category_padding_cat_px", val: int(3) },
        { key: "category_margin_val_px", val: int(5) },
        { key: "category_min_width_cat_px", val: int(15) },
        { key: "category_base_width_val_px", val: int(40) },
        { key: "x_axis_render_markers_height_px", val: dbl(24) },
        { key: "x_axis_render_label_height_px", val: dbl(16) },
      ),
      ...categories,
    ),
  );
}

function canvasContext(
  overrides: Partial<CanvasRenderingContext2D> = {},
): CanvasRenderingContext2D {
  return {
    beginPath: jasmine.createSpy("beginPath"),
    clearRect: jasmine.createSpy("clearRect"),
    closePath: jasmine.createSpy("closePath"),
    fill: jasmine.createSpy("fill"),
    fillRect: jasmine.createSpy("fillRect"),
    lineTo: jasmine.createSpy("lineTo"),
    moveTo: jasmine.createSpy("moveTo"),
    restore: jasmine.createSpy("restore"),
    save: jasmine.createSpy("save"),
    setTransform: jasmine.createSpy("setTransform"),
    stroke: jasmine.createSpy("stroke"),
    fillStyle: "",
    globalAlpha: 1,
    lineJoin: "miter",
    lineWidth: 1,
    strokeStyle: "",
    ...overrides,
  } as unknown as CanvasRenderingContext2D;
}

describe("TraceMinimap", () => {
  it("renders a canvas overview and publishes brushed temporal bounds", () => {
    const appCore = new AppCore();
    appCore.publish();
    const fillRect = jasmine.createSpy("fillRect");
    const context = {
      clearRect: jasmine.createSpy("clearRect"),
      fillRect,
      setTransform: jasmine.createSpy("setTransform"),
      fillStyle: "",
    } as unknown as CanvasRenderingContext2D;
    spyOn(HTMLCanvasElement.prototype, "getContext").and.returnValue(context);

    let published: ValueMap | undefined;
    const interactions = new Interactions().withAction(
      new Action(TRACE_MINIMAP_TARGET, TRACE_MINIMAP_SET_DOMAIN_ACTION, [
        new (class extends Update {
          override update(localState?: ValueMap): void {
            published = localState;
          }

          override get autoDocument(): string {
            return "records a minimap temporal domain";
          }
        })(),
      ]),
    );
    const { container } = render(
      <AppCoreContext.Provider value={appCore}>
        <TraceMinimap
          trace={buildTestTrace()}
          widthPx={800}
          heightPx={60}
          interactions={interactions}
        />
      </AppCoreContext.Provider>,
    );
    expect(fillRect).toHaveBeenCalled();

    const overlay = container.querySelector<SVGSVGElement>("svg");
    expect(overlay).not.toBeNull();
    Object.defineProperties(overlay!, {
      getBoundingClientRect: {
        configurable: true,
        value: () => ({
          left: 0,
          top: 0,
          right: 800,
          bottom: 60,
          width: 800,
          height: 60,
          x: 0,
          y: 0,
          toJSON: () => ({}),
        }),
      },
      setPointerCapture: { configurable: true, value: () => undefined },
      releasePointerCapture: { configurable: true, value: () => undefined },
    });

    fireEvent(
      overlay!,
      new TestPointerEvent("pointerdown", {
        bubbles: true,
        button: 0,
        clientX: 100,
        pointerId: 1,
      }) as unknown as Event,
    );
    fireEvent(
      overlay!,
      new TestPointerEvent("pointermove", {
        bubbles: true,
        clientX: 400,
        pointerId: 1,
      }) as unknown as Event,
    );
    fireEvent(
      overlay!,
      new TestPointerEvent("pointerup", {
        bubbles: true,
        clientX: 400,
        pointerId: 1,
      }) as unknown as Event,
    );

    const start = published?.get(TRACE_ZOOM_START_KEY);
    const end = published?.get(TRACE_ZOOM_END_KEY);
    expect(start).toBeInstanceOf(TimestampValue);
    expect(end).toBeInstanceOf(TimestampValue);
    expect((start as TimestampValue).val.seconds).toBe(37);
    expect((start as TimestampValue).val.nanos).toBe(500_000_000);
    expect((end as TimestampValue).val.seconds).toBe(150);

    const wheelEvent = new dom.window.WheelEvent("wheel", {
      bubbles: true,
      cancelable: true,
      clientX: 400,
      deltaY: -100,
    });
    overlay!.dispatchEvent(wheelEvent);
    expect(wheelEvent.defaultPrevented).toBeTrue();
  });

  it("draws hidden matches as outlined expansion markers", () => {
    const appCore = new AppCore();
    appCore.publish();
    const stroke = jasmine.createSpy("stroke");
    const fillRect = jasmine.createSpy("fillRect");
    const context = canvasContext({ stroke, fillRect });
    spyOn(HTMLCanvasElement.prototype, "getContext").and.returnValue(context);

    render(
      <AppCoreContext.Provider value={appCore}>
        <TraceMinimap
          trace={buildMatchTrace([TRACE_MINIMAP_REQUIRES_EXPANSION])}
          widthPx={800}
          heightPx={60}
        />
      </AppCoreContext.Provider>,
    );

    expect(stroke).toHaveBeenCalledTimes(2);
    expect(context.moveTo).toHaveBeenCalled();
    expect(context.closePath).toHaveBeenCalled();

    // A single 15px source row is capped at 10px and centered in the 60px
    // minimap instead of being stretched to fill its height.
    const rasterRows = fillRect.calls
      .allArgs()
      .map((args) => Number(args[1]));
    expect(Math.min(...rasterRows)).toBe(25);
    expect(Math.max(...rasterRows)).toBe(34);
  });

  it("composes direct and hidden matches as a diamond within a triangle", () => {
    const appCore = new AppCore();
    appCore.publish();
    const fill = jasmine.createSpy("fill");
    const stroke = jasmine.createSpy("stroke");
    const context = canvasContext({ fill, stroke });
    spyOn(HTMLCanvasElement.prototype, "getContext").and.returnValue(context);

    render(
      <AppCoreContext.Provider value={appCore}>
        <TraceMinimap
          trace={buildMatchTrace([
            TRACE_MINIMAP_DIRECT_MATCH,
            TRACE_MINIMAP_REQUIRES_EXPANSION,
          ])}
          widthPx={800}
          heightPx={60}
        />
      </AppCoreContext.Provider>,
    );

    expect(stroke).toHaveBeenCalledTimes(2);
    expect(fill).toHaveBeenCalledTimes(2);
  });

  it("coverage-rasterizes more source rows than the minimap has pixels", () => {
    const appCore = new AppCore();
    appCore.publish();
    const fillRect = jasmine.createSpy("fillRect");
    const context = canvasContext({ fillRect });
    spyOn(HTMLCanvasElement.prototype, "getContext").and.returnValue(context);

    render(
      <AppCoreContext.Provider value={appCore}>
        <TraceMinimap
          trace={buildDenseTrace(120)}
          widthPx={20}
          heightPx={60}
        />
      </AppCoreContext.Provider>,
    );

    const rasterRows = fillRect.calls
      .allArgs()
      .map((args) => Number(args[1]));
    expect(new Set(rasterRows).size).toBe(60);
    expect(Math.min(...rasterRows)).toBe(0);
    expect(Math.max(...rasterRows)).toBe(59);
  });
});
