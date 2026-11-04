import { readFile, writeFile, mkdir } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'
import { createServer } from 'vite'

const vite = await createServer({ server: { middlewareMode: true }, appType: 'custom' })
try {
  const { renderPage, metadataHtml, publicPaths, siteOrigin } = await vite.ssrLoadModule('/src/seo/prerender.tsx')
  const template = await readFile('dist/index.html', 'utf8')
  const document = (path, language, body = '') => template
    .replace('<html lang="th">', `<html lang="${language}">`)
    .replace('<!--seo-head-->', metadataHtml(path, language))
    .replace('<div id="root"></div>', `<div id="root">${body}</div>`)
  for (const path of publicPaths) {
    for (const language of ['th', 'en']) {
      const target = resolve('dist/__pages', language, (path === '/' ? 'home' : path.slice(1)) + '.html')
      await mkdir(dirname(target), { recursive: true })
      await writeFile(target, document(path, language, renderPage(path, language)))
    }
  }
  // The SPA fallback is private by default; never ship indexable account pages.
  await writeFile('dist/index.html', document('/login', 'th'))
  await writeFile('dist/404.html', document('/not-found', 'th', '<main><h1>ไม่พบหน้าที่ต้องการ / Page not found</h1><p><a href="/">กลับหน้าแรก / Return home</a></p></main>'))
  const urls = publicPaths.flatMap(path => [path, path + '?lang=en']).map(path => `<url><loc>${siteOrigin}${path.replaceAll('&', '&amp;')}</loc></url>`).join('\n')
  await writeFile('dist/sitemap.xml', `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls}\n</urlset>\n`)
  // Account pages remain crawlable so search engines can see their noindex rule.
  await writeFile('dist/robots.txt', `User-agent: *\nAllow: /\nDisallow: /api/\nDisallow: /__pages/\nSitemap: ${siteOrigin}/sitemap.xml\n`)
  console.log(`Prerendered ${publicPaths.length * 2} public pages; generated sitemap and robots.txt.`)
} finally {
  await vite.close()
}
