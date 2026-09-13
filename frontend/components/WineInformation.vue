<script setup>
import { RefreshCw, Search, LoaderCircle, MapPin, Grape, Sprout, Wine, Utensils, BookOpen, Sparkles, Check, ChevronDown } from 'lucide-vue-next'
const props=defineProps({bottle:{type:Object,required:true}})
const emit=defineEmits(['updated'])
const info=ref(null),loading=ref(false),busy=ref(false),error=ref(''),query=ref(''),candidates=ref(null),matching=ref(false)
let generation=0,timer=null,controller=null
// Display provider text as plain text and omit temperature advice, per the app's preferences.
const text=value=>{const raw=typeof value==='string'?value:value?.text||'';return typeof raw==='string'?raw.split(/(?<=[.!?])\s+|\n/).filter(line=>!(/temperature|°|celsius|fahrenheit|degrees/i.test(line))).join(' ').trim():''}
const data=computed(()=>info.value?.data)
const grapes=computed(()=>Array.isArray(data.value?.grapes)?data.value.grapes.map(g=>text(g.name)).filter(Boolean):[])
const paragraphs=computed(()=>[{label:'The story',icon:BookOpen,value:text(data.value?.description)},{label:'Tasting notes',icon:Sparkles,value:text(data.value?.tasting_notes)},{label:'Food pairings',icon:Utensils,value:text(data.value?.pairing)}].filter(item=>item.value))
const statusMessage=computed(()=>info.value?.message||({not_fetched:'Wine information has not been fetched yet.',fetching:'Fetching wine information…',ready:'Information saved from GrapeMinds.'})[info.value?.status]||'')
const preview=value=>value.length>110?value.slice(0,107).replace(/\s+\S*$/,'')+'?':value
function stop(){clearTimeout(timer);controller?.abort()}
async function load(){
 stop();const current=++generation;controller=new AbortController();const signal=controller.signal;info.value=null;error.value='';loading.value=true;busy.value=false;matching.value=false;candidates.value=null;query.value=props.bottle.name
 try{const result=await $fetch(`/api/bottles/${props.bottle.id}/information`,{signal,retry:0});if(current!==generation)return;info.value=result;candidates.value=result.candidates;poll(current)}
 catch(e){if(current===generation&&!signal.aborted)error.value='Could not load saved wine information.'}
 finally{if(current===generation)loading.value=false}
}
function poll(current){
 if(info.value?.status!=='fetching')return
 timer=setTimeout(async()=>{try{const result=await $fetch(`/api/bottles/${props.bottle.id}/information`,{signal:controller.signal,retry:0});if(current===generation){info.value=result;if(result.status!=='fetching')emit('updated');poll(current)}}catch{if(current===generation)error.value='Could not check the fetch status. Reload to try again.'}},2000)
}
async function fetchInformation(sourceId=0){
 if(busy.value)return;busy.value=true;error.value='';clearTimeout(timer);const current=generation
 try{const result=await $fetch(`/api/bottles/${props.bottle.id}/information`,{method:'POST',body:{sourceId},signal:controller.signal,retry:0,timeout:25000});if(current!==generation)return;info.value=result;emit('updated');candidates.value=result.candidates;if(result.status==='ready'){matching.value=false;candidates.value=null}poll(current)}
 catch(e){if(current===generation)error.value=typeof e.data==='string'?e.data:'Could not fetch wine information. Your bottle is still saved; try again later.'}
 finally{if(current===generation)busy.value=false}
}
async function search(){
 if(busy.value)return;busy.value=true;error.value='';candidates.value=null;const current=generation
 try{const result=await $fetch(`/api/bottles/${props.bottle.id}/information/search`,{method:'POST',body:{query:query.value},signal:controller.signal,retry:0,timeout:15000});if(current===generation)candidates.value=result.candidates}
 catch(e){if(current===generation)error.value=typeof e.data==='string'?e.data:'Could not search GrapeMinds.'}
 finally{if(current===generation)busy.value=false}
}
watch(()=>props.bottle.id,load,{immediate:true})
onBeforeUnmount(()=>{generation++;stop()})
</script>

