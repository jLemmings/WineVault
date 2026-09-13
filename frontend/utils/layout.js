export const footprint = r => ({ x:r.x,y:r.y,w:r.rotation===90?r.depth:r.width,h:r.rotation===90?r.width:r.depth,name:`Shelf ${r.id}` })
const overlaps=(a,b)=>a.x<b.x+b.w-.001&&a.x+a.w>b.x+.001&&a.y<b.y+b.h-.001&&a.y+a.h>b.y+.001
export function outline(room){const w=room.width,h=room.depth,l=room.layout;return l.shape==='l-shape'?`M0 0 H${w-l.cutoutWidth} V${l.cutoutDepth} H${w} V${h} H0 Z`:`M0 0 H${w} V${h} H0 Z`}
export function layoutIssue(room){
 if(!room.name.trim()||!room.room.trim())return 'Give your cellar and room a name.'
 if(room.width<2||room.width>30||room.depth<2||room.depth>30)return 'Room dimensions must be between 2 and 30 meters.'
 const l=room.layout
 if(l.shape==='l-shape'&&(l.cutoutWidth<.3||l.cutoutDepth<.3||l.cutoutWidth>room.width-1||l.cutoutDepth>room.depth-1))return 'The cutout must leave at least a one-meter-wide room.'
 let length=['north','south'].includes(l.doorWall)?room.width:room.depth,start=0
 if(l.shape==='l-shape'&&l.doorWall==='north')length-=l.cutoutWidth
 if(l.shape==='l-shape'&&l.doorWall==='east')start=l.cutoutDepth
 if(l.doorWidth<.6||l.doorWidth>2||l.doorOffset<start||l.doorOffset+l.doorWidth>length+.001)return 'The door must fit on its outer wall. Adjust its offset or width.'
 const door=l.doorWall==='north'?{x:l.doorOffset,y:0,w:l.doorWidth,h:.5}:l.doorWall==='south'?{x:l.doorOffset,y:room.depth-.5,w:l.doorWidth,h:.5}:l.doorWall==='west'?{x:0,y:l.doorOffset,w:.5,h:l.doorWidth}:{x:room.width-.5,y:l.doorOffset,w:.5,h:l.doorWidth}
 if(room.racks.some(r=>r.width<.2||r.depth<.2||r.width>10||r.depth>10))return 'Shelf footprints must be between 0.2 and 10 meters.'
 const objects=room.racks.map(footprint)
 if(l.tableEnabled){if(l.tableWidth<.4||l.tableDepth<.4||l.tableWidth>5||l.tableDepth>5)return 'Table dimensions must be between 0.4 and 5 meters.';objects.push({x:l.tableX,y:l.tableY,w:l.tableWidth,h:l.tableDepth,name:'Tasting table'})}
 for(let i=0;i<objects.length;i++){const a=objects[i];if(a.x<0||a.y<0||a.x+a.w>room.width+.001||a.y+a.h>room.depth+.001||(l.shape==='l-shape'&&overlaps(a,{x:room.width-l.cutoutWidth,y:0,w:l.cutoutWidth,h:l.cutoutDepth})))return `${a.name} is outside the room. Move it or enlarge the room.`;if(overlaps(a,door))return `${a.name} blocks the door. Leave 0.5 meters clear inside the entrance.`;for(const b of objects.slice(0,i))if(overlaps(a,b))return `${a.name} overlaps ${b.name}.`}
 return ''
}
