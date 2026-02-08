import { createContext, useContext } from 'react';

import type { AppCore } from '@traceviz/client-core';

export const AppCoreContext = createContext<AppCore | null>(null);

// Provides the global, singleton AppCore, allowing TraceViz React components to
// use its services.  Any component using this must be a descendant of an
// AppCoreContext.Provider; thus, TraceViz React tools should wrap their entire
// generated React component tree in such a provider.
export function useAppCore(): AppCore {
  const core = useContext(AppCoreContext);
  if (!core) {
    throw new Error('AppCore not available (missing AppCoreContext provider)');
  }
  return core;
}
