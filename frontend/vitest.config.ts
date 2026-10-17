import { defineConfig, mergeConfig } from 'vitest/config'
import viteConfig from './vite.config.ts'

export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: 'jsdom',
      setupFiles: ['./src/test/setup.ts'],
      include: ['src/**/*.test.{ts,tsx}'],
      // Fixed so a local .env can't point tests at a real backend.
      env: { VITE_API_URL: 'http://api.test', VITE_MAPBOX_TOKEN: '' },
      css: false,
      coverage: {
        provider: 'v8',
        include: ['src/**/*.{ts,tsx}'],
        exclude: ['src/routeTree.gen.ts', 'src/main.tsx', 'src/test/**', 'src/components/ui/**', 'src/**/*.test.*']
      }
    }
  })
)
