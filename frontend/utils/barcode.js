// EAN/UPC symbols consist of alternating runs measured in seven-module digits.
// Decode locally so barcode photos and live camera frames never leave the device.
const widths=[[3,2,1,1],[2,2,2,1],[2,1,2,2],[1,4,1,1],[1,1,3,2],[1,2,3,1],[1,1,1,4],[1,3,1,2],[1,2,1,3],[3,1,1,2]]
const parity=['LLLLLL','LLGLGG','LLGGLG','LLGGGL','LGLLGG','LGGLLG','LGGGLL','LGLGLG','LGLGGL','LGGLGL']
export function normalizeBarcode(value){const code=String(value).replace(/[\s-]/g,'');if(!/^(\d{8}|\d{12}|\d{13})$/.test(code))return '';let sum=0;for(let i=code.length-2,weight=3;i>=0;i--,weight=4-weight)sum+=Number(code[i])*weight;if((10-sum%10)%10!==Number(code.at(-1)))return '';return code.length===12?'0'+code:code}
function digit(runs,offset,allowG,unit){
 const values=runs.slice(offset,offset+4);if(values.length!==4)return null
 const total=values.reduce((s,r)=>s+r.width,0);if(total/7<unit*.55||total/7>unit*1.6)return null
 let best=null,error=1.2
 for(let n=0;n<(allowG?20:10);n++){const pattern=n<10?widths[n]:[...widths[n-10]].reverse();const score=values.reduce((sum,r,i)=>sum+Math.abs(r.width*7/total-pattern[i]),0);if(score<error){error=score;best={value:n%10,kind:n<10?'L':'G'}}}
 return best
}
const guard=(runs,offset,count,unit)=>runs.slice(offset,offset+count).length===count&&runs.slice(offset,offset+count).every(r=>Math.abs(r.width/unit-1)<.65)
function decodeRuns(runs,start,count,unit){
 let cursor=start+3,left='',kinds='',right=''
 for(let n=0;n<count;n++){const d=digit(runs,cursor,count===6,unit);if(!d)return '';left+=d.value;kinds+=d.kind;cursor+=4}
 if(!guard(runs,cursor,5,unit))return '';cursor+=5
 for(let n=0;n<count;n++){const d=digit(runs,cursor,false,unit);if(!d)return '';right+=d.value;cursor+=4}
 if(!guard(runs,cursor,3,unit))return ''
 if(runs[cursor+3]&&runs[cursor+3].width<unit*3)return ''
 if(count===6){const first=parity.indexOf(kinds);if(first<0)return '';left=first+left}
 return normalizeBarcode(left+right)
}
function scanLine(values){
 const hist=new Uint32Array(256);let total=0;for(const v of values){hist[v]++;total+=v}
 let leftCount=0,leftSum=0,variance=-1,threshold=127
 for(let t=0;t<255;t++){leftCount+=hist[t];leftSum+=hist[t]*t;const rightCount=values.length-leftCount;if(!leftCount||!rightCount)continue;const delta=leftSum/leftCount-(total-leftSum)/rightCount,score=leftCount*rightCount*delta*delta;if(score>variance){variance=score;threshold=t}}
 if(variance<=0)return ''
 const runs=[];for(const value of values){const black=value<=threshold;const last=runs.at(-1);if(last&&last.black===black)last.width++;else runs.push({black,width:1})}
 for(let i=0;i<runs.length-42;i++){if(!runs[i].black)continue;const unit=(runs[i].width+runs[i+1].width+runs[i+2].width)/3;if(unit<.8||!guard(runs,i,3,unit)||(i>0&&runs[i-1].width<unit*3))continue;const result=decodeRuns(runs,i,6,unit)||decodeRuns(runs,i,4,unit);if(result)return result}
 return ''
}
export function decodeBarcodePixels({data,width,height}){
 if(!width||!height||data.length<width*height*4)return ''
 const votes=new Map()
 for(const vertical of [false,true]){const length=vertical?height:width,cross=vertical?width:height
  for(const slope of [0,-.12,.12])for(let fraction=.08;fraction<.95;fraction+=.04){
   const line=new Uint8Array(length)
   for(let i=0;i<length;i++){const j=Math.round(cross*fraction+(i-length/2)*slope);if(j<0||j>=cross){line[i]=255;continue};const index=(vertical?(i*width+j):(j*width+i))*4;line[i]=Math.round((data[index]*.299+data[index+1]*.587+data[index+2]*.114)*(data[index+3]/255)+255*(1-data[index+3]/255))}
   const code=scanLine(line)||scanLine(line.reverse());if(code){const count=(votes.get(code)||0)+1;votes.set(code,count);if(count>=2)return code}
  }
 }
 return ''
}
let detectorPromise
export async function detectBarcodeCanvas(canvas){
 if(!detectorPromise)detectorPromise=(async()=>{if(!globalThis.BarcodeDetector)return null;try{const supported=await globalThis.BarcodeDetector.getSupportedFormats();const formats=['ean_13','ean_8','upc_a'].filter(f=>supported.includes(f));return formats.length?new globalThis.BarcodeDetector({formats}):null}catch{return null}})()
 const detector=await detectorPromise
 if(detector){try{const codes=await detector.detect(canvas);for(const code of codes){const valid=normalizeBarcode(code.rawValue);if(valid)return valid}}catch{/* Local decoder covers browsers without a working native detector. */}}
 const context=canvas.getContext('2d',{willReadFrequently:true});return decodeBarcodePixels(context.getImageData(0,0,canvas.width,canvas.height))
}
