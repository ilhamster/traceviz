# TraceViz React Notes

## Scope
Applies to code under `client/react`.

## TypeScript/React
- Prefer explicit types for component props, hook state, and helper functions.
- Keep React helpers in `core/` and visual components in `components/`.
- Avoid implicit `any` and keep module boundaries tidy (barrel exports in `index.ts`).
