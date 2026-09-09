import { useEffect, useState } from 'react'
import { ClipboardList, MapPin, Plus } from 'lucide-react'
import type { Site } from '../types/domain'
import { api } from '../services/api'

type CustomerSite = Site & { customerStatus: 'pending_review' | 'analysis_completed' }

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

export function LiffMySitesPage() {
  const [sites, setSites] = useState<CustomerSite[]>()
  const [error, setError] = useState<string>()

  useEffect(() => {
    let active = true
    const initialise = async () => {
      try {
        const [sdk, configResponse] = await Promise.all([
          loadLiffSDK(),
          fetch('/api/v1/liff/config').then(async response => {
            if (!response.ok) throw new Error('ไม่สามารถเตรียมข้อมูลได้')
            return response.json() as Promise<{ liffId?: string }>
          }),
        ])
        if (!configResponse.liffId) throw new Error('ระบบยังไม่พร้อม กรุณาติดต่อทีมงานในแชต')
        await sdk.init({ liffId: configResponse.liffId })
        if (!sdk.isLoggedIn()) {
          sdk.login({ redirectUri: location.href })
          return
        }
        const token = sdk.getIDToken()
        if (!token) throw new Error('กรุณาเปิดเมนูใหม่จาก LINE เพื่อยืนยันตัวตน')
        const result = await api.listLiffSites(token)
        if (active) setSites(result)
      } catch (nextError) {
        if (active) setError(nextError instanceof Error ? nextError.message : 'ไม่สามารถโหลดข้อมูลได้')
      }
    }
    void initialise()
    return () => { active = false }
  }, [])

  if (!sites && !error) return <main className="mx-auto max-w-xl px-5 py-12"><section className="rounded-2xl border border-slate-200 bg-white p-7 text-center shadow-panel"><ClipboardList className="mx-auto text-emerald-600" size={32}/><h1 className="mt-4 text-2xl font-extrabold text-[#08244d]">พื้นที่ของฉัน</h1><p className="mt-3 text-slate-500">กำลังโหลดข้อมูลพื้นที่จากบัญชี LINE ของคุณ…</p></section></main>
  if (error) return <main className="mx-auto max-w-xl px-5 py-12"><section className="rounded-2xl border border-rose-100 bg-rose-50 p-7 text-center"><h1 className="text-2xl font-extrabold text-rose-800">เปิดข้อมูลไม่ได้</h1><p className="mt-3 text-rose-700">{error}</p><button className="mt-6 rounded-lg bg-[#008e50] px-5 py-3 font-bold text-white" onClick={() => location.reload()}>ลองใหม่</button></section></main>
  return <main className="mx-auto max-w-xl px-5 py-8"><header className="flex items-start justify-between gap-4"><div><p className="text-xs font-extrabold tracking-[0.14em] text-emerald-700">RBC EV STATION</p><h1 className="mt-1 text-3xl font-extrabold text-[#08244d]">พื้นที่ของฉัน</h1><p className="mt-2 text-sm text-slate-500">ดูข้อมูลที่คุณส่งให้ทีม RBC ตรวจสอบ</p></div><a href="https://liff.line.me/2011499417-DaYmvXaf" className="grid h-11 w-11 place-items-center rounded-xl bg-emerald-600 text-white"><Plus size={22}/></a></header><section className="mt-6 space-y-3">{sites?.length ? sites.map(site => <article key={site.id} className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm"><div className="flex gap-3"><span className="grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-emerald-50 text-emerald-700"><MapPin size={20}/></span><div><h2 className="font-extrabold text-[#08244d]">{site.name}</h2>{site.referenceCode ? <p className="mt-1 text-xs font-bold tracking-wide text-emerald-700">เลขที่รายการ: {site.referenceCode}</p> : null}<p className="mt-1 text-sm text-slate-500">{site.address || (site.latitude !== undefined && site.longitude !== undefined ? `${site.latitude.toFixed(5)}, ${site.longitude.toFixed(5)}` : 'รอตรวจสอบตำแหน่ง')}</p><p className={`mt-3 inline-flex rounded-full px-2.5 py-1 text-xs font-bold ${site.customerStatus === 'analysis_completed' ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600'}`}>{site.customerStatus === 'analysis_completed' ? 'วิเคราะห์เบื้องต้นเสร็จแล้ว' : 'ทีม RBC กำลังตรวจสอบ'}</p></div></div></article>) : <section className="rounded-2xl border border-dashed border-slate-300 bg-slate-50 p-8 text-center"><MapPin className="mx-auto text-slate-300" size={32}/><h2 className="mt-3 font-extrabold text-[#08244d]">ยังไม่มีพื้นที่ที่ส่งไว้</h2><p className="mt-2 text-sm text-slate-500">กดปุ่ม + เพื่อส่งข้อมูลพื้นที่แรก</p></section>}</section></main>
}
