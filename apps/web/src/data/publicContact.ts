import { isPublicPath, publicHref } from '../seo/metadata'

export const publicLineUrl = 'https://line.me/R/ti/p/@424cdtee'

export function publicLoginHref(from: string, language: 'th' | 'en', params = new URLSearchParams()) {
  params.set('from', from)
  params.set('lang', language)
  return '/login?' + params.toString()
}

export function publicLoginReturn(from: string | null, language: 'th' | 'en') {
  const base = 'https://www.rbcevstation.com'
  try {
    const url = new URL(from || '/', base)
    if (url.origin === base && isPublicPath(url.pathname)) {
      return publicHref(url.pathname + url.search + url.hash, language)
    }
  } catch { /* Use the public homepage for an invalid return path. */ }
  return publicHref('/', language)
}
