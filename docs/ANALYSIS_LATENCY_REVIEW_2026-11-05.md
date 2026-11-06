# ตรวจและแก้ความช้าการวิเคราะห์พื้นที่ — 5 พฤศจิกายน 2569

ยืนยันจากโค้ดและการวัดจริงว่า production ต่อฐานข้อมูลถนน PostGIS อยู่แล้ว ข้อมูลถนนไม่ได้รอ Overpass เมื่อ lookup สำเร็จ เวลาประมาณ 30 วินาทีของการทดสอบครั้งก่อนเป็นการทดสอบ Overpass โดยตรงที่ไม่ได้ต่อ LocalRoads จึงนำมาแทนเวลาของ production ไม่ได้ รอบนี้แก้ชื่อและคำอธิบาย live test ให้ชัดเจนแล้ว

## ข้อผิดพลาดที่พบและแก้

1. **ตัวอัปเดตถนนหยุดโดยระบบตรวจสุขภาพไม่เห็น** — container หยุดตั้งแต่ 21 ต.ค. เวลา 14:44 UTC, exit 137, `OOMKilled=false`; จึงสรุปไม่ได้ว่าเกิดจากหน่วยความจำหมด Runtime restart policy เป็น `no` ต่างจาก compose ที่กำหนด `unless-stopped` ชุดถนนค้างวันที่ 13 ต.ค. จำนวน 2,826,704 เส้น และกำลังเข้าใกล้ข้อจำกัดอายุ 30 วัน ซึ่งจะทำให้กลับไปเรียก Overpass ทุกครั้ง
   - เริ่ม worker จาก compose ปัจจุบันแล้ว ยืนยัน restart policy `unless-stopped` และสถานะ healthy
   - เพิ่มการกู้ worker ที่เคย deploy ไว้ใน `scripts/Invoke-RBC-Operations.ps1` ทั้ง Recover และ Monitor; แยก `degraded-roads` จากเว็บล่ม ไม่ restart API เพียงเพราะข้อมูลถนนเก่า
   - `scripts/roads/update_roads.py` จำกัดเวลาต่อ DB 10 วินาที และ retry เมื่อผิดพลาดหลัง 5/10/20/40/60 นาที แทนรอ 6 ชั่วโมง พร้อม log ขั้นตอนและ error code ที่ไม่เผย secrets
2. **ดึงข้อมูลคู่แข่งเกินรัศมีที่ใช้จริง** — เดิมขอข้อมูล 1/2/3 กม. ตามรัศมีรายงาน แต่ตัดผลคู่แข่งเหลือ 1 กม. เสมอ การเลือกรัศมี 3 กม. จึงค้นพื้นที่เกินจำเป็น 9 เท่า และ cache ซ้ำคนละ key
   - `apps/api/internal/provider/osm.go` ขอคู่แข่ง 1 กม. และแชร์ cache ข้ามรัศมีรายงาน ถนนยังใช้รัศมีที่ลูกค้าเลือกตามเดิม
3. **เวลา local lookup และ remote fallback บวกต่อกัน** — เดิม local lookup สูงสุด 2 วินาที ตามด้วย remote budget ใหม่ 30 วินาที
   - รวมไว้ใน collection deadline เดียว โดยรักษา parent deadline และ provenance ของ snapshot เดิม
4. **ฐานข้อมูลผิดพลาดถูกแปลว่าไม่มี snapshot** — `apps/api/internal/roads/store.go` เคยแปลงทุก error จากการอ่าน dataset เป็น unavailable
   - ตอนนี้แยก `pgx.ErrNoRows` ออกจาก timeout/cancellation/DB error และเพิ่ม `local_road_lookup` telemetry พร้อมระยะเวลาและเหตุผล fallback ก่อนเรียกภายนอก
5. **รอผลภาพหลังงานถูกยกเลิก** — `apps/api/internal/analysis/service.go` เคยรับจาก readiness channel โดยไม่เลือก context cancellation
   - หยุดรอตาม deadline เก็บผลแหล่งข้อมูลอื่นที่เสร็จแล้ว บันทึกส่วนภาพที่ยังประเมินไม่เสร็จตามจริง และรักษาผลภาพที่อยู่ใน buffer แล้ว
6. **log รายงาน success ทั้งที่ข้อมูลหาย** — provider หลายตัวคืน missing observation พร้อม error=nil
   - `apps/api/internal/provider/composite.go` แยก success/partial/missing/error โดยไม่รวม placeholder ของ metric นอกหน้าที่ provider การเปลี่ยนนี้ไม่แก้คะแนนหรือสถานะ metric บนหน้าเว็บ

