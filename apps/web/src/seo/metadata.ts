export type SiteLanguage = 'th' | 'en'
export const siteOrigin = 'https://www.rbcevstation.com'
export const publicPaths = ['/', '/investment', '/branches', '/contact', '/guide/customer'] as const
export type PublicPath = typeof publicPaths[number]

const pages: Record<PublicPath, Record<SiteLanguage, { title: string; description: string }>> = {
  '/': {
    th: { title: 'ลงทุนสถานีชาร์จ EV และประเมินทำเล | RBC EV Station', description: 'ศึกษาการลงทุนสถานีชาร์จ EV กับ RBC Group เปรียบเทียบรูปแบบโครงการ ดูสาขา Super Black Coffee และส่งข้อมูลที่ดินเพื่อประเมินทำเลเบื้องต้นก่อนสำรวจหน้างาน' },
    en: { title: 'EV Charging Station Investment & Site Review | RBC EV Station', description: 'Explore EV charging station projects with RBC Group. Compare station formats, discover Super Black Coffee locations and submit land details for a preliminary site review.' },
  },
  '/investment': {
    th: { title: 'รูปแบบลงทุนสถานีชาร์จ EV และเครื่องชาร์จ AC/DC | RBC EV Station', description: 'เปรียบเทียบโครงการสถานีชาร์จ EV ขนาด S M L พร้อมร้านกาแฟหรือสถานีชาร์จ ศึกษาเครื่องชาร์จ AC/DC การเตรียมพื้นที่และระบบไฟฟ้าก่อนปรึกษาทีม RBC' },
    en: { title: 'EV Station Formats & AC/DC Charging Solutions | RBC EV Station', description: 'Compare S, M and L EV station formats, café concepts and AC/DC chargers. Review land and electrical requirements before discussing your project with RBC.' },
  },
  '/branches': {
    th: { title: 'สาขาและโครงการ Super Black Coffee พร้อม EV Charging | RBC EV Station', description: 'ดูบริการและอัลบั้ม Super Black Coffee สาขาอยุธยาและพิษณุโลก พร้อมแบบโครงการประชาอุทิศที่ยังไม่ก่อสร้าง แผนที่และช่องทางติดต่อแต่ละสาขา' },
    en: { title: 'Super Black Coffee Locations & EV Projects | RBC EV Station', description: 'View services, photos, maps and contacts for Ayutthaya and Phitsanulok. Explore the proposed Pracha Uthit station design, which is not yet built.' },
  },
  '/contact': {
    th: { title: 'ติดต่อปรึกษาลงทุนสถานีชาร์จ EV | RBC EV Station', description: 'ปรึกษาทีม RBC EV Station เรื่องทำเลและรูปแบบสถานีชาร์จ EV โทร 02-970-7552 อีเมล rbcevstation@gmail.com หรือ LINE @424cdtee พร้อมข้อมูลติดต่อและแผนที่บริษัท' },
    en: { title: 'Contact the EV Station Project Team | RBC EV Station', description: 'Discuss your EV charging station site with RBC. Call +66 2 970 7552, email rbcevstation@gmail.com or contact LINE @424cdtee. Find our company address and map.' },
  },
  '/guide/customer': {
    th: { title: 'เตรียมข้อมูลที่ดินเพื่อประเมินสถานีชาร์จ EV | RBC EV Station', description: 'คู่มือเตรียมพิกัด ขนาดที่ดิน รูปพื้นที่และเอกสารประกอบ วิธีส่งข้อมูลและอ่านผลประเมินสถานีชาร์จ EV พร้อมข้อจำกัดของข้อมูลและสิ่งที่ต้องตรวจหน้างาน' },
    en: { title: 'Prepare Your Land for an EV Site Review | RBC EV Station', description: 'Prepare coordinates, land area, site photos and supporting documents. Learn how to submit a site and read a preliminary EV station assessment with its limitations.' },
  },
}

export function isPublicPath(pathname: string): pathname is PublicPath {
  return (publicPaths as readonly string[]).includes(pathname)
}

export function urlLanguage(search: string): SiteLanguage {
  return new URLSearchParams(search).get('lang') === 'en' ? 'en' : 'th'
}

export function publicHref(to: string, language: SiteLanguage): string {
  const url = new URL(to, siteOrigin)
  if (url.origin !== siteOrigin || !isPublicPath(url.pathname)) return to
  url.searchParams.delete('lang')
  if (language === 'en') url.searchParams.set('lang', 'en')
  return url.pathname + url.search + url.hash
}

