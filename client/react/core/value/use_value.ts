import { useEffect, useState } from 'react';

import type { Value } from '@traceviz/client-core';

// TraceViz values expose a `val` getter/setter but the interface does not
// declare it. This captures the runtime shape we rely on.
export type ValueWithVal<T> = Value & { val: T };

// Returns a React state value that tracks the provided TraceViz Value.
export function useValue<T>(
  value: ValueWithVal<T> | undefined,
  fallback?: T,
): T | undefined {
  const [current, setCurrent] = useState<T | undefined>(() =>
    value ? value.val : fallback,
  );

  useEffect(() => {
    if (!value) {
      setCurrent(fallback);
      return;
    }
    // Sync immediately in case the value object changed.
    setCurrent(value.val);
    const sub = value.subscribe((next: Value) => {
      setCurrent((next as ValueWithVal<T>).val);
    });
    return () => sub.unsubscribe();
  }, [value, fallback]);

  return current;
}
