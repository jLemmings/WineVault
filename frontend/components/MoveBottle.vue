<script setup>
import { ArrowRight, Check } from 'lucide-vue-next'
const props=defineProps({bottle:{type:Object,required:true},cellar:{type:Object,required:true}}),emit=defineEmits(['moved','busy'])
const open=ref(false),busy=ref(false),error=ref(''),inventory=ref(props.cellar),rack=ref(props.bottle.rack),slot=ref(null)
const selectedRack=computed(()=>inventory.value.racks.find(r=>r.id===rack.value))
const at=n=>inventory.value.bottles.find(b=>b.rack===rack.value&&b.slot===n)
const label=n=>`${String.fromCharCode(65+n%(selectedRack.value?.columns||6))}${Math.floor(n/(selectedRack.value?.columns||6))+1}`
watch(()=>props.bottle.id,()=>{open.value=false;rack.value=props.bottle.rack;slot.value=null;error.value=''})
watch(rack,()=>{slot.value=null})
function start(){inventory.value=props.cellar;rack.value=props.bottle.rack;slot.value=null;error.value='';open.value=true}
async function move(){
 if(busy.value||slot.value===null)return
 busy.value=true;emit('busy',true);error.value=''
 try{const bottle=await $fetch(`/api/bottles/${encodeURIComponent(props.bottle.id)}/location`,{method:'PUT',body:{rack:rack.value,slot:slot.value},retry:0});open.value=false;emit('moved',bottle)}
 catch(e){error.value=typeof e.data==='string'?e.data:'Could not move this bottle. Please try again.';if(e.status===409||e.statusCode===409){try{inventory.value=await $fetch('/api/cellar');slot.value=null}catch{}}}
 finally{busy.value=false;emit('busy',false)}
}
</script>
<template>
 <div class="move-bottle"><button v-if="!open" class="primary" @click="start">Move bottle<ArrowRight :size="16"/></button>
 <form v-else @submit.prevent="move"><h3>Move this bottle</h3><p>Choose an empty slot in this rack or another rack.</p><fieldset :disabled="busy"><label>Destination rack<select v-model="rack" required><option v-for="r in inventory.racks" :key="r.id" :value="r.id">{{r.id}} · {{r.name}}</option></select></label><div class="scanner-slots-scroll"><div class="scanner-slots" :style="{'--slot-columns':selectedRack?.columns||6}" role="group" aria-label="Destination slot"><button v-for="n in selectedRack?.capacity||0" :key="n" type="button" :disabled="!!at(n-1)" :class="{occupied:at(n-1),chosen:slot===n-1}" :aria-label="label(n-1)+(at(n-1)?.id===bottle.id?' current location':at(n-1)?' occupied':' empty')" :aria-pressed="slot===n-1" @click="slot=n-1">{{label(n-1)}}<Check v-if="slot===n-1" :size="13"/></button></div></div><p v-if="slot!==null" aria-live="polite">Move to Rack {{rack}} · Slot {{label(slot)}}</p><p v-if="error" class="scanner-error" role="alert">{{error}}</p><div class="move-actions"><button type="button" class="editor-secondary" @click="open=false">Cancel move</button><button class="primary" :disabled="slot===null||busy">{{busy?'Moving…':'Confirm move'}}<ArrowRight :size="16"/></button></div></fieldset></form></div>
</template>
<style scoped>
.move-bottle{margin:18px 0;text-align:left}.move-bottle fieldset{padding:0;margin:0;border:0;min-width:0}.move-bottle h3{font-size:17px}.move-bottle p{font-size:12px;color:#857b70;margin:10px 0;line-height:1.6}.move-actions{display:flex;gap:12px;align-items:center}.move-actions .primary{margin-top:0;flex:1}
</style>
