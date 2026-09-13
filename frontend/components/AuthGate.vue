<script setup>
import { Wine, LockKeyhole } from 'lucide-vue-next'
const emit=defineEmits(['authenticated','signed-out'])
const checking=ref(true),authenticated=ref(false),setup=ref(false),busy=ref(false),error=ref('')
const form=reactive({username:'',password:'',confirm:'',setupCode:''})
async function check(){checking.value=true;error.value='';try{const state=await $fetch('/api/auth/status');setup.value=state.setupRequired;authenticated.value=state.authenticated;if(state.authenticated)emit('authenticated')}catch{error.value='Could not connect to WineVault. Check that the backend is running.'}finally{checking.value=false}}
function signedOut(){authenticated.value=false;form.password='';form.confirm='';emit('signed-out');check()}
onMounted(()=>{window.addEventListener('winevault-signed-out',signedOut);check()})
onBeforeUnmount(()=>window.removeEventListener('winevault-signed-out',signedOut))
async function submit(){
 if(busy.value)return
 error.value=''
 if(setup.value&&form.password!==form.confirm){error.value='The passwords do not match.';return}
 busy.value=true
 try{await $fetch(setup.value?'/api/auth/setup':'/api/auth/login',{method:'POST',body:{username:form.username,password:form.password,...(setup.value?{setupCode:form.setupCode.trim()}:{})},retry:0});form.password='';form.confirm='';form.setupCode='';setup.value=false;authenticated.value=true;emit('authenticated')}
 catch(e){error.value=typeof e.data==='string'?e.data:'Could not sign in. Please try again.'}
 finally{busy.value=false}
}
</script>

<template>
 <slot v-if="authenticated"/>
 <main v-else class="auth-screen"><section class="auth-card"><div class="auth-brand"><Wine :size="30"/>WineVault</div><template v-if="checking"><p role="status">Checking your account…</p></template><template v-else><LockKeyhole :size="24"/><h1>{{setup?'Set up your cellar':'Welcome back'}}</h1><p>{{setup?'Create the single owner account for this WineVault installation.':'Sign in to your personal wine cellar.'}}</p><form @submit.prevent="submit"><fieldset :disabled="busy"><label v-if="setup">Setup code<input v-model="form.setupCode" required autocomplete="off" placeholder="Code from the backend console"/><small>Shown once on backend startup until setup is complete.</small></label><label>Username<input v-model="form.username" required maxlength="80" autocomplete="username"/></label><label>Password<input v-model="form.password" required type="password" :minlength="setup?12:undefined" :maxlength="setup?72:undefined" :autocomplete="setup?'new-password':'current-password'"/></label><template v-if="setup"><small>Use at least 12 characters (up to 72 UTF-8 bytes).</small><label>Confirm password<input v-model="form.confirm" required type="password" autocomplete="new-password"/></label></template><p v-if="error" class="scanner-error" role="alert">{{error}}</p><button class="primary" :disabled="busy">{{busy?'Please wait…':setup?'Create owner account':'Sign in'}}</button></fieldset></form><button class="auth-retry" @click="check" :disabled="busy">Check connection / account status</button></template></section></main>
</template>

<style scoped>
.auth-screen{min-height:100dvh;display:grid;place-items:center;padding:24px;background:#f8f7f4}.auth-card{width:min(440px,100%);background:#fffdf9;border:1px solid #e4d9ca;border-radius:14px;padding:34px;box-shadow:0 16px 60px #2919190d}.auth-brand{display:flex;align-items:center;gap:10px;color:#782b3e;font:26px Georgia;margin-bottom:32px}.auth-card h1{font-size:30px;margin:14px 0 8px}.auth-card>p{color:#857b70;line-height:1.6}.auth-card fieldset{border:0;padding:0;margin:22px 0 0;min-width:0}.auth-card label{display:grid;gap:7px;margin:15px 0;font-size:12px}.auth-card input{width:100%;padding:12px;border:1px solid #d9cdbd;border-radius:6px;background:white}.auth-card small{color:#857b70;font-size:11px;line-height:1.5}.auth-card .primary{width:100%;margin-top:16px;padding:13px}.auth-retry{font-size:11px;color:#857b70;margin-top:18px}
</style>
