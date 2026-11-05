# EV Station — หลักฐานขึ้นระบบและทดสอบซ้ำ 5 พฤศจิกายน 2569

## เวอร์ชันที่ขึ้นระบบจริง

- Repository: EV-Station monorepo, branch `Dev`.
- Source: `d825d377cb56bd5bfb987bcbad857d4e93b6ffba` (push แล้ว).
- รวม security fixes `acd8938`, SEO `60a10a7` และแพตช์ minimatch ใน `d825d37`.
- API เริ่ม 5 พ.ย. 2569 09:49:31 น.; web 09:49:32 น. (Asia/Bangkok).
- API image: `sha256:7a545f559a313e16245c6ba19f926b698904e8d5d034d4b8c9f9a6556ca39cae`.
- Web image: `sha256:d0412f217cc4d52b75471a20362279fb85e37c6260d473df899d5c05b0be072b`.
- API binary SHA256: `a498c67aac692b725b5b96fc37d3d5348c309ad24d1ec45e2e90e0a56c9f4a16` ตรงกันระหว่าง build stage ที่ทดสอบกับ container production.
- โดเมน: https://www.rbcevstation.com ตรวจ HTTP จบ 09:50:55 น.; ตรวจ rendered browser ต่อผ่านทั้ง desktop/mobile.

## ผลทดสอบที่รันจริง

| คำสั่ง / ขอบเขต | ผล |
| --- | --- |
| `go test -json ./...` ใน disposable Go build container, APP_ENV=test, network none | 301 tests / 14 packages ผ่าน, 0 fail |
| `go vet ./...` ใน build container | ผ่าน exit 0 |
| `pnpm --dir apps/web test` | 67/67 ผ่าน |
| `pnpm --dir apps/web lint` | ผ่าน |
| `pnpm --dir apps/web build` | TypeScript / Vite / prerender ผ่าน; ยังมี bundle-size warning |
| `pnpm --dir apps/web test:seo` | 10 language pages, 160+ assertions ผ่าน |
| `powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-e2e.ps1` | 16/16 ผ่าน; PostgreSQL, Redis, Mailhog และข้อมูลจำลองแยก project; ล้างเฉพาะ test volumes หลังจบ |
| `pnpm --dir apps/web exec playwright test --config playwright.seo.config.ts` | 4/4 desktop/mobile ผ่าน รวม no-JavaScript HTML และ language navigation |
| `pnpm audit --json` และ `pnpm --dir apps/web audit --json` หลังแพตช์ | ทั้งสอง lockfiles: critical/high/moderate/low = 0 ณ เวลาตรวจ |
| `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...` และ `-show verbose` | 0 reachable symbol vulnerabilities; มี advisory ที่ไม่พบ call path ดูข้อจำกัด |
| `docker compose build api web` / `docker run --rm --entrypoint nginx ev-station-web -t` | build และ nginx config ผ่าน |
| Production HTTP smoke check | 25/25 รายการผ่าน |
| Production headless Chromium | `/investment` TH/EN ที่ 1440 และ 390 px ผ่าน; H1 มองเห็น, ไม่ล้นแนวนอน, ไม่มี pageerror |
| `GET http://127.0.0.1:8080/health` | 200, status ok |

Security regression ครอบคลุม signed LINE webhook เปลี่ยนผู้รับไม่ได้, recipient ที่อนุมัติยังใช้ได้, legacy recipient ไม่มีผล, reset password เพิกถอน token เก่า, OTP expiry/attempt limit/one-time-use, SMTP cancellation, rate limits แยก source, proxy header และ owner bootstrap ที่ต้องระบุอีเมลชัดเจน.

E2E ของ EV ยืนยัน upload/retry/reload, แก้ไขพื้นที่, วิเคราะห์และเปิดรายงานที่บันทึก, offline/expired session, server permissions ของ sales/customer และเอกสารสัญญา private versions/release/audit access. การวิเคราะห์ใน E2E ใช้ provider ทดสอบ จึงไม่รับรองข้อมูลภายนอกสดครบทุกจังหวัด.

## ตรวจโดเมนจริง

- `/`, `/investment`, `/branches`, `/contact`, `/guide/customer` ทั้ง TH และ `?lang=en`: 200, HTML ภาษาถูกต้อง, H1 หนึ่งรายการ, canonical/OG URL ตรง, hreflang 3 รายการ, JSON-LD อ่านได้ และ share image ตอบ 200 แบบ image.
- `/login`, `/dashboard`, `/analysis/release-test`, `/sites/release-test/documents`, `/guide/admin`, `/liff/site-submission`: private SPA มี noindex ทั้ง HTTP header และ HTML.
- Unknown page, unknown asset และ direct internal prerender URL: HTTP 404 จริง.
- `/api/v1/sites`, `/api/v1/users`, `/api/v1/sites/release-test/documents` เมื่อไม่มี token: 401. ใช้ GET เท่านั้น ไม่อ่านข้อมูลลูกค้าจริง.
- Sitemap มีเฉพาะ public 10 language URLs, ไม่มีรายงาน/เอกสารลูกค้า; robots ตอบ 200 และอ้าง sitemap.
- Trailing slash `/investment/?lang=en` เปลี่ยนเส้นทาง 301 โดยรักษาภาษา.
- **noindex ไม่ใช่การควบคุมสิทธิ์**: ข้อมูลลูกค้าต้องอาศัย auth/authorization ของ API ด้วย. ยังไม่ได้ส่ง Search Console หรือยืนยันการนำ URL เก่าออกจากดัชนี Google.

