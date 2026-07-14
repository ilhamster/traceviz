import { act, fireEvent, render } from "@testing-library/react";
import { JSDOM } from "jsdom";
import {
  Action,
  AppCore,
  Equals,
  GlobalRef,
  Interactions,
  LocalValue,
  Reaction,
  StringValue,
  Update,
  renderCategoryHierarchyForHorizontalSpans,
  renderHorizontalTraceSpans,
} from "@traceviz/client-core";
import {
  HorizontalTrace,
  TRACE_EDGES_TARGET,
  TRACE_HIGHLIGHT_REACTION,
  TRACE_SPAN_CLICK_ACTION,
  TRACE_SPANS_TARGET,
} from "./horizontal_trace.tsx";
import { buildTestTrace } from "../../testcases/trace.ts";
import { AppCoreContext } from "../../core/index.ts";
import type {
  RenderedTraceEdge,
  RenderedTraceSpan,
} from "@traceviz/client-core";
import { RectangularCategoryHierarchyYAxis } from "../axes/category_hierarchy_y.tsx";

// Provide a DOM for d3 + testing-library.
const dom = new JSDOM("<!doctype html><html><body></body></html>");
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).window = dom.window;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).document = dom.window.document;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).HTMLElement = dom.window.HTMLElement;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).SVGElement = dom.window.SVGElement;
// eslint-disable-next-line @typescript-eslint/no-explicit-any
(globalThis as any).__tracevizApplyWindow?.(dom.window);

