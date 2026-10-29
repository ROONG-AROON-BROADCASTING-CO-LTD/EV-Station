# Security review — 29 October 2026

ตรวจโค้ดทั้ง repository ที่ commit `f0033c0` ไม่จำกัด diff ล่าสุด ไม่แก้ source code หรือ dependencies และไม่ทดสอบ production ตัวทดสอบใช้สำเนาโค้ดใน disposable container กับข้อมูลจำลอง ผลทดสอบและ scanner ฉบับดิบเก็บใน `.runtime/security-review` ซึ่งไม่รวมใน Git รายงานนี้ไม่บรรจุ secrets หรือข้อมูลลูกค้าจริง

## ระบบและขอบเขต

- React 19 / TypeScript / Vite / React Router / TanStack Query / Zod
- Go / Gin / pgx / PostgreSQL-PostGIS / Redis; Docker Compose, nginx และ Cloudflare Tunnel
- บทบาทเว็บ: super_admin, admin, sales, customer; LIFF ตรวจตัวตน LINE แยกจากบัญชีเว็บ
- ข้อมูลสำคัญ: ข้อมูลติดต่อ พิกัด ภาพหน้างาน เอกสารสัญญา และผลวิเคราะห์
- API สำคัญ: `/auth/*`, `/users`, `/sites/*`, `/analyses/*`, `/documents`, `/document-templates`, `/liff/*`, `/line/webhook`
- ไม่พบระบบจองหรือรับชำระเงินใน repository นี้ เครื่องมือ financial เป็นการประมาณ ROI ตามสมมติฐาน ไม่ใช่ checkout

## ช่องโหว่ที่ยืนยันพฤติกรรมได้

### 1. High: webhook กลุ่ม LINE เปลี่ยนปลายทางรับข้อมูลลูกค้า

ความมั่นใจสูง ยืนยันด้วย webhook และ HTTP server จำลอง ไม่ได้ยืนยันการตั้งค่าอนุญาตเชิญ OA เข้ากลุ่มบน production

หลักฐาน: `apps/api/internal/line/notifier.go:111-138` ตรวจ HMAC แล้วบันทึก groupId ของทุก group event เป็นผู้รับแจ้งเตือน; บรรทัด 62-81 ให้ค่าที่บันทึกไว้แทน recipient เดิม; บรรทัด 156-161 ใส่โทรศัพท์และ Maps URL ในข้อความ

เงื่อนไข: หากบุคคลภายนอกนำ OA เข้ากลุ่มที่ควบคุมได้ แล้วสร้าง event จากกลุ่มนั้น LINE จะสร้างลายเซ็นที่ถูกต้อง แต่ไม่มีการอนุมัติกลุ่มในแอป ผลทดสอบ `TestReviewUnapprovedGroupRedirectsCustomerNotification` ยืนยันว่ากลุ่มจำลองที่ไม่ได้อนุมัติเปลี่ยน recipient และรับ notification จำลองถัดไปได้

ผลกระทบ: ข้อมูลติดต่อและทำเลลูกค้ารั่วไปกลุ่มอื่น HMAC รับรองต้นทาง LINE ไม่ได้รับรองสิทธิ์ของกลุ่ม

แก้ไข: ให้ผู้ดูแลอนุมัติหรือกำหนด recipient โดยตรง ห้าม generic webhook เขียนทับ ทดสอบ signed event จากกลุ่มอื่นและ replay event เก่าว่าเปลี่ยน recipient ไม่ได้ พร้อมตรวจกลุ่มที่อนุมัติยังรับข้อความได้

### 2. Medium: token เดิมยังใช้ได้หลังเปลี่ยนรหัสผ่าน

ความมั่นใจสูง ยืนยันด้วยบัญชี admin จำลอง

หลักฐาน: `apps/api/internal/auth/service.go:61-73` เปลี่ยน password hash แต่ไม่เพิกถอน token; บรรทัด 109 เป็นต้นไปตรวจสถานะและบทบาทบัญชีโดยไม่มี password/session version; `apps/api/internal/config/config.go:118` มี TTL เริ่มต้น 2160 ชั่วโมง; `apps/web/src/services/session.ts` เก็บ token ใน localStorage

เงื่อนไข: ผู้โจมตีต้องได้ token มาก่อน การเปลี่ยนรหัสผ่านไม่ยุติ token นั้นจนหมดอายุ ปิดบัญชี หรือเปลี่ยน signing key ผล `TestReviewTokenSurvivesPasswordReset` ยืนยัน login → เปลี่ยนรหัส → VerifyUser(token เดิม) ยังสำเร็จ

