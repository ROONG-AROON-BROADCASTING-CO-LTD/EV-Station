import { expect, test } from '@playwright/test'
import { join } from 'node:path'
import { tmpdir } from 'node:os'

function pdf(label:string){
 let data='%PDF-1.4\n';const stream=`BT /F1 14 Tf 50 750 Td (${label}) Tj ET\n`;const offsets=[0]
 const objects=['<< /Type /Catalog /Pages 2 0 R >>','<< /Type /Pages /Kids [3 0 R] /Count 1 >>','<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>','<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',`<< /Length ${stream.length} >>\nstream\n${stream}endstream`]
 objects.forEach((value,i)=>{offsets.push(Buffer.byteLength(data));data+=`${i+1} 0 obj\n${value}\nendobj\n`});const start=Buffer.byteLength(data)
 return Buffer.from(data+`xref\n0 6\n0000000000 65535 f \n${offsets.slice(1).map(offset=>`${String(offset).padStart(10,'0')} 00000 n \n`).join('')}trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n${start}\n%%EOF\n`)
}
test('project contracts retain private versions, enforce release permissions and audit access',async({page,request,browser},info)=>{
 const root='http://127.0.0.1:18080/api/v1'; const password='E2e-password-123!'
 const session=(await(await request.post(root+'/auth/login',{data:{email:'owner@e2e.local',password}})).json()).data
 const headers={Authorization:`Bearer ${session.token}`};const email=`docs-${info.project.name}-${Date.now()}@e2e.local`
 await request.post(root+'/auth/register/request-otp',{data:{email}})
 let otp='';await expect.poll(async()=>{const mail=await(await request.get('http://127.0.0.1:18025/api/v2/messages')).json();const message=mail.items.find((m:{To:{Mailbox:string;Domain:string}[]})=>m.To.some(to=>`${to.Mailbox}@${to.Domain}`===email));otp=String(message?.Content?.Body||'').match(/\b\d{6}\b/)?.[0]||'';return otp}).toMatch(/^\d{6}$/)
 expect((await request.post(root+'/auth/register',{data:{email,displayName:'Document customer',password,otp}})).ok()).toBeTruthy()
 const customer=(await(await request.post(root+'/auth/login',{data:{email,password}})).json()).data;const customerHeaders={Authorization:`Bearer ${customer.token}`}
 expect((await request.get(root+'/document-templates',{headers:customerHeaders})).status()).toBe(403)
 const template=await request.post(root+'/document-templates',{headers,multipart:{title:'Shared QA example',file:{name:'sample.pdf',mimeType:'application/pdf',buffer:pdf('Shared internal sample')}}})
 expect(template.ok()).toBeTruthy();const templateID=(await template.json()).data.id
 expect((await request.get(`${root}/document-templates/${templateID}/file`,{headers:customerHeaders})).status()).toBe(403)
 expect((await request.post(root+'/document-templates',{headers:customerHeaders,multipart:{title:'Denied',file:{name:'denied.pdf',mimeType:'application/pdf',buffer:pdf('Denied')}}})).status()).toBe(403)
 const salesEmail='template-sales-'+Date.now()+'@e2e.local'
 expect((await request.post(root+'/users',{headers,data:{email:salesEmail,displayName:'Template sales',password,role:'sales'}})).ok()).toBeTruthy()
 const sales=(await(await request.post(root+'/auth/login',{data:{email:salesEmail,password}})).json()).data;const salesHeaders={Authorization:`Bearer ${sales.token}`}
 expect((await request.get(`${root}/document-templates/${templateID}/file`,{headers:salesHeaders})).status()).toBe(200)
 expect((await request.post(root+'/document-templates',{headers:salesHeaders,multipart:{title:'Denied',file:{name:'denied.pdf',mimeType:'application/pdf',buffer:pdf('Denied')}}})).status()).toBe(403)
 const site=(await(await request.post(root+'/sites',{headers:customerHeaders,data:{name:'Contract QA '+Date.now(),latitude:13.75,longitude:100.5,landSize:400,landSizeUnit:'sqm'}})).json()).data
 const other=(await(await request.post(root+'/sites',{headers,data:{name:'Unrelated QA',latitude:13.7,longitude:100.4,landSize:400,landSizeUnit:'sqm'}})).json()).data
 await page.addInitScript(s=>{localStorage.setItem('rbc-language','en');localStorage.setItem('rbc-session',JSON.stringify(s));localStorage.setItem('rbc-session-token',s.token)},session)
 const errors:string[]=[];page.on('pageerror',e=>errors.push(e.message))
 await page.goto(`/sites/${site.id}/documents`)
 await page.getByText('Add project document',{exact:true}).click()
 await page.getByLabel('Document title',{exact:true}).fill('QA contract confidential')
 const original=pdf('Original signed contract QA')
 await page.getByLabel('PDF · 10 MB',{exact:true}).setInputFiles({name:'contract.pdf',mimeType:'application/pdf',buffer:original})
 const [submission]=await Promise.all([page.waitForRequest(r=>r.method()==='POST'&&r.url().endsWith(`/sites/${site.id}/documents`)),page.getByRole('button',{name:'Upload PDF',exact:true}).click()])
 await expect(page.getByRole('heading',{name:'QA contract confidential',exact:true})).toBeVisible()
 let doc=(await(await request.get(`${root}/sites/${site.id}/documents`,{headers})).json()).data[0]
 expect(doc.metadata.shared).toBe(false);expect(doc.metadata.status).toBe('review')
 const endpoint=`${root}/sites/${site.id}/documents/${doc.id}`
 expect((await(await request.get(`${root}/sites/${site.id}/images`,{headers})).json()).data).toHaveLength(0)
 const retry=await request.post(`${root}/sites/${site.id}/documents`,{headers:{...headers,'X-Request-ID':(await submission.headerValue('x-request-id'))!},multipart:{title:'QA contract confidential',category:'contract',file:{name:'contract.pdf',mimeType:'application/pdf',buffer:original}}})
 expect(retry.ok()).toBeTruthy();expect((await retry.json()).data.id).toBe(doc.id)
 expect((await request.delete(`${root}/sites/${site.id}`,{headers})).status()).toBe(409)
 expect((await(await request.get(`${root}/documents`,{headers:customerHeaders})).json()).data).toHaveLength(0)
 expect((await request.get(`${endpoint}/versions/1/file`,{headers:customerHeaders})).status()).toBe(404)
 expect((await request.put(endpoint,{headers:customerHeaders,data:doc.metadata})).status()).toBe(403)
 expect((await request.get(`${root}/sites/${other.id}/documents`,{headers:customerHeaders})).status()).toBe(403)
 expect((await request.get(`${root}/sites/${other.id}/documents/${doc.id}/versions`,{headers})).status()).toBe(404)
 expect((await request.put(endpoint,{headers,data:{...doc.metadata,status:'active',expectedVersion:1}})).status()).toBe(400)
 await page.getByLabel('Contract number',{exact:true}).fill('QA-2026-01')
 await page.getByLabel('Counterparty',{exact:true}).fill('QA partner')
 await page.getByLabel('Release the latest complete PDF',{exact:false}).check()
 await page.getByRole('button',{name:'Save document details',exact:true}).click()
 await expect.poll(async()=>{doc=(await(await request.get(`${root}/sites/${site.id}/documents`,{headers})).json()).data[0];return doc.metadata.shared}).toBe(true)
 const visible=(await(await request.get(`${root}/documents`,{headers:customerHeaders})).json()).data
 expect(visible).toHaveLength(1);expect(visible[0].metadata.contractNumber).toBe('QA-2026-01')
 const downloaded=await request.get(`${endpoint}/versions/1/file?download=1`,{headers:customerHeaders})
 expect(downloaded.status()).toBe(200);expect(downloaded.headers()['cache-control']).toContain('no-store');expect(await downloaded.body()).toEqual(original)
 expect((await request.get(`${endpoint}/events`,{headers:customerHeaders})).status()).toBe(403)
 await page.reload();await page.getByRole('button',{name:/QA contract confidential/}).click()
 await page.getByRole('button',{name:'Preview',exact:true}).first().click()
 await expect(page.getByTitle('PDF preview')).toBeVisible()
 await page.getByRole('button',{name:'Close preview',exact:true}).click()
 const next=pdf('Revised contract QA version 2')
 await page.getByLabel('Upload new version',{exact:false}).setInputFiles({name:'revised.pdf',mimeType:'application/pdf',buffer:next})
 await expect.poll(async()=>{doc=(await(await request.get(`${root}/sites/${site.id}/documents`,{headers})).json()).data[0];return doc.latestVersion}).toBe(2)
 expect(doc.metadata.shared).toBe(false);expect(doc.metadata.signaturesConfirmed).toBe(false)
 expect((await request.put(endpoint,{headers,data:{...doc.metadata,shared:true,expectedVersion:1}})).status()).toBe(409)
 expect((await request.get(`${endpoint}/versions/1/file`,{headers:customerHeaders})).status()).toBe(404)
 // Uploading the same bytes after a lost response must not create version 3.
 expect((await request.post(endpoint+'/versions',{headers,multipart:{file:{name:'retry.pdf',mimeType:'application/pdf',buffer:next}}})).ok()).toBeTruthy()
 expect((await(await request.get(endpoint+'/versions',{headers})).json()).data).toHaveLength(2)
 expect(await(await request.get(`${endpoint}/versions/1/file`,{headers})).body()).toEqual(original)
 expect((await request.put(endpoint,{headers,data:{...doc.metadata,shared:true,expectedVersion:2}})).ok()).toBeTruthy()
 expect((await request.get(`${endpoint}/versions/1/file`,{headers:customerHeaders})).status()).toBe(404)
 expect((await request.get(`${endpoint}/versions/2/file`,{headers:customerHeaders})).status()).toBe(200)
 expect((await(await request.get(endpoint+'/versions',{headers:customerHeaders})).json()).data).toHaveLength(1)
 expect((await request.post(endpoint+'/versions',{headers,multipart:{file:{name:'bad.pdf',mimeType:'application/pdf',buffer:Buffer.from('not a PDF')}}})).status()).toBe(400)
 const events=(await(await request.get(endpoint+'/events',{headers})).json()).data
 expect(events.some((e:{action:string;actorId:string})=>e.action==='downloaded'&&e.actorId===customer.user.id)).toBe(true)
 await page.reload();await page.getByRole('button',{name:/QA contract confidential/}).click()
 await expect(page.getByText('v2 · revised.pdf',{exact:true})).toBeVisible()
 expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true)
 await page.screenshot({path:join(tmpdir(),`rbc-contracts-${info.project.name}.png`),fullPage:true})
 await page.getByRole('button',{name:'ไทย',exact:true}).click()
 await expect(page.getByRole('heading',{name:'เอกสารและสัญญา',exact:true})).toBeVisible()
 if(info.project.name==='desktop')for(const width of [320,768,1440]){await page.setViewportSize({width,height:900});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1)).toBe(true)}
 await page.screenshot({path:join(tmpdir(),`rbc-contracts-th-${info.project.name}.png`),fullPage:true})
 const customerContext=await browser.newContext({baseURL:'http://127.0.0.1:15173',viewport:page.viewportSize()!});const customerPage=await customerContext.newPage()
 await customerPage.addInitScript(s=>{localStorage.setItem('rbc-language','en');localStorage.setItem('rbc-session',JSON.stringify(s));localStorage.setItem('rbc-session-token',s.token)},customer)
 await customerPage.goto('/documents');await customerPage.getByRole('button',{name:/QA contract confidential/}).click()
 await expect(customerPage.getByRole('heading',{name:'Shared contract templates · all projects',exact:true})).toHaveCount(0)
 await expect(customerPage.getByText('v2 · revised.pdf',{exact:true})).toBeVisible()
 await expect(customerPage.getByText('v1 · contract.pdf',{exact:true})).toHaveCount(0)
 await expect(customerPage.getByRole('button',{name:'Save document details',exact:true})).toHaveCount(0)
 const [download]=await Promise.all([customerPage.waitForEvent('download'),customerPage.getByRole('button',{name:'Download',exact:true}).click()])
 expect(await download.failure()).toBeNull()
 await customerContext.close()
 expect(errors).toEqual([])
})
