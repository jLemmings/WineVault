import { authenticate, authHeaders } from './authenticate.mjs'
﻿import { chromium } from '@playwright/test'
import assert from 'node:assert/strict'
const baseURL = process.env.APP_URL || 'http://localhost:3000'
const browser = await chromium.launch({ channel: 'msedge', headless: true })
const page = await browser.newPage({extraHTTPHeaders:authHeaders, viewport: { width: 1440, height: 1050 } })
const errors=[]
const wineName=`Browser test ${Date.now()}`
page.on('pageerror', e=>errors.push(e.message))
await authenticate(page,baseURL)
const snapshot=await (await page.request.get(`${baseURL}/api/cellar`)).json()
const baseline=snapshot.bottles.length
const freeRack=snapshot.racks.find(r=>snapshot.bottles.filter(b=>b.rack===r.id).length<r.capacity)
assert.ok(freeRack,'The browser inventory test needs one free slot')
const waitCount=async count=>page.waitForFunction(n=>Number.parseInt(document.querySelector('.stat strong')?.textContent)===n,count)
try {
 await page.goto(baseURL)
 await waitCount(baseline)
 assert.ok((await page.locator('.cellar-title h2').textContent()).includes(snapshot.name))
 assert.ok((await page.locator('.profile strong').textContent()).includes(snapshot.owner))
 await page.screenshot({path:'tests/desktop.png',fullPage:true})
 await page.locator(`.rack-${freeRack.id}`).click()
 await page.getByRole('heading',{name:freeRack.wall,exact:true}).waitFor()
 await page.getByRole('button',{name:'Rack view',exact:true}).click()
 assert.equal(await page.locator('.rack-card').count(),snapshot.racks.length)
 await page.getByRole('button',{name:'Add wine',exact:true}).click()
 await page.getByLabel('Wine name').fill(wineName)
 await page.getByLabel('Region',{exact:true}).fill('Bordeaux, France')
 await page.getByRole('button',{name:'Select first available',exact:true}).click()
 await page.getByRole('button',{name:'Add to cellar'}).click()
 await waitCount(baseline+1)
 await page.reload()
 await waitCount(baseline+1)
 await page.getByRole('button',{name:'Wine collection',exact:false}).first().click()
 await page.locator('.search input').fill(wineName)
 await page.locator('.wine-table>button').click()
 await page.getByRole('button',{name:'Mark as enjoyed'}).click()
 await waitCount(baseline)
 await page.getByRole('button',{name:'My cellar',exact:true}).click()
 await page.getByRole('button',{name:'Floor plan',exact:true}).click()
 await page.locator('.toast button').click()
 await page.setViewportSize({width:390,height:844})
 await page.screenshot({path:'tests/mobile.png',fullPage:true})
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>window.innerWidth),false,'Mobile page overflows horizontally')
 // A failed database read must show an error, never a demo-data fallback.
 await page.route('**/api/cellar',route=>route.fulfill({status:503,body:'Database unavailable'}))
 await page.reload()
 await page.locator('.error-banner').waitFor()
 assert.equal(await page.locator('.stats').count(),0)
 await page.unroute('**/api/cellar')
 await page.getByRole('button',{name:'Retry connection'}).click()
 await waitCount(baseline)
 assert.deepEqual(errors,[])
 console.log('PASS: database metadata, persisted inventory after reload, desktop/mobile, add/enjoy, and database-unavailable recovery.')
} finally {
 const response=await page.request.get(`${baseURL}/api/bottles`)
 if(response.ok())for(const b of await response.json())if(b.name===wineName)await page.request.delete(`${baseURL}/api/bottles?id=${b.id}`)
 await browser.close()
}
