# Causal Tracing Plan

## Summary

Build causal tracing tooling under `causal_tracing` in stages:

1. Load extended OpenTelemetry/Jaeger causal trace JSON into Tracey.
2. Design and review a generic `RenderableTrace` model that can render any
   suitable Tracey-backed trace through TraceViz.
3. Build a React-based TraceViz visualizer on top of that renderable-trace
   model.
4. Add critical path and span-focus workflows after the base visualizer is
   working.

The visualizer must be generic over trace formats. Extended OTel is only the
first adapter. The tool should work for any trace implementation that can
convert data into `trace.Trace` and provide a `RenderableTrace`
implementation.

## Ground Rules

- `RenderableTrace` and related render interfaces are the central design
  artifact. They should be implemented and reviewed before building the full
  visualizer app.
- Keep shared interfaces and TraceViz React components general. Do not add
  OTel-specific fields, styling, assumptions, or behavior to generic TraceViz
  code or renderable-trace abstractions.
- Use the existing TraceViz data model as the frontend wire model: traces,
  categories, spans, subspans, trace edges, payloads, Values, labels, colors,
  and render settings.
- Keep the frontend thin. High-semantic work that understands trace data
  belongs in the Go backend; frontend state should live in TraceViz `Value`s
  wherever practical.
- Backend requests and responses must remain TraceViz
  `DataSeriesRequest`/`DataSeriesResponse`. Avoid ad hoc tool-specific
  protocols.
- Styling and rendering choices should be backend-parameterized and
  overrideable by tools. Prefer TraceViz render settings, colors, labels, and
  properties over hard-coded CSS or frontend logic.
- Design for large traces, including O(100k) spans. The backend should filter,
  bin, and elide data based on hierarchy state, zoom extent, search state, and
  viewport width.
- Elided data should usually be recoverable through UI actions such as zooming,
  expanding categories, narrowing search, or changing display modes.
- Testing should use realistic fixtures and should validate TraceViz
  `DataSeriesResponse` output directly where possible.

## Part 1: Extended OTel Tracey Loader

Build a new Go-based loader for the DeathStarBench causal
Jaeger/OpenTelemetry JSON format. The loader will preserve OTel span structure
as metadata, create Tracey spans as independent root spans, and construct
Tracey causality only from causal instrumentation logs.

Use Tracey `time.Duration` moments relative to the trace's earliest timestamp
for v1. Store the absolute base timestamp and original microsecond timestamps
in the wrapper and payloads.

### Loader Changes

- Add a new Go module under `causal_tracing` with Bazel targets matching the
  repo's partial Bazel style.
- Pin `github.com/ilhamster/tracey v0.0.0-20260113235238-f00f37f166c1`.
- Add fixture files under
  `causal_tracing/testdata/deathstarbench_social_network/`:
  - `causaltraces-schema.json`
  - `compose-post-ct-logs.json`
- Add package `extendedotel` for public trace-specific types:
  - Raw JSON structs for the schema: trace set, trace, span, reference,
    process, log, and key/value fields.
  - `Trace` wrapper containing the Tracey trace, raw trace ID, base time,
    process/span indexes, conversion diagnostics, and original raw trace.
  - `CategoryPayload`, `SpanPayload`, and `DependencyPayload`, all implementing
    `fmt.Stringer`.
  - Namer and enumerations for supported hierarchy and dependency types.
- Add package `extendedotel/load` for conversion:
  - `DecodeTraceSet(r io.Reader) (*extendedotel.RawTraceSet, error)`
  - `ConvertExtendedOtelTrace(raw extendedotel.RawTrace, opts ...Option) (*extendedotel.Trace, error)`
  - `ConvertExtendedOtelTraceSet(raw *extendedotel.RawTraceSet, opts ...Option) ([]*extendedotel.Trace, error)`

### Loader Behavior

- Create one Tracey root span per OTel span. Do not call Tracey `NewChildSpan`
  for OTel `CHILD_OF` or `FOLLOWS_FROM`.
- Preserve OTel references in `SpanPayload.References` and wrapper indexes
  only. They are not Tracey dependencies in v1.
- Add category hierarchies:
  - `Span`: one category leaf per span.
  - `ServiceProcessSpan`: service -> process ID -> span, using `processes`.
  - `OperationSpan`: operation name -> span.
- Add explicit suspends from paired `suspend_start` / `suspend_stop` logs
  within the same span.
