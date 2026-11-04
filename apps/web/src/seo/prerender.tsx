import { renderToString } from 'react-dom/server'
import { MemoryRouter } from 'react-router-dom'
import { I18nProvider } from '../i18n/I18nProvider'
import { LandingPage } from '../pages/LandingPage'
import { InvestmentPage } from '../pages/InvestmentPage'
import { BranchesPage } from '../pages/BranchesPage'
import { ContactPage } from '../pages/ContactPage'
import { GuidePage } from '../pages/GuidePage'
import { metadataHtml, publicHref, publicPaths, siteOrigin, type SiteLanguage } from './metadata'

export { metadataHtml, publicPaths, siteOrigin }

export function renderPage(pathname: string, language: SiteLanguage) {
  const page = pathname === '/' ? <LandingPage /> : pathname === '/investment' ? <InvestmentPage /> : pathname === '/branches' ? <BranchesPage /> : pathname === '/contact' ? <ContactPage /> : <GuidePage audience="customer" />
  return renderToString(<MemoryRouter initialEntries={[publicHref(pathname, language)]}><I18nProvider initialLanguage={language}>{page}</I18nProvider></MemoryRouter>)
}
