import { describe, expect, it } from 'vitest'
import { addThaiLineBreaks, reportContent, reportDefinition, reportFilename } from './analysisPDF'

function report(html: string) { const element = document.createElement('div'); element.innerHTML = html; return element }

describe('downloadable report content', () => {
  it('includes Thai detail inside collapsed sections and excludes interactive or hidden UI', () => {
    const element = report('<h1>ประชาอุทิศ</h1><section><h2>สภาพพื้นที่</h2><p>ยังประเมินไม่ได้</p><details><summary>หลักฐาน</summary><p>ไม่มีภาพหน้างาน</p><ul><li>ขอภาพทางเข้า</li></ul></details><button>สร้างใหม่</button><div class="print-hide">private UI</div><div class="screen-map">map widget</div><script>bad()</script></section>')
    const result = JSON.stringify(reportContent(element))
    for (const value of ['ประชาอุทิศ', 'ยังประเมินไม่ได้', 'ไม่มีภาพหน้างาน', 'ขอภาพทางเข้า']) expect(result).toContain(value)
    for (const value of ['สร้างใหม่', 'private UI', 'map widget', 'bad()', '0/100']) expect(result).not.toContain(value)
  })

  it('keeps table headers, all rows, notes, and a valid zero score', () => {
    const element = report('<table><thead><tr><th>Metric</th><th>Result</th></tr></thead><tbody><tr><td>Surface</td><td><span>0/100</span><span class="block">Assessed from photos</span></td></tr><tr><td>Population</td><td>1000</td></tr></tbody></table>')
    const result = JSON.stringify(reportContent(element))
    expect(result).toContain('"headerRows":1')
    for (const value of ['Surface', '0/100', 'Assessed from photos', 'Population', '1000']) expect(result).toContain(value)
  })

  it('uses a safe download filename and identifies the report and language', () => {
    expect(reportFilename('รายงาน/พื้นที่: QA?', 'abc-123')).toBe('รายงาน-พื้นที่- QA--abc-123.pdf')
    const doc = reportDefinition({ element: report('<p>เนื้อหารายงาน</p>'), title: 'รายงาน', language: 'th', reportId: 'abc' })
    expect(doc.defaultStyle?.font).toBe('IBMPlexSansThai')
    expect(doc.info?.subject).toBe('abc')
    expect(JSON.stringify(doc.content)).toContain('เนื้อหารายงาน')
  })

  it('exports selected state text but omits inactive choices and hidden descendants', () => {
    const element = report('<section><div><p>Recommended to invest</p><p hidden>Hidden data</p></div><div data-pdf-ignore="true"><p>Not recommended</p></div><div data-pdf-text="Internet available"><button>Yes</button><button>No</button></div><p>Result <span hidden>secret</span></p></section>')
    const result = JSON.stringify(reportContent(element))
    expect(result).toContain('Recommended to invest')
    expect(result).toContain('Internet available')
    for (const value of ['Not recommended', 'Hidden data', 'secret', 'Yes']) expect(result).not.toContain(value)
  })

  it('marks metrics for pagination and exports only a marked safe map link', () => {
    const element = report('<a class="print-hide" data-pdf-link="true" href="https://www.google.com/maps?q=13,100">Map</a><section class="print-breakable-table"><h2>Metrics</h2><table><tr><td>Road</td><td>Known</td></tr></table></section>')
    const doc = reportDefinition({ element, title: 'Test', language: 'en', reportId: 'a' })
    const result = JSON.stringify(doc.content)
    expect(result).toContain('"style":"metricsSection"')
    expect(result).toContain('https://www.google.com/maps?q=13,100')
    element.querySelector('a')!.href = 'javascript:alert(1)'
    expect(JSON.stringify(reportDefinition({ element, title: 'Test', language: 'en', reportId: 'a' }).content)).not.toContain('javascript:')
  })

  it('adds Thai word breaks without losing letters or detaching vowel marks', () => {
    const original = 'การวิเคราะห์สภาพพื้นที่จริง'
    const result = addThaiLineBreaks(original)
    expect(result).toContain('\u200b')
    expect(result.replace(/\u200b/g, '')).toBe(original)
    expect(result).not.toMatch(/\u200b[\u0E31\u0E34-\u0E3A\u0E47-\u0E4E]/)
    expect(addThaiLineBreaks('Road access 80/100')).toBe('Road access 80/100')
  })
})
