import { authenticate, authHeaders } from './authenticate.mjs'
import { chromium } from '@playwright/test'
import assert from 'node:assert/strict'

const base=process.env.APP_URL||'http://localhost:3000'
const browser=await chromium.launch({channel:'msedge',headless:true})
const page=await browser.newPage({extraHTTPHeaders:authHeaders,viewport:{width:1440,height:1000}})
const errors=[];page.on('pageerror',e=>errors.push(e.message))
const read=async()=>await(await page.request.get(`${base}/api/cellar`)).json()
await authenticate(page,base)
const baseline=await read(),unique=`Scanner test ${Date.now()}`
const rack=baseline.racks.find(r=>baseline.bottles.filter(b=>b.rack===r.id).length<=r.capacity-2)
assert.ok(rack,'Scanner test needs one free bottle slot')
const testResult={status:'recognized',name:unique,vintage:2018,nonVintage:false,region:'Bordeaux, France',type:'Red',confidence:'medium',notes:'Test response: check the vintage before saving.'}
// This suite mocks recognition only. Upload preparation and database storage are real.
await page.route('**/api/wine-scan/status',route=>route.fulfill({json:{available:true,maxImageBytes:8388608}}))
let mode='recognized',scans=0
await page.route('**/api/wine-scan',async route=>{
 scans++;assert.equal(route.request().method(),'POST');assert.ok(route.request().headers()['content-type'].includes('multipart/form-data'))
 assert.ok(route.request().postDataBuffer().includes(Buffer.from('image/jpeg')),'Photo should be normalized to JPEG')
 const result=mode==='recognized'?testResult:{status:mode,name:'',vintage:null,nonVintage:false,region:'',type:'',confidence:'low',notes:'Try another image.'}
 await route.fulfill({json:result})
})
try{
 await page.goto(base);await page.getByRole('button',{name:'Scan label',exact:true}).waitFor()
 const fixture=await page.evaluate(()=>{const canvas=document.createElement('canvas');canvas.width=700;canvas.height=900;const ctx=canvas.getContext('2d');ctx.fillStyle='#f3e8cc';ctx.fillRect(0,0,700,900);ctx.strokeStyle='#8c6d35';ctx.lineWidth=8;ctx.strokeRect(40,40,620,820);ctx.fillStyle='#76283d';ctx.font='48px Georgia';ctx.textAlign='center';ctx.fillText('TEST ESTATE',350,270);ctx.font='32px Georgia';ctx.fillText('GRAND RESERVE',350,355);ctx.fillText('BORDEAUX',350,480);ctx.font='58px Georgia';ctx.fillText('2018',350,610);return canvas.toDataURL('image/png').split(',')[1]})
 const upload={name:'wine-label.png',mimeType:'image/png',buffer:Buffer.from(fixture,'base64')}
 await page.getByRole('button',{name:'Scan label',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'Wine label scanner'})
 await dialog.waitFor();assert.equal(await dialog.getByLabel('Take a wine label photo').getAttribute('capture'),'environment')
 assert.equal(await dialog.getByRole('button',{name:'Identify wine',exact:true}).isDisabled(),true)
 await page.screenshot({path:'tests/scanner-desktop.png',fullPage:true})
 await dialog.getByLabel('Upload a wine label image').setInputFiles({name:'bad.png',mimeType:'image/png',buffer:Buffer.from('not an image')})
 await dialog.getByRole('alert').waitFor();assert.equal(scans,0)
 await dialog.getByLabel('Upload a wine label image').setInputFiles(upload)
 await dialog.locator('.photo-dropzone img').waitFor();assert.equal(scans,0,'Selecting a photo must not send it automatically')
 await dialog.getByRole('button',{name:'Identify wine',exact:true}).click();await dialog.getByLabel('Wine name',{exact:true}).waitFor()
 assert.equal(await dialog.getByLabel('Wine name',{exact:true}).inputValue(),unique)
 assert.equal((await read()).bottles.length,baseline.bottles.length,'Recognition must not save bottles')
 await dialog.getByLabel('Wine name',{exact:true}).fill(unique+' reviewed')
 await dialog.getByLabel('This is a non-vintage wine (NV)').check()
 await dialog.getByLabel('Shelf',{exact:true}).selectOption(rack.id)
 await dialog.getByLabel('Number of bottles').fill('2')
 await dialog.getByRole('button',{name:'Select first available',exact:true}).click()
 assert.equal(await dialog.locator('.scanner-slots button[aria-pressed="true"]').count(),2)
 await page.screenshot({path:'tests/scanner-review.png',fullPage:true})
 await dialog.getByRole('button',{name:'Add to cellar',exact:true}).click();await dialog.waitFor({state:'hidden'})
 assert.equal((await read()).bottles.filter(b=>b.name===unique+' reviewed').length,2)
 const stored=(await read()).bottles.find(b=>b.name===unique+' reviewed');assert.ok(stored);assert.equal(stored.vintage,0);assert.equal(stored.rack,rack.id)
 await page.reload();await page.getByRole('button',{name:'Wine collection',exact:false}).first().click();await page.locator('.search input').fill(unique);await page.locator('.wine-table>button').first().waitFor();assert.ok((await page.locator('.wine-table>button').first().textContent()).includes('NV'))
 await page.setViewportSize({width:390,height:844,isMobile:true})
 await page.getByRole('button',{name:'Scan label',exact:true}).click();await dialog.waitFor()
 await page.screenshot({path:'tests/scanner-mobile.png',fullPage:true})
 mode='unreadable';await dialog.getByLabel('Take a wine label photo').setInputFiles(upload);await dialog.locator('.photo-dropzone img').waitFor();await dialog.getByRole('button',{name:'Identify wine',exact:true}).click();await dialog.locator('.scanner-feedback').waitFor();assert.equal(await dialog.getByLabel('Wine name',{exact:true}).count(),0)
 await dialog.getByRole('button',{name:'Enter details manually',exact:false}).click();await dialog.getByLabel('Wine name',{exact:true}).waitFor();assert.equal(await dialog.getByLabel('Wine name',{exact:true}).inputValue(),'')
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
 await page.keyboard.press('Escape');await dialog.waitFor({state:'hidden'})
 await page.unroute('**/api/wine-scan/status');await page.route('**/api/wine-scan/status',route=>route.fulfill({json:{available:false,maxImageBytes:8388608}}))
 await page.getByRole('button',{name:'Scan label',exact:true}).click();await dialog.locator('.scanner-service-note strong').waitFor();assert.equal(await dialog.getByRole('button',{name:'Identify wine',exact:true}).isDisabled(),true)
 await dialog.getByRole('button',{name:'Enter details manually',exact:false}).click();await dialog.getByLabel('Wine name',{exact:true}).waitFor();await dialog.getByRole('button',{name:'Close wine scanner',exact:true}).click()
 assert.deepEqual(errors,[])
 console.log('PASS: camera input, JPEG preparation, invalid image, explicit identify/review/save, editable recognition, NV persistence, unreadable labels, missing-key fallback, desktop/mobile. Recognition was mocked; no paid API calls.')
}finally{
 for(const b of (await read()).bottles.filter(b=>b.name.startsWith(unique)))await page.request.delete(`${base}/api/bottles?id=${b.id}`)
 await browser.close()
}
