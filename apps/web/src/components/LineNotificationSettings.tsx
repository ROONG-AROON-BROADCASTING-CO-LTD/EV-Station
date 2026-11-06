import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useI18n } from '../i18n/I18nProvider'
import { api } from '../services/api'

export function LineNotificationSettings() {
  const { language } = useI18n()
  const th = language === 'th'
  const text = (thai: string, english: string) => th ? thai : english
  const client = useQueryClient()
  const [now, setNow] = useState(Date.now)
  const [copied, setCopied] = useState(false)
  const [copyFailed, setCopyFailed] = useState(false)
  const status = useQuery({ queryKey: ['line-notification-settings'], queryFn: api.getLineNotificationStatus, refetchInterval: query => query.state.data?.pairingExpiresAt && new Date(query.state.data.pairingExpiresAt).getTime() > Date.now() ? 2000 : false })
  const refresh = () => client.invalidateQueries({ queryKey: ['line-notification-settings'] })
  const pairing = useMutation({ mutationFn: api.startLineNotificationPairing, onSuccess: () => { setCopied(false); setCopyFailed(false); void refresh() } })
  const cancel = useMutation({ mutationFn: api.cancelLineNotificationPairing, onSuccess: () => { pairing.reset(); void refresh() } })
  const test = useMutation({ mutationFn: api.testLineNotification, onSettled: () => { void refresh() } })
  const expiresAt = pairing.data?.expiresAt ?? status.data?.pairingExpiresAt
  useEffect(() => {
    if (!expiresAt) return
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [expiresAt])
  const remaining = expiresAt ? Math.max(0, Math.ceil((new Date(expiresAt).getTime() - now) / 1000)) : 0
  const pending = Boolean(status.data?.pairingExpiresAt && remaining > 0)
  const linked = status.data?.tokenConfigured && status.data.recipientConfigured
  const copy = async () => {
    try { await navigator.clipboard.writeText(pairing.data!.command); setCopied(true); setCopyFailed(false) } catch { setCopyFailed(true) }
  }
  return <section className="mt-6 max-w-3xl rounded-2xl border border-line bg-white p-5 shadow-panel sm:p-6" aria-label={text('แจ้งเตือนเจ้าหน้าที่ผ่าน LINE', 'Staff LINE notifications')}>
    <h2 className="section-title">{text('แจ้งเตือนเจ้าหน้าที่ผ่าน LINE', 'Staff LINE notifications')}</h2>
    <p className="mt-2 text-sm leading-6 text-muted">{text('เชื่อมแชตของเจ้าหน้าที่เพื่อรับแจ้งเมื่อมีลูกค้าส่งพื้นที่ เจ้าของระบบเท่านั้นที่ตั้งค่าปลายทางได้', 'Link a staff chat to receive new customer-site notifications. Only the account owner can configure the recipient.')}</p>
    {status.isPending ? <p className="mt-4 text-sm" role="status">{text('กำลังตรวจสถานะ…', 'Checking status…')}</p> : status.isError ? <div className="mt-4" role="alert"><p className="text-sm text-red-700">{text('อ่านสถานะ LINE ไม่สำเร็จ กรุณาลองใหม่', 'Unable to read LINE settings. Please try again.')}</p><button className="button-secondary mt-3" onClick={() => status.refetch()}>{text('ตรวจสถานะอีกครั้ง', 'Check status again')}</button></div> : <>
      <p className={`mt-4 rounded-xl px-4 py-3 text-sm font-semibold ${linked ? 'bg-emerald-50 text-emerald-800' : 'bg-amber-50 text-amber-900'}`} role="status">{linked ? text('เชื่อมปลายทางแล้ว', 'Recipient linked') : text('ยังไม่พร้อมแจ้งเตือนเจ้าหน้าที่', 'Staff notifications are not ready')}{status.data?.maskedRecipient ? ` · ${status.data.recipientType} ${status.data.maskedRecipient}` : ''}</p>
      {!status.data?.tokenConfigured || !status.data?.webhookConfigured ? <p className="mt-3 text-sm leading-6 text-muted">{text('ผู้ดูแลเซิร์ฟเวอร์ต้องตั้งค่า LINE channel access token และ channel secret ก่อน จากนั้นเปิด Use webhook ใน LINE Developers', 'A server administrator must configure the LINE channel access token and channel secret, then enable Use webhook in LINE Developers.')}</p> : null}
      {status.data?.recipientSource === 'environment' ? <p className="mt-3 text-sm text-muted">{text('ปลายทางนี้ตั้งค่าที่เซิร์ฟเวอร์ หากต้องการเปลี่ยนให้ผู้ดูแลแก้การตั้งค่าเซิร์ฟเวอร์', 'This recipient is managed by server configuration. Ask a server administrator to change it.')}</p> : null}
      {status.data?.canPair ? <>
        <ol className="mt-4 list-decimal space-y-2 pl-5 text-sm leading-6 text-muted"><li>{text('เพิ่ม LINE OA ของ RBC เป็นเพื่อน หรือเชิญเข้ากลุ่มเจ้าหน้าที่ที่ต้องการรับแจ้งเตือน หากเป็นกลุ่มต้องเปิด Allow bot to join group chats ใน LINE Developers', 'Add the RBC LINE OA as a friend, or invite it to the intended staff group. For groups, enable Allow bot to join group chats in LINE Developers.')}</li><li>{text('สร้างคำสั่งเชื่อมต่อ แล้วส่งคำสั่งนั้นในแชตเจ้าหน้าที่ที่ต้องการรับข้อมูลลูกค้าเท่านั้น คำสั่งใช้ครั้งเดียวและหมดอายุใน 10 นาที', 'Generate a pairing command and send it only in the staff chat authorized to receive customer information. It is single-use and expires after 10 minutes.')}</li><li>{text('รอหน้านี้แสดงว่าเชื่อมแล้ว จากนั้นกดส่งข้อความทดสอบและตรวจใน LINE', 'Wait for this page to show the linked status, then send a test notification and check LINE.')}</li></ol>
        <button type="button" className="button-primary mt-4" disabled={pairing.isPending || cancel.isPending} onClick={() => pairing.mutate()}>{pairing.isPending ? text('กำลังสร้าง…', 'Generating…') : text('สร้างคำสั่งเชื่อมต่อ LINE', 'Generate LINE pairing command')}</button>
        {pending ? <div className="mt-4 rounded-xl border border-line bg-slate-50 p-4"><p className="text-sm font-semibold">{text('ส่งคำสั่งนี้ในแชตเจ้าหน้าที่', 'Send this command in the staff chat')}</p>{pairing.data ? <><input aria-label={text('คำสั่งเชื่อมต่อ LINE', 'LINE pairing command')} className="input mt-2 w-full font-mono text-xs" readOnly value={pairing.data.command} onFocus={event => event.target.select()} /><button className="button-secondary mt-2" type="button" onClick={copy}>{copied ? text('คัดลอกแล้ว', 'Copied') : text('คัดลอกคำสั่ง', 'Copy command')}</button>{copyFailed ? <p role="alert" className="mt-2 text-sm text-amber-800">{text('คัดลอกอัตโนมัติไม่ได้ กรุณาเลือกข้อความในช่องแล้วคัดลอก', 'Automatic copy failed. Select the command in the field and copy it.')}</p> : null}</> : <p className="mt-2 text-sm">{text('มีคำสั่งที่ยังไม่หมดอายุ หากไม่มีคำสั่งเดิมให้สร้างใหม่', 'A pairing command is still active. Generate another if you no longer have it.')}</p>}<p className="mt-2 text-sm" role="status">{text('เหลือเวลา', 'Expires in')} {Math.floor(remaining / 60)}:{String(remaining % 60).padStart(2, '0')}</p><button className="mt-3 text-sm font-semibold text-red-700" type="button" disabled={cancel.isPending} onClick={() => cancel.mutate()}>{text('ยกเลิกคำสั่งเชื่อมต่อ', 'Cancel pairing command')}</button></div> : null}
        {!linked && expiresAt && remaining === 0 ? <p className="mt-3 text-sm text-amber-800">{text('คำสั่งหมดอายุแล้ว กรุณาสร้างใหม่หากยังไม่ได้เชื่อมต่อ', 'The pairing command has expired. Generate another if the chat is not linked.')}</p> : null}
      </> : null}
      {pairing.isError || cancel.isError ? <p role="alert" className="mt-3 text-sm text-red-700">{text('ตั้งค่าการเชื่อมต่อไม่สำเร็จ กรุณาตรวจสถานะแล้วลองใหม่', 'Unable to update pairing. Check the status and try again.')}</p> : null}
      {linked ? <div className="mt-4"><button type="button" className="button-secondary" disabled={test.isPending} onClick={() => test.mutate()}>{test.isPending ? text('กำลังส่งข้อความทดสอบ…', 'Sending test notification…') : text('ส่งข้อความทดสอบ LINE', 'Send LINE test notification')}</button>{test.isSuccess ? <p role="status" className="mt-3 text-sm text-emerald-800">{text('LINE รับคำขอส่งข้อความทดสอบแล้ว กรุณาตรวจในแชตเจ้าหน้าที่ ข้อความนี้ไม่มีข้อมูลลูกค้า', 'LINE accepted the test notification. Check the staff chat; the test contains no customer data.')}</p> : null}{test.isError ? <p role="alert" className="mt-3 text-sm text-red-700">{text('ส่งข้อความทดสอบไม่สำเร็จ ตรวจว่าบอตยังอยู่ในแชต รวมถึงสิทธิ์ โทเคน และโควตาข้อความ', 'Test delivery failed. Check bot membership, permissions, token and message quota.')}</p> : null}</div> : null}
      {status.data?.lastDelivery ? <p className="mt-4 text-xs text-muted">{text('คำขอส่งครั้งล่าสุดหลังเริ่มเซิร์ฟเวอร์:', 'Last send request since server start:')} {status.data.lastDelivery.succeeded ? text('LINE รับคำขอแล้ว', 'Accepted by LINE') : text('ไม่สำเร็จ', 'Failed')} · {new Date(status.data.lastDelivery.attemptedAt).toLocaleString(th ? 'th-TH' : 'en-US')}</p> : null}
    </>}
  </section>
}