export function pageMetadata(pathname: string, language: SiteLanguage) {
  const page = isPublicPath(pathname) ? pages[pathname][language] : undefined
  const canonical = page ? siteOrigin + publicHref(pathname, language) : undefined
  return {
    title: page?.title ?? (language === 'th' ? 'ระบบบัญชีและข้อมูลพื้นที่ | RBC EV Station' : 'Account & Site Workspace | RBC EV Station'),
    description: page?.description ?? (language === 'th' ? 'เข้าสู่ระบบเพื่อจัดการข้อมูลพื้นที่และรายงานของคุณ' : 'Sign in to manage your site records and reports.'),
    canonical,
    robots: page ? 'index, follow, max-image-preview:large' : 'noindex, nofollow',
    language,
    image: siteOrigin + '/rbc-motion/hero-40s.webp',
    alternates: page ? [
      { language: 'th', href: siteOrigin + pathname },
      { language: 'en', href: siteOrigin + publicHref(pathname, 'en') },
      { language: 'x-default', href: siteOrigin + pathname },
    ] : [],
    structuredData: page ? {
      '@context': 'https://schema.org',
      '@graph': [
        { '@type': 'Organization', '@id': siteOrigin + '/#organization', name: 'RBC EV Station', legalName: 'บริษัท รุ่งอรุณบรอดคาสติ้ง จำกัด', url: siteOrigin + '/', logo: siteOrigin + '/rbc-group-logo.png', telephone: '+66-2-970-7552', email: 'rbcevstation@gmail.com', address: { '@type': 'PostalAddress', streetAddress: '9 ซอยรามอินทรา 21 แยก 1 ถนนรามอินทรา แขวงท่าแร้ง', addressLocality: 'เขตบางเขน', addressRegion: 'กรุงเทพมหานคร', postalCode: '10220', addressCountry: 'TH' }, sameAs: ['https://line.me/R/ti/p/@424cdtee', 'https://www.facebook.com/profile.php?id=100083313259502'] },
        { '@type': 'WebSite', '@id': siteOrigin + '/#website', url: siteOrigin + '/', name: 'RBC EV Station', inLanguage: ['th', 'en'], publisher: { '@id': siteOrigin + '/#organization' } },
        { '@type': 'WebPage', '@id': canonical + '#webpage', url: canonical, name: page.title, description: page.description, inLanguage: language, isPartOf: { '@id': siteOrigin + '/#website' }, publisher: { '@id': siteOrigin + '/#organization' } },
        ...(pathname === '/' ? [] : [{ '@type': 'BreadcrumbList', itemListElement: [
          { '@type': 'ListItem', position: 1, name: language === 'th' ? 'หน้าแรก' : 'Home', item: siteOrigin + publicHref('/', language) },
          { '@type': 'ListItem', position: 2, name: page.title.split(' | ')[0], item: canonical },
        ] }]),
      ],
    } : undefined,
  }
}

export function safeJson(value: unknown): string {
  return JSON.stringify(value).replace(/</g, '\\u003c')
}

const escapeHtml = (value: string) => value.replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[c]!)

export function metadataHtml(pathname: string, language: SiteLanguage): string {
  const meta = pageMetadata(pathname, language)
  const tag = (name: string, content: string, property = false) => `<meta ${property ? 'property' : 'name'}="${name}" content="${escapeHtml(content)}" data-rbc-seo>`
  return [
    `<title data-rbc-seo>${escapeHtml(meta.title)}</title>`,
    tag('description', meta.description), tag('robots', meta.robots),
    ...(meta.canonical ? [`<link rel="canonical" href="${escapeHtml(meta.canonical)}" data-rbc-seo>`] : []),
    ...meta.alternates.map(item => `<link rel="alternate" hreflang="${item.language}" href="${escapeHtml(item.href)}" data-rbc-seo>`),
    tag('og:type', 'website', true), tag('og:site_name', 'RBC EV Station', true), tag('og:title', meta.title, true), tag('og:description', meta.description, true),
    tag('og:image', meta.image, true), tag('og:image:alt', language === 'th' ? 'ภาพแนวคิดสถานีชาร์จ RBC EV Station' : 'RBC EV Station concept visual', true),
    tag('og:locale', language === 'th' ? 'th_TH' : 'en_US', true),
    ...(meta.canonical ? [tag('og:url', meta.canonical, true)] : []),
    tag('twitter:card', 'summary_large_image'), tag('twitter:title', meta.title), tag('twitter:description', meta.description), tag('twitter:image', meta.image),
    ...(meta.structuredData ? [`<script type="application/ld+json" data-rbc-seo>${safeJson(meta.structuredData)}</script>`] : []),
  ].join('\n')
}
