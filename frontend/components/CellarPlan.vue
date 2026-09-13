<script setup>
import { footprint, outline } from '~/utils/layout'
const props=defineProps({room:{type:Object,required:true},selected:{type:String,default:''},editable:Boolean,snap:{type:Boolean,default:true}})
const emit=defineEmits(['select','move'])
const svg=ref(null),drag=ref(null),id=useId().replaceAll(':','')
const layout=computed(()=>props.room.layout)
const floorColors={stone:'#f5efe4',wood:'#eee0c9',concrete:'#e9e9e5'}
const doorTransform=computed(()=>{const l=layout.value;return l.doorWall==='north'?`translate(${l.doorOffset} 0)`:l.doorWall==='south'?`translate(${l.doorOffset+l.doorWidth} ${props.room.depth}) rotate(180)`:l.doorWall==='west'?`translate(0 ${l.doorOffset+l.doorWidth}) rotate(-90)`:`translate(${props.room.width} ${l.doorOffset}) rotate(90)`})
function point(e){const p=svg.value.createSVGPoint();p.x=e.clientX;p.y=e.clientY;return p.matrixTransform(svg.value.getScreenCTM().inverse())}
function start(e,key){emit('select',key);if(!props.editable||e.button!==0)return;e.preventDefault();const p=point(e),r=key==='table'?{x:layout.value.tableX,y:layout.value.tableY}:props.room.racks.find(r=>r.id===key);drag.value={key,x:r.x,y:r.y,startX:p.x,startY:p.y};svg.value.setPointerCapture(e.pointerId)}
function move(e){if(!drag.value)return;const p=point(e),d=drag.value;place(d.key,d.x+p.x-d.startX,d.y+p.y-d.startY)}
function place(key,x,y){const r=key==='table'?{w:layout.value.tableWidth,h:layout.value.tableDepth}:footprint(props.room.racks.find(r=>r.id===key));const round=n=>props.snap?Math.round(n*10)/10:Math.round(n*100)/100;emit('move',{id:key,x:Math.max(0,Math.min(round(x),props.room.width-r.w)),y:Math.max(0,Math.min(round(y),props.room.depth-r.h))})}
function keyboard(e,key){if(e.key==='Enter'||e.key===' '){e.preventDefault();emit('select',key)}if(!props.editable||!['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(e.key))return;e.preventDefault();const r=key==='table'?{x:layout.value.tableX,y:layout.value.tableY}:props.room.racks.find(r=>r.id===key),step=e.shiftKey?.5:.1;place(key,r.x+(e.key==='ArrowRight'?step:e.key==='ArrowLeft'?-step:0),r.y+(e.key==='ArrowDown'?step:e.key==='ArrowUp'?-step:0))}
const count=id=>(props.room.bottles??[]).filter(b=>b.rack===id).length
</script>

<template>
 <svg ref="svg" :class="['cellar-canvas',{editing:editable}]" :viewBox="`-.4 -.4 ${room.width+.8} ${room.depth+.8}`" aria-label="Bird’s-eye cellar floor plan" @pointermove="move" @pointerup="drag=null" @pointercancel="drag=null" @lostpointercapture="drag=null">
  <defs><pattern :id="id+'grid'" width=".25" height=".25" patternUnits="userSpaceOnUse"><path d="M .25 0 H0 V.25" fill="none" stroke="#b5a48b" stroke-width=".007" opacity=".36"/></pattern><pattern :id="id+'wood'" width=".5" height=".12" patternUnits="userSpaceOnUse"><path d="M0 .12 H.5 M.5 0 V.12" fill="none" stroke="#ae8b55" stroke-width=".009" opacity=".25"/></pattern></defs>
  <path :d="outline(room)" :fill="floorColors[layout.floor]" stroke="#e2d8c8" stroke-width=".15" stroke-linejoin="round"/>
  <path :d="outline(room)" :fill="`url(#${id+(layout.floor==='wood'?'wood':'grid')})`" stroke="#827567" stroke-width=".065" stroke-linejoin="round"/>
  <g :transform="doorTransform" class="plan-door"><path :d="`M0 0 H${layout.doorWidth}`" stroke="#f8f7f4" stroke-width=".1"/><path :d="`M0 ${layout.doorWidth} A${layout.doorWidth} ${layout.doorWidth} 0 0 0 ${layout.doorWidth} 0`" fill="none" stroke="#aa7b75" stroke-width=".013" stroke-dasharray=".045 .035"/><path :d="`M0 0 V${layout.doorWidth}`" stroke="#8d4b53" stroke-width=".035"/></g>
  <g v-for="r in room.racks" :key="r.id" :class="['plan-shelf','rack-'+r.id,{chosen:selected===r.id}]" :transform="`translate(${r.x} ${r.y})`" role="button" tabindex="0" :aria-label="`Select shelf ${r.id}: ${r.name}`" @pointerdown="start($event,r.id)" @keydown="keyboard($event,r.id)" @click="emit('select',r.id)">
   <rect :width="footprint(r).w" :height="footprint(r).h" rx=".045" :fill="selected===r.id?'#fff8e7':'#fffdf9'" :stroke="selected===r.id?'#ba944a':'#bcae99'" :stroke-width="selected===r.id?.033:.015"/>
   <g :transform="r.rotation===90?`translate(${r.depth} 0) rotate(90)`:''">
    <rect x=".045" y=".06" :width="Math.max(.1,r.width-.09)" height=".028" rx=".01" :fill="r.color==='white'?'#7d8249':r.color==='gold'?'#b99a54':'#813348'"/>
    <text x=".1" :y="Math.max(.19,r.depth*.44)" class="svg-shelf-label">{{r.id}} · {{r.short.slice(0,Math.max(3,Math.floor(r.width*14)))}}</text>
    <text x=".1" :y="Math.max(.28,r.depth*.76)" class="svg-shelf-count">{{count(r.id)}} / {{r.capacity}} bottles</text>
   </g>
   <circle v-if="selected===r.id" :cx="footprint(r).w" cy="0" r=".043" fill="#ba944a" stroke="white" stroke-width=".015"/>
  </g>
  <g v-if="layout.tableEnabled" :class="['plan-table',{chosen:selected==='table'}]" :transform="`translate(${layout.tableX} ${layout.tableY})`" role="button" :tabindex="editable?0:-1" aria-label="Select tasting table" @pointerdown="start($event,'table')" @keydown="keyboard($event,'table')">
   <circle v-for="x in [layout.tableWidth*.22,layout.tableWidth*.78]" :cx="x" cy="-.06" r=".07" fill="#ddd1bf" stroke="#b9aa95" stroke-width=".012"/><circle v-for="x in [layout.tableWidth*.22,layout.tableWidth*.78]" :cx="x" :cy="layout.tableDepth+.06" r=".07" fill="#ddd1bf" stroke="#b9aa95" stroke-width=".012"/>
   <rect :width="layout.tableWidth" :height="layout.tableDepth" rx=".12" fill="#fffcf5" :stroke="selected==='table'?'#ba944a':'#c5b497'" stroke-width=".025"/>
   <path :transform="`translate(${layout.tableWidth/2-.065} ${layout.tableDepth*.2})`" d="M0 0 H.13 V.1 Q.13 .17 .065 .17 Q0 .17 0 .1 Z M.065 .17 V.27 M.01 .27 H.12" stroke="#b49960" stroke-width=".013" fill="none"/>
   <text :x="layout.tableWidth/2" :y="layout.tableDepth*.77" text-anchor="middle" class="svg-table-label">Tasting table</text>
  </g>
  <g v-if="editable">
  <text :x="room.width/2" y="-.19" text-anchor="middle" class="svg-dimension">{{Number(room.width).toFixed(2)}} m</text><text :transform="`translate(-.2 ${room.depth/2}) rotate(-90)`" text-anchor="middle" class="svg-dimension">{{Number(room.depth).toFixed(2)}} m</text>
  <text x="0" :y="room.depth+.26" class="svg-dimension">N ↑</text><path :d="`M${room.width-1} ${room.depth+.15} v.06 h1 v-.06`" fill="none" stroke="#a79a87" stroke-width=".012"/><text :x="room.width-.5" :y="room.depth+.34" text-anchor="middle" class="svg-dimension">1 meter</text>
  </g>
 </svg>
</template>
