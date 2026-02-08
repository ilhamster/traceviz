import { type AppCore, type Value } from '@traceviz/client-core';

import { useValue, type ValueWithVal } from './use_value.ts';

// Returns a global Value from the AppCore's global state.
export function getGlobalValue(
  core: AppCore | null | undefined,
  key: string,
): Value | undefined {
  return core?.globalState.get(key);
}

// Returns a React state value that tracks the global TraceViz Value with the
// provided key.
export function useGlobalValue<T>(
  core: AppCore | null | undefined,
  key: string,
  fallback?: T,
): T | undefined {
  return useValue(
    getGlobalValue(core, key) as ValueWithVal<T> | undefined,
    fallback,
  );
}
