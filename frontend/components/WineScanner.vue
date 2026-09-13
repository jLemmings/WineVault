<script setup>
import { Camera, Upload, ScanLine, X, ArrowRight, ArrowLeft, Wine, Check, LoaderCircle, AlertCircle, Sparkles, Grid2X2, Barcode } from 'lucide-vue-next'
import { prepareWinePhoto } from '~/utils/prepareWinePhoto'
const props=defineProps({cellar:{type:Object,required:true},preferredRack:{type:String,default:''}}),emit=defineEmits(['close','saved'])
const mode=ref('label')
const root=ref(null),cameraInput=ref(null),uploadInput=ref(null),preview=ref(''),photo=ref(null),filename=ref(''),stage=ref('photo'),preparing=ref(false),analyzing=ref(false),saving=ref(false),error=ref(''),result=ref(null),service=ref(null),statusLoading=ref(false),dragging=ref(false)
const inventory=ref(props.cellar)
const count=id=>inventory.value.bottles.filter(b=>b.rack===id).length
const availableRacks=computed(()=>inventory.value.racks.filter(r=>count(r.id)<r.capacity))
const initialRack=availableRacks.value.find(r=>r.id===props.preferredRack)?.id??availableRacks.value[0]?.id??''
const form=reactive({name:'',vintage:'',nonVintage:false,region:'',type:'',rack:initialRack,quantity:1,slots:[],barcode:''})
const selectedRack=computed(()=>inventory.value.racks.find(r=>r.id===form.rack))
const freeSlots=computed(()=>selectedRack.value?Array.from({length:selectedRack.value.capacity},(_,i)=>i).filter(slot=>!inventory.value.bottles.some(b=>b.rack===form.rack&&b.slot===slot)):[])
const slotLabel=slot=>`${String.fromCharCode(65+slot%(selectedRack.value?.columns||6))}${Math.floor(slot/(selectedRack.value?.columns||6))+1}`
const usable=computed(()=>form.name.trim()&&form.region.trim()&&form.type&&form.rack&&Number.isInteger(form.quantity)&&form.quantity>0&&form.quantity<=freeSlots.value.length&&form.slots.length===form.quantity&&form.slots.every(slot=>freeSlots.value.includes(slot))&&(form.nonVintage||(Number.isInteger(Number(form.vintage))&&Number(form.vintage)>=1900&&Number(form.vintage)<=new Date().getFullYear()+1)))
const resultMessage=computed(()=>({unreadable:'We couldn’t read this label clearly. Try a closer photo with less glare.',not_wine:'We couldn’t find a wine label in this photo. Photograph the front label of one bottle.',multiple:'There’s more than one wine in this photo. Take a photo of just one label.'})[result.value?.status]||'')
let controller=null,photoVersion=0
watch(()=>form.rack,()=>{form.slots=[]})
watch(freeSlots,()=>{form.slots=form.slots.filter(slot=>freeSlots.value.includes(slot))})
watch(()=>form.quantity,()=>{form.slots=form.slots.slice(0,Math.max(0,Number(form.quantity)||0))})
function toggleSlot(slot){if(form.slots.includes(slot))form.slots=form.slots.filter(s=>s!==slot);else if(form.slots.length<form.quantity)form.slots.push(slot)}
function close(){if(saving.value)return;controller?.abort();emit('close')}
useEditorDialog(root,close)
onMounted(loadStatus)
onBeforeUnmount(()=>{photoVersion++;controller?.abort();if(preview.value)URL.revokeObjectURL(preview.value)})
async function loadStatus(){statusLoading.value=true;try{service.value=await $fetch('/api/wine-scan/status')}catch{service.value=null}finally{statusLoading.value=false}}
async function chooseFile(file){
 if(!file||saving.value)return
 controller?.abort();analyzing.value=false;const version=++photoVersion;preparing.value=true;error.value=''
 try{const blob=await prepareWinePhoto(file);if(version!==photoVersion)return;if(preview.value)URL.revokeObjectURL(preview.value);photo.value=blob;preview.value=URL.createObjectURL(blob);filename.value=file.name;result.value=null;stage.value='photo'}catch(e){if(version===photoVersion)error.value=e.message}finally{if(version===photoVersion)preparing.value=false}
}
function fileChange(e){const file=e.target.files?.[0];e.target.value='';chooseFile(file)}
function drop(e){dragging.value=false;if(e.dataTransfer.files.length!==1){error.value='Choose one label photo at a time.';return}chooseFile(e.dataTransfer.files[0])}
function manual(){controller?.abort();analyzing.value=false;error.value='';result.value=null;stage.value='review';form.name='';form.vintage='';form.nonVintage=false;form.region='';form.type='';form.barcode=''}
function barcodeRecognized(detected){result.value=detected;form.name=detected.name;form.vintage='';form.nonVintage=false;form.region=detected.region;form.type=detected.type;form.barcode=detected.barcode;stage.value='review';error.value=''}
function barcodeManual(code){manual();form.barcode=code}
async function analyze(){
 if(!photo.value||analyzing.value||preparing.value||!service.value?.available)return
 controller=new AbortController();const request=controller,version=photoVersion;analyzing.value=true;error.value='';result.value=null
 try{const body=new FormData();body.append('image',photo.value,'wine-label.jpg');const detected=await $fetch('/api/wine-scan',{method:'POST',body,signal:request.signal,retry:0,timeout:55000});if(request.signal.aborted||version!==photoVersion)return;result.value=detected
  if(detected.status==='recognized'){form.name=detected.name;form.vintage=detected.vintage??'';form.nonVintage=detected.nonVintage;form.region=detected.region;form.type=detected.type;form.barcode='';stage.value='review'}
 }catch(e){if(!request.signal.aborted&&version===photoVersion)error.value=typeof e.data==='string'?e.data:'The label could not be analyzed. Try again or enter the details manually.'}finally{if(controller===request)analyzing.value=false}
}
function stopAnalysis(){controller?.abort();analyzing.value=false}
async function save(){
 if(saving.value||!usable.value)return;saving.value=true;error.value=''
 try{
  inventory.value=await $fetch('/api/cellar')
  if(form.slots.length!==form.quantity||!form.slots.every(slot=>freeSlots.value.includes(slot)))throw new Error('A selected slot is no longer available. Review your selection below.')
  await $fetch('/api/bottles/batch',{method:'POST',body:{bottle:{name:form.name.trim(),vintage:form.nonVintage?0:Number(form.vintage),region:form.region.trim(),type:form.type,rack:form.rack,barcode:form.barcode},slots:form.slots},retry:0})
  emit('saved')
 }catch(e){error.value=typeof e.data==='string'?e.data:e.message||'Could not save this wine. Please try again.';if(e.status===409||e.statusCode===409){try{inventory.value=await $fetch('/api/cellar')}catch{}}}finally{saving.value=false}
}
</script>

