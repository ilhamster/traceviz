import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: '/react/',
  resolve: {
    preserveSymlinks: true,
  },
  build: {
    minify: false,
    sourcemap: true,
  },
});