ผลกระทบ: ผู้ถือ token ที่ขโมยมายังเข้าถึงตามสิทธิ์เดิมได้ แม้เจ้าของเปลี่ยนรหัสแล้ว การเก็บ localStorage เพิ่มผลกระทบหากมี XSS แต่รอบนี้ไม่พบ XSS ที่ยืนยันได้

แก้ไข: เพิ่ม session_version/password_changed_at หรือ session store ที่เพิกถอนได้ ลดอายุ access token ทดสอบ token ก่อน reset ได้ 401 และ login ใหม่สำเร็จ รวมถึงหลายเครื่อง

### 3. Medium: anonymous requests ใช้เพดานล็อกอินรวมขวางผู้ใช้อื่น

ความมั่นใจสูง ยืนยันใน HTTP handler จำลอง

หลักฐาน: `apps/api/internal/httpapi/auth_limit.go:61` ใช้เพดานรวม 120 requests/minute ก่อนตรวจ credentials และบรรทัด 76 จำกัดรายอีเมล 10 requests/15 minutes

เงื่อนไข: ส่ง login 120 ครั้งด้วยอีเมลสุ่มทำให้คำขอแรกของเหยื่อที่ไม่เกี่ยวข้องได้ 429 ผล `TestReviewAnonymousRequestsBlockVictimLogin` ยืนยันแล้ว ผู้โจมตีไม่ต้องมีบัญชี และสามารถทำซ้ำได้ เพดานรายอีเมลเปิดให้ขวางบัญชีเป้าหมายชั่วคราวได้ด้วย

ผลกระทบ: login/register/OTP ไม่พร้อมใช้งานชั่วคราว ไม่ได้เพิกถอน token ของผู้ที่ล็อกอินอยู่แล้ว

แก้ไข: จำกัดแบบหลายชั้นตามแหล่งคำขอที่เชื่อถือได้ บัญชี และจำนวนความล้มเหลว ใช้ progressive delay; กำหนด trusted proxy ก่อนใช้ forwarded IP; ใช้ shared counters เมื่อเพิ่ม instance ทดสอบผู้โจมตีแหล่งหนึ่งไม่ขวางผู้ใช้จากอีกแหล่งและปลอม forwarded headers ไม่ได้

## Dependencies: มีประกาศและเส้นทางที่ต้องแก้ แต่ไม่เท่ากับ exploit ทุกกรณี

### 4. High: Go toolchain เก่าในเส้นทางอ่านข้อมูลภายนอก

ความมั่นใจสูงต่อผล version/call graph ปานกลางต่อ exploitability ของ deployment จริง ยังไม่ทดสอบ payload หรือสแกน binary production

`apps/api/Dockerfile:1` ใช้ golang:1.23-alpine; image ที่ scanner ใช้เป็น Go 1.23.12 `govulncheck` พบ 46 advisory IDs ที่มี symbol call path ต้องประเมินเงื่อนไขทีละรายการ ไม่ใช่ช่องโหว่ยืนยัน 46 จุด

