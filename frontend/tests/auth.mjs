import { chromium } from '@playwright/test'
import assert from 'node:assert/strict'

const browser=await chromium.launch({channel:'msedge',headless:true})
const page=await browser.newPage()
const errors=[];page.on('pageerror',e=>errors.push(e.message))
let setup=true,signedIn=false,changed=false,privateReads=0
const cellar={id:'test',name:'Test cellar',owner:'owner',room:'Room',width:5,depth:4,revision:1,racks:[],bottles:[],layout:{shape:'rectangle',floor:'stone',doorWall:'south',doorOffset:0,doorWidth:.8,tableEnabled:false}}
await page.route('**/api/**',async route=>{
 const request=route.request(),path=new URL(request.url()).pathname
 if(request.method()==='POST')assert.equal(request.headers()['x-winevault-request'],'1','CSRF header missing')
 if(path==='/api/auth/status')return route.fulfill({json:{setupRequired:setup,authenticated:signedIn,username:signedIn?'owner':''}})
 if(path==='/api/auth/setup'){assert.equal(request.postDataJSON().setupCode,'test-setup-code');setup=false;signedIn=true;return route.fulfill({status:201,json:{username:'owner'}})}
 if(path==='/api/auth/login'){
  if(request.postDataJSON().password!==(changed?'replacement-password-123':'initial-password-123'))return route.fulfill({status:401,body:'Incorrect username or password.'})
  signedIn=true;return route.fulfill({json:{username:'owner'}})
 }
 if(path==='/api/auth/password'){assert.equal(request.postDataJSON().currentPassword,'initial-password-123');assert.equal(request.postDataJSON().newPassword,'replacement-password-123');changed=true;signedIn=false;return route.fulfill({status:204})}
 if(path==='/api/auth/logout'){signedIn=false;return route.fulfill({status:204})}
 if(path==='/api/cellar'){privateReads++;assert.ok(signedIn,'Private data fetched before login');return route.fulfill({json:cellar})}
 return route.fulfill({status:404})
})
try{
 await page.goto(process.env.APP_URL||'http://localhost:3000')
 await page.getByRole('heading',{name:'Set up your cellar'}).waitFor()
 assert.equal(privateReads,0)
 await page.getByLabel('Setup code').fill('test-setup-code')
 await page.getByLabel('Username',{exact:true}).fill('owner')
 await page.getByLabel('Password',{exact:true}).fill('initial-password-123')
 await page.getByLabel('Confirm password',{exact:true}).fill('initial-password-123')
 await page.getByRole('button',{name:'Create owner account',exact:true}).click()
 await page.getByRole('button',{name:'Preferences',exact:true}).waitFor()
 await page.reload()
 await page.getByRole('button',{name:'Preferences',exact:true}).click()
 await page.getByLabel('Current password').fill('initial-password-123')
 await page.getByLabel('New password',{exact:true}).fill('replacement-password-123')
 await page.getByLabel('Confirm new password').fill('replacement-password-123')
 await page.getByRole('button',{name:'Change password',exact:true}).click()
 await page.getByRole('heading',{name:'Welcome back'}).waitFor()
 assert.equal(await page.locator('.app-shell').count(),0)
 await page.getByLabel('Username',{exact:true}).fill('owner')
 await page.getByLabel('Password',{exact:true}).fill('initial-password-123')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await page.getByRole('alert').waitFor()
 await page.getByLabel('Password',{exact:true}).fill('replacement-password-123')
 await page.getByRole('button',{name:'Sign in',exact:true}).click()
 await page.getByRole('button',{name:'Sign out',exact:true}).click()
 await page.getByRole('heading',{name:'Welcome back'}).waitFor()
 await page.setViewportSize({width:390,height:844})
 assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false)
 assert.deepEqual(errors,[])
 console.log('PASS setup, login errors, password change, logout, CSRF headers, private-data gate, mobile. Auth API mocked; no real account created.')
}finally{await browser.close()}