<template>
 <div class="editor-backdrop scanner-backdrop"><section ref="root" class="wine-scanner" role="dialog" aria-modal="true" aria-label="Wine label scanner" tabindex="-1">
  <header class="editor-header"><div class="editor-heading-icon"><ScanLine :size="24"/></div><div><span class="eyebrow">FROM LABEL TO CELLAR</span><h2>{{stage==='photo'?'Meet your next bottle.':'A new addition, nearly home.'}}</h2></div><button class="editor-close" :disabled="saving" aria-label="Close wine scanner" @click="close"><X :size="21"/></button></header>
  <div class="scanner-steps"><span :class="{current:stage==='photo',complete:stage==='review'}"><i><Check v-if="stage==='review'" :size="12"/><template v-else>1</template></i>Photo</span><span class="step-line"></span><span :class="{current:stage==='review'}"><i>2</i>Review & store</span></div>
  <div v-if="stage==='photo'" class="scanner-modes"><button :class="{active:mode==='label'}" :disabled="analyzing||preparing" @click="mode='label'"><ScanLine :size="16"/>Label photo</button><button :class="{active:mode==='barcode'}" :disabled="analyzing||preparing" @click="mode='barcode'"><Barcode :size="17"/>Barcode</button></div>
  <div class="scanner-body" :class="{'review-stage':stage==='review'}">
   <BarcodeScanner v-if="stage==='photo' && mode==='barcode'" @recognized="barcodeRecognized" @manual="barcodeManual" @label="mode='label'"/><div v-if="mode==='barcode' && stage==='review'" class="scanner-photo-side barcode-review-side"><Barcode :size="110" :stroke-width="1"/><span class="little-label">YOUR SCANNED BARCODE</span><strong>{{form.barcode || 'Manual entry'}}</strong><h3>A bottle, ready for its place.</h3><p>Confirm the vintage and wine details on your label before adding it to your cellar.</p><a v-if="result?.source==='Open Food Facts'" :href="'https://world.openfoodfacts.org/product/'+form.barcode" target="_blank" rel="noopener noreferrer">Source: Open Food Facts (ODbL)</a><span v-else-if="result?.source">Source: {{result.source}}</span></div><div v-if="mode==='label'" class="scanner-photo-side"><div :class="['photo-dropzone',{dragging,'has-photo':preview}]" @dragover.prevent="dragging=true" @dragleave.prevent="dragging=false" @drop.prevent="drop">
    <template v-if="preview"><img :src="preview" alt="Your selected wine label"/><div class="photo-frame"><i v-for="n in 4" :key="n"></i></div><div v-if="analyzing" class="analysis-overlay"><div class="scan-sweep"></div><span><LoaderCircle :size="16" class="spinning"/>Reading your label…</span></div></template>
    <template v-else><div class="scanner-bottle-art" aria-hidden="true"><div class="illustrated-neck"></div><div class="illustrated-bottle"><div class="illustrated-label"><Wine :size="29"/><span>YOUR NEXT<br>DISCOVERY</span><i>—</i></div></div><span class="photo-corner one"></span><span class="photo-corner two"></span><span class="photo-corner three"></span><span class="photo-corner four"></span></div><h3>Every label tells a story.</h3><p>Take a photo or drop an image here.<br>We’ll help you fill in the details.</p></template>
    <div v-if="preparing" class="preparing-overlay"><LoaderCircle :size="25" class="spinning"/><span>Preparing your photo…</span></div>
   </div><div v-if="preview" class="photo-file-caption"><span>{{filename}}</span><span>Ready for recognition</span></div>
   <input ref="cameraInput" class="scanner-file-input" type="file" accept="image/*" capture="environment" aria-label="Take a wine label photo" @change="fileChange"/>
   <input ref="uploadInput" class="scanner-file-input" type="file" accept="image/*" aria-label="Upload a wine label image" @change="fileChange"/>
   <div class="photo-source-buttons"><button class="editor-secondary" :disabled="saving||preparing||analyzing" @click="cameraInput.click()"><Camera :size="17"/>{{preview?'Retake photo':'Take a photo'}}</button><button class="editor-secondary" :disabled="saving||preparing||analyzing" @click="uploadInput.click()"><Upload :size="16"/>{{preview?'Change image':'Upload image'}}</button></div>
   <p class="photo-privacy">Your photo is sent to OpenAI only when you choose “Identify wine.” WineVault does not save the photo.</p>
   </div>
   <div v-if="stage==='photo' && mode==='label'" class="scanner-intro"><span class="little-label">A LITTLE LESS TYPING</span><h3>One photo.<br>A place in your collection.</h3><p>Identify the wine, check the details, and find a space on your shelf.</p><ol class="scanner-tips"><li><span>01</span><div><strong>Keep the label in focus</strong><p>Fill the frame with one bottle’s front label.</p></div></li><li><span>02</span><div><strong>Let the details shine</strong><p>Use even light and avoid reflections.</p></div></li><li><span>03</span><div><strong>You have the final say</strong><p>Review the name, vintage, and region before saving.</p></div></li></ol>
    <div v-if="statusLoading" class="scanner-service-note"><LoaderCircle class="spinning" :size="15"/>Checking recognition…</div><div v-else-if="!service?.available" class="scanner-service-note"><AlertCircle :size="17"/><div><strong>{{service?'Recognition isn’t connected yet.':'Recognition is unavailable.'}}</strong><p>You can still enter the wine manually.</p><button class="text-button" @click="loadStatus">Check again</button></div></div>
    <div v-if="resultMessage" class="scanner-feedback" role="status"><AlertCircle :size="18"/><div>{{resultMessage}}</div></div>
    <div v-if="error" class="scanner-error" role="alert">{{error}}</div>
    <p class="scanner-format-note">JPEG, PNG & WebP · Up to 20 MB<br>Phone photos are resized automatically.</p>
   </div>
   <form v-if="stage==='review'" id="scanner-review-form" class="scanner-review" @submit.prevent="save"><fieldset :disabled="saving"><div class="recognition-heading"><span class="little-label">{{result?(mode==='barcode'?'SUGGESTED FROM BARCODE':'SUGGESTED FROM YOUR LABEL'):'WINE DETAILS'}}</span><span v-if="result" :class="['confidence-tag',result.confidence]"><Sparkles :size="12"/>{{result.confidence==='high'?'Suggested match':result.confidence==='medium'?'Check the details':'Uncertain match'}}</span></div><h3>{{result?'Does this look right?':'Tell us about your wine.'}}</h3><p class="review-explanation">{{result?'Suggested matches can be wrong. Check and edit every field.':'Fill in the details from the bottle label.'}}</p><p v-if="result?.notes" class="recognition-notes">{{result.notes}}</p>
    <label>Wine name<input v-model="form.name" required maxlength="150" placeholder="Producer & wine name" autocomplete="off"/></label><div class="scanner-field-row"><label>Vintage<input v-model="form.vintage" :disabled="form.nonVintage" type="number" :required="!form.nonVintage" min="1900" :max="new Date().getFullYear()+1" placeholder="Year on label"/></label><label>Wine type<select v-model="form.type" required><option disabled value="">Choose type</option><option>Red</option><option>White</option><option>Rosé</option><option>Sparkling</option><option>Dessert</option></select></label></div><label class="scanner-checkbox"><input v-model="form.nonVintage" type="checkbox"/>This is a non-vintage wine (NV)</label><label>Region<input v-model="form.region" required maxlength="200" placeholder="e.g. Bordeaux, France"/></label>
    <div class="scanner-storage-title"><Grid2X2 :size="15"/><span>A place in your cellar</span></div>
    <div v-if="!availableRacks.length" class="scanner-error" role="status">Your shelves are full. Add a shelf or free a slot, then try again.</div>
    <div class="scanner-field-row"><label>Shelf<select v-model="form.rack" required><option value="" disabled>Choose a shelf</option><option v-for="r in inventory.racks" :key="r.id" :value="r.id" :disabled="count(r.id)>=r.capacity">{{r.id}} · {{r.short}} ({{r.capacity-count(r.id)}} free)</option></select></label><label>Number of bottles<input v-model.number="form.quantity" type="number" min="1" :max="freeSlots.length" step="1" required/></label></div>
    <p v-if="form.quantity>freeSlots.length" class="scanner-error" role="alert">This shelf does not have enough free slots for that quantity.</p>
    <div class="scanner-slot-heading"><span aria-live="polite">{{form.slots.length}} of {{form.quantity || 0}} slots selected</span><button type="button" class="text-button" :disabled="!Number.isInteger(form.quantity)||form.quantity<1||form.quantity>freeSlots.length" @click="form.slots=freeSlots.slice(0,form.quantity)">Select first available</button></div>
    <p class="review-explanation">Select each slot where you placed a bottle.</p>
    <div class="scanner-slots-scroll"><div class="scanner-slots" :style="{'--slot-columns':selectedRack?.columns||6}" role="group" aria-label="Bottle slots">
     <button v-for="n in selectedRack?.capacity||0" :key="n" type="button" :aria-label="'Slot '+slotLabel(n-1)+(!freeSlots.includes(n-1)?' occupied':'')" :aria-pressed="form.slots.includes(n-1)" :disabled="!freeSlots.includes(n-1)||(!form.slots.includes(n-1)&&form.slots.length>=form.quantity)" :class="{chosen:form.slots.includes(n-1),occupied:!freeSlots.includes(n-1)}" @click="toggleSlot(n-1)">{{slotLabel(n-1)}}<Check v-if="form.slots.includes(n-1)" :size="13"/></button>
    </div></div><p class="review-explanation">Filled slots are unavailable. Selected slots are highlighted.</p><div v-if="error" class="scanner-error" role="alert">{{error}}</div>
    </fieldset></form>
  </div>
  <footer class="scanner-footer" v-if="stage==='review' || mode==='label'"><template v-if="stage==='photo'"><button class="scanner-manual" :disabled="preparing" @click="manual">Enter details manually<ArrowRight :size="14"/></button><button v-if="analyzing" class="editor-secondary" @click="stopAnalysis">Cancel analysis</button><button class="primary" :disabled="!photo||preparing||analyzing||!service?.available" @click="analyze"><LoaderCircle v-if="analyzing" :size="17" class="spinning"/><ScanLine v-else :size="17"/>{{analyzing?'Identifying…':'Identify wine'}}</button></template><template v-else><button class="editor-secondary" :disabled="saving" @click="stage='photo';error='' "><ArrowLeft :size="15"/>{{mode==='barcode'?'Back to barcode':'Back to photo'}}</button><span>One bottle · Saved only after confirmation</span><button class="primary" type="submit" form="scanner-review-form" :disabled="saving||!usable"><LoaderCircle v-if="saving" :size="16" class="spinning"/><Check v-else :size="16"/>{{saving?'Adding…':'Add to cellar'}}</button></template></footer>
 </section></div>
</template>