<template>
 <section class="wine-information" aria-label="Wine information">
  <header><h3>Wine at a glance</h3><span v-if="data" class="information-saved"><Check :size="12"/>Saved details</span></header>
  <p v-if="loading" role="status">Loading saved information…</p>
  <template v-else>
   <p v-if="statusMessage && info?.status!=='ready'" class="information-status" role="status">{{statusMessage}}</p>
   <template v-if="data">
    <div class="wine-overview">
     <div class="wine-overview-icon"><Wine :size="30" :stroke-width="1.4"/></div><div><span class="wine-overview-label">{{text(data.color)||bottle.type}}<template v-if="text(data.sub_type)"> ? {{text(data.sub_type)}}</template></span><h4>{{text(data.display_name)}}</h4><span v-if="text(data.residual_sugar)" class="wine-style">{{text(data.residual_sugar)}}</span></div>
    </div>
    <div class="wine-facts">
     <div v-if="text(data.producer?.display_name||data.producer?.name)" class="wine-fact"><Sprout :size="19"/><div><span>Producer</span><strong>{{text(data.producer.display_name||data.producer.name)}}</strong></div></div>
     <div v-if="text(data.region?.name)" class="wine-fact"><MapPin :size="19"/><div><span>Origin</span><strong>{{text(data.region.name)}}</strong><small v-if="data.region.country">{{text(data.region.country)}}</small></div></div>
    </div>
    <div v-if="grapes.length" class="wine-grapes"><span class="grape-label"><Grape :size="16"/>Grapes</span><div><span v-for="grape in grapes" :key="grape" class="grape-chip">{{grape}}</span></div></div>
    <div class="wine-notes"><details v-for="part in paragraphs" :key="part.label" class="wine-note"><summary><span class="note-icon"><component :is="part.icon" :size="18"/></span><span class="note-heading"><strong>{{part.label}}</strong><span class="note-preview">{{preview(part.value)}}</span></span><ChevronDown class="note-chevron" :size="15"/></summary><p>{{part.value}}</p></details></div>
    <div class="information-source"><a href="https://grapeminds.eu" target="_blank" rel="noopener noreferrer">GrapeMinds</a><span v-if="info.fetchedAt">Updated {{new Date(info.fetchedAt).toLocaleDateString()}}</span><span title="Catalogue information may not describe your specific vintage.">Wine-level details</span></div>
   </template>
   <p v-if="error" class="scanner-error" role="alert">{{error}}</p>
   
   <div class="information-actions"><button v-if="!info" class="editor-secondary" @click="load">Reload information</button><button v-else class="editor-secondary" :aria-label="data?'Reload from GrapeMinds':undefined" title="Fetches fresh data using your GrapeMinds API allowance. Saved details are free to reopen." :disabled="busy||info.status==='fetching'" @click="fetchInformation()"><LoaderCircle v-if="busy" :size="15" class="spinning"/><RefreshCw v-else :size="15"/>{{busy?'Fetching…':data?'Reload from GrapeMinds':info.status==='not_fetched'?'Fetch information':'Retry fetch'}}</button><button class="text-button" :disabled="busy" @click="matching=!matching">{{data?'Change match':'Find a matching wine'}}</button></div>
   <form v-if="matching||info?.status==='needs_match'||info?.status==='not_found'" class="information-search" @submit.prevent="search"><label>Producer and wine name<input v-model="query" minlength="3" maxlength="200" required :disabled="busy"/></label><button class="editor-secondary" :disabled="busy"><Search :size="15"/>Search GrapeMinds</button><p v-if="candidates?.length===0">No matches found. Try a different producer or wine name.</p><ul v-if="candidates?.length"><li v-for="candidate in candidates" :key="candidate.id"><div><strong>{{text(candidate.display_name)}}</strong><small>{{text(candidate.producer_display_name)}} {{text(candidate.color)}}</small></div><button type="button" class="editor-secondary" :disabled="busy" @click="fetchInformation(candidate.id)">Use this wine</button></li></ul></form>
  </template>
 </section>
</template>

