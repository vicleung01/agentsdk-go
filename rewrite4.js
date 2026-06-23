const fs=require('fs');
const F="pkg/api/runloop_v2.go";
let lines=fs.readFileSync(F,'utf-8').split('\n');
let start=-1,end=-1;
for(let i=0;i<lines.length;i++){
  if(lines[i].includes('// --- Tool execution (inline, reuses v1 helpers) ---')) start=i;
  else if(start>=0 && lines[i].trim()==='continue' && lines[i+1] && lines[i+1].trim()==='}'){ end=i+2; break; }
}
if(start<0||end<0){console.log("NOT FOUND s="+start+" e="+end);process.exit(1);}
console.log('replace '+(start+1)+'-'+end);
const T='\t',t2=T+T,t3=t2+T,t4=t3+T,t5=t4+T,t6=t5+T;
const nb=[
t2+'// --- Tool execution (origin/main Execute; preserves concurrency + middleware + tracer) ---',
t2+'if len(resp.Message.ToolCalls) > 0 {',
t3+'calls := resp.Message.ToolCalls',
t3+'var firstMiddlewareErr error',
t3+'outs := make([]*toolpkg.CallResult, len(calls))',
'',
t3+'runTool := func(i int) {',
t4+'state.ToolCall = calls[i]',
t4+'if err := chain.Execute(ctx, middleware.StageBeforeTool, state); err != nil && firstMiddlewareErr == nil {',
t5+'firstMiddlewareErr = err',
t4+'}',
t4+'toolSpan := SpanContext(nil)',
t4+'if tracer != nil {',
t5+'toolSpan = tracer.StartToolSpan(agentSpan, strings.TrimSpace(calls[i].Name))',
t4+'}',
t4+'res, err := tools.Execute(ctx, calls[i])',
t4+'if tracer != nil {',
t5+'tracer.EndToolSpan(toolSpan, map[string]any{',
t6+'"session_id":  strings.TrimSpace(prep.normalized.SessionID),',
t6+'"request_id":  strings.TrimSpace(prep.normalized.RequestID),',
t6+'"tool_use_id": strings.TrimSpace(calls[i].ID),',
t6+'"tool_name":   strings.TrimSpace(calls[i].Name),',
t5+'}, err)',
t4+'}',
t4+'outs[i] = res',
t3+'}',
'',
t3+'parallel := len(calls) > 1',
t3+'if parallel {',
t4+'var wg sync.WaitGroup',
t4+'for i := range calls {',
t5+'wg.Add(1)',
t5+'go func(i int) { defer wg.Done(); runTool(i) }(i)',
t4+'}',
t4+'wg.Wait()',
t3+'} else {',
t4+'for i := range calls {',
t5+'runTool(i)',
t4+'}',
t3+'}',
'',
t3+'for i := range calls {',
t4+'state.ToolCall = calls[i]',
t4+'state.ToolResult = outs[i]',
t4+'if err := chain.Execute(ctx, middleware.StageAfterTool, state); err != nil && firstMiddlewareErr == nil {',
t5+'firstMiddlewareErr = err',
t4+'}',
t3+'}',
t3+'if firstMiddlewareErr != nil {',
t4+'loopState.lastTransition = transitionError',
t4+'runErr = firstMiddlewareErr',
t4+'return last, firstMiddlewareErr',
t3+'}',
t3+'loopState.turnCount++',
t3+'loopState.lastTransition = transitionContinue',
t3+'continue',
t2+'}'
];
lines.splice(start,end-start,nb.join('\n'));
let s=lines.join('\n').replace(/\nfunc isPromptTooLongError\(err error\) bool \{[\s\S]*?\n\}/,'\n');
fs.writeFileSync(F,s,'utf-8');
console.log("OK rewrite4 + del isPromptTooLongError");
