import { Link, type LinkProps } from 'react-router-dom'
import { useI18n } from '../i18n/I18nProvider'
import { publicHref } from '../seo/metadata'

/** Keep crawlable language URLs when navigating between public pages. */
export function PublicLink({ to, ...props }: LinkProps) {
  const { language } = useI18n()
  return <Link {...props} to={typeof to === 'string' ? publicHref(to, language) : to} />
}
