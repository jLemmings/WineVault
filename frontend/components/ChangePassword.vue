<script setup>
const form=reactive({currentPassword:'',newPassword:'',confirm:''}),busy=ref(false),error=ref('')
async function save(){
 if(busy.value)return;error.value=''
 if(form.newPassword!==form.confirm){error.value='The new passwords do not match.';return}
 busy.value=true
 try{await $fetch('/api/auth/password',{method:'POST',body:{currentPassword:form.currentPassword,newPassword:form.newPassword},retry:0});form.currentPassword='';form.newPassword='';form.confirm='';window.dispatchEvent(new Event('winevault-signed-out'))}
 catch(e){error.value=typeof e.data==='string'?e.data:'Could not change your password.'}finally{busy.value=false}
}
</script>
<template>
 <form class="password-form" @submit.prevent="save"><h3>Change password</h3><p>Changing your password signs you out on all devices. Sign in again with your new password.</p><fieldset :disabled="busy"><label>Current password<input v-model="form.currentPassword" type="password" autocomplete="current-password" required/></label><label>New password<input v-model="form.newPassword" type="password" autocomplete="new-password" minlength="12" maxlength="72" required/></label><small>At least 12 characters, up to 72 UTF-8 bytes.</small><label>Confirm new password<input v-model="form.confirm" type="password" autocomplete="new-password" required/></label><p v-if="error" class="scanner-error" role="alert">{{error}}</p><button class="primary" :disabled="busy">{{busy?'Changing password…':'Change password'}}</button></fieldset></form>
</template>
<style scoped>
.password-form{border-top:1px solid #e4d9ca;margin-top:24px;padding-top:24px;text-align:left}.password-form h3{font-size:17px}.password-form p,.password-form small{font-size:12px;color:#857b70;line-height:1.6;margin:8px 0}.password-form fieldset{border:0;padding:0;margin:0;min-width:0}
</style>
