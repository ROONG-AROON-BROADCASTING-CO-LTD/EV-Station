import { useEffect, useState } from 'react'
import { NewSitePage } from './NewSitePage'

type LiffSDK = {
  init: (options: { liffId: string }) => Promise<void>
  isLoggedIn: () => boolean
  login: (options: { redirectUri: string }) => void
  getIDToken: () => string | null
}

declare global { interface Window { liff?: LiffSDK } }

function loadLiffSDK(): Promise<LiffSDK> {
  if (window.liff) return Promise.resolve(window.liff)
  return new Promise((resolve, reject) => {
    const script = document.createElement('script')
    script.src = 'https://static.line-scdn.net/liff/edge/2/sdk.js'
    script.async = true
    script.onload = () => window.liff ? resolve(window.liff) : reject(new Error('โหลด LINE SDK ไม่สำเร็จ'))
    script.onerror = () => reject(new Error('ไม่สามารถเชื่อมต่อ LINE ได้'))
    document.head.appendChild(script)
  })
}

export function LiffSiteSubmissionPage() {
  const [idToken, setIDToken] = useState<string>()
  const [error, setError] = useState<string>()
  const [completed, setCompleted] = useState<boolean>()

  useEffect(() => {
    let active = true
    const initialise = async () => {
      try {
        const [sdk, configResponse] = await Promise.all([
          loadLiffSDK(),
          fetch('/api/v1/liff/config').then(async response => {
            if (!response.ok) throw new Error('ไม่สามารถเตรียมฟอร์มได้')
            return response.json() as Promise<{ liffId?: string }>
          }),
        ])
        if (!configResponse.liffId) throw new Error('ระบบยังไม่พร้อม กรุณาติดต่อทีมงานในแชต')
        await sdk.init({ liffId: configResponse.liffId })
        if (!sdk.isLoggedIn()) {
          sdk.login({ redirectUri: `${location.origin}${location.pathname}` })
          return
        }
        const token = sdk.getIDToken()
        if (!token) throw new Error('กรุณาเปิดฟอร์มใหม่จาก LINE เพื่อยืนยันตัวตน')
        if (active) { setIDToken(token); setError('') }
      } catch (nextError) {
        if (active) setError(nextError instanceof Error ? nextError.message : 'ไม่สามารถเชื่อมต่อ LINE ได้')
      }
    }
    void initialise()
    return () => { active = false }
  }, [])

  if (completed !== undefined) return <main className="mx-auto max-w-xl px-5 py-12"><section className="rounded-xl border border-line bg-white p-7 text-center shadow-panel"><h1 className="page-title">ส่งข้อมูลพื้นที่สำเร็จ</h1><p className="mt-3 text-muted">{completed ? 'ระบบส่งข้อความยืนยันไปยังแชต LINE ของคุณแล้ว ทีมงานจะตรวจสอบและติดต่อกลับ' : 'บันทึกข้อมูลแล้ว แต่ยังส่งข้อความยืนยันไม่ได้ กรุณาเพิ่มเพื่อนหรือปลดบล็อก RBC EV Station แล้วลองอีกครั้ง'}</p><a className="button-primary mt-6 inline-flex" href="https://line.me/R/ti/p/@424cdtee">กลับไปแชต RBC EV Station</a></section></main>
  if (!idToken) return <main className="mx-auto max-w-xl px-5 py-12"><section className="rounded-xl border border-line bg-white p-7 text-center shadow-panel"><h1 className="page-title">ส่งข้อมูลพื้นที่</h1><p className={error ? 'mt-3 text-red-600' : 'mt-3 text-muted'}>{error || 'กำลังเชื่อมต่อบัญชี LINE…'}</p>{error && <button className="button-primary mt-6" onClick={() => location.reload()}>ลองใหม่</button>}</section></main>
  return <main className="px-4 py-6 sm:px-6"><NewSitePage role="customer" liff={{ idToken, onSubmitted: setCompleted }}/></main>
}
