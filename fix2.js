const fs=require('fs');
const F="pkg/api/runloop_v2.go";
let lines=fs.readFileSync(F,'utf-8').split('\n');
let idx=-1;
for(let i=0;i<lines.length;i++) if(lines[i].includes('return last, firstMiddlewareErr')){idx=i;break;}
if(idx<0){console.log("NOT FOUND return");process.exit(1);}
let cnt=0;for(let k=idx+1;k<=idx+4;k++) if(lines[k]&&lines[k].trim()==='}') cnt++;
console.log("return 后 } 数: "+cnt);
if(cnt>=3){lines.splice(idx+3,1);console.log("删 idx+3 多余 }");}
fs.writeFileSync(F,lines.join('\n'),'utf-8');