- Add causal dependencies from instrumentation logs only:
  - `ConnectionCall`: matching `call` to service-side `start` by
    `connection_id`.
  - `ConnectionFinish`: matching service-side `finish` to caller/client-side
    `finish` by `connection_id` when a matching call/start pair exists.
  - `LockRelease`: previous `lock_released` to next `lock_acquired` with the
    same `lock_id`, within the same raw trace.
- Preserve unmatched or ambiguous causal events in span payloads and add wrapper
  diagnostics instead of guessing.
- Record span-scope diagnostics with a documented assumption: each OTel span is
  treated as one logical line of execution, but the loader does not try to split
  spans into hidden fibers/tasks.

## Part 2: Renderable Trace Model

This is the first visualizer deliverable and should be designed and reviewed
carefully before building the full app. The goal is a clean Go interface layer
between Tracey traces and TraceViz rendering.

### Design Goals

- The model is generic over trace formats and specific `trace.Trace`
  specializations. Extended OTel is one implementation, not the target API.
- The model owns Tracey-aware semantics server-side: hierarchy selection,
  search, expansion state, synthetic collapsed spans, causal events, viewport
  thinning, and later critical-path/span-focus computations.
- The frontend wire format remains the existing TraceViz data model:
  `server/go/trace`, `trace_edge`, subspans, payloads, labels, colors, and
  URL-state Values.
- Use `time.Duration` as the visualization-domain temporal type. Trace
  implementations with native temporal types such as `time.Time` should track
  their own origin/scale and translate between native moments and duration
  offsets internally.

### Proposed Interfaces

- `RenderableTrace`: wraps a Tracey trace and exposes stable metadata needed
  by the renderer:
  - trace ID, display name, full time range, and default render extent
  - render-time formatting for `time.Duration` moments
  - available category hierarchies and default hierarchy
  - stable category and span IDs based on Tracey unique paths
  - search support using Tracey span specifier parsing
  - root category views for a render view
  - optional defaults such as interesting critical-path endpoints.
- `CategoryView` and `SpanView`: non-generic view elements selected by the
  renderer:
  - category views expose child categories and root span views
  - span views expose child span views
  - each view renders itself into the existing TraceViz trace data model
  - span views may be real Tracey spans or synthetic spans, without the common
    renderer needing to distinguish those cases
  - subspans such as heatmap buckets, causal event chips, suspend intervals,
    and markers are trace-format-specific rendering choices owned by the span
    view implementation.
- `RenderRequest`: all view-state inputs:
  - hierarchy type for the main trace view
  - critical-path stack type for overtime critical-path ancestry; this is
    related to, but intentionally independent from, category hierarchy type
  - explicit expanded category IDs
  - search text
  - show-only-matches
  - expand-matches
  - hide-empty-categories
  - critical-path-only display mode, once critical paths are available
  - temporal domain for the visible time interval
  - trace-view range in pixels for the horizontal render extent
  - minimum feature width for pixel-based thinning.
- Rendering writes directly into TraceViz builders:
  - the generic renderer walks selected category and span views
  - category views render TraceViz categories
  - span views render TraceViz spans and any trace-specific subspans or
    payloads
  - critical-path `trace_edge` payload placement is intentionally not fixed yet.

### Rendering Rules

- Initial expansion: root categories open, deeper non-leaf categories collapsed,
  leaf/span categories visible.
- A category can be explicitly expanded, expanded-to-reveal-match, collapsed,
  or leaf/non-expanding.
- When a category is collapsed, its category view can expose a synthetic span
  directly under that category:
  - start = earliest visible descendant span/event moment
  - end = latest visible descendant span/event moment
  - properties identify it as synthetic and reference the backing category ID.
- Collapsed heatmap buckets, causal event chips, suspend intervals, and similar
  trace-specific details are rendered by the responsible span view, typically
  as TraceViz subspans.
- Use Tracey search parsing server-side. Matching spans and categories receive
  TraceViz properties/colors; show-only mode renders matches plus required
  ancestry.
- Hide-empty-categories mode removes categories with no rendered content in the
  current view, including categories whose work is entirely outside the zoom
  extent.
- Viewport width controls backend thinning: avoid emitting event chips,
  suspend subspans, or heatmap bins that cannot be visually distinguished at
  the current pixel scale.

## Part 3: Causal Tracing Visualizer MVP

Build a React TraceViz demo tool under `causal_tracing` that works on
`RenderableTrace` implementations. The app is generic over trace provenance;
the extended OTel loader supplies the first data source.

### Backend Queries

