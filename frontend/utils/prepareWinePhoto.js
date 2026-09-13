// Use the browser's image decoder for phone orientation and format support.
// Drawing a fresh JPEG also removes the original photo's EXIF metadata.
export async function prepareWinePhoto(file){
 if(!file)throw new Error('Choose a photo first.')
 if(file.size>20*1024*1024)throw new Error('Choose a photo smaller than 20 MB.')
 if(!file.size)throw new Error('This file is empty. Choose another photo.')
 if(file.type&&!file.type.startsWith('image/'))throw new Error('Choose an image, such as a JPEG, PNG, or WebP photo.')
 const url=URL.createObjectURL(file)
 try{
  const img=await new Promise((resolve,reject)=>{const image=new Image();image.onload=()=>resolve(image);image.onerror=()=>reject(new Error('This image could not be opened. If it is HEIC, export it as JPEG or take a new photo.'));image.src=url})
  if(img.naturalWidth<32||img.naturalHeight<32)throw new Error('This image is too small. Take a close-up photo of the label.')
  const scale=Math.min(1,1800/Math.max(img.naturalWidth,img.naturalHeight))
  const canvas=document.createElement('canvas');canvas.width=Math.round(img.naturalWidth*scale);canvas.height=Math.round(img.naturalHeight*scale)
  const context=canvas.getContext('2d');if(!context)throw new Error('Your browser could not prepare this photo. Try another browser.')
  context.fillStyle='#fff';context.fillRect(0,0,canvas.width,canvas.height);context.drawImage(img,0,0,canvas.width,canvas.height)
  const blob=await new Promise(resolve=>canvas.toBlob(resolve,'image/jpeg',.9))
  if(!blob||blob.size>8*1024*1024)throw new Error('This photo could not be resized. Choose a smaller image.')
  return blob
 }finally{URL.revokeObjectURL(url)}
}
