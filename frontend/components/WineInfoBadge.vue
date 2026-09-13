<script setup>
import { Check, RefreshCw, AlertCircle, Search } from 'lucide-vue-next'
const props=defineProps({bottle:{type:Object,required:true}})
const badge=computed(()=>{
 const status=props.bottle.informationStatus
 if(props.bottle.hasInformation)return {kind:'loaded',icon:Check,label:'API details loaded',hint:status==='ready'?'GrapeMinds details are saved and available offline.':'Saved details are available; the latest refresh has not completed successfully.'}
 if(status==='fetching')return {kind:'pending',icon:RefreshCw,label:'Fetching API details',hint:'Wine information is being fetched.'}
 if(status==='needs_match')return {kind:'missing',icon:Search,label:'Choose API match',hint:'Open this wine and select a catalogue match to load details.'}
 if(['failed','rate_limited','not_found','not_configured'].includes(status))return {kind:'missing',icon:AlertCircle,label:'API details missing · Retry',hint:'Open this wine to fetch or retry its GrapeMinds details.'}
 return {kind:'missing',icon:AlertCircle,label:'API details not loaded',hint:'Open this wine and choose Fetch information.'}
})
</script>
<template><span class="wine-info-badge" :class="badge.kind" :title="badge.hint"><component :is="badge.icon" :size="12" aria-hidden="true"/><span>{{badge.label}}</span></span></template>
<style scoped>
.wine-info-badge{display:inline-flex!important;align-items:center;gap:5px!important;width:fit-content;margin-top:7px;padding:4px 7px;border-radius:4px;font-size:10px!important;font-weight:500;line-height:1.3;white-space:normal}.loaded{color:#286447;background:#e7f2eb;border:1px solid #c2dfce}.missing{color:#895811;background:#fff4dc;border:1px solid #eed6a5}.pending{color:#526c87;background:#edf2f8;border:1px solid #cfdce9}@media(max-width:680px){.wine-info-badge{font-size:9px!important;padding:3px 5px;gap:3px!important}}
</style>
