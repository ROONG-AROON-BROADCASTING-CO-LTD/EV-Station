import type { Content, TDocumentDefinitions } from 'pdfmake/interfaces'

type ExportOptions = { element: HTMLElement; title: string; language: 'th' | 'en'; reportId: string }
const excluded = '.print-hide,.screen-map,[data-pdf-ignore="true"],[role="dialog"],script,style,svg,iframe,button,input,canvas'
const blockTags = new Set(['P', 'DIV', 'SECTION', 'ARTICLE', 'HEADER', 'FOOTER', 'FIGURE', 'FIGCAPTION', 'DETAILS', 'SUMMARY', 'H1', 'H2', 'H3', 'H4', 'UL', 'OL', 'LI', 'TABLE', 'TR'])

function text(element: Node): string {
  if (element.nodeType === Node.TEXT_NODE) return element.textContent || ''
  if (!(element instanceof HTMLElement)) return ''
  if (element.matches(excluded) || element.hidden) return ''
  if (element.dataset.pdfText !== undefined) return element.dataset.pdfText
  if (element.tagName === 'BR') return '\n'
  if (element instanceof HTMLSelectElement) return element.selectedOptions[0]?.textContent || ''
  const value = Array.from(element.childNodes).map(text).join('')
  return blockTags.has(element.tagName) || element.classList.contains('block') ? `\n${value}\n` : value
}

function clean(value: string): string {
  return value.replace(/[\t\r ]+/g, ' ').replace(/ *\n */g, '\n').replace(/\n{3,}/g, '\n\n').trim()
}

// Export the report's already-authorized, translated view. In particular, do not
// reconstruct scores from raw API data and resurrect legacy/unconfirmed values.
export function reportContent(element: HTMLElement): Content[] {
  function walk(node: Node): Content[] {
    if (node.nodeType === Node.TEXT_NODE) {
      const value = clean(node.textContent || '')
      return value ? [{ text: value, margin: [0, 0, 0, 3] }] : []
    }
    if (!(node instanceof HTMLElement) || node.matches(excluded) || node.hidden) return []
    if (node.tagName === 'IMG') return []
    if (node.dataset.pdfText !== undefined) return [{ text: node.dataset.pdfText, margin: [0, 0, 0, 5] }]
    if (node.classList.contains('print-breakable-table')) {
      return [{ stack: walkChildren(node), style: 'metricsSection' }]
    }
    if (node instanceof HTMLSelectElement) return [{ text: clean(text(node)) }]
    if (node instanceof HTMLTableElement) {
      const body = Array.from(node.rows).map(row => Array.from(row.cells).map(cell => ({
        stack: walkChildren(cell), margin: [4, 4, 4, 4] as [number, number, number, number],
        ...(cell.tagName === 'TH' ? { bold: true, fillColor: '#eaf2f4' } : {}),
      })))
      if (!body.length) return []
      const count = Math.max(...body.map(row => row.length))
      const normalized = body.map(row => Array.from({ length: count }, (_, index) => row[index] || { stack: [{ text: '' }], margin: [4, 4, 4, 4] as [number, number, number, number] }))
      // Fixed A4 widths prevent long Thai words from expanding star columns
      // beyond the page. Give the evidence column most of the available space.
      const widths = count === 3 ? [105, 285, 105] : Array.from({ length: count }, () => (523 - count * 9) / count)
      return [{ table: { headerRows: node.tHead ? node.tHead.rows.length : 0, widths, body: normalized }, layout: 'lightHorizontalLines', margin: [0, 8, 0, 12] }]
    }
    if (/^H[1-4]$/.test(node.tagName) || node.tagName === 'SUMMARY') {
      const value = clean(text(node))
      return value ? [{ text: value, bold: true, color: '#10304b', fontSize: node.tagName === 'H1' ? 20 : node.tagName === 'H2' ? 14 : 11, margin: [0, 9, 0, 5], headlineLevel: Number(node.tagName.slice(1)) || 3 }] : []
    }
    if (node.tagName === 'UL' || node.tagName === 'OL') {
      const items = Array.from(node.children).map(item => clean(text(item)).replace(/^[•·]\s*/, '')).filter(Boolean)
      return items.length ? [{ ul: items, margin: [0, 3, 0, 8] }] : []
    }
    if (node.tagName === 'P' || node.tagName === 'SPAN' || !Array.from(node.children).some(child => blockTags.has(child.tagName))) {
      const value = clean(text(node))
      if (!value) return []
      const href = node instanceof HTMLAnchorElement && /^https?:/.test(node.href) ? node.href : undefined
      return [{ text: value, ...(href ? { link: href, color: '#007c80' } : {}), margin: [0, 0, 0, 5] }]
    }
    return walkChildren(node)
  }
  function walkChildren(node: HTMLElement): Content[] { return Array.from(node.childNodes).flatMap(walk) }
  return walkChildren(element)
}