<style scoped>
.wine-information{border-top:1px solid #e4d9ca;margin:24px 0;padding-top:20px;text-align:left}.wine-information header{display:flex;align-items:center;justify-content:space-between;gap:10px}.wine-information h3{font-size:17px}.wine-information header a{font-size:11px;color:#782b3e;text-decoration:underline}.wine-information p{font-size:12px;line-height:1.7;margin:10px 0;color:#70665c}.wine-information h4{font-size:13px;margin-top:14px}.wine-information .catalogue-note{font-size:10px;color:#928575}.wine-information dl{display:grid;grid-template-columns:70px 1fr;gap:8px;font-size:12px;margin:14px 0}.wine-information dt{color:#928575}.wine-information dd{margin:0}.information-actions{display:flex;flex-wrap:wrap;gap:12px;margin:15px 0}.information-search{background:#f8f4ed;border-radius:8px;padding:14px}.information-search ul{list-style:none;margin:12px 0 0;padding:0;max-height:260px;overflow:auto}.information-search li{display:flex;align-items:center;gap:12px;padding:10px 0;border-top:1px solid #e4d9ca}.information-search li>div{flex:1;min-width:0}.information-search strong{font-size:12px;overflow-wrap:anywhere}.information-search small{display:block;font-size:10px;color:#928575;margin-top:5px}.information-search li button{flex-shrink:0;font-size:10px}.information-paragraph p{white-space:pre-line}

.wine-information h3{font-family:'Playfair Display',Georgia,serif;font-size:21px;font-weight:500}.information-saved{display:inline-flex;align-items:center;gap:4px;color:#397765;font-size:10px;background:#edf5ee;padding:5px 7px;border-radius:20px}.wine-overview{display:flex;align-items:center;gap:14px;margin:18px 0;padding:17px;background:linear-gradient(115deg,#f4e8ed,#faf5ef);border:1px solid #ebdce0;border-radius:10px}.wine-overview-icon{display:grid;place-items:center;color:#782b3e;flex-shrink:0;width:49px;height:62px;border-radius:25px 25px 10px 10px;background:#fffaf7}.wine-overview-label{text-transform:uppercase;letter-spacing:1.2px;font-size:9px;color:#9a6371}.wine-overview h4{margin:5px 0;font-size:17px;font-family:Georgia,serif;font-weight:500;line-height:1.35}.wine-style{font-size:10px;color:#782b3e}.wine-facts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}.wine-fact{display:flex;align-items:flex-start;gap:10px;padding:13px;border:1px solid #ece5da;border-radius:8px;background:#fffdf9}.wine-fact>svg{color:#a08a65;margin-top:3px}.wine-fact>div{min-width:0}.wine-fact span{display:block;font-size:9px;text-transform:uppercase;letter-spacing:.8px;color:#9b8c7a}.wine-fact strong{display:block;font-size:12px;font-weight:500;line-height:1.5;margin-top:3px;overflow-wrap:anywhere}.wine-fact small{font-size:10px;color:#9b8c7a}.wine-grapes{padding:15px 0;display:flex;align-items:flex-start;gap:12px}.grape-label{display:flex;align-items:center;gap:5px;color:#8d7d67;font-size:10px;padding-top:5px}.wine-grapes>div{display:flex;gap:6px;flex-wrap:wrap}.grape-chip{background:#f1ece2;color:#736045;border:1px solid #e7ddcd;padding:5px 9px;border-radius:20px;font-size:10px}.wine-notes{display:grid;gap:8px;margin-top:8px}.wine-note{border:1px solid #e9e2d8;border-radius:8px;background:#fcfaf6}.wine-note summary{display:flex;align-items:center;gap:11px;cursor:pointer;list-style:none;padding:13px}.wine-note summary::-webkit-details-marker{display:none}.note-icon{width:32px;height:32px;display:grid;place-items:center;flex-shrink:0;color:#782b3e;background:#f2e8e9;border-radius:8px}.note-heading{flex:1;min-width:0}.note-heading strong{font-size:12px;font-weight:500}.note-preview{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;font-size:11px;color:#978a7b;line-height:1.5;margin-top:3px}.wine-note[open] .note-preview{display:none}.wine-note[open] .note-chevron{transform:rotate(180deg)}.wine-note>p{padding:0 15px 12px 56px;margin:0;line-height:1.7}.note-chevron{color:#aa9983}.wine-note summary:focus-visible{outline:2px solid #be9a59;outline-offset:2px;border-radius:8px}.information-source{display:flex;flex-wrap:wrap;gap:6px 12px;font-size:9px;color:#a39787;margin-top:15px}.information-source a{color:#967589;text-decoration:underline}.information-actions{padding-top:12px;border-top:1px solid #eee7dd;margin-top:12px}.information-actions button{font-size:10px}.wine-overview>div:last-child{min-width:0;overflow-wrap:anywhere}@media(max-width:420px){.wine-facts{gap:7px}.wine-fact{padding:10px;gap:7px}.wine-overview{padding:12px}.wine-overview h4{font-size:15px}.wine-information h3{font-size:19px}.wine-note>p{padding-left:15px}}
</style>
