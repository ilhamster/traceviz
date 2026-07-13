import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: '/react/',
  resolve: {
    // The app and workspace-linked React package import client-core through
    // different symlink paths.  They must share one copy so TraceViz Value
    // runtime type checks work across component interaction boundaries.
    dedupe: ['@traceviz/client-core'],
    preserveSymlinks: true,
  },
  build: {
    minify: false,
    sourcemap: true,
  },
});