export function reportFilename(title: string, reportId: string): string {
  const safeTitle = Array.from(title, character => character.charCodeAt(0) < 32 ? '-' : character).join('').replace(/[<>:"/\\|?*]/g, '-').trim().slice(0, 90) || 'RBC-analysis'
  return `${safeTitle}-${reportId.replace(/[^a-zA-Z0-9-]/g, '').slice(0, 36)}.pdf`
}

export function reportDefinition(options: ExportOptions): TDocumentDefinitions {
  const th = options.language === 'th'
  const mapLink = options.element.querySelector<HTMLAnchorElement>('a[data-pdf-link="true"]')
  const mapUrl = mapLink && /^https?:/.test(mapLink.href) ? mapLink.href : undefined
  return {
    pageSize: 'A4', pageMargins: [36, 40, 36, 42],
    info: { title: options.title, author: 'RBC EV Station', subject: options.reportId },
    defaultStyle: { font: 'IBMPlexSansThai', fontSize: 10, color: '#20364e', lineHeight: 1.2 },
    styles: { metricsSection: {} },
    content: [
      { text: 'RBC EV STATION', bold: true, fontSize: 16, color: '#007c80', margin: [0, 0, 0, 6] },
      { text: `${th ? 'เลขที่รายงาน' : 'Report ID'}: ${options.reportId}`, fontSize: 8, color: '#64748b', margin: [0, 0, 0, 10] },
      { text: th ? 'รายงานผลวิเคราะห์แบบข้อความและตาราง · ดูภาพประกอบในรายงานบนเว็บไซต์' : 'Analysis results in text and tables · View illustrations in the web report', fontSize: 8, color: '#64748b', margin: [0, 0, 0, 6] },
      ...(mapUrl ? [{ text: th ? 'เปิดพิกัดพื้นที่ใน Google Maps' : 'Open site coordinates in Google Maps', link: mapUrl, color: '#007c80', fontSize: 9, margin: [0, 0, 0, 8] as [number, number, number, number] }] : []),
      ...reportContent(options.element),
    ],
    footer: (page, total) => ({ text: `RBC EV Station | ${th ? 'หน้า' : 'Page'} ${page} / ${total}`, alignment: 'center', fontSize: 8, color: '#64748b', margin: [0, 10, 0, 0] }),
    // Keep a section heading with its first paragraph, including long reports.
    pageBreakBefore: (node, container) => Boolean(
      (node.style === 'metricsSection' && node.startPosition?.verticalRatio > 0.72)
      || (node.headlineLevel && (node.startPosition?.verticalRatio > 0.88 || container.getFollowingNodesOnPage().length === 0)),
    ),
  }
}

const thaiWords = new Intl.Segmenter('th', { granularity: 'word' })
export function addThaiLineBreaks(value: string): string {
  // pdfmake's Unicode line breaker does not perform Thai dictionary breaking.
  // Break between words, never between a Thai base letter and its vowel marks.
  return value.replace(/[\u0E00-\u0E7F]+/g, run => Array.from(thaiWords.segment(run), item => item.segment).join('\u200b'))
}

function preparePDFText(value: unknown, key = ''): unknown {
  if (typeof value === 'string') return key === 'link' ? value : addThaiLineBreaks(value)
  if (Array.isArray(value)) return value.map(item => preparePDFText(item, key))
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([name, item]) => [name, preparePDFText(item, name)]))
  return value
}

let fontsPromise: Promise<Record<string, string>> | undefined
async function fonts() {
  if (!fontsPromise) {
    fontsPromise = Promise.all(['IBMPlexSansThai-Regular.ttf', 'IBMPlexSansThai-Bold.ttf'].map(async name => {
      const response = await fetch(`/fonts/ibm-plex-sans-thai/${name}`, { signal: AbortSignal.timeout(15_000) })
      if (!response.ok) throw new Error('PDF_FONT_UNAVAILABLE')
      const bytes = new Uint8Array(await response.arrayBuffer())
      let binary = ''
      for (let offset = 0; offset < bytes.length; offset += 0x8000) binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000))
      return [name, btoa(binary)] as const
    })).then(Object.fromEntries).catch(error => { fontsPromise = undefined; throw error })
  }
  return fontsPromise
}

export async function downloadAnalysisPDF(options: ExportOptions): Promise<void> {
  // Snapshot before loading the PDF engine so a background refresh cannot mix
  // two report versions into a single download. No report is sent to a service.
  const definition = reportDefinition(options)
  definition.content = preparePDFText(definition.content) as Content
  const [{ default: pdfMake }, fontFiles] = await Promise.all([import('pdfmake/build/pdfmake'), fonts()])
  pdfMake.addVirtualFileSystem(fontFiles)
  pdfMake.addFonts({ IBMPlexSansThai: { normal: 'IBMPlexSansThai-Regular.ttf', bold: 'IBMPlexSansThai-Bold.ttf', italics: 'IBMPlexSansThai-Regular.ttf', bolditalics: 'IBMPlexSansThai-Bold.ttf' } })
  await pdfMake.createPdf(definition).download(reportFilename(options.title, options.reportId))
}
