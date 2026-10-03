<script setup>
import { priceText, purchasePayload, money } from '~/utils/purchases'
const props=defineProps({bottle:Object}),emit=defineEmits(['saved','busy'])
const editing=ref(false),busy=ref(false),error=ref(''),draft=reactive({})
function start(){Object.assign(draft,{price:priceText(props.bottle.priceMinor),currency:props.bottle.currency||'CHF',purchaseDate:props.bottle.purchaseDate||'',seller:props.bottle.seller||''});editing.value=true;error.value=''}
watch(()=>props.bottle.id,()=>editing.value=false)
async function save(){busy.value=true;emit('busy',true);error.value='';try{await $fetch(`/api/bottles/${props.bottle.id}/purchase`,{method:'PUT',body:{...purchasePayload(draft),revision:props.bottle.revision},retry:0});editing.value=false;emit('saved')}catch(e){error.value=typeof e.data==='string'?e.data:e.message||'Could not save purchase details.'}finally{busy.value=false;emit('busy',false)}}
</script>
<template><section class="bottle-purchase"><template v-if="!editing"><h3>Purchase</h3><p>{{bottle.priceMinor==null?'Price unknown':money(bottle.priceMinor,bottle.currency)}} · {{bottle.purchaseDate||'Date unknown'}}</p><p v-if="bottle.seller">Seller: {{bottle.seller}}</p><button class="editor-secondary" @click="start">Edit purchase details</button></template><form v-else @submit.prevent="save"><fieldset :disabled="busy"><PurchaseFields :draft="draft"/><p v-if="error" role="alert" class="scanner-error">{{error}}</p><button class="editor-secondary" type="button" @click="editing=false">Cancel</button><button class="primary">{{busy?'Saving…':'Save purchase'}}</button></fieldset></form></section></template>
<style scoped>.bottle-purchase{text-align:left;margin:20px 0}.bottle-purchase h3{font-size:16px}.bottle-purchase p{font-size:12px;margin:10px 0;color:#857b70}fieldset{border:0;padding:0;min-width:0}</style>
