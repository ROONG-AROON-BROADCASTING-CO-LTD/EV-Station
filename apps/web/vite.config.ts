import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import type { Plugin } from 'vite'
import { readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { isPublicPath, metadataHtml, urlLanguage } from './src/seo/metadata'

function seoPages(): Plugin {
  return {
    name: 'rbc-public-seo',
    transformIndexHtml: {
      order: 'pre',
      handler(html, context) {
        if (!context.server) return html
        const url = new URL(context.originalUrl ?? context.path, 'http://localhost')
        const language = urlLanguage(url.search)
        return html.replace('<html lang="th">', `<html lang="${language}">`).replace('<!--seo-head-->', metadataHtml(url.pathname, language))
      },
    },
    configurePreviewServer(server) {
      server.middlewares.use(async (request, response, next) => {
        if (request.method !== 'GET' && request.method !== 'HEAD') return next()
        const url = new URL(request.url ?? '/', 'http://localhost')
        const publicPage = isPublicPath(url.pathname)
        const privatePage = /^\/(login|dashboard|documents|sites|analysis|analyses|settings|api-usage|not-investing|data-sources|liff|guide\/admin)(\/|$)/.test(url.pathname)
        if (!publicPage && !privatePage && !request.headers.accept?.includes('text/html')) return next()
        const file = publicPage ? `__pages/${urlLanguage(url.search)}/${url.pathname === '/' ? 'home' : url.pathname.slice(1)}.html` : privatePage ? 'index.html' : '404.html'
        try {
          const html = await readFile(resolve(server.config.root, 'dist', file))
          response.statusCode = publicPage || privatePage ? 200 : 404
          response.setHeader('Content-Type', 'text/html; charset=utf-8')
          response.setHeader('Cache-Control', 'no-store')
          if (!publicPage) response.setHeader('X-Robots-Tag', 'noindex, nofollow')
          response.end(request.method === 'HEAD' ? undefined : html)
        } catch (error) { next(error) }
      })
    },
  }
}

export default defineConfig({
  plugins: [react(), seoPages()],
  server: { port: 5173 },
  test: { exclude: ['**/e2e/**', '**/node_modules/**'] },
})
