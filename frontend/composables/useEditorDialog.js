export function useEditorDialog(root, close){
 let previous, previousOverflow
 function keydown(e){
  if(e.key==='Escape'){e.preventDefault();close();return}
  if(e.key!=='Tab')return
  const elements=[...root.value.querySelectorAll('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled),[tabindex="0"]')].filter(el=>el.getClientRects().length)
  if(!elements.length)return
  const first=elements[0],last=elements.at(-1)
  if(e.shiftKey&&document.activeElement===first){e.preventDefault();last.focus()}
  else if(!e.shiftKey&&document.activeElement===last){e.preventDefault();first.focus()}
 }
 onMounted(()=>{previous=document.activeElement;previousOverflow=document.body.style.overflow;document.body.style.overflow='hidden';root.value?.focus();document.addEventListener('keydown',keydown)})
 onBeforeUnmount(()=>{document.removeEventListener('keydown',keydown);document.body.style.overflow=previousOverflow;previous?.focus()})
}
