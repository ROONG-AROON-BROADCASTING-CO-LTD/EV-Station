# เริ่มระบบหลังเปิดคอมพิวเตอร์

ใช้ `scripts\Start-RBC-EV-Station.ps1` เพื่อเปิด Docker Desktop, ฐานข้อมูล, API, เว็บ และ Cloudflare Quick Tunnel ตามลำดับ แล้วเปิดหน้าเว็บด้วย Microsoft Edge:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Start-RBC-EV-Station.ps1 -OpenEdge
```

หากต้องการให้เริ่มเองทุกครั้งหลังลงชื่อเข้า Windows ให้รันครั้งเดียว:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\Install-RBC-AutoStart.ps1
```

ตัวติดตั้งจะวางคำสั่งเริ่มระบบในโฟลเดอร์ Startup ของผู้ใช้ จึงไม่ต้องใช้สิทธิ์ผู้ดูแลระบบ.

## ข้อจำกัดของ Quick Tunnel

`trycloudflare.com` เหมาะกับการทดสอบเท่านั้น URL จะเปลี่ยนหลัง Tunnel หยุดหรือเครื่องรีสตาร์ต โปรแกรมจะบันทึก URL ล่าสุดที่ `.runtime\tunnel-url.txt` และแสดงในหน้าต่าง PowerShell แต่ LINE Developers ต้องอัปเดต LIFF endpoint ให้ตรงกับ URL ใหม่นั้นทุกครั้ง

ก่อนเปิดใช้กับลูกค้าจริง ให้ใช้ Cloudflare Named Tunnel พร้อมโดเมนของ RBC เพื่อให้ LIFF endpoint เป็น URL ถาวร