## การเปลี่ยนแปลงและขั้นตอน deploy

พบ advisory ใหม่ GHSA-g3hv-75wg-m249 ใน minimatch 10.2.6 ของเครื่องมือ lint. Override เฉพาะ `@typescript-eslint/typescript-estree>minimatch` เป็น 11.0.1 ในสอง workspace/lockfiles; ไม่ได้ยืนยัน exploit ใน production. แยก SEO browser suite ออกจาก core suite เพราะต้องใช้ production preview คนละ port; ทั้งสอง suite รันผ่านจริง ไม่ได้ข้ามทดสอบ.

จำกัด `AUTH_TRUSTED_PROXY_CIDRS` ใน production `.env` เป็น subnet ของ `ev-station_default` ที่ตรวจจริง `172.24.0.0/16` แทน default Docker private range กว้าง. ตรวจ container หลัง deploy: APP_ENV=production, AUTH_REQUIRED=true และ subnet ตรง. ต้องทบทวน config หากสร้าง Docker network ใหม่.

Deploy ด้วย `docker compose up -d --no-deps --no-build api web` หลัง build ผ่าน ไม่ migrate/reset DB ไม่ลบ production volumes. เก็บ rollback tags `ev-station-api:rollback-before-security-20261105` และ `ev-station-web:rollback-before-seo-60a10a7`.

เช้าวันนี้ Docker Desktop ล้มจาก stale inference socket ทำให้เว็บ 502. เก็บโฟลเดอร์เดิมเป็น `C:\Users\friend\AppData\Local\Docker\run.stale-20261105-0935`, restart เฉพาะ Docker Desktop/WSL distro docker-desktop แล้วเปิด EV dependencies กลับ. ไม่ factory reset และไม่ลบข้อมูล. การกู้คืนครั้งนี้ไม่ใช่หลักฐานว่าผ่านการ reboot เครื่องทั้งระบบหรือมี high availability.

## ข้อจำกัด / งานที่ยังไม่ปิด

1. LINE_NOTIFICATION_RECIPIENT_ID ยังว่าง: ระบบไม่ส่งข้อมูลลูกค้าไปกลุ่มที่ค้นพบอัตโนมัติ ต้องตั้งผู้รับที่อนุมัติก่อนเปิด notification; ไม่ทดลองส่งข้อความจริง.
2. OTP/SMTP ทดสอบด้วย fake SMTP และ cancellation แล้ว แต่ไม่ได้ยืนยัน delivery/inbox ของอีเมล production. ต้องทดสอบด้วยผู้รับที่ยินยอมก่อนรับรองส่งเมลจริง.
3. govulncheck รายงาน GO-2026-6866 และ GO-2026-6860 ใน x/net 0.60.0 (fixed 0.62.0) ที่ระดับ imported package แต่ไม่พบเส้นทางเรียก vulnerable symbols; GO-2026-5932 ใน x/crypto ที่ระดับ module (openpgp ไม่ได้ใช้). ยังไม่แพตช์กลุ่มนี้ใน release นี้ ต้องประเมินและอัปเดตแบบเจาะจงต่อไป. 0 reachable ไม่ได้แปลว่าไม่มีความเสี่ยงทุกกรณี.
4. การตรวจนี้ไม่ใช่ production penetration test: ไม่ brute-force, ไม่ยิง malicious upload/webhook จริง, ไม่เปลี่ยนข้อมูลลูกค้า และไม่ทดสอบ token reset ด้วยบัญชี production. บัญชีเดิมอาจต้อง login ใหม่จาก credential-bound token.
5. Offsite backup/restore, ช่องทางเตือนเว็บล่ม และ full-machine reboot verification ยังไม่ได้ปิดในรอบนี้.
6. EV repository ไม่มี booking/payment/ticket/check-in. ไม่ได้ตรวจ River Life และไม่รับรองอีเมล/ตั๋ว/เช็กอินของ River Life จากรายงานนี้.

หลักฐาน log/JSON/screenshots ในเครื่อง: `.runtime/release-20261105/` (ignored, ไม่เก็บข้อมูลลับลง Git). รายงานนี้เป็นหลักฐาน ณ วันตรวจ ไม่ใช่ข้อสรุปว่าระบบปลอดภัยทั้งหมด.
