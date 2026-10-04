import { createReadStream, copyFileSync, existsSync, mkdirSync } from 'node:fs'
import type { IncomingMessage, ServerResponse } from 'node:http'
import { resolve } from 'node:path'
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

function previewNotFound() {
  return {
    name: 'preview-not-found',
    configurePreviewServer(server: { middlewares: { use: (fn: (req: IncomingMessage, res: ServerResponse, next: () => void) => void) => void } }) {
      return () => {
        server.middlewares.use((req, res, next) => {
          if (res.headersSent) return next()
          const url = req.url?.split('?')[0] ?? ''
          if (url.includes('/assets/') || /\.\w+$/.test(url)) return next()
          const file = resolve(__dirname, 'dist/404.html')
          if (!existsSync(file)) return next()
          res.statusCode = 404
          res.setHeader('Content-Type', 'text/html; charset=utf-8')
          createReadStream(file).pipe(res)
        })
      }
    },
  }
}

function copyRootFiles() {
  return {
    name: 'copy-root-files',
    closeBundle() {
      const dist = resolve(__dirname, 'dist')
      mkdirSync(dist, { recursive: true })
      copyFileSync(resolve(__dirname, 'install.sh'), resolve(dist, 'install.sh'))
      copyFileSync(resolve(__dirname, 'install.ps1'), resolve(dist, 'install.ps1'))
    },
  }
}

export default defineConfig({
  base: '/rock/',
  appType: 'mpa',
  plugins: [react(), tailwindcss(), copyRootFiles(), previewNotFound()],
  optimizeDeps: {
    include: ['stats.js', 'three', '@react-three/fiber', '@react-three/drei', '@react-three/postprocessing'],
  },
  build: {
    modulePreload: {
      resolveDependencies(_filename, deps) {
        return deps.filter((dep) => !dep.includes('clay-') && !dep.includes('YardCanvas'))
      },
    },
    rollupOptions: {
      input: {
        main: resolve(__dirname, 'index.html'),
        notfound: resolve(__dirname, '404.html'),
      },
      output: {
        manualChunks(id) {
          if (
            id.includes('node_modules/three') ||
            id.includes('@react-three') ||
            id.includes('/mokei/pkg/kit/clay') ||
            id.includes('three/')
          ) {
            return 'clay'
          }
        },
      },
    },
  },
})
