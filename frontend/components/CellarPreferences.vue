<script setup>
const props=defineProps({cellar:Object}),emit=defineEmits(['saved','busy'])
const viewMode=ref(props.cellar.preferences?.viewMode||'floor-plan'),busy=ref(false),error=ref('')
async function save(){busy.value=true;emit('busy',true);error.value='';try{await $fetch('/api/cellar/preferences',{method:'PUT',body:{revision:props.cellar.revision,viewMode:viewMode.value,typeRacks:{...props.cellar.preferences?.typeRacks}},retry:0});emit('saved')}catch(e){error.value=typeof e.data==='string'?e.data:'Could not save preferences. Please try again.'}finally{busy.value=false;emit('busy',false)}}
</script>
<template><form class="cellar-preferences" @submit.prevent="save"><fieldset :disabled="busy"><legend>Cellar preferences</legend><label>Cellar view<select v-model="viewMode" aria-label="Cellar view"><option value="floor-plan">Floor plan and rack view</option><option value="racks-only">Rack view only</option></select></label><p v-if="error" class="scanner-error" role="alert">{{error}}</p><button class="primary">{{busy?'Saving…':'Save preferences'}}</button></fieldset></form></template>
<style scoped>
.cellar-preferences{text-align:left;margin:24px 0;padding-bottom:20px;border-bottom:1px solid #e8e5df}.cellar-preferences fieldset{padding:0;border:0;min-width:0}.cellar-preferences legend{font-size:17px;font-weight:600;margin-bottom:14px}.cellar-preferences p{font-size:12px;line-height:1.6;margin:10px 0}
</style>
