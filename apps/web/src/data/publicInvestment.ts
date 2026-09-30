type LocalizedText = { th: string; en: string }

type BaseModel = {
  code: 'S' | 'M' | 'L'
  nameTh: string
  nameEn: string
  image: string
  imageEn?: string
  imageAlt: LocalizedText
  area: LocalizedText
  chargers: LocalizedText
}

type CafeModel = BaseModel & {
  investment: LocalizedText
  franchise: LocalizedText
  features: LocalizedText[]
}

type StandaloneModel = BaseModel & {
  parking: LocalizedText
  electrical: LocalizedText
}

export const chargerModels: CafeModel[] = [
  {
    code: 'S', nameTh: 'SMART CAFÉ', nameEn: 'SMART CAFÉ', image: '/rbc-format-s-smart-cafe.png', imageEn: '/rbc-format-s-smart-cafe-en.png',
    imageAlt: { th: 'ผังโครงการ S – Smart Café', en: 'Concept plan for the S Smart Café format' },
    area: { th: '100 ตร.ว.', en: '100 sq wah (400 m²)' },
    chargers: { th: '1 สถานี', en: '1 charging station' },
    investment: { th: '1.5–2.5 ล้านบาท', en: 'THB 1.5–2.5 million' },
    franchise: { th: '300,000 บาท', en: 'THB 300,000' },
    features: [
      { th: 'EV Charging 1 สถานี', en: '1 EV charging station' },
      { th: 'ร้านกาแฟ : Super Black Coffee', en: 'Super Black Coffee café' },
    ],
  },
  {
    code: 'M', nameTh: 'LIFESTYLE CAFÉ', nameEn: 'LIFESTYLE CAFÉ', image: '/rbc-format-m-lifestyle-cafe.png', imageEn: '/rbc-format-m-lifestyle-cafe-en.png',
    imageAlt: { th: 'ผังโครงการ M – Lifestyle Café', en: 'Concept plan for the M Lifestyle Café format' },
    area: { th: '200 ตร.ว.', en: '200 sq wah (800 m²)' },
    chargers: { th: '2 สถานี', en: '2 charging stations' },
    investment: { th: '3.5–5 ล้านบาท', en: 'THB 3.5–5 million' },
    franchise: { th: '500,000 บาท', en: 'THB 500,000' },
    features: [
      { th: 'EV Charging 2 สถานี', en: '2 EV charging stations' },
      { th: 'พื้นที่จัดงาน ประชุม และกิจกรรม', en: 'Event, meeting and activity space' },
      { th: 'ไปรษณีย์ : BPOST65 EXPRESS', en: 'Post office: BPOST65 EXPRESS' },
      { th: 'ร้านอาหาร & เบเกอรี่', en: 'Restaurant & bakery' },
      { th: 'ร้านกาแฟ : Super Black Coffee', en: 'Super Black Coffee café' },
    ],
  },
  {
    code: 'L', nameTh: 'LIFESTYLE HUB', nameEn: 'LIFESTYLE HUB', image: '/rbc-format-l-lifestyle-hub.png', imageEn: '/rbc-format-l-lifestyle-hub-en.png',
    imageAlt: { th: 'ผังโครงการ L – Lifestyle Hub', en: 'Concept plan for the L Lifestyle Hub format' },
    area: { th: '400 ตร.ว.', en: '400 sq wah (1,600 m²)' },
    chargers: { th: '4 สถานี', en: '4 charging stations' },
    investment: { th: '7–10 ล้านบาทขึ้นไป', en: 'THB 7–10 million or more' },
    franchise: { th: '700,000 บาท', en: 'THB 700,000' },
    features: [
      { th: 'EV Charging 4 สถานี', en: '4 EV charging stations' },
      { th: 'ไปรษณีย์ : BPOST65 EXPRESS', en: 'Post office: BPOST65 EXPRESS' },
      { th: 'ร้านอาหาร & เบเกอรี่', en: 'Restaurant & bakery' },
      { th: 'ร้านกาแฟ : Super Black Coffee', en: 'Super Black Coffee café' },
      { th: 'Co-working / พื้นที่ทำงาน', en: 'Co-working / work zone' },
      { th: 'Mobile Café / ฟู้ดทรัก', en: 'Mobile Café / food truck' },
      { th: 'พื้นที่จัดงาน ประชุม และกิจกรรม', en: 'Event, meeting and activity space' },
    ],
  },
]

export const standaloneStationModels: StandaloneModel[] = [
  {
    code: 'S', nameTh: 'EV CHARGING STATION S', nameEn: 'EV CHARGING STATION S', image: '/rbc-ev-station-s.png',
    imageAlt: { th: 'แนวคิดสถานีชาร์จ EV แบบเดี่ยว ขนาด S', en: 'Concept plan for a standalone S EV charging station' },
    area: { th: '20 × 20 ม.', en: '20 × 20 m' },
    chargers: { th: '1 ตู้ · 2 หัวชาร์จ', en: '1 DC charger · 2 connectors' },
    parking: { th: '2 ช่องจอด EV', en: '2 EV parking bays' },
    electrical: { th: 'MDB-EV 1 ตู้ · เสาไฟ 1 ต้น', en: '1 MDB-EV cabinet · 1 light pole' },
  },
  {
    code: 'M', nameTh: 'EV CHARGING STATION M', nameEn: 'EV CHARGING STATION M', image: '/rbc-ev-station-m.png',
    imageAlt: { th: 'แนวคิดสถานีชาร์จ EV แบบเดี่ยว ขนาด M', en: 'Concept plan for a standalone M EV charging station' },
    area: { th: '28 × 28 ม.', en: '28 × 28 m' },
    chargers: { th: '2 ตู้ · 4 หัวชาร์จ', en: '2 DC chargers · 4 connectors' },
    parking: { th: '4 ช่องจอด EV', en: '4 EV parking bays' },
    electrical: { th: 'MDB-EV 1 ตู้ · เสาไฟ 2 ต้น', en: '1 MDB-EV cabinet · 2 light poles' },
  },
  {
    code: 'L', nameTh: 'EV CHARGING HUB L', nameEn: 'EV CHARGING HUB L', image: '/rbc-ev-station-l.png',
    imageAlt: { th: 'แนวคิดสถานีชาร์จ EV แบบเดี่ยว ขนาด L', en: 'Concept plan for a standalone L EV charging hub' },
    area: { th: '40 × 40 ม.', en: '40 × 40 m' },
    chargers: { th: '4 ตู้ · 8 หัวชาร์จ', en: '4 DC chargers · 8 connectors' },
    parking: { th: '8 ช่องจอด EV', en: '8 EV parking bays' },
    electrical: { th: 'หม้อแปลงและ MDB-EV 1 ชุด · เสาไฟ 4 ต้น', en: '1 transformer and MDB-EV set · 4 light poles' },
  },
]
