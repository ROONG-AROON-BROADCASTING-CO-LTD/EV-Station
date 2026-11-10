import { useEffect } from 'react'
import { useLocation } from 'react-router-dom'
import { useI18n } from '../i18n/I18nProvider'
import { isPublicPath, metadataHtml, urlLanguage } from './metadata'

export function PageSeo() {
  const { pathname, search } = useLocation()
  const { language, setLanguage } = useI18n()
  const usesUrlLanguage = isPublicPath(pathname) || (pathname === '/login' && new URLSearchParams(search).has('lang'))
  const pageLanguage = usesUrlLanguage ? urlLanguage(search) : language
  useEffect(() => {
    if (usesUrlLanguage && language !== pageLanguage) setLanguage(pageLanguage)
    document.documentElement.lang = pageLanguage
    document.head.querySelectorAll('[data-rbc-seo]').forEach(element => element.remove())
    // The generated tags contain only escaped, application-owned metadata.
    const template = document.createElement('template')
    template.innerHTML = metadataHtml(pathname, pageLanguage)
    document.head.append(template.content)
  }, [pathname, pageLanguage, language, setLanguage, usesUrlLanguage])
  return null
}
