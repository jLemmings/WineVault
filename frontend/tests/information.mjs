import { chromium } from '@playwright/test'
import assert from 'node:assert/strict'
const browser=await chromium.launch({channel:'msedge',headless:true})
const page=await browser.newPage()
const errors=[];page.on('pageerror',e=>errors.push(e.message))
let state={status:'not_fetched',data:null,message:''},fetches=0
const rack={id:'A',name:'Shelf',short:'Shelf',wall:'North',rows:2,columns:3,capacity:6,temp:12,x:0,y:0,width:1,depth:1,rotation:0,color:'red'}
const wine={name:'Estate Reserve',vintage:2020,region:'France',type:'Red',rack:'A'}
await page.route('**/api/**',route=>{
 const r=route.request(),path=new URL(r.url()).pathname
 if(path==='/api/auth/status')return route.fulfill({json:{authenticated:true,setupRequired:false,username:'Owner'}})
 if(path==='/api/cellar')return route.fulfill({json:{id:'test',name:'Cellar',owner:'Owner',room:'Room',width:5,depth:4,racks:[rack],bottles:[{...wine,id:'1',slot:0},{...wine,id:'2',slot:1}],layout:{shape:'rectangle',floor:'stone',doorWall:'south',doorOffset:0,doorWidth:.8,tableEnabled:false}}})
 if(path.endsWith('/information')){
  if(r.method()==='POST'){
   fetches++
   if(fetches===1)state={status:'needs_match',data:null,message:'Choose the correct wine.',candidates:[{id:42,display_name:'Estate Reserve Special',color:'red'}]}
   else if(fetches===2){assert.equal(r.postDataJSON().sourceId,42);state={status:'ready',sourceId:42,fetchedAt:'2026-09-13T12:00:00Z',data:{display_name:'Estate Reserve Special',description:{text:'Rich dark fruit. Serve at 14°C.'},grapes:[{name:'Merlot'}]},message:''}}
   else state={...state,status:'failed',message:'GrapeMinds is unavailable. Please retry later.'}
  }
  return route.fulfill({json:state})
 }
 throw new Error(`Unexpected API request: ${path}`)
})
try{
 await page.goto(process.env.APP_URL||'http://localhost:3000')
 await page.getByRole('button',{name:'Wine collection',exact:false}).first().click()
 await page.locator('.wine-table>button').first().click()
 const info=page.getByRole('region',{name:'Wine information',exact:true})
 await info.getByRole('button',{name:'Fetch information',exact:true}).click()
 await info.getByRole('button',{name:'Use this wine',exact:true}).click()
 await info.getByText('Merlot',{exact:true}).waitFor()
 assert.equal(fetches,2)
 assert.ok(!(await info.innerText()).includes('14°C'))
 await info.getByRole('button',{name:'Reload from GrapeMinds',exact:true}).click()
 await info.getByText('GrapeMinds is unavailable. Please retry later.').waitFor()
 assert.ok((await info.innerText()).includes('Merlot'),'Failed refresh must retain cached details')
 await page.getByRole('button',{name:'Close dialog',exact:true}).click()
 await page.locator('.wine-table>button').nth(1).click()
 await info.getByText('Merlot',{exact:true}).waitFor()
 assert.equal(fetches,3,'Opening cached information must not fetch again')
 await page.setViewportSize({width:390,height:844})
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
 assert.deepEqual(errors,[])
 console.log('PASS missing information, cached match selection, fetched details, failed-refresh retention, zero-request reopening, mobile. API mocked.')
}finally{await browser.close()}
