import { defineConfig } from 'vitest/config'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// Dev: run `dungeon serve` (default 127.0.0.1:7878), open its printed URL once
// so the session cookie is set for 127.0.0.1, then browse the Vite server at
// http://127.0.0.1:5173. The proxy rewrites Origin so the server's guard sees
// a same-origin request.
interface ProxyLike {
  on(event: string, fn: (req: { setHeader(name: string, value: string): void }) => void): void
}

const target = process.env.DUNGEON_URL ?? 'http://127.0.0.1:7878'

export default defineConfig({
  plugins: [svelte()],
  // Vitest must load Svelte's browser build, not the server one.
  resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
  build: { outDir: 'dist', emptyOutDir: true, sourcemap: false },
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/api': {
        target,
        ws: true,
        changeOrigin: true,
        configure: (proxy: ProxyLike) => {
          const origin = new URL(target).origin
          proxy.on('proxyReq', (req) => req.setHeader('origin', origin))
          proxy.on('proxyReqWs', (req) => req.setHeader('origin', origin))
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts'],
    setupFiles: ['./src/test-setup.ts'],
  },
})
