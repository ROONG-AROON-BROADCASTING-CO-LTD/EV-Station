import { createContext, useCallback, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react'
import { useInRouterContext, useLocation, useNavigate } from 'react-router-dom'
import { isPublicPath } from '../seo/metadata'
import { translations, type Language } from './translations'

type TranslationKey = keyof typeof translations.th
type I18nValue = { language: Language; setLanguage: (language: Language) => void; t: (key: string) => string }

const I18nContext = createContext<I18nValue | null>(null)

export function I18nProvider({ children, initialLanguage }: PropsWithChildren<{ initialLanguage?: Language }>) {
  const [language, setLanguageState] = useState<Language>(() => {
    if (initialLanguage) return initialLanguage
    try { return localStorage.getItem('rbc-language') === 'en' ? 'en' : 'th' } catch { return 'th' }
  })
  useEffect(() => { document.documentElement.lang = language }, [language])
  const setLanguage = useCallback((next: Language) => {
    setLanguageState(next)
    try { localStorage.setItem('rbc-language', next) } catch { /* Language switching also works without browser storage. */ }
  }, [])
  const value = useMemo(() => ({
    language,
    setLanguage,
    t: (key: string) => {
      if (language !== 'th') return key
      const translated = key in translations.th ? translations.th[key as TranslationKey] : key
      return translated
        .replaceAll('คะแนนเบื้องต้น preliminary-v1', 'คะแนนคัดกรองรุ่นเบื้องต้น v1')
        .replaceAll('สูตรคะแนน preliminary-v1', 'สูตรคะแนนคัดกรองรุ่นเบื้องต้น v1')
    },
  }), [language, setLanguage])
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const value = useContext(I18nContext)
  if (!value) throw new Error('useI18n must be used within I18nProvider')
  return value
}

function LanguageButtons({ onChange }: { onChange?: (language: Language) => void }) {
  const { language, setLanguage } = useI18n()
  const change = (next: Language) => { setLanguage(next); onChange?.(next) }
  return <div className="language-switcher" aria-label="Language"><button type="button" className={language === 'th' ? 'active' : ''} onClick={() => change('th')}>ไทย</button><button type="button" className={language === 'en' ? 'active' : ''} onClick={() => change('en')}>EN</button></div>
}

function RoutedLanguageButtons() {
  const location = useLocation()
  const navigate = useNavigate()
  return <LanguageButtons onChange={language => {
    if (!isPublicPath(location.pathname)) return
    const params = new URLSearchParams(location.search)
    params.delete('lang')
    if (language === 'en') params.set('lang', 'en')
    navigate({ pathname: location.pathname, search: params.toString(), hash: location.hash }, { replace: true })
  }}/>
}

export function LanguageSwitcher() {
  const routed = useInRouterContext()
  return routed ? <RoutedLanguageButtons /> : <LanguageButtons />
}
