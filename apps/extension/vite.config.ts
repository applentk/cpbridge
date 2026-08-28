import { defineConfig } from 'vite';
import { resolve } from 'path';

export default defineConfig(({ mode }) => ({
  define: {
    __CPBRIDGE_DEV__: mode === 'development'
  },
  build: {
    outDir: mode === 'development' ? '.dev/dist' : 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: {
        background: resolve(__dirname, 'src/background.ts'),
        bridge: resolve(__dirname, 'src/bridge.ts'),
        'codeforces-submit': resolve(__dirname, 'src/codeforces-submit.ts')
      },
      output: {
        entryFileNames: '[name].js',
        chunkFileNames: '[name].js',
        assetFileNames: '[name].[ext]'
      }
    }
  }
}));
