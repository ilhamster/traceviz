import "jasmine";

import {
  AppCore,
  Axis,
  AxisType,
  Duration,
  Timestamp,
} from "@traceviz/client-core";
import { render } from "@testing-library/react";
import { JSDOM } from "jsdom";
import React from "react";

import {
  ContinuousAxisRenderSettings,
  StandardContinuousXAxis,
} from "./continuous_axis_x.tsx";
import { AppCoreContext } from "../../core/index.ts";

function installDom(): void {
  const dom = new JSDOM("<!doctype html><html><body></body></html>");
  const { window } = dom;
  const g = globalThis as typeof globalThis & {
    window: typeof window;
    document: Document;
    navigator: Navigator;
    HTMLElement: typeof window.HTMLElement;
    SVGElement: typeof window.SVGElement;
    Node: typeof window.Node;
    getComputedStyle: typeof window.getComputedStyle;
  };
  g.window = window;
  g.document = window.document;
  g.navigator = window.navigator;
  g.HTMLElement = window.HTMLElement;
  g.SVGElement = window.SVGElement;
  g.Node = window.Node;
  g.getComputedStyle = window.getComputedStyle.bind(window);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (globalThis as any).__tracevizApplyWindow?.(window);
}

function makeDoubleAxis(min: number, max: number): Axis<unknown> {
  return new Axis<number>(
    AxisType.DOUBLE,
    { id: "x", displayName: "x", description: "" },
    min,
    max,
    (properties, key) => properties.expectNumber(key),
    (a, b) => b - a,
  ) as Axis<unknown>;
}

function makeDurationAxis(nanos: number): Axis<unknown> {
  return new Axis<Duration>(
    AxisType.DURATION,
    { id: "x", displayName: "dur", description: "" },
    new Duration(0),
    new Duration(nanos),
    (properties, key) => properties.expectDuration(key),
    (a, b) => b.sub(a).nanos,
  ) as Axis<unknown>;
}

function makeTimestampAxis(
  startSeconds: number,
  spanNanos: number,
): Axis<unknown> {
  const min = new Timestamp(startSeconds, 0);
  const max = min.add(new Duration(spanNanos));
  return new Axis<Timestamp>(
    AxisType.TIMESTAMP,
    { id: "x", displayName: "time", description: "" },
    min,
    max,
    (properties, key) => properties.expectTimestamp(key),
    (a, b) => b.sub(a).nanos,
  ) as Axis<unknown>;
}

function makeRenderSettings(): ContinuousAxisRenderSettings {
  const renderSettings = new ContinuousAxisRenderSettings();
  renderSettings.axisMarkersDepthPx = 24;
  renderSettings.axisLabelDepthPx = 16;
  return renderSettings;
}

function getTickTexts(container: HTMLElement): string[] {
  return Array.from(container.querySelectorAll("g.x-axis g.tick text")).map(
    (el) => el.textContent ?? "",
  );
}

type CoreRenderResult = ReturnType<typeof render> & {
  errors: string[];
  sub: { unsubscribe(): void };
};

function renderWithCore(node: React.ReactElement): CoreRenderResult {
  const appCore = new AppCore();
  const errors: string[] = [];
  const sub = appCore.configurationErrors.subscribe((err) => {
    errors.push(err.toString());
  });
  appCore.publish();
  const rendered = render(
    <AppCoreContext.Provider value={appCore}>{node}</AppCoreContext.Provider>,
  );
  return { errors, sub, ...rendered };
}

describe("StandardContinuousXAxis DOM", () => {
  beforeAll(() => {
    installDom();
  });

  it("renders a domain line, ticks, and in-range double labels", () => {
    const axis = makeDoubleAxis(1, 3);
    const renderSettings = makeRenderSettings();

    const { container, errors, sub } = renderWithCore(
      <StandardContinuousXAxis
        axis={axis}
        widthPx={400}
        renderSettings={renderSettings}
      />,
    );

    expect(container.querySelector("g.x-axis path.domain")).not.toBeNull();
    expect(
      container.querySelectorAll("g.x-axis g.tick line").length,
    ).toBeGreaterThan(0);

    const tickTexts = getTickTexts(container);
    const numericTicks = tickTexts
      .map((txt) => Number.parseFloat(txt))
      .filter((val) => !Number.isNaN(val));

    expect(numericTicks.length).toBeGreaterThan(0);
    for (const val of numericTicks) {
      expect(val).toBeGreaterThanOrEqual(1);
      expect(val).toBeLessThanOrEqual(3);
    }

    expect(errors).toEqual([]);
    sub.unsubscribe();
  });

  it("shows domain extents for duration axes", () => {
    const axis = makeDurationAxis(1_000_000_000);
    const { container, errors, sub } = renderWithCore(
      <StandardContinuousXAxis
        axis={axis}
        widthPx={500}
        renderSettings={makeRenderSettings()}
      />,
    );
    const ticks = getTickTexts(container);
    expect(ticks[0]).toBe("0ns");
    expect(ticks[ticks.length - 1]).toBe("1.000s");

    expect(errors).toEqual([]);
    sub.unsubscribe();
  });

  it("shows domain extents and a correct start label for timestamp axes", () => {
    // 0001-01-01T01:00:00Z in seconds-since-epoch.
    const startSeconds = -62135593200;
    const axis = makeTimestampAxis(startSeconds, 1_000_000_000);
    const { container, errors, sub } = renderWithCore(
      <StandardContinuousXAxis
        axis={axis}
        widthPx={500}
        renderSettings={makeRenderSettings()}
      />,
    );
    const ticks = getTickTexts(container);
    expect(ticks[0]).toBe("0ns");
    expect(ticks[ticks.length - 1]).toBe("1.000s");

    const startLabel = container.querySelector(
      "g.x-axis-start-label text",
    )?.textContent;
    expect(startLabel).toBe("start 0001-01-01T01:00:00.000Z");

    expect(errors).toEqual([]);
    sub.unsubscribe();
  });
});
