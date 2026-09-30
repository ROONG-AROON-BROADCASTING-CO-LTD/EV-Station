import { createContext, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react'
import { translations, type Language } from './translations'

type TranslationKey = keyof typeof translations.th
type I18nValue = { language: Language; setLanguage: (language: Language) => void; t: (key: string) => string }

const I18nContext = createContext<I18nValue | null>(null)

export function I18nProvider({ children }: PropsWithChildren) {
  const [language, setLanguageState] = useState<Language>(() => (localStorage.getItem('rbc-language') as Language) || 'th')
  useEffect(() => { document.documentElement.lang = language }, [language])
  const setLanguage = (next: Language) => { setLanguageState(next); localStorage.setItem('rbc-language', next) }
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
  }), [language])
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>
}

export function useI18n() {
  const value = useContext(I18nContext)
  if (!value) throw new Error('useI18n must be used within I18nProvider')
  return value
}

export function LanguageSwitcher() {
  const { language, setLanguage } = useI18n()
  return <div className="language-switcher" aria-label="Language"><button type="button" className={language === 'th' ? 'active' : ''} onClick={() => setLanguage('th')}>ไทย</button><button type="button" className={language === 'en' ? 'active' : ''} onClick={() => setLanguage('en')}>EN</button></div>
}
