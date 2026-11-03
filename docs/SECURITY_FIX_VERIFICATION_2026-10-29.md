# Security remediation และผลตรวจซ้ำ — 29 ตุลาคม 2569

ตรวจและแก้จาก working tree ที่เริ่มต้นสะอาดบน commit `61e0aa5` ของ EV-Station monorepo โดยรักษารายงานเดิม `SECURITY_REVIEW_2026-10-29.md` เป็นหลักฐานก่อนแก้ งานรอบนี้ไม่ deploy ไม่ใช้ฐานข้อมูลหรือบัญชี production และไม่ทำธุรกรรมจริง ผลด้าน dependency เป็นภาพ ณ เวลาตรวจ ไม่ใช่การรับรองความปลอดภัยตลอดไป

## ระบบและขอบเขตที่ตรวจ

- Frontend: React 19, TypeScript, Vite, React Router, TanStack Query, Zod; API: Go/Gin, pgx, PostgreSQL/PostGIS และ Redis; เสิร์ฟผ่าน nginx/Cloudflare Tunnel
- บทบาท: super_admin, admin, sales, customer; LIFF ตรวจ LINE identity แยกจากเว็บ
- API ที่ตรวจ: auth/register/OTP/login, users, sites/images/analyses, documents/templates/versions/downloads, LINE webhook/notifications, LIFF และ Maps resolver
- ข้อมูลอ่อนไหว: ข้อมูลติดต่อ พิกัด รูปพื้นที่ เอกสารสัญญา และผลประเมิน ไม่มี checkout/การจอง/การรับชำระเงินใน repository นี้; financial เป็นการคำนวณ ROI ตามสมมติฐาน
- ตรวจสิทธิ์ server, SQL แบบ bound parameters, HTML/command sinks, URL/redirect checks, MIME/body limits, file ownership และ download headers ร่วมกับชุดทดสอบเดิม ไม่พบ bypass ที่ยืนยันเพิ่มเติมในเส้นทางเหล่านี้ การไม่พบไม่เท่ากับพิสูจน์ว่าไม่มีช่องโหว่

## ปัญหาที่มีหลักฐานและการแก้ไข

### 1. High — LINE webhook เปลี่ยนผู้รับข้อมูลลูกค้าได้

**ความมั่นใจสูง; ยืนยันก่อนและหลังด้วย signed webhook และ HTTP server จำลอง**

ก่อนแก้ group event ที่มีลายเซ็น LINE ถูกต้องเขียน groupId ลง recipient store และการแจ้งเตือนครั้งถัดไปเลือกค่าดังกล่าวแทน config ผู้โจมตีต้องทำให้ OA รับ event จากกลุ่มที่ตนควบคุมได้ ลายเซ็นรับรองต้นทาง LINE แต่ไม่ได้อนุมัติกลุ่ม ผลกระทบคือข้อมูลติดต่อและพิกัดลูกค้ารั่วไปผู้รับที่ไม่ได้รับอนุญาต ไม่ได้ทดลองเชิญ OA ของ production

แก้ `apps/api/internal/line/notifier.go:56` และ `:101`: ใช้เฉพาะ recipient ที่ผู้ดูแลกำหนดผ่าน config; webhook ตรวจ signature/JSON แต่ไม่แก้ผู้รับ; ไม่ใช้ recipient เดิมที่ค้นพบอัตโนมัติ หากไม่ได้ตั้งค่าผู้รับจะไม่ส่งข้อมูลออก

Regression ใน `internal/line/security_regression_test.go` ยืนยัน signed event ของกลุ่มอื่นเขียนทับไม่ได้, legacy row เปลี่ยนปลายทางไม่ได้, ผู้รับที่อนุมัติยังได้รับข้อความ และไม่มี config แล้วไม่ส่ง ปรับ expectation ของเทส webhook เดิมให้ตรงนโยบายอนุมัติผู้รับ ไม่ได้ข้ามการตรวจ signature

### 2. High — Go toolchain/dependencies มีประกาศช่องโหว่ใน call graph

**ความมั่นใจสูงต่อเวอร์ชันและ scanner; ไม่ได้ยืนยัน exploit production**

