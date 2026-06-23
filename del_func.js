const fs=require('fs');
const F="pkg/api/runloop_v2.go";
let s=fs.readFileSync(F,'utf-8');
const re=/\nfunc isPromptTooLongError\(err error\) bool \{[\s\S]*?\n\}/;
const m=s.match(re);
if(m){s=s.replace(re,'\n');console.log("删除 isPromptTooLongError: "+m[0].split('\n').length+" 行");}
else console.log("NOT FOUND");
fs.writeFileSync(F,s,'utf-8');
