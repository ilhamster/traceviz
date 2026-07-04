# Causal Tracing

## What This Tool Does

This directory contains a TraceViz tool for loading and exploring causal trace
data. The current data source loads extended OpenTelemetry/Jaeger JSON traces
from the DeathStarBench social network benchmark and converts each trace into a
Tracey trace with explicit span suspends and causal dependencies.

The visualizer renders the selected trace through TraceViz's trace data model.
It supports trace corpus browsing, selectable category hierarchies, collapsible
categories, synthetic collapsed-category heatmaps, suspend and causal-event
subspans, search, span-focus causality inspection, critical path views, and
Tracey transform templates.

## Running The Tool

From the repository root, install dependencies if needed:

```sh
pnpm install
pnpm run install:causal-tracing
```

For normal local development, run:

```sh
pnpm run dev:causal-tracing
```

This builds the Go server and React client, starts a client watch build, and
serves the app at:

```text
http://localhost:7420/react/
```

For a non-watch local run after building the client:

```sh
pnpm run build:causal-tracing
pnpm run run:causal-tracing
```

The default scripts run with `--trace_root /`, which keeps root containment
checking enabled while allowing whole-filesystem corpus paths. If the UI corpus
path is empty, the backend uses `--default_trace_path`. To run the server target
directly:

```sh
bazel run //causal_tracing/server:server -- \
  --react_root "$PWD/causal_tracing/react-client/dist" \
  --trace_root / \
  --default_trace_path "$PWD/causal_tracing/testdata/compose-post-ct-logs.json"
```

Useful flags:

- `--port`: server port, default `7420`.
- `--react_root`: built React static asset directory. When set, `/react/`
  serves the client.
- `--trace_root`: root directory for trace files requested by the client. Use
  `/` for local whole-filesystem access, or a narrower directory to restrict
  loadable corpora.
- `--default_trace_path`: default corpus path. When `--trace_root` is set, this
  path must resolve inside that root.
- `--client_watch_cmd`: optional frontend watch command started and stopped
  with the server.

## Extended OTel Format Interpretation

The loader accepts a Jaeger-style JSON response with a top-level `data` array.
Each entry is treated as an independent trace in a corpus. Spans use Jaeger
fields such as `traceID`, `spanID`, `operationName`, `startTime`, `duration`,
`processID`, `references`, `tags`, and `logs`. `startTime`, `duration`, and log
`timestamp` values are interpreted as microseconds.

The converter keeps each raw span as a Tracey root span unless the span is
covered by a complete `tracey_call` / `tracey_return` pair. OTel references are
retained in span payloads but are not interpreted as causal Tracey parent/child
structure.

The extended OTel renderer currently exposes a flat service hierarchy and a
service-spawning hierarchy. The service-spawning hierarchy uses OTel `CHILD_OF`
references to place a span's service under the service path of the span that
created it. This is a visual hierarchy only; it does not make the OTel reference
causal.

The following log fields are interpreted when present:

- `type=suspend_start` and `type=suspend_stop`: suspend the containing span
  between the two timestamps.
- `type=lock_released` and `type=lock_acquired` with `lock_id`: create a lock
  dependency from the release to the later acquire.
- `type=call`, `type=start`, and `type=finish` with `connection_id`: create RPC
  call and return dependencies by pairing client call, server start, server
  finish, and client finish events.
- `type=mark` with `label`: create a Tracey mark at the log timestamp.
- `type=tracey_call` and `type=tracey_return` with `child_span_id` and
  optional `call_id`: model blocking calls in flat span lists. The caller is
  suspended between the call and return timestamps, the child span is created
  as an actual Tracey child span, a Tracey `call` dependency links the call
  event to the child span start, and a Tracey `return` dependency links the
  child span end to the return event.
- `type=tracey_dependency_origin` and
  `type=tracey_dependency_destination` with `dependency_id` and
  `dependency_type`: create a direct Tracey-style dependency. Supported direct
  dependency types are `spawn`, `send`, and `signal`.

`testdata/tracey-trace1-ct-logs.json` is a small single-trace corpus adapted
from Tracey's `test_trace.Trace1`. It is intended for exercising causal UI
features such as marks, suspends, direct dependency event chips, and critical
path rendering. It intentionally uses the current extended OTel conversion
model, so its Tracey child spans are encoded as distinct raw spans with stable
path-like span IDs. The converter uses `tracey_call` / `tracey_return` events
to reconstruct nested Tracey child spans from those flat raw spans.

## Testing

Run the causal tracing test suite from the repository root:

```sh
pnpm run test:causal-tracing
```

or equivalently:

```sh
bazel test //causal_tracing/...
```

The React client build can be checked independently with:

```sh
pnpm --filter ./causal_tracing/react-client run build
```

For build-affecting changes that may touch shared TraceViz packages, prefer the
repository-level validation flow from `AGENTS.md` when practical.

## Directory Layout

- `extendedotel`: extended OTel raw types, Tracey conversion, render adapter,
  causal-event rendering, span focus data, and transform support.
- `rendertrace`: trace-format-agnostic render interfaces and shared render
  request/model logic.
- `concurrency`: generic concurrency profile utilities used for collapsed
  category heatmaps.
- `data_source`: TraceViz `DataSeriesRequest` handlers, corpus loading,
  variant caching, and table/trace query responses.
- `service`: HTTP service wiring for TraceViz data requests.
- `server`: runnable local server binary.
- `react-client`: React UI for the causal tracing tool.
- `testdata`: sample extended OTel corpora and schema.
