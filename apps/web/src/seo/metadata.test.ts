import { describe, expect, it } from 'vitest'
import { isPublicPath, metadataHtml, pageMetadata, publicHref, publicPaths, safeJson, urlLanguage } from './metadata'

describe('SEO route policy', () => {
  it('creates unique canonical metadata and reciprocal language URLs for every public page', () => {
    const titles = new Set<string>()
    for (const path of publicPaths) for (const language of ['th', 'en'] as const) {
      const meta = pageMetadata(path, language)
      titles.add(meta.title)
      expect(meta.robots).toContain('index, follow')
      expect(meta.canonical).toBe(`https://www.rbcevstation.com${path}${language === 'en' ? '?lang=en' : ''}`)
      expect(meta.alternates.map(item => item.language)).toEqual(['th', 'en', 'x-default'])
      expect(JSON.stringify(meta.structuredData)).not.toMatch(/AggregateRating|reviewRating|priceCurrency/)
    }
    expect(titles.size).toBe(10)
  })

  it.each(['/login', '/dashboard', '/analysis/customer-id', '/sites/secret/documents', '/documents', '/guide/admin', '/liff/site-submission', '/unknown'])('does not expose public SEO on %s', path => {
    const meta = pageMetadata(path, 'th')
    expect(meta.robots).toBe('noindex, nofollow')
    expect(meta.canonical).toBeUndefined()
    expect(meta.alternates).toEqual([])
    expect(meta.structuredData).toBeUndefined()
    expect(metadataHtml(path, 'th')).not.toContain('customer-id')
  })

  it('keeps query strings and hashes for public navigation without modifying private or external links', () => {
    expect(publicHref('/branches?album=ayutthaya#photos', 'en')).toBe('/branches?album=ayutthaya&lang=en#photos')
    expect(publicHref('/branches?lang=en&album=ayutthaya', 'th')).toBe('/branches?album=ayutthaya')
    expect(publicHref('/login?returnTo=%2Fsites%2Fnew', 'en')).toBe('/login?returnTo=%2Fsites%2Fnew')
    expect(publicHref('https://example.org/', 'en')).toBe('https://example.org/')
    expect(isPublicPath('/branches/secret')).toBe(false)
    expect(urlLanguage('?lang=invalid')).toBe('th')
  })

  it('escapes JSON-LD script terminators', () => {
    expect(safeJson({ text: '</script><script>alert(1)</script>' })).not.toContain('<')
  })
})
