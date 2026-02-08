import React from "react";
import { render } from "@testing-library/react";
import { JSDOM } from "jsdom";
import {
  Action,
  AppCore,
  GlobalRef,
  Interactions,
  Set,
  str,
  LocalValue,
  renderCategoryHierarchyForHorizontalSpans,
  StringValue,
} from "@traceviz/client-core";
import { RectangularCategoryHierarchyYAxis } from "./category_hierarchy_y.tsx";
import { buildTestTrace } from "../../testcases/trace.ts";
import { AppCoreContext, getGlobalValue } from "../../core/index.ts";

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

describe("RectangularCategoryHierarchyYAxis", () => {
  it("renders category nodes, labels, and colors", () => {
    const appCore = new AppCore();
    const errors: string[] = [];
    const sub = appCore.configurationErrors.subscribe((err) => {
      errors.push(err.toString());
    });
    appCore.publish();

    const trace = buildTestTrace();
    const hierarchy = renderCategoryHierarchyForHorizontalSpans(trace);
    const { container } = render(
      <AppCoreContext.Provider value={appCore}>
        <RectangularCategoryHierarchyYAxis
          renderedCategories={hierarchy}
          transitionDurationMs={0}
        />
      </AppCoreContext.Provider>,
    );

    const nodes = container.querySelectorAll("svg.traceviz-category-node");
    expect(nodes.length).toBe(6);

    const labels = Array.from(container.querySelectorAll("text"))
      .map((el) => el.textContent)
      .sort();
    expect(labels).toEqual(["a", "a/b", "a/b/c", "a/b/d", "a/e", "a/e/a"]);

    const firstCatRect = container.querySelector(
      "svg.traceviz-category-node rect.cat",
    );
    expect(firstCatRect?.getAttribute("fill")).toBe("#888888");

    expect(errors).toEqual([]);
    sub.unsubscribe();
  });

  it("invokes click interactions with the category local values", () => {
    const appCore = new AppCore();
    const errors: string[] = [];
    const sub = appCore.configurationErrors.subscribe((err) => {
      errors.push(err.toString());
    });
    appCore.globalState.set("clicked_label_format", str(""));
    appCore.publish();

    const trace = buildTestTrace();
    const hierarchy = renderCategoryHierarchyForHorizontalSpans(trace);

    const localRef = new LocalValue("label_format");

    // const capture = new CaptureUpdate();
    const interactions = new Interactions().withAction(
      new Action("category_headers", "click", [
        new Set(new GlobalRef(appCore, "clicked_label_format"), localRef),
      ]),
    );

    const { container } = render(
      <AppCoreContext.Provider value={appCore}>
        <RectangularCategoryHierarchyYAxis
          renderedCategories={hierarchy}
          interactions={interactions}
          transitionDurationMs={0}
        />
      </AppCoreContext.Provider>,
    );

    const targetLabel = Array.from(container.querySelectorAll("text")).find(
      (el) => el.textContent === "a/b",
    );
    expect(targetLabel).toBeTruthy();

    targetLabel?.dispatchEvent(
      new dom.window.MouseEvent("click", { bubbles: true }),
    );

    expect(errors).toEqual([]);
    expect(
      (getGlobalValue(appCore, "clicked_label_format") as StringValue).val,
    ).toBe("a/b");
    sub.unsubscribe();
  });
});
