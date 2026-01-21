# Get started with TraceViz

TraceViz comes bundled with a demo app, LogViz.  You can read more about its
structure at [A TraceViz Tool](./a_traceviz_tool.md), but you can try it out
right away.  You'll need to ensure you have
[Node.js](https://nodejs.org/), and
pnpm (e.g. `corepack enable && corepack prepare pnpm@latest --activate`),
[Angular](https://angular.dev/installation), and
[Go](https://go.dev/doc/install) installed.  Then, from the repository root,

```sh
traceviz$ pnpm run demo
```

TraceViz also supports a Bazel build path.  To run all tests and then run the
demo with Bazel, ensure that Bazel is installed, and then run:

```sh
traceviz$ bash bazel_demo.sh
```

For cleanup helpers, the root `package.json` includes:

```sh
pnpm run bazel-reset
pnpm run reset-all
```

If all goes well, either of these commands will:

*   build and test the TraceViz client core libraries;
*   build and test the TraceViz core Angular library and a set of included
    Angular components;
*   build the LogViz client application; and
*   launch the LogViz server.

and conclude with a message like:

```sh
Serving LogViz at http://mac.lan:7410
```

Open that link in a browser (you may be able to click it) to explore LogViz.
You can read [its template](../logviz/client/src/app/app.component.html) to
learn about its capabilities, but among other things,

*   Mousing over a row in the 'Source files' table (upper left) highlights log
    entries for the moused-over source file in the 'Raw event' table (upper
    right).
*   Clicking a row in the 'Source files' table filters both the 'Raw event'
    table and the 'Log messages over time' timeline (lower) to only log messages
    from the clicked row.  Shift-clicking supports multiselect; clicking again
    deselects.
*   Mousing over a row in the 'Raw event' table drops a vertical rule in the
    'Log messages over time' timeline at the moment of the moused-over event.
*   Brushing (clicking-and-dragging) in the 'Log messages over time' timeline
    zooms into the brushed time range, and filters all other views to that
    zoomed time range.  Double-clicking the timeline resets this view.
*   The WASD keys zoom in and out (W and S respectively) and pan left and right
    (A and D respectively) in the timeline (with the same global time filtering
    behavior).

## Bazel notes

Some Bazel targets wrap the existing pnpm/ng scripts.  These wrappers will
install `node_modules` on demand if missing, but you can also install manually:

```sh
pnpm --filter ./client/core install
```

For Angular library builds/tests, install dependencies once in the Angular
workspace:

```sh
pnpm --filter ./client/angular install
```

If you run Angular builds via Bazel, make sure the client core package is built
first (the wrapper will do this if needed):

```sh
pnpm --filter ./client/core run build
```

Logviz's client depends on both the core TS library and the Angular library.  If
you use the Bazel wrappers for Logviz, the wrapper will build those libraries
first if needed.

## Build validation

For a full validation pass of both `pnpm` and `bazel` build and tests paths, run
the root `validate.sh` script:

```sh
./validate.sh
```

This script resets pnpm and Bazel state, runs comprehensive pnpm build/test
commands, then runs Bazel build/test flows for `client/core`, `client/angular`,
and Logviz.  It is intentionally thorough (and therefore slow).