- `causal_tracing.trace_list`: trace IDs, labels, span counts, time ranges, and
  diagnostics for a trace-set file.
- `causal_tracing.trace_view`: one TraceViz trace response for the current
  `RenderRequest`.
- `causal_tracing.pan_and_zoom`: duration-based zoom/pan response compatible
  with the app's URL state.

### React UI

- Use `@traceviz/client-core` and `@traceviz/client-react`.
- Extend React trace components as needed for:
  - span click/mouseover interactions
  - brush zoom and reset zoom
  - WASD zoom/pan
  - category click interactions for expand/collapse
  - subspan rendering suitable for event chips, suspend intervals, and heatmap
    buckets.
- Add controls:
  - collection/file input
  - trace dropdown
  - hierarchy dropdown
  - search box
  - show-only-matches toggle
  - expand-matches toggle
  - hide-empty-categories toggle
  - reset zoom button.
- Persist view state through `UrlHash`: selected collection, trace ID,
  hierarchy ID, expanded category IDs, search text, match toggles, and zoom
  extent.

## Part 4: Critical Path And Span Focus

These are follow-on features after the visualizer MVP.

### Critical Path

- Add strategy dropdown backed by Tracey `criticalpath.CommonStrategies`.
- Add endpoint controls using Tracey position specifiers.
- Add display modes for critical-path exploration, including showing only work
  that lies on the current critical path while preserving required category and
  span ancestry.
- Add an overtime stack-policy selector backed by renderable-trace stack
  types. A stack policy may use a category hierarchy, but it can also include
  span parentage, synthetic category frames, RPC/process/thread boundaries, or
  other trace-specific ancestry.
- Render the main trace overlay with TraceViz `trace_edge`.
- Add an overtime critical-path pane as a second TraceViz trace sharing the
  same duration axis.
- When a critical-path segment passes through a hidden span, remap the segment
  to the deepest displayed ancestor or synthetic collapsed-category span.
- Treat zoom-window edge cases carefully: segments with only one visible
  endpoint may need clipping, elision, or a clear partial-edge representation.

### Span Focus

- Maintain selected span IDs as a stack in TraceViz Values.
- When the stack is nonempty, show a focused trace view with the stack spans
  and required ancestry.
- Add an event table for the stack head showing timestamp, event type, labels,
  and dependency other-end information.
- Clicking a dependency endpoint pushes the other-end span onto the stack.
- Provide controls to pop the stack or close focus mode.

## Test Plan

- Loader tests:
  - decode schema-shaped JSON into raw structs
  - convert spans into Tracey root spans with stable payload names/IDs
  - build category hierarchies
  - preserve `CHILD_OF` / `FOLLOWS_FROM` without creating dependencies
  - convert paired suspends into elementary-span gaps
  - create connection and lock dependencies only when keys match
  - emit diagnostics for unmatched suspend starts/stops, missing processes,
    unknown log event types, and unmatched causal keys.
- Renderable-trace tests:
  - hierarchy selection and stable IDs
  - root-open expansion defaults
  - explicit expansion/collapse
  - search matching, show-only rendering, and expand-matches behavior
  - hide-empty-categories behavior under zoom and filtering
  - collapsed-category synthetic spans and subspan heatmap buckets
  - suspend intervals and causal event chips emitted as subspans
  - viewport/event thinning decisions.
- React tests:
  - hierarchy/category click updates expansion state
  - search controls trigger query state changes
  - URL hash round-trips state
  - brush/reset/WASD update zoom values.
- End-to-end smoke test:
  - load `compose-post-ct-logs.json`
  - select a trace
  - render each hierarchy
  - search and collapse categories without frontend errors.
- Validation commands:
  - `go test ./...` from `causal_tracing`
  - Bazel test target for `causal_tracing/...`
  - relevant pnpm React/client tests
  - for build-affecting changes, run `./validate.sh` if time/cost is
    acceptable.

## Assumptions

- OTel references are semicausal metadata, not executable Tracey causality.
- Causal instrumentation logs are the only source of Tracey dependencies and
  suspends in v1.
- Categories are for navigation/reference, not causality.
- The existing TraceViz trace model is the frontend wire model.
- The visualizer operates on `RenderableTrace` implementations, not on
  extended OTel directly.
- The first implementation specializes to `time.Duration`; generalizing
  temporal type is a design-review topic for the renderable-trace model.
- Current viewport width is part of the render request so the backend can make
  pixel-aware performance decisions.
