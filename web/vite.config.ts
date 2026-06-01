import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'

export default defineConfig({
  plugins: [react()],
  css: {
    transformer: 'lightningcss',
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    cssMinify: 'lightningcss',
    target: 'esnext',
    sourcemap: false,
    minify: 'esbuild',
    modulePreload: { polyfill: false },
    commonjsOptions: {
      transformMixedEsModules: true,
    },
    rollupOptions: {
      output: {
        manualChunks: {
          mui: ['@mui/material', '@mui/icons-material', '@emotion/react', '@emotion/styled'],
        },
      },
    },
  },
  esbuild: {
    legalComments: 'none',
  },
  server: {
    port: 5174,
    proxy: {
      '/api': 'http://127.0.0.1:5173',
      '/ws': {
        target: 'ws://127.0.0.1:5173',
        ws: true,
      },
    },
  },
})
