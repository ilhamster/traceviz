import { render } from "@testing-library/react";
import { JSDOM } from "jsdom";
import {
  AppCore,
  renderCategoryHierarchyForHorizontalSpans,
  renderHorizontalTraceSpans,
} from "@traceviz/client-core";
import { HorizontalTrace } from "./horizontal_trace.tsx";
import { buildTestTrace } from "../../testcases/trace.ts";
import { AppCoreContext } from "../../core/index.ts";
import type { RenderedTraceEdge } from "@traceviz/client-core";
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
});
