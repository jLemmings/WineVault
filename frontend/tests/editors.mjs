import { authenticate, authHeaders } from './authenticate.mjs'
import { chromium } from '@playwright/test'
import assert from 'node:assert/strict'

const base=process.env.APP_URL||'http://localhost:3000'
const browser=await chromium.launch({channel:'msedge',headless:true})
const page=await browser.newPage({extraHTTPHeaders:authHeaders,viewport:{width:1440,height:1000}})
const errors=[];page.on('pageerror',e=>errors.push(e.message))
const read=async()=>await (await page.request.get(`${base}/api/cellar`)).json()
await authenticate(page,base)
const original=await read(),first=original.racks[0],testName=`Editor shelf ${Date.now()}`
const rackBody=(r,revision)=>({revision,name:r.name,short:r.short,wall:r.wall,grapes:r.grapes,temp:r.temp,color:r.color,rows:r.rows,columns:r.columns})
let touchedRoom=false,touchedShelf=false
try{
 await page.goto(base);await page.getByRole('button',{name:'Edit room',exact:true}).waitFor()
 await page.getByRole('button',{name:'Edit room',exact:true}).click()
 const room=page.getByRole('dialog',{name:'Room layout editor'})
 await room.waitFor()
 await room.getByLabel('Room width (m)',{exact:true}).fill(String(Math.max(original.width,6)))
 await room.getByLabel('Floor material').selectOption(original.layout.floor==='wood'?'stone':'wood')
 const shelf=room.locator(`.rack-${first.id}`)
 const bounds=await shelf.boundingBox()
 await page.mouse.move(bounds.x+bounds.width/2,bounds.y+bounds.height/2)
 await page.mouse.down();await page.mouse.move(bounds.x+bounds.width/2+12,bounds.y+bounds.height/2,{steps:5});await page.mouse.up()
 assert.notEqual(Number(await room.getByLabel('Shelf X (m)').inputValue()),first.x,'Shelf drag did not move its position')
 await room.getByLabel('Shelf X (m)').fill(String(first.x))
 await room.getByRole('button',{name:/Rotate 90/}).click();await room.getByRole('button',{name:/Rotate 90/}).click()
 await room.getByRole('button',{name:'Room',exact:true}).click()
 await room.getByLabel('Room shape').selectOption('l-shape')
 await room.getByRole('button',{name:'Door',exact:true}).click()
 await room.getByLabel('Door offset (m)').fill('35')
 assert.equal(await room.getByRole('button',{name:'Save layout',exact:true}).isDisabled(),true,'An out-of-bounds door should block saving')
 await room.getByLabel('Door offset (m)').fill(String(original.layout.doorOffset))
 await room.getByRole('button',{name:'Table',exact:true}).click()
 await room.getByLabel('Include a tasting table').uncheck();assert.equal(await room.locator('.plan-table').count(),0)
 if(original.layout.tableEnabled)await room.getByLabel('Include a tasting table').check()
 await room.getByRole('button',{name:'Room',exact:true}).click()
 await page.screenshot({path:'tests/room-editor.png',fullPage:true})
 const saved=page.waitForResponse(r=>r.url().endsWith('/api/cellar/layout')&&r.request().method()==='PUT')
 await room.getByRole('button',{name:'Save layout',exact:true}).click();assert.equal((await saved).status(),200);touchedRoom=true
 await room.waitFor({state:'hidden'});await page.reload();await page.getByRole('button',{name:'Edit room',exact:true}).waitFor()
 const changed=await read();assert.notEqual(changed.layout.floor,original.layout.floor)
 // Cancel must discard changes, not issue a save.
 await page.getByRole('button',{name:'Edit room',exact:true}).click();await room.getByLabel('Room label',{exact:true}).fill('Unsaved room label');await room.getByRole('button',{name:'Cancel',exact:true}).click();assert.equal((await read()).room,original.room)
 await page.locator(`.rack-${first.id}`).click();await page.getByRole('button',{name:'Edit shelf',exact:true}).click()
 const editor=page.getByRole('dialog',{name:'Shelf editor',exact:true})
 await editor.getByLabel('Shelf name',{exact:true}).fill(first.name+' edited')
 await editor.getByLabel('Rows',{exact:true}).fill('1');await editor.getByLabel('Columns',{exact:true}).fill('1')
 assert.equal(await editor.getByRole('button',{name:'Save shelf',exact:true}).isDisabled(),true,'Occupied slot shrink should be disabled')
 await editor.getByLabel('Rows',{exact:true}).fill(String(first.rows+1));await editor.getByLabel('Columns',{exact:true}).fill(String(first.columns))
 await page.screenshot({path:'tests/shelf-editor.png',fullPage:true})
 const shelfSaved=page.waitForResponse(r=>r.url().endsWith(`/api/racks/${first.id}`)&&r.request().method()==='PUT')
 await editor.getByRole('button',{name:'Save shelf',exact:true}).click();assert.equal((await shelfSaved).status(),200);touchedShelf=true
 await editor.waitFor({state:'hidden'});await page.reload();await page.getByRole('button',{name:'Edit room',exact:true}).waitFor()
 let current=await read();assert.equal(current.racks[0].rows,first.rows+1);assert.deepEqual(current.bottles,original.bottles)
 await page.getByRole('button',{name:'Add shelf',exact:true}).click();const create=page.getByRole('dialog',{name:'Add shelf',exact:true});await create.getByLabel('Shelf name',{exact:true}).fill(testName)
 await create.getByRole('button',{name:'Create shelf',exact:true}).click();await create.waitFor({state:'hidden'});current=await read();const added=current.racks.find(r=>r.name===testName);assert.ok(added)
 await page.getByRole('button',{name:'Rack view',exact:true}).click();await page.getByRole('button',{name:`Edit shelf ${added.id}`,exact:true}).click();await editor.getByRole('button',{name:'Remove shelf',exact:true}).click();await editor.getByRole('button',{name:'Yes, remove shelf',exact:true}).click();await editor.waitFor({state:'hidden'});assert.ok(!(await read()).racks.some(r=>r.id===added.id))
 await page.setViewportSize({width:390,height:844});await page.getByRole('button',{name:'Edit room',exact:true}).click();await room.waitFor();assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);await page.screenshot({path:'tests/room-editor-mobile.png',fullPage:true});await room.getByRole('button',{name:'Cancel',exact:true}).click()
 await page.getByRole('button',{name:`Edit shelf ${first.id}`,exact:true}).click();await editor.waitFor();await page.screenshot({path:'tests/shelf-editor-mobile.png',fullPage:true});assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);await page.keyboard.press('Escape');await editor.waitFor({state:'hidden'})
 assert.deepEqual(errors,[])
 console.log('PASS: draggable room layout, save/reload, cancel, shelf preview and resize, bottle preservation, create/remove shelf, mobile editors, and keyboard close.')
}finally{
 let current=await read()
 for(const rack of current.racks.filter(r=>r.name===testName)){await page.request.delete(`${base}/api/racks/${rack.id}`,{data:{revision:current.revision}});current=await read()}
 if(touchedShelf){const response=await page.request.put(`${base}/api/racks/${first.id}`,{data:rackBody(first,current.revision)});assert.equal(response.status(),200,await response.text());current=await read()}
 if(touchedRoom){const {name,room,width,depth,layout,racks}=original;const response=await page.request.put(`${base}/api/cellar/layout`,{data:{revision:current.revision,name,room,width,depth,layout,racks}});assert.equal(response.status(),200,await response.text())}
 await browser.close()
}
