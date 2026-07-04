# Causal Tracing Agent Notes

## General style

- Keep packages small and purpose-specific. Prefer a clean directory boundary
  over a broad utility package.
- Document every exported package, type, function, constant group, and
  non-obvious behavior. Comments should describe semantics, invariants, or
  tradeoffs, not restate syntax.
- Avoid unnecessary abstractions. Add an abstraction only when it carries a
  clear interface boundary, removes meaningful duplication, or protects
  generic code from trace-format-specific semantics.
- Avoid special-cases in generic layers. When a special-case is unavoidable,
  keep it in the trace-format-specific adapter and document why the generic
  model cannot express it.
- Keep source files organized consistently: public model/API near the top,
  followed by constructors/helpers, then private implementation details.
- Prefer explicit names over abbreviations when readability benefits.
- Keep TODOs specific and actionable; avoid leaving obsolete plan text in the
  codebase.

## Interface design

- Treat `RenderableTrace` and related render interfaces as the central design
  artifact. Review interface changes carefully before building higher-level UI
  features on top of them.
- Keep interfaces general over trace provenance. Extended OTel is one adapter,
  not the model the tool is built around.
- Do not hard-code OTel-specific concepts into shared TraceViz React
  components, TraceViz trace data model helpers, renderable-trace interfaces,
  or generic rendered-span/category abstractions.
- Prefer reusable TraceViz concepts: traces, categories, spans, subspans,
  trace edges, payloads, Values, labels, colors, and render settings.

## Frontend/backend boundary

- Keep the frontend thin. High-semantic work that understands the visualized
  trace data belongs in Go backend code.
- Frontend state should live in TraceViz `Value`s wherever practical, including
  selected trace, hierarchy, zoom extent, expansion state, search text, and
  display-mode toggles.
- Backend communication must use TraceViz `DataSeriesRequest` and
  `DataSeriesResponse`; do not add ad hoc REST endpoints or custom response
  protocols for tool data.
- Current viewport width is meaningful render input. Pass it as a TraceViz
  `Value` on relevant requests so the backend can thin, bin, and elide data.

## Rendering and styling

- Use the existing TraceViz trace data model as the frontend wire model.
- Represent causal event chips, suspend intervals, heatmap buckets, and similar
  temporal overlays with TraceViz subspans when possible.
- Styling should be general and backend-parameterized through TraceViz render
  settings, labels, colors, and properties. Avoid tool-specific styling baked
  into shared React components.
- Tool-specific styling must remain overrideable by the tool and must not leak
  into generic TraceViz packages.

## Performance

- Design for very large traces, including traces with O(100k) spans.
- Avoid sending raw trace-scale data to the browser when the current view cannot
  show it. Prefer backend filtering, binning, and elision.
- Elided data should usually be discoverable through user actions such as zoom,
  hierarchy expansion, search narrowing, or display-mode changes.
- Collapsed category rendering should hide large subtraces without losing
  navigational cues.

## Testing

- Prefer realistic fixtures that can exercise the loader, renderable-trace
  model, backend data series, and React view together.
- Invest in a good extended-OTel-style JSON test trace, adapted from Tracey
  test cases where useful.
- Test backend TraceViz `DataSeriesResponse` output directly where possible;
  this gives near-browser coverage without requiring a browser.
- Use table-driven tests for parser, rendering, and data-source behavior.
- For rendered responses, assert meaningful properties and include comments in
  tests when a fixture expectation is subtle or surprising.
- Add focused regression tests for parser/selector semantics, trace transforms,
  critical path rendering, zoom clipping, and category expansion behavior when
  changing those areas.

## Validation

- Prefer `bazel test //causal_tracing/...` or `pnpm run test:causal-tracing`
  for Go/backend changes.
- Run `pnpm --filter ./causal_tracing/react-client run build` for React
  changes.
- For changes that affect shared TraceViz packages or workspace build wiring,
  follow the repository-level guidance in `../AGENTS.md`.