ตัวอย่าง [GO-2026-6608](https://pkg.go.dev/vuln/GO-2026-6608) เกี่ยวกับ memory-limit bypass ใน multipart parser เส้นทาง uploads อยู่ที่ `apps/api/internal/httpapi/handlers.go:590` และ `documents.go:179` Body caps ช่วยจำกัดความเสียหายแต่ไม่แก้ parser ผลกระทบที่ต้องป้องกันคือ resource exhaustion/DoS จากข้อมูลผิดรูปแบบ

แก้ไข: ใช้ Go รุ่นรองรับที่มี patch ตาม advisory แล้ว rebuild และสแกน binary ทดสอบ malformed/concurrent multipart uploads ในระบบแยกพร้อมวัด RAM/CPU

ผล pnpm audit จาก lockfile: 19 รายการ (Critical 2, High 7, Moderate 9, Low 1); production-only High 1 รายการ ตัวเลขรวมมีหลาย advisory ต่อแพ็กเกจและไม่ได้พิสูจน์ production exploit

| เวอร์ชันจริง | ประกาศ | การตีความและแนวแก้ |
|---|---|---|
| pgx v5.7.5 | [GO-2026-5004](https://pkg.go.dev/vuln/GO-2026-5004) | SQL injection ต้องใช้ non-default simple protocol และ dollar-quoted SQL ที่มีเงื่อนไข ยังไม่พบรูปแบบนั้นใน query ที่ตรวจ อัปเดตและทดสอบ queries ก่อน deploy |
| zod 4.5.4 | [GHSA-mrhx-c7xc-m3w5](https://github.com/advisories/GHSA-mrhx-c7xc-m3w5) | High ในฐานข้อมูล แก้ใน 4.6.6; ใช้ตรวจฟอร์มใน browser ยังไม่พบ backend DoS path |
| tinypool 1.1.1 ผ่าน Vitest | [GHSA-85c8-ppgw-ccpr](https://github.com/advisories/GHSA-85c8-ppgw-ccpr) | Critical ในฐานข้อมูล ต้องมี prototype-pollution/control prerequisites ยังไม่พบเส้นทางจากเว็บ production; ตรวจและอัปเดต test tooling |
| vite 7.3.6 | [GHSA-587h-7wmj-h5pm](https://github.com/advisories/GHSA-587h-7wmj-h5pm) | launch-editor บน Windows เป็น dev-server risk; production ใช้ nginx เสิร์ฟ build ไม่มีการยืนยัน exploit บนเครื่องพัฒนา |

Lockfile anchors: `apps/web/pnpm-lock.yaml:1774` tinypool, `:1850` Vite, `:1890` Vitest, `:1979` Zod

## ข้อสงสัยที่ยังต้องตรวจเพิ่ม

- OTP/SMTP: `apps/api/internal/auth/otp.go:28-39` เก็บ OTP ก่อนส่ง, smtp.SendMail ไม่กำหนด deadline ในโค้ดนี้, expired entries ไม่ถูกล้างตามรอบ ควรทดสอบ stalled SMTP และการเติบโตของ state ก่อนสรุป DoS ความเสี่ยงเบื้องต้น Medium
- Owner bootstrap: `handlers.go:89-109` ให้ผู้สมัครคนแรกเป็น super_admin เมื่อไม่มี owner ต้องทดสอบ concurrent registration และการเปิด production บนฐานข้อมูลว่าง ไม่ใช่หลักฐาน takeover ฐานข้อมูลปัจจุบัน ความเสี่ยงช่วงติดตั้ง Medium
- PDF: ยังไม่ทดสอบ malicious documents หรือ viewer ทุก browser; MIME/size checks ไม่ยืนยันความปลอดภัยของเนื้อหา

## ส่วนที่ตรวจแล้ว

- Auth: bcrypt, HMAC, expiry และการตรวจ current role/is_active จาก server; พบ session reset issue ข้างต้น
- Authorization: owner/admin/staff guards, assigned-site access, shared documents และ LIFF ownership; existing cross-user tests ผ่าน ไม่พบ bypass ในเส้นทางที่ตรวจ
- SQL: queries ที่ตรวจใช้ bound parameters ไม่พบ direct user-input concatenation; แยก pgx advisory ตามเงื่อนไข
- XSS: ไม่พบ dangerous HTML/eval sink ใน application paths ที่ค้น ยังไม่ใช่ browser penetration test ครบทุกหน้า
- CSRF: เว็บใช้ Bearer headers ไม่ใช่ automatic auth cookies ไม่พบ confirmed CSRF path
- SSRF: Maps resolver ตรวจ HTTPS/host ทั้ง URL แรกและ redirect; ไม่ทำ DNS/redirect fuzzing
- Command injection: ไม่พบ HTTP input ไป os/exec หรือ shell ใน Go API
- Upload/traversal: ตรวจ body caps, MIME, UUID, permission, DB file storage และ download headers; ไม่พบ filename ไป arbitrary filesystem path
- Secrets: custom signature scan 240 tracked text files ไม่พบ private-key/Google/GitHub/AWS signatures ตามรูปแบบที่ตรวจ ไม่ได้ทำ entropy scan หรือ Git-history ทุก blob ไม่อ่าน production secrets และไม่ใส่ค่า secrets ในรายงาน
- LINE: verify ID token, HMAC, ownership, submission idempotency; recipient authorization มีข้อบกพร่องตามข้อ 1
- Financial: server-side ROI calculation จากสมมติฐาน ไม่มี payment/booking transaction ให้ตรวจ

## คำสั่งและข้อจำกัด

รันจริง: `pnpm --dir apps/web audit --json`, `pnpm --dir apps/web audit --prod --json`; `govulncheck -json ./...` v1.1.4 บน Go 1.23.12 ฐานข้อมูลอัปเดต 2026-10-27; `go test ./internal/auth ./internal/line ./internal/httpapi ./internal/provider -v` ในสำเนาโค้ดพร้อมสอง proof tests; แยก HTTP limiter proof test; custom signature scan, rg source/sink search, git history path check ของ .env และ git status

Proof tests ผ่านทั้งสาม ไม่แก้ working tree ที่ tracked ไม่รัน test กับ production ไม่ตรวจ Cloudflare account settings, production secrets, binary/container OS CVEs ทั้งหมด, browser ทุกหน้า, destructive/load tests หรือระบบผู้ให้บริการภายนอก

ลำดับแก้: recipient authorization → token revocation → limiter isolation → Go/dependency patches → OTP/bootstrap review การไม่พบช่องโหว่ในส่วนที่ตรวจไม่ใช่การรับรองว่าระบบปลอดภัย
