<script setup>
const props=defineProps({label:String,busy:Boolean,wide:Boolean}),emit=defineEmits(['close'])
const root=ref(null)
useEditorDialog(root,()=>{if(!props.busy)emit('close')})
</script>
<template><div class="modal-backdrop" @click.self="!busy&&emit('close')"><section ref="root" class="modal" :class="{'wine-detail-modal':wide}" role="dialog" aria-modal="true" :aria-label="label" tabindex="-1"><slot/></section></div></template>
