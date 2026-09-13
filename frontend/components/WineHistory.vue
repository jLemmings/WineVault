<script setup>
import { Plus, Wine, History } from 'lucide-vue-next'
const entries=ref([]),filter=ref(''),cursor=ref(''),loading=ref(false),error=ref('')
let version=0
async function load(more=false){
 const current=++version;loading.value=true;error.value=''
 if(!more){entries.value=[];cursor.value=''}
 try{const data=await $fetch('/api/history',{query:{action:filter.value,...(more?{before:cursor.value}:{})}});if(current!==version)return;entries.value=more?[...entries.value,...data.entries]:data.entries;cursor.value=data.nextCursor}
 catch{if(current===version)error.value='Could not load wine history. Check the backend connection and try again.'}
 finally{if(current===version)loading.value=false}
}
watch(filter,()=>load())
onMounted(()=>load())
onBeforeUnmount(()=>version++)
const date=value=>new Date(value).toLocaleString(undefined,{dateStyle:'medium',timeStyle:'short'})
const slot=e=>`${String.fromCharCode(65+e.slot%e.columns)}${Math.floor(e.slot/e.columns)+1}`
</script>

<template>
 <section class="panel history-panel" aria-label="Wine history">
  <header><div><h2>Your cellar over time</h2><p>A record of each bottle added and enjoyed. History starts when tracking is enabled; earlier activity is unavailable.</p></div><History :size="24"/></header>
  <div class="history-filters" role="group" aria-label="Filter history"><button v-for="tab in [{value:'',label:'All activity'},{value:'added',label:'Added'},{value:'enjoyed',label:'Enjoyed'}]" :key="tab.value" :aria-pressed="filter===tab.value" :class="{active:filter===tab.value}" @click="filter=tab.value">{{tab.label}}</button></div>
  <ol class="history-list"><li v-for="entry in entries" :key="entry.id"><span class="history-icon" :class="entry.action"><Plus v-if="entry.action==='added'" :size="19"/><Wine v-else :size="19"/></span><div><span class="history-action">{{entry.action==='added'?'Added to cellar':'Enjoyed'}}</span><h3>{{entry.name}}</h3><p>{{entry.vintage || 'NV'}} · {{entry.region}} · {{entry.type}}</p><p>Rack {{entry.rack}} · {{entry.rackName}} · Slot {{slot(entry)}}</p></div><time :datetime="entry.occurredAt">{{date(entry.occurredAt)}}</time></li></ol>
  <p v-if="error" class="scanner-error" role="alert">{{error}} <button @click="load(entries.length>0)">Try again</button></p>
  <p v-else-if="loading" class="empty" role="status">Loading history…</p>
  <p v-else-if="!entries.length" class="empty">{{filter==='enjoyed'?'No wines marked as enjoyed yet.':filter==='added'?'No additions recorded yet.':'Your next addition or enjoyed bottle will appear here.'}}</p>
  <button v-if="cursor&&!loading&&!error" class="history-more" @click="load(true)">Load older activity</button>
 </section>
</template>

<style scoped>
.history-panel{padding:28px}.history-panel header{display:flex;justify-content:space-between;gap:20px}.history-panel h2{font-size:24px}.history-panel header p{color:#857b70;font-size:12px;line-height:1.6;margin-top:8px;max-width:600px}.history-filters{display:flex;gap:8px;margin:24px 0}.history-filters button{padding:9px 15px;border-radius:6px;border:1px solid #e8e0d6}.history-filters button.active{background:#782b3e;color:white;border-color:#782b3e}.history-list{list-style:none;padding:0;margin:0}.history-list li{display:flex;gap:16px;padding:22px 0;border-top:1px solid #eee6dc}.history-icon{display:flex;align-items:center;justify-content:center;flex-shrink:0;width:38px;height:38px;border-radius:50%;background:#f3e4e9;color:#782b3e}.history-icon.added{background:#e6eee7;color:#397765}.history-action{font-size:10px;text-transform:uppercase;letter-spacing:1px;color:#81786e}.history-list h3{font-size:15px;margin:5px 0}.history-list p{color:#857b70;font-size:12px;line-height:1.7}.history-list time{margin-left:auto;color:#81786e;font-size:11px;white-space:nowrap}.history-more{display:block;margin:20px auto 0;padding:12px 20px;border:1px solid #d9c9b7;border-radius:6px}@media(max-width:600px){.history-panel{padding:20px}.history-list li{flex-wrap:wrap;gap:10px}.history-list li>div{flex:1;min-width:0}.history-list time{width:100%;padding-left:48px}.history-list h3{overflow-wrap:anywhere}}
</style>
