export default defineNuxtPlugin(()=>{
 const original=globalThis.$fetch
 globalThis.$fetch=original.create({
  onRequest({request,options}){
   if(typeof request==='string'&&request.startsWith('/api/')){
    const headers=new Headers(options.headers);headers.set('X-WineVault-Request','1');options.headers=headers
   }
  },
  onResponseError({request,response}){
   if(response.status===401&&typeof request==='string'&&request.startsWith('/api/')&&!request.startsWith('/api/auth/login'))window.dispatchEvent(new Event('winevault-signed-out'))
  }
 })
})
