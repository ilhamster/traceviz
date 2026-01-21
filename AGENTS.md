# TraceViz Agent Notes

## Build philosophy
- Prefer the partial Bazel setup: Bazel wraps `pnpm`/`ng` for JS/TS/Angular. Avoid full Bazelification unless explicitly requested.
- For Logviz and other TraceViz tools, use Bazel targets that invoke the existing build tools (pnpm/ng/go) rather than re-implementing their build graphs.
- Keep pnpm build paths working. Avoid changes that only work under Bazel.
- For build-affecting changes, prefer running `./validate.sh` to exercise both pnpm and Bazel flows.
- Prefer clean baselines before validation; use `pnpm run bazel-reset`, `pnpm run reset`, or `pnpm run reset-all` as appropriate.

## Go code style
- Prefer descriptive names, except for very short-lived locals (e.g., `i`, `j`, `n` in tight loops).
- Prefer explicit names over abbreviations when it improves readability.
- Add comments for public types/functions and for non-obvious logic; avoid redundant comments for self-explanatory code.
- Prefer table-driven tests; keep tests human-readable with clear case names and expected values.
- Keep tests focused on behavior and use small, explicit fixtures.

## Cross-language architecture
- TODO: Document the data flow between Go producers (visualization data), TS libraries (data consumers/visualizable objects), and Angular components (view rendering).
