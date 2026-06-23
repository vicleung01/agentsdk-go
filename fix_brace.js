const fs=require('fs');
const F="pkg/api/runloop_v2.go";
let s=fs.readFileSync(F,'utf-8');
const re=/(\t+return last, firstMiddlewareErr\n\t+}\n\t+}\n)(\t+}\n)/;
const m=s.match(re);
if(m){s=s.replace(re,'$1');console.log("删多余 } 1 个");}
else console.log("NOT MATCH");
fs.writeFileSync(F,s,'utf-8');
