import {useEffect, useRef} from "react";
import {
  Coloring,
  ConfigurationError,
  Interactions,
  RenderedCategory,
  RenderedCategoryHierarchy,
  getLabel,
} from "@traceviz/client-core";
import * as d3 from "d3";
import {useAppCore} from "../../core/index.ts";

const CATEGORY_NODE_CLASS = "traceviz-category-node";

enum Targets {
  CATEGORY_HEADERS = "category_headers",
}

enum Actions {
  CLICK = "click",
  MOUSEOVER = "mouseover",
  MOUSEOUT = "mouseout",
}

/** The name for a highlight reaction. */
export const REACTION_HIGHLIGHT = "highlight";

const supportedActions: Array<[string, string]> = [
  [Targets.CATEGORY_HEADERS, Actions.CLICK],
  [Targets.CATEGORY_HEADERS, Actions.MOUSEOVER],
  [Targets.CATEGORY_HEADERS, Actions.MOUSEOUT],
];
const supportedReactions: Array<[string, string]> = [
  [Targets.CATEGORY_HEADERS, REACTION_HIGHLIGHT],
];
const supportedWatches: string[] = [];

export type CategoryHierarchyYAxisProps = {
  renderedCategories: RenderedCategoryHierarchy;
  interactions?: Interactions;
  transitionDurationMs?: number;
  className?: string;
};

type SelectionOrTransition<T extends d3.BaseType, D> =
  | d3.Selection<T, D, SVGSVGElement, unknown>
  | d3.Transition<T, D, SVGSVGElement, unknown>;

function applyMaybeTransition<T extends d3.BaseType, D>(
  selection: d3.Selection<T, D, SVGSVGElement, unknown>,
  durationMs: number,
  updater: (sel: SelectionOrTransition<T, D>) => void,
): void {
  const target: SelectionOrTransition<T, D> =
    durationMs > 0 ? selection.transition().duration(durationMs) : selection;
  updater(target);
}

function setNodeFrame(
  selection: SelectionOrTransition<SVGSVGElement, RenderedCategory>,
): void {
  selection.attr("x", (rc) => rc.x0Px);
  selection.attr("y", (rc) => rc.y0Px);
  selection.attr("width", (rc) => rc.width);
  selection.attr("height", (rc) => rc.height);
}

export function RectangularCategoryHierarchyYAxis(
  props: CategoryHierarchyYAxisProps,
): JSX.Element {
  const {
    renderedCategories,
    interactions,
    transitionDurationMs = 300,
    className,
  } = props;
  const appCore = useAppCore();
  const svgRef = useRef<SVGSVGElement | null>(null);

  useEffect((): void => {
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
  }, [appCore, interactions]);

  useEffect((): void => {
    if (!svgRef.current) {
      return;
    }

    try {
      const coloring = new Coloring(renderedCategories.properties);
      const svg = d3.select(svgRef.current);
      svg
        .attr("width", renderedCategories.widthPx)
        .attr("height", renderedCategories.heightPx);

      const nodes = svg
        .selectAll<
          SVGSVGElement,
          RenderedCategory
        >(`svg.${CATEGORY_NODE_CLASS}`)
        .data(
          renderedCategories.categories,
          (rc) => (rc as RenderedCategory).category.id,
        );

      nodes.exit().remove();

      const enteredNodes = nodes
        .enter()
        .append("svg")
        .attr("class", CATEGORY_NODE_CLASS)
        .call(setNodeFrame)
        .on("mouseover", function (_event, rc) {
          d3.select(this)
            .select<SVGRectElement>("rect.cat")
            .attr("stroke", coloring.colors(rc.properties).stroke || "#cecece");
          try {
            interactions?.update(
              Targets.CATEGORY_HEADERS,
              Actions.MOUSEOVER,
              rc.properties,
            );
          } catch (err: unknown) {
            appCore.err(
              err instanceof Error ? err : new ConfigurationError(String(err)),
            );
          }
        })
        .on("mouseout", function (_event, rc) {
          d3.select(this)
            .select<SVGRectElement>("rect.cat")
            .attr("stroke", "none");
          try {
            interactions?.update(
              Targets.CATEGORY_HEADERS,
              Actions.MOUSEOUT,
              rc.properties,
            );
          } catch (err: unknown) {
            appCore.err(
              err instanceof Error ? err : new ConfigurationError(String(err)),
            );
          }
        })
        .on("click", function (_event, rc) {
          try {
            interactions?.update(
              Targets.CATEGORY_HEADERS,
              Actions.CLICK,
              rc.properties,
            );
          } catch (err: unknown) {
            appCore.err(
              err instanceof Error ? err : new ConfigurationError(String(err)),
            );
          }
        });

      enteredNodes.append("rect").attr("class", "cat").attr("stroke", "none");

      enteredNodes.append("rect").attr("class", "handle");

      enteredNodes.append("text").attr("dominant-baseline", "hanging");

      const mergedNodes = enteredNodes.merge(nodes);

      applyMaybeTransition(mergedNodes, transitionDurationMs, setNodeFrame);

      applyMaybeTransition(
        mergedNodes.select<SVGRectElement>("rect.cat"),
        transitionDurationMs,
        (update) => {
          update.attr("width", (rc) => rc.width);
          update.attr("height", (rc) => rc.height);
          update.attr(
            "fill",
            (rc) => coloring.colors(rc.properties).primary || "",
          );
        },
      );

      applyMaybeTransition(
        mergedNodes.select<SVGRectElement>("rect.handle"),
        transitionDurationMs,
        (update) => {
          update.attr("width", (rc) => rc.renderSettings.categoryHandleValPx);
          update.attr("height", (rc) => rc.renderSettings.categoryHeaderCatPx);
          update.attr(
            "fill",
            (rc) => coloring.colors(rc.properties).secondary || "",
          );
        },
      );

      mergedNodes
        .select<SVGTextElement>("text")
        .attr("y", 1)
        .attr("x", (rc) => rc.renderSettings.categoryHandleValPx)
        .attr("fill", (rc) => coloring.colors(rc.properties).stroke || "")
        .text((rc) => getLabel(rc.properties));
    } catch (err: unknown) {
      appCore.err(
        err instanceof Error ? err : new ConfigurationError(String(err)),
      );
    }
  }, [appCore, interactions, renderedCategories, transitionDurationMs]);

  return (
    <svg ref={svgRef} className={className} style={{ display: "block" }} />
  );
}
