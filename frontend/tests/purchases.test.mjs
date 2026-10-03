import test from 'node:test'
import assert from 'node:assert/strict'
import { priceMinor, priceText, purchaseSummary } from '../utils/purchases.js'
test('exact cents, zero, missing and invalid prices',()=>{
 for(const [input,expected] of [['12.35',1235],['0',0],['0.01',1],['12.3',1230],['',null]])assert.equal(priceMinor(input),expected)
 for(const input of ['-1','1.234','NaN','1e3'])assert.throws(()=>priceMinor(input))
 assert.equal(priceText(1235),'12.35');assert.equal(priceText(null),'')
})
test('spending distinguishes currencies, current cost, undated spending, and zero prices',()=>{
 const result=purchaseSummary([{priceMinor:1235,currency:'CHF',purchaseDate:'2026-01-02',inCellar:true},{priceMinor:100,currency:'CHF',purchaseDate:'',inCellar:false},{priceMinor:900,currency:'EUR',purchaseDate:'2026-01-02',inCellar:false},{priceMinor:0,currency:'CHF',purchaseDate:'2026-02-02',inCellar:true},{priceMinor:null,currency:'',purchaseDate:'2026-02-02',inCellar:true}])
 assert.deepEqual(result.totals,[{currency:'CHF',total:1335,cellar:1235,bottles:3,undated:100},{currency:'EUR',total:900,cellar:0,bottles:1,undated:0}])
 assert.equal(result.months.length,3);assert.equal(result.months[0].month,'2026-02');assert.equal(result.months[0].amount,0)
})
