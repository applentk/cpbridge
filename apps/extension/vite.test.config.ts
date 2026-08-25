import { resolve } from 'path';
import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    target: 'es2022',
    outDir: 'dist/test',
    emptyOutDir: true,
    minify: false,
    rollupOptions: {
      preserveEntrySignatures: 'strict',
      input: {
        atcoder: resolve(__dirname, 'src/platforms/atcoder.ts'),
        codeforces: resolve(__dirname, 'src/platforms/codeforces.ts')
      },
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: '[name].js'
      }
    }
  }
});