describe("HorizontalTrace", () => {
  it("renders spans and edges from the core renderers", () => {
    const appCore = new AppCore();
    const errors: string[] = [];
    const errSub = appCore.configurationErrors.subscribe((err) => {
      errors.push(err.toString());
    });
    appCore.publish();

    const trace = buildTestTrace();
    const widthPx = 900;

    const renderedCategories = renderCategoryHierarchyForHorizontalSpans(trace);
    const traceAreaWidthPx = Math.max(0, widthPx - renderedCategories.widthPx);
    const renderedTrace = renderHorizontalTraceSpans(trace, traceAreaWidthPx);

    const { container } = render(
      <AppCoreContext.Provider value={appCore}>
        <HorizontalTrace
          trace={trace}
          widthPx={widthPx}
          transitionDurationMs={0}
          renderYAxis={({ renderedCategories, traceHeightPx }) => (
            <RectangularCategoryHierarchyYAxis
              renderedCategories={renderedCategories}
            />
          )}
        />
      </AppCoreContext.Provider>,
    );

    const spanNodes = container.querySelectorAll("g.spans > svg");
    expect(spanNodes.length).toBe(renderedTrace.spans.length);

    const spanNodesLabels = Array.from(
      container.querySelectorAll("g.spans > svg text"),
    )
      .map((el) => el.textContent)
      .filter((t): t is string => Boolean(t));
    expect(spanNodesLabels).toEqual(["a", "b", "c", "d", "e", "a", "local"]);

    const edgeNodes = Array.from(container.querySelectorAll("g.edges > line"));
    expect(edgeNodes.length).toBe(5);
    const edgesFromDOM: RenderedTraceEdge[] = edgeNodes.map(
      (el) => (el as Element & { __data__?: RenderedTraceEdge }).__data__!,
    );
    expect(edgesFromDOM.map((e: RenderedTraceEdge) => e.renderID)).toEqual([
      "0_0-0_0-a/b",
      "220_0-220_0-a/e",
      "20_0-20_0-a/b/c",
      "140_0-140_0-a/b/d",
      "240_0-240_0-a/e/a",
    ]);

    const categoryLabels = Array.from(
      container.querySelectorAll("svg.traceviz-category-node text"),
    )
      .map((el) => el.textContent)
      .filter((t): t is string => Boolean(t));
    expect(categoryLabels).toEqual([
      "a",
      "a/b",
      "a/b/c",
      "a/b/d",
      "a/e",
      "a/e/a",
    ]);
    expect(errors).toEqual([]);
    errSub.unsubscribe();
  });

  it("dispatches ordinary span clicks", () => {
    const appCore = new AppCore();
    appCore.publish();
    let spanClickCount = 0;
    const interactions = new Interactions().withAction(
      new Action(TRACE_SPANS_TARGET, TRACE_SPAN_CLICK_ACTION, [
        new (class extends Update {
          override update(): void {
            spanClickCount++;
          }

          override get autoDocument(): string {
            return "records a span click";
          }
        })(),
      ]),
    );

    const { container } = render(
      <AppCoreContext.Provider value={appCore}>
        <HorizontalTrace
          trace={buildTestTrace()}
          widthPx={900}
          transitionDurationMs={0}
          interactions={interactions}
        />
      </AppCoreContext.Provider>,
    );

    const span = container.querySelector<SVGSVGElement>("g.spans > svg");
    expect(span).not.toBeNull();
    fireEvent.click(span!);
    expect(spanClickCount).toBe(1);
  });

  it("reacts to generic span and edge highlight predicates", () => {
    const appCore = new AppCore();
    const highlightedRPC = new StringValue("");
    const highlightedEdgeColor = new StringValue("");
    appCore.globalState.set("highlighted_rpc", highlightedRPC);
    appCore.globalState.set("highlighted_edge_color", highlightedEdgeColor);
    const interactions = new Interactions()
      .withReaction(
        new Reaction(
          TRACE_SPANS_TARGET,
          TRACE_HIGHLIGHT_REACTION,
          new Equals(
            new LocalValue("rpc"),
            new GlobalRef(appCore, "highlighted_rpc"),
          ),
        ),
      )
      .withReaction(
        new Reaction(
          TRACE_EDGES_TARGET,
          TRACE_HIGHLIGHT_REACTION,
          new Equals(
            new LocalValue("primary_color"),
            new GlobalRef(appCore, "highlighted_edge_color"),
          ),
        ),
      );
    appCore.publish();

    const { container } = render(
      <AppCoreContext.Provider value={appCore}>
        <HorizontalTrace
          trace={buildTestTrace()}
          widthPx={900}
          transitionDurationMs={0}
          interactions={interactions}
        />
      </AppCoreContext.Provider>,
    );

    const renderedSpans = Array.from(
      container.querySelectorAll<SVGSVGElement>("g.spans > svg"),
    ).map((element) =>
      (element as SVGSVGElement & { __data__?: RenderedTraceSpan }).__data__!,
    );
    const rpcB = renderedSpans.find(
      (span) => span.properties.has("rpc") && span.properties.expectString("rpc") === "b",
    );
    const renderedEdges = Array.from(
      container.querySelectorAll<SVGLineElement>("g.edges > line"),
    ).map((element) =>
      (element as SVGLineElement & { __data__?: RenderedTraceEdge }).__data__!,
    );
    const firstEdge = renderedEdges[0];
    expect(rpcB).toBeDefined();
    expect(firstEdge).toBeDefined();
    expect(rpcB!.highlighted).toBe(false);
    expect(firstEdge.highlighted).toBe(false);

    act(() => {
      highlightedRPC.val = "b";
      highlightedEdgeColor.val = "#888888";
    });
    expect(rpcB!.highlighted).toBe(true);
    expect(firstEdge.highlighted).toBe(true);
    const rpcBElement = Array.from(
      container.querySelectorAll<SVGSVGElement>("g.spans > svg"),
    ).find(
      (element) =>
        (element as SVGSVGElement & { __data__?: RenderedTraceSpan }).__data__ ===
        rpcB,
    );
    expect(rpcBElement?.querySelector("rect")?.style.filter).toContain(
      "drop-shadow",
    );

    act(() => {
      highlightedRPC.val = "";
      highlightedEdgeColor.val = "";
    });
    expect(rpcB!.highlighted).toBe(false);
    expect(firstEdge.highlighted).toBe(false);
    expect(rpcBElement?.querySelector("rect")?.style.filter).toBe("");
  });
});
