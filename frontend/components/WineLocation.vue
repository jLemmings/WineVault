<script setup>
import { Wine, Check } from 'lucide-vue-next'
const props=defineProps({bottle:{type:Object,required:true},racks:{type:Array,required:true},bottles:{type:Array,required:true},disabled:{type:Boolean,default:false}})
const emit=defineEmits(['select'])
function selectBottle(rack,slot){const candidate=at(rack,slot);if(!props.disabled&&sameWine(candidate))emit('select',candidate)}
const sameWine=b=>b&&b.name.trim().toLowerCase()===props.bottle.name.trim().toLowerCase()&&b.vintage===props.bottle.vintage&&b.type===props.bottle.type&&b.region.trim().toLowerCase()===props.bottle.region.trim().toLowerCase()
const matches=computed(()=>props.bottles.filter(sameWine))
const shelves=computed(()=>props.racks.filter(r=>r.id===props.bottle.rack||matches.value.some(b=>b.rack===r.id)))
const at=(rack,slot)=>props.bottles.find(b=>b.rack===rack&&b.slot===slot)
const address=(r,slot)=>`${String.fromCharCode(65+slot%r.columns)}${Math.floor(slot/r.columns)+1}`
const description=(r,slot)=>{const b=at(r.id,slot);return `${address(r,slot)}: ${b?b.name:'Empty'}${b?.id===props.bottle.id?', selected bottle':sameWine(b)?', same wine and vintage':''}`}
</script>

<template>
 <div class="wine-location">
  <h3>Find this wine</h3><p>{{matches.length}} {{matches.length===1?'bottle':'bottles'}} of this wine and vintage in your cellar.</p><p v-if="matches.length>1" class="location-instruction">Select the highlighted slot you took your bottle from, then mark it as enjoyed.</p>
  <section v-for="rack in shelves" :key="rack.id" class="location-shelf" :aria-label="`Rack ${rack.id} front view`">
   <header><strong>Rack {{rack.id}} · {{rack.name}}</strong><span>Front view · {{rack.rows}} × {{rack.columns}}</span></header>
   <div class="location-scroll"><div class="location-grid" :style="{'--columns':rack.columns}">
    <button type="button" v-for="n in rack.capacity" :key="n" class="location-slot" :class="{filled:at(rack.id,n-1),match:sameWine(at(rack.id,n-1)),selected:at(rack.id,n-1)?.id===bottle.id}" :aria-label="description(rack,n-1)" :disabled="disabled||!sameWine(at(rack.id,n-1))" :aria-pressed="at(rack.id,n-1)?.id===bottle.id" @click="selectBottle(rack.id,n-1)" :title="description(rack,n-1)">
     <Check v-if="at(rack.id,n-1)?.id===bottle.id" :size="18"/><Wine v-else-if="at(rack.id,n-1)" :size="18"/><span v-else class="location-empty">·</span><small>{{address(rack,n-1)}}</small>
    </button>
   </div></div>
  </section>
  <div class="location-legend"><span><i class="selected"></i>Selected bottle</span><span><i class="match"></i>Same wine & vintage</span><span><i></i>Other wine</span></div>
 </div>
</template>

<style scoped>
.wine-location{margin:24px 0;text-align:left}.wine-location h3{font-size:16px;margin-bottom:6px}.wine-location>p{font-size:12px}.location-shelf{margin-top:18px}.location-shelf header{display:flex;flex-wrap:wrap;gap:6px;justify-content:space-between;margin-bottom:10px;font-size:12px}.location-shelf header span{font-size:10px;color:#81776b}.location-scroll{overflow-x:auto;padding:4px}.location-grid{display:grid;grid-template-columns:repeat(var(--columns),minmax(44px,1fr));gap:7px;padding:10px;background:#eee5d8;border:1px solid #d6c5ac;border-radius:8px}.location-slot{min-height:54px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:3px;border:1px dashed #c4b39c;border-radius:6px;color:#948572;background:#f8f3ea}.location-slot small{font-size:10px}.location-slot.filled{background:#d7c9b7;border-style:solid;color:#655747}.location-slot.match{background:#f2dce2;color:#782b3e;border:2px solid #782b3e}.location-slot.selected{background:#782b3e;color:white;outline:2px solid #be9a59;outline-offset:2px}.location-empty{font-size:18px}.location-legend{display:flex;gap:12px;flex-wrap:wrap;margin-top:14px;font-size:10px}.location-legend span{display:flex;align-items:center;gap:5px}.location-legend i{width:12px;height:12px;background:#d7c9b7;border-radius:3px}.location-legend i.match{background:#f2dce2;border:2px solid #782b3e}.location-legend i.selected{background:#782b3e;outline:1px solid #be9a59}
.location-slot:disabled{opacity:1;cursor:default;filter:none}.location-slot.match:not(:disabled){cursor:pointer}.location-slot.match:focus-visible{outline:3px solid #be9a59;outline-offset:2px}.location-instruction{margin-top:8px!important;color:#782b3e}
</style>