Regression tests ใหม่ล้มเหลวกับโค้ดก่อนแก้และผ่านหลังแก้ สำหรับรัศมี/cache, collection deadline, local-road telemetry, DB cancellation, readiness cancellation และ provider status รวมถึง DB connection timeout ของ updater

## เวลาใช้งานที่วัดจริง

พิกัดประชาอุทิศ 13.771269, 100.582770 รัศมี 3 กม. ใช้ constructor และค่าตั้งค่าจาก API production รัน provider composite โดยตรง ไม่สร้างรายงานลูกค้า ไม่บันทึก API usage และไม่เรียก Gemini ฐานข้อมูลใช้ `default_transaction_read_only=on` ส่วน Redis อ่านอย่างเดียว โดยเก็บข้อมูล cache ใหม่ในหน่วยความจำของ test

| ส่วน | รอบแรก (ms) | รอบถัดไป (ms) |
|---|---:|---:|
| Composite รวม | 9,002 | 205 |
| ถนนในฐานข้อมูล | 217 | 173 |
| OSM รวมถนนและคู่แข่ง | 9,002 | 173 |
| GISTDA Gateway | 2,738 | 32 |
| Google Places | 1,466 | 1 |
| MEA | 1,026 | 184 |
| DRR | 943 | 7 |
| Provincial chargers | 787 | 48 |
| DLT | 631 | 66 |
| OpenChargeMap | 565 | <1 |
| DOH | 499 | 13 |
| GISTDA elevation | 336 | <1 |
| DDPM | 17 | 16 |
| WorldPop | 8 | 16 |

เวลาต่าง ๆ รันซ้อนกัน จึงไม่ควรนำมาบวกเป็นเวลารวม รอบแรกมี cache จาก production อยู่แล้ว โดยเฉพาะ WorldPop จึงไม่ใช่การวัด cold cache ทุกบริการ รอบถัดไปใช้ทั้ง cache และ cooldown ของ endpoint ที่ล้มเหลว ไม่ใช่การพิสูจน์ว่าดึงข้อมูลสดได้ใน 205 ms

**Overpass สำหรับคู่แข่งยัง timeout อยู่จริง**: 3 endpoint ประมาณครั้งละ 3 วินาที รวม 9 วินาที แม้ลดรัศมีคำขอแล้ว ถนนยังได้จาก `local_open_data_snapshot`; คู่แข่งในผลรวมได้จากแหล่งอื่น DOH/DRR ไม่มี AADT ที่ใช้งานได้ในตัวอย่างนี้ ส่วน traffic ใช้ข้อมูลถนนเป็นค่าประมาณโดยแยกจากจำนวนรถจริง

การวัดก่อนและหลังแก้ query แบบแยก OSM ที่ไม่มี cache ต่างก็รอประมาณ 9 วินาที ดังนั้นยังไม่มีหลักฐานว่าการลด query ทำให้ upstream ที่ล้มเหลวกลับมาตอบเร็ว การแก้ลดงานส่วนเกินและป้องกันเส้นทางผิดพลาด แต่ไม่ได้แก้บริการภายนอกเอง

เวลานี้ไม่รวม HTTP queue, การรับภาพ/เอกสาร, Gemini และการบันทึกรายงานทั้งหมด หาก cache ว่าง แหล่งอื่นยังอาจช้าได้ เช่น GISTDA annual budget 30 วินาที และ provider composite มี deadline 35 วินาที

## ถนนชุดใหม่และผลทดสอบหลายพื้นที่

Worker นำเข้าจาก Geofabrik สำเร็จ ใช้ข้อมูลต้นทาง `2026-11-03T21:24:23Z`, จำนวน **2,829,714 เส้น**, skipped geometry 0, ใช้เวลานำเข้า 148.14 วินาที ตารางและดัชนีชุดใหม่ 1,064,001,536 bytes เก็บชุด 13 ต.ค. เป็น previous ตามนโยบายเก็บสองชุด สลับ current หลังสร้างดัชนีและตรวจความครอบคลุมสำเร็จเท่านั้น

ทดสอบข้อมูลใหม่ 7 พื้นที่ พื้นที่ละ 10 ครั้ง แบบอ่านอย่างเดียว:

