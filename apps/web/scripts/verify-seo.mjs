import assert from 'node:assert/strict'
import { readFile, access } from 'node:fs/promises'
import { JSDOM } from 'jsdom'

const paths = ['/', '/investment', '/branches', '/contact', '/guide/customer']
const titles = new Set()
let checks = 0
for (const path of paths) for (const language of ['th', 'en']) {
  const file = `dist/__pages/${language}/${path === '/' ? 'home' : path.slice(1)}.html`
  const html = await readFile(file, 'utf8')
  const document = new JSDOM(html).window.document
  const canonical = `https://www.rbcevstation.com${path}${language === 'en' ? '?lang=en' : ''}`
  assert.equal(document.documentElement.lang, language, file)
  assert.equal(document.querySelectorAll('h1').length, 1, `One H1: ${file}`)
  assert.ok(document.querySelector('#root').textContent.length > 300, `Prerendered content: ${file}`)
  assert.equal(document.querySelector('link[rel="canonical"]').href, canonical)
  assert.equal(document.querySelector('meta[property="og:url"]').content, canonical)
  assert.match(document.querySelector('meta[name="robots"]').content, /^index, follow/)
  assert.equal(document.querySelectorAll('link[rel="alternate"][hreflang]').length, 3)
  assert.equal(document.querySelectorAll('title').length, 1)
  assert.equal(document.querySelectorAll('meta[name="description"]').length, 1)
  assert.ok(document.querySelector('meta[name="description"]').content.length > 80)
  const graph = JSON.parse(document.querySelector('script[type="application/ld+json"]').textContent)['@graph']
  assert.ok(graph.some(item => item['@type'] === 'Organization'))
  assert.ok(!graph.some(item => item.aggregateRating || item.offers), 'No invented reviews or commercial offers')
  for (const link of document.querySelectorAll('a[href^="/"]')) {
    const target = new URL(link.href, 'https://www.rbcevstation.com')
    if (paths.includes(target.pathname)) assert.equal(target.searchParams.get('lang'), language === 'en' ? 'en' : null, `Language link in ${file}`)
  }
  for (const image of document.querySelectorAll('img')) {
    assert.ok(image.hasAttribute('alt'), `Image alternative: ${file}`)
    const source = image.getAttribute('src')
    if (source?.startsWith('/')) await access('dist' + source)
  }
  await access('dist' + new URL(document.querySelector('meta[property="og:image"]').content).pathname)
  if (path === '/branches') assert.match(document.querySelector('#root').textContent, language === 'th' ? /ยังไม่ได้ก่อสร้าง|ยังไม่ก่อสร้าง/ : /not yet built/)
  titles.add(document.title)
  checks += 16
}
assert.equal(titles.size, 10)
const sitemap = new JSDOM(await readFile('dist/sitemap.xml', 'utf8'), { contentType: 'application/xml' }).window.document
assert.equal(sitemap.querySelectorAll('url').length, 10)
assert.doesNotMatch(sitemap.documentElement.textContent, /login|dashboard|analysis|documents|sites\/|localhost|trycloudflare/)
for (const file of ['dist/index.html', 'dist/404.html']) {
  const doc = new JSDOM(await readFile(file, 'utf8')).window.document
  assert.equal(doc.querySelector('meta[name="robots"]').content, 'noindex, nofollow')
  assert.equal(doc.querySelector('link[rel="canonical"]'), null)
}
const robots = await readFile('dist/robots.txt', 'utf8')
assert.match(robots, /Sitemap: https:\/\/www\.rbcevstation\.com\/sitemap.xml/)
assert.doesNotMatch(robots, /Disallow: \/(?:login|dashboard|analysis|sites)/)
console.log(`SEO build verified: 10 prerendered language pages; ${checks}+ checks; sitemap, image files and private fallbacks verified.`)
