# เริ่มระบบหลังเปิดคอมพิวเตอร์

ใช้ `scripts\Start-RBC-EV-Station.ps1` เพื่อเปิด Docker Desktop, ฐานข้อมูล, API, เว็บ และ Cloudflare Named Tunnel ตามลำดับ แล้วเปิดหน้าเว็บด้วย Microsoft Edge ที่ `https://www.rbcevstation.com`:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-RBC-EV-Station.ps1 -OpenEdge
```

หากต้องการให้เริ่มเองทุกครั้งหลังลงชื่อเข้า Windows ให้รันครั้งเดียว:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Install-RBC-AutoStart.ps1
```

ตัวติดตั้งจะวางคำสั่งเริ่มระบบในโฟลเดอร์ Startup ของผู้ใช้ จึงไม่ต้องใช้สิทธิ์ผู้ดูแลระบบ.

## Cloudflare Named Tunnel

สคริปต์นี้ใช้ Named Tunnel ที่ติดตั้งเป็น Windows service ชื่อ `cloudflared` และเปิดเว็บผ่าน `https://www.rbcevstation.com` เท่านั้น ไม่สร้าง URL ชั่วคราว `trycloudflare.com` อีกต่อไป