ก่อนแก้ Docker build ใช้ Go 1.23.12; source scan ก่อนหน้าพบ 46 advisory IDs ที่มี symbol trace รวม parser ที่รับข้อมูลภายนอก เช่น multipart memory-limit bypass [GO-2026-6608](https://pkg.go.dev/vuln/GO-2026-6608) การมี call path ไม่ได้แปลว่า exploit ได้ทุกประกาศหรือเป็นช่องโหว่ยืนยัน 46 จุด

แก้ `apps/api/Dockerfile:1`, `apps/api/go.mod:3` และ go.sum: Go 1.26.10, pgx 5.9.2, x/net 0.60.0 และ transitive modules ตามข้อกำหนด dependency ใช้ patch เพิ่มถึง 1.26.10 เพราะรุ่น 1.26.9 ยังถูกระบุใน [GO-2026-6809](https://pkg.go.dev/vuln/GO-2026-6809) ไม่อัปเกรดไลบรารีทุกตัวแบบเหมารวม

Frontend lockfile เดิมมี 19 audit findings (Critical 2 / High 7 / Moderate 9 / Low 1; หลายรายการเป็น dev tooling และยังไม่ได้ยืนยันเส้นทาง exploit production) แพตช์ Zod 4.6.6, Vite 7.3.7, Vitest 4.1.12 และ override เฉพาะเวอร์ชัน transitive ที่ได้รับผลกระทบใน root/web workspace ทั้งสองชุด

หลังแก้ pnpm audit ทั้ง root และ web ไม่พบ advisory; govulncheck source scan ไม่พบ symbol-reachable finding ไม่ได้สแกน OS packages ทุกตัวหรือ binary ของ production

### 3. Medium — เปลี่ยนรหัสผ่านแล้ว token เก่ายังใช้ได้

**ความมั่นใจสูง; regression ล้มเหลวก่อนแก้และผ่านหลังแก้**

ผู้โจมตีต้องมี token มาก่อน การเปลี่ยน password hash เดิมไม่ยุติ token ซึ่งอาจอยู่ได้นานตาม JWT TTL การปิดบัญชี/เปลี่ยน role มีการตรวจอยู่แล้ว แต่ไม่ปิดกรณีรหัสถูกเปลี่ยน

แก้ `apps/api/internal/auth/service.go:59`, `:64`, `:118`: ใส่ credential tag ที่สร้างด้วย HMAC จาก hash ปัจจุบันใน JWT แล้วเทียบกับ credential ปัจจุบันจาก repository เมื่อยืนยันผู้ใช้ ไม่ส่ง password hash ไป client และไม่ต้อง migration ตาราง

`TestPasswordResetRevokesOldToken` ยืนยัน token ก่อน reset ถูกปฏิเสธ, รหัสเดิมใช้ไม่ได้ และ login/token ใหม่ใช้งานได้ ชุดเทสเดิมตรวจ forged/expired token, inactive user และ current role ยังผ่าน **เมื่อ deploy ภายหลัง token เก่าที่ไม่มี tag จะต้องเข้าสู่ระบบใหม่**

### 4. Medium — ผู้ไม่ล็อกอินทำให้บัญชีอื่นถูกจำกัดทั้งระบบ

**ความมั่นใจสูง; regression ล้มเหลวก่อนแก้และผ่านหลังแก้**

ก่อนแก้ 120 requests/minute ใช้ bucket รวม ผู้โจมตีใช้อีเมลสุ่มจนผู้ใช้คนละแหล่งได้ 429 การนับ login สำเร็จสะสมยังทำให้ใช้งานปกติหลายครั้งแล้วล็อกชั่วคราว

แก้ `apps/api/internal/httpapi/auth_limit.go:69`: จำกัดตาม socket peer ที่เชื่อถือได้ และตาม source+route+email; ไม่ใช้ global bucket; successful login ล้าง account bucket แต่เก็บ source budget; จำกัดจำนวน bucket และไม่เปลี่ยนการเต็มของ state เป็นการปิดผู้ใช้ใหม่ทั่วระบบ

`router.go`, `apps/web/nginx.conf:3`, `docker-compose.yml` เพิ่ม trusted-proxy chain สำหรับ deployment ที่ public traffic ผ่าน local Cloudflare tunnel และ nginx ส่ง header ทับให้ API คำขอจาก peer ที่ไม่อยู่ใน CIDR เชื่อถือไม่ได้ปลอม forwarded IP เพื่อเลี่ยง limit

Regression ตรวจคนละแหล่งไม่ถูกขวาง, header ปลอม, invalid forwarded IP, login สำเร็จ 25 ครั้ง และเทสเดิมตรวจจำนวนครั้ง/ขนาด body การรัน E2E พบการล้าง bucket ใช้ path ไม่รวม `/api/v1`; แก้และยกระดับเทสให้ใช้ path จริงก่อนรันซ้ำ ไม่ลดจำนวนหรือข้ามเทส

ข้อจำกัด: คนใช้ NAT เดียวกันยังแชร์ source budget, counters ยังเป็น process-local และไม่ใช่การป้องกัน distributed brute force ทั้งหมด ต้องใช้ Redis/WAF เมื่อเพิ่ม instance หรือพบการโจมตีหลายต้นทาง

### 5. Medium — SMTP ค้างและ OTP state/การลองรหัสไม่มีขอบเขตเพียงพอ

**ความมั่นใจสูงต่อพฤติกรรมหลังแก้และต้นเหตุจากโค้ด; ไม่ได้ทำ load attack ก่อนแก้**

เดิม smtp.SendMail ไม่มี application deadline และไม่ผูก request cancellation; code map/expired entries และการลอง OTP ไม่มีเพดานตามที่ควรเป็น การเรียก endpoint สาธารณะกับ SMTP ที่ช้าสามารถค้าง goroutine และสะสม state ได้

แก้ `apps/api/internal/auth/otp.go:18`, `:38`, `:74`, `:127` และ handler: bound การส่งครบวงจร 10 วินาที, ปิด socket เมื่อ request ถูกยกเลิก, ใช้ STARTTLS เมื่อมีและบังคับ TLS ก่อน auth, ล้าง expired entries/จำกัด 10,000 entries, ลบรหัสที่ส่งไม่สำเร็จ, ลองผิดได้ 5 ครั้งและรหัสใช้ครั้งเดียว ใช้ crypto/rand.Int สร้างรหัสหกหลัก

`otp_security_test.go` ใช้ TCP server จำลองที่ไม่ตอบ SMTP greeting ยืนยัน cancellation กลับภายใน 2 วินาทีและไม่เหลือรหัสที่ไม่ได้ส่ง ตรวจผิดครบเพดาน, หมดอายุ, case normalization และใช้ครั้งเดียว E2E ใช้ Mailhog ยืนยันเส้นทางส่งและสมัครปกติ ไม่มีการส่งอีเมลจริง

### 6. Medium — ผู้สมัครคนแรกได้ owner โดยอัตโนมัติบนฐานข้อมูลว่าง

**ความมั่นใจสูงจากเงื่อนไขโค้ดและ unit test; เป็นความเสี่ยงช่วงติดตั้ง ไม่ใช่หลักฐานยึดบัญชีปัจจุบัน**

เดิมผู้ที่ผ่าน OTP คนแรกเมื่อระบบยังไม่มี owner ได้ super_admin จึงเกิดการแย่งสิทธิ์ตอนเปิด instance ใหม่

แก้ `apps/api/internal/httpapi/handlers.go:107`, `:118`: public registration เป็น customer เป็นค่าเริ่มต้น การ bootstrap ต้องไม่มี owner และอีเมลตรง `BOOTSTRAP_OWNER_EMAIL` ที่ผู้ติดตั้งตั้งใจระบุ โดยยังต้องผ่าน OTP และ unique email เดิม ค่าเริ่มต้น config ว่างจึงไม่ให้สิทธิ์ owner อัตโนมัติ

`TestOwnerBootstrapIsExplicitAndEmailBound` ตรวจค่าไม่ตั้ง, อีเมลผิด, อีเมลที่อนุมัติ และกรณีมี owner แล้ว บัญชี owner เดิมไม่เปลี่ยนแปลง ไม่ได้เปิด production ด้วยฐานข้อมูลว่างเพื่อทดลอง

## คำสั่งและผลทดสอบจริง

Go รันใน disposable Docker container เพื่อไม่พึ่ง toolchain เครื่อง; mount เฉพาะ source, cache ทดสอบ `rbc-security-go-cache` และ `.runtime/security-fix` ไม่มี production volume

| คำสั่ง/การตรวจ | ก่อนแก้ | หลังแก้ |
|---|---|---|
| `go test ./...` และ `go vet ./...` | ผ่านบน Go 1.23.12 | ผ่านบน Go 1.26.10 |
| `pnpm --dir apps/web test` | 55 tests / 12 files ผ่าน | 55 tests / 12 files ผ่านบน Vitest ใหม่ |
| `pnpm --dir apps/web lint` | ผ่าน | ผ่าน |
| `pnpm --dir apps/web build` (`tsc -b && vite build`) | ไม่ได้เก็บ baseline build รอบนี้ | typecheck/build ผ่าน; ยังมี bundle-size warning เดิม |
| regression token/LINE/source isolation | ล้มเหลวทั้งสามตามช่องโหว่ | ผ่าน |
| regression successful login บน path จริง | พบ 429 หลัง login สำเร็จสะสม | ผ่าน 25 ครั้ง |
| `CGO_ENABLED=1 go test -race ./internal/auth ./internal/line ./internal/httpapi` | ไม่ได้รัน baseline race | ผ่านหลังแก้; รอบแรกขาด CGO แล้วติดตั้ง gcc/musl ใน container ทดสอบและรันใหม่ |
| `go build -trimpath -o /logs/api-test-build ./cmd/api` | ไม่ได้เก็บ baseline | ผ่าน; binary อยู่ใน ignored runtime |
| `pnpm --dir apps/web audit --json` และ `pnpm audit --json` | 19 findings ตามรายงานก่อนแก้ | 0 findings ทั้งสอง lockfiles |
| `govulncheck -json ./...` v1.1.4 | 46 symbol-reachable advisory IDs จาก audit เดิม | 0 symbol-reachable IDs บน Go 1.26.10 |
| `docker build -t rbc-security-web-test apps/web` | ไม่ได้เก็บ baseline | ผ่านโดยไม่ส่ง real API keys |
| `nginx -t` ใน container ทดสอบ | ไม่ได้เก็บ baseline | ผ่าน |
| `powershell -NoProfile -File scripts/test-e2e.ps1` | ไม่ได้รันก่อนเริ่มแก้ | 16/16 ผ่าน ทั้ง desktop/mobile (2.4 นาที) |
| `git diff --check` | working tree เริ่มต้นสะอาด | ผ่าน |

E2E ใช้ compose project `rbc-ev-station-e2e`, PostgreSQL แยก, Mailhog และข้อมูลจำลอง API port 18080 / web 15173 / Mailhog 18025; script ลบเฉพาะ volumes ของ project ทดสอบเมื่อจบ ครอบคลุม upload/retry/draft/session expiry, sales/customer denial, owner access, private/shared contract versions และ audit access ทั้ง desktop/mobile

รอบแรกและรอบที่สองของ E2E ผ่าน 5/16 ก่อนพบ 429 จาก limiter บน path จริง รอบสุดท้ายหลังแก้ผ่าน 16/16 ไม่มีเทสถูกลบหรือ skip เทสหลักและ race รอบสุดท้ายผ่าน ไม่มี test failure ที่ยังค้างอยู่ในชุดที่รัน

หลักฐานดิบอยู่ใน ignored `.runtime/security-fix`: baseline-go.log, baseline-web.log, regressions-before.log, after-web.log, go-final-retest.log, race-retest.log, govuln-final.json, audit-after.json, root-audit-after.json, docker-web.log และ e2e-retest.log ไฟล์ชั่วคราวและ binary ไม่รวมใน Git

## ข้อสงสัย ข้อจำกัด และงานต่อ

- ยังไม่ได้ fuzz malicious PDF/image หรือทุก redirect/DNS edge case, ทำ stress test ขนาดใหญ่, ตรวจ Cloudflare account/WAF, SMTP provider จริง, OS package CVEs ทุก image หรือ Git history ทุก blob
- ค้น secret signatures ใน tracked source และไฟล์ใหม่ 246 text files ไม่พบรูปแบบ private key/Google/GitHub/AWS ตาม scanner ที่ใช้ ไม่ใช่ entropy/history audit เต็มรูปแบบ ไม่อ่านหรือแสดงค่าจริงจาก production env/credential files
- Docker build เตือนชื่อ `VITE_GOOGLE_MAPS_BROWSER_API_KEY` ใน ARG/ENV ซึ่งเป็น browser key ที่แอปต้องส่งให้ client อยู่แล้ว ไม่ใช้ real key ใน build ทดสอบนี้ การจำกัด HTTP referrer, API และ quota ใน Google Cloud ยังไม่ได้ตรวจ จึงไม่อ้างว่าตั้งค่าแล้ว
- JWT ยังเก็บใน localStorage และ TTL deployment ยังตั้งค่าได้นาน การเปลี่ยนรหัสยุติ token ได้แล้ว แต่หากเกิด XSS ภายหลังยังเสี่ยง token theft; แนะนำ access token สั้น + revocable refresh session เป็นงานออกแบบแยก
- ข้อสมมติ proxy สำคัญ: frontend port ต้องยัง bind loopback และรับสาธารณะผ่าน Cloudflare tunnel เท่านั้น กำหนด `AUTH_TRUSTED_PROXY_CIDRS` ให้แคบตรง subnet nginx จริงเมื่อส่งมอบ ห้ามเปิด listener นี้สู่ internet โดยรับ CF-Connecting-IP จากใครก็ได้
- ก่อน deploy ในอนาคต ต้องกำหนด LINE recipient ที่อนุมัติและตรวจกลุ่มรับข้อมูล, แจ้งผู้ใช้ให้ login ใหม่ และทดสอบ SMTP/Cloudflare config ใน staging ข้อมูลจำลองอีกครั้ง
- ยังไม่ deploy และการแก้ในเครื่องยังไม่ปิดความเสี่ยงของ production ที่ใช้เวอร์ชันเก่า การปล่อยจริงและการเปลี่ยน secrets ต้องทำเป็นขั้นตอนส่งมอบที่ตรวจสอบได้ต่างหาก

ผลผ่านเป็นหลักฐานของกรณีที่ทดสอบ ไม่ใช่ข้อสรุปว่าระบบปลอดภัยทั้งหมด
