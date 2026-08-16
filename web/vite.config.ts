import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The built assets are embedded into the sa binary (see web.go), so the
// output has to stay in dist/. During development everything under /_ is
// proxied to a running sa server (sa --foreground).
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/_': {
        target: 'http://localhost:6280',
        changeOrigin: false,
      },
    },
  },
})
