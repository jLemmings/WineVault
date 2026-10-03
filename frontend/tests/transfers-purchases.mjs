import { chromium, expect } from '@playwright/test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
const browser=await chromium.launch({headless:true}),page=await browser.newPage({viewport:{width:1440,height:1000}})
const errors=[];page.on('pageerror',e=>errors.push(e.message))
const rack={id:'A',name:'Main shelf',short:'Main',wall:'North',rows:2,columns:3,capacity:6,temp:12,x:.1,y:.1,width:1,depth:.5,rotation:0,color:'red'}
const cellar={id:'test',name:'Cellar',owner:'Owner',room:'Room',revision:1,width:5,depth:4,racks:[rack],bottles:[{id:'1',name:'Existing wine',vintage:2020,region:'France',type:'Red',rack:'A',slot:0,revision:1,priceMinor:900,currency:'EUR',purchaseDate:'2026-01-01',seller:'Original shop'}],preferences:{viewMode:'racks-only',typeRacks:{}},layout:{shape:'rectangle',floor:'stone',doorWall:'south',doorOffset:0,doorWidth:.8,tableEnabled:false}}
const purchases=()=>cellar.bottles.filter(b=>b.priceMinor!=null||b.purchaseDate||b.seller).map(b=>({...b,bottleId:b.id,inCellar:true}))
let attempts=0
const csv='name,vintage,region,type,quantity,rack,price,currency,purchase_date,seller\n"Estate, Reserve",NV,France,White,2,A,12.35,CHF,2026-02-02,"Shop, Zurich"\n'
await page.route('**/api/**',async route=>{
 const req=route.request(),path=new URL(req.url()).pathname
 if(path==='/api/auth/status')return route.fulfill({json:{authenticated:true,setupRequired:false,username:'Owner'}})
 if(path==='/api/cellar')return route.fulfill({json:cellar})
 if(path==='/api/collection/export')return route.fulfill({contentType:'text/csv',body:'name,vintage,region,type,quantity,rack,slot,barcode,price,currency,purchase_date,seller\nExisting wine,2020,France,Red,1,A,A1,,9.00,EUR,2026-01-01,Original shop\n'})
 if(path==='/api/collection/import/preview'){assert.equal(req.postData(),csv);return route.fulfill({json:{rows:[{row:2,bottle:{name:'Estate, Reserve',vintage:0,region:'France',type:'White',rack:'A',barcode:'',priceMinor:1235,currency:'CHF',purchaseDate:'2026-02-02',seller:'Shop, Zurich'},vintageText:'NV',price:'12.35',quantity:2,slotAddress:'',errors:[]}],totalBottles:2}})}
 if(path==='/api/collection/import'){
  attempts++;const body=req.postDataJSON();assert.equal(body.bottles.length,2);for(const b of body.bottles){assert.equal(b.priceMinor,1235);assert.equal(b.currency,'CHF');assert.equal(b.vintage,0);assert.equal(b.seller,'Shop, Zurich')}
  if(attempts===1){cellar.bottles.push({id:'occupied',name:'Concurrent addition',vintage:2020,region:'France',type:'Red',rack:'A',slot:1,revision:1});return route.fulfill({status:409,body:'The selected slot is unavailable. Choose another empty slot.'})}
  assert.deepEqual(body.bottles.map(b=>b.slot),[2,3]);cellar.bottles.push(...body.bottles.map((b,i)=>({...b,id:String(i+10),revision:1})));return route.fulfill({status:201,json:{imported:2}})
 }
 if(path==='/api/purchases')return route.fulfill({json:purchases()})
 if(path.endsWith('/information'))return route.fulfill({json:{status:'disabled',data:null,message:'Not configured'}})
 if(path.endsWith('/purchase')){const b=cellar.bottles.find(b=>b.id===path.split('/')[3]),body=req.postDataJSON();assert.equal(body.revision,b.revision);Object.assign(b,body);b.revision++;return route.fulfill({json:{revision:b.revision}})}
 throw Error(`Unexpected API request ${req.method()} ${path}`)
})
try{
 await page.goto(process.env.APP_URL||'http://localhost:3000')
 await page.getByRole('button',{name:'Wine collection',exact:false}).first().click()
 await page.getByRole('button',{name:'Import / export CSV',exact:true}).click()
 const dialog=page.getByRole('dialog',{name:'CSV import and export'})
 let wait=page.waitForEvent('download');await dialog.getByRole('button',{name:'Export collection CSV',exact:true}).click();let download=await wait
 assert.equal(download.suggestedFilename(),'winevault-collection.csv');assert.match(await readFile(await download.path(),'utf8'),/9\.00,EUR/)
 wait=page.waitForEvent('download');await dialog.getByRole('button',{name:'Download CSV template',exact:true}).click();download=await wait;assert.match(await readFile(await download.path(),'utf8'),/purchase_date,seller/)
 await dialog.getByLabel('Import CSV file',{exact:true}).setInputFiles({name:'collection.csv',mimeType:'text/csv',buffer:Buffer.from(csv)})
 await dialog.getByRole('heading',{name:'Review collection.csv'}).waitFor()
 assert.equal(cellar.bottles.length,1,'Preview must not write inventory')
 await expect(dialog.getByText('Placement: Main shelf · B1, C1',{exact:true})).toBeVisible()
 await dialog.getByRole('button',{name:'Confirm import · 2 bottles',exact:true}).click()
 await expect(dialog.getByText(/selected slot is unavailable/)).toBeVisible()
 await expect(dialog.getByText('Placement: Main shelf · C1, A2',{exact:true})).toBeVisible()
 await dialog.getByRole('button',{name:'Confirm import · 2 bottles',exact:true}).click();await expect(dialog).toHaveCount(0)
 assert.equal(cellar.bottles.length,4)
 await page.getByRole('button',{name:'Purchases',exact:true}).click()
 const tracker=page.locator('.purchase-tracker');await expect(tracker.getByText('CHF',{exact:true})).toBeVisible();await expect(tracker.getByText('EUR',{exact:true})).toBeVisible()
 await expect(tracker.getByRole('heading',{name:/24\.70/})).toBeVisible()
 await tracker.getByRole('button',{name:'Estate, Reserve',exact:true}).first().click()
 const detail=page.getByRole('dialog',{name:'Bottle details'})
 await detail.getByRole('button',{name:'Edit purchase details',exact:true}).click()
 await detail.getByLabel('Price per bottle',{exact:true}).fill('15.50')
 await detail.getByLabel('Seller',{exact:true}).fill('Updated seller')
 await detail.getByRole('button',{name:'Save purchase',exact:true}).click()
 await expect(detail.getByText('Seller: Updated seller',{exact:true})).toBeVisible()
 await detail.getByRole('button',{name:'Close dialog',exact:true}).click()
 await expect(tracker.getByRole('heading',{name:/27\.85/})).toBeVisible()
 await page.setViewportSize({width:390,height:844})
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
 await page.getByRole('button',{name:'Cellar settings',exact:true}).click()
 await page.getByRole('button',{name:'Import / export CSV',exact:true}).click()
 await expect(dialog).toBeVisible();assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
 assert.deepEqual(errors,[])
 console.log('PASS CSV downloads, preview without writes, reviewed batch placement, conflict recovery, purchase totals by currency, purchase editing, and mobile transfer/tracker. API mocked.')
}finally{await browser.close()}