| พื้นที่ | Median (ms) | ค่าสูงสุดจาก 10 ครั้ง (ms) |
|---|---:|---:|
| กรุงเทพฯ / ประชาอุทิศ | 89.91 | 134.32 |
| อยุธยา | 42.89 | 68.07 |
| เชียงใหม่ | 104.10 | 118.09 |
| ขอนแก่น | 37.60 | 84.73 |
| สงขลา | 38.67 | 58.56 |
| น่าน | 41.55 | 58.47 |
| ชนบทกาญจนบุรี | 19.57 | 24.23 |

นี่เป็นการสุ่มตรวจ 7 จุด ไม่ใช่การรับรองข้อมูลครบทุกพิกัดทั่วประเทศ กรณีไม่พบถนนชั้นหลักจะไม่สร้างระยะทางขึ้นเอง

## คำสั่งและการตรวจยืนยัน

- `go test -json ./... -count=1`: ผ่าน 322 tests/subtests, 14 packages; live tests ที่ต้อง opt-in ข้ามในชุด offline
- `go vet ./...` และ `go build -o /tmp/api ./cmd/api`: ผ่าน
- `CGO_ENABLED=1 go test -race ./internal/analysis ./internal/provider ./internal/roads -count=1`: ผ่านทั้ง 3 packages
- `go test -count=1 -v -run TestNationalSnapshotDatabase ./internal/roads`: ผ่าน 7 จุด/70 queries ทั้งก่อนและหลังสลับ snapshot ผ่าน DSN ที่บังคับ read-only
- `python -m unittest -v test_update_roads`: ผ่าน 5/5 แบบไม่มี network
- `./scripts/Test-RBCRoadRecovery.ps1`: ผ่าน 5/5 โดย mock Docker ไม่มีการ restart production จาก test
- `git diff --check`: ผ่าน มีเพียงคำเตือน CRLF ของ Git
- รัน Go ภายใน Docker image `rbc-ev-security-build-20261105` (Go 1.26.10) โดย mount source ปัจจุบัน; race test ติดตั้ง gcc/musl-dev ใน container ชั่วคราว ไม่เปลี่ยน dependencies ของโปรเจกต์

หลักฐานละเอียดอยู่ใน `C:/Users/friend/AppData/Local/Temp/rbc-road-latency-20261105/` โดยใช้ `provider-path-production-config.log` เป็นหลักฐาน composite; ไฟล์ `provider-path-latency.log` เป็นรอบตรวจเบื้องต้นที่ยังไม่ใช้ค่า WorldPop ปี 2025 ตาม production จึงไม่ใช้ในตารางข้างต้น

## สถานะใช้งานหลังแก้

- รัน `docker compose --profile roads up -d --build --no-deps road-updater` และ `docker compose build api` / `docker compose up -d --no-deps --no-build --wait api`
- API ใหม่เริ่ม **5 พ.ย. 2569 เวลา 11:04:49 น. ประเทศไทย**; ตรวจแล้ว environment ไม่มี drift จากค่าก่อน deploy และไม่มี pending/running analysis ก่อนเปลี่ยน container
- API image: `sha256:c5ae3be6e2b01b4e6fb6c7055182ac7bd51bce5697d8a7f6abc5fe9274eaea47`
- Road worker image: `sha256:7243c2f31aaf66c834f4e5a9ce57ad06da3efb4da88b614ef0b9e5580f87d7bd`
- Rollback API: `ev-station-api:rollback-before-road-latency-20261105`
- `/health` ตอบ 200 ประมาณ 2 ms; หน้าเว็บจริงตอบ 200; `/api/v1/sites` ที่ไม่เข้าสู่ระบบตอบ 401 ตามเดิม
- Road worker healthy, restart unless-stopped; scheduled Monitor รายงาน healthy หลังข้อมูลใหม่เข้าใช้งาน
- ไม่มีการเปลี่ยนข้อมูลรายงานลูกค้าเพื่อทดสอบ และไม่มีการเปลี่ยน frontend ในงานนี้ การเขียนฐานข้อมูล production มีเฉพาะกระบวนการอัปเดตชุดถนนที่กู้คืนตามงานนี้

ข้อจำกัดที่ยังเหลือคือการรอ Overpass สดสำหรับคู่แข่งในรอบที่ไม่มี cache/cooldown และแหล่งข้อมูลภายนอกอื่นเมื่อข้อมูลยังไม่ถูก cache ต้องแยกจากฐานข้อมูลถนนที่ยืนยันว่าเลือกใช้แล้ว ไม่ควรลด deadline อย่างเดียวจนสูญเสียข้อมูลโดยไม่ระบุสถานะ
