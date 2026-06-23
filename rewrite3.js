const fs=require('fs');
const F="pkg/api/runloop_v2.go";
let lines=fs.readFileSync(F,'utf-8').split('\n');
let start=-1,end=-1;
for(let i=0;i<lines.length;i++){
  if(lines[i].includes('// --- Tool execution (inline, reuses v1 helpers) ---')) start=i;
  else if(start>=0 && lines[i].includes('return last, firstMiddlewareErr')){end=i+1;break;}
}
if(start<0){console.log("NOT FOUND");process.exit(1);}
const T='\t',t3=T+T+T,t4=t3+T,t5=t4+T,t6=t5+T,t7=t6+T;
const nb=[
t3+'// --- Tool execution (origin/main Execute; preserves concurrency + middleware + tracer) ---',
t3+'if len(resp.Message.ToolCalls) > 0 {',
t4+'calls := resp.Message.ToolCalls',
t4+'var firstMiddlewareErr error',
t4+'outs := make([]*toolpkg.CallResult, len(calls))',
'',
t4+'runTool := func(i int) {',
t5+'state.ToolCall = calls[i]',
t5+'if err := chain.Execute(ctx, middleware.StageBeforeTool, state); err != nil && firstMiddlewareErr == nil {',
t6+'firstMiddlewareErr = err',
t5+'}',
t5+'toolSpan := SpanContext(nil)',
t5+'if tracer != nil {',
t6+'toolSpan = tracer.StartToolSpan(agentSpan, strings.TrimSpace(calls[i].Name))',
t5+'}',
t5+'res, err := tools.Execute(ctx, calls[i])',
t5+'if tracer != nil {',
t6+'tracer.EndToolSpan(toolSpan, map[string]any{',
t7+'"session_id":  strings.TrimSpace(prep.normalized.SessionID),',
t7+'"request_id":  strings.TrimSpace(prep.normalized.RequestID),',
t7+'"tool_use_id": strings.TrimSpace(calls[i].ID),',
t7+'"tool_name":   strings.TrimSpace(calls[i].Name),',
t6+'}, err)',
t5+'}',
t5+'outs[i] = res',
t4+'}',
'',
t4+'parallel := len(calls) > 1',
t4+'if parallel {',
t5+'var wg sync.WaitGroup',
t5+'for i := range calls {',
t6+'wg.Add(1)',
t6+'go func(i int) { defer wg.Done(); runTool(i) }(i)',
t5+'}',
t5+'wg.Wait()',
t4+'} else {',
t5+'for i := range calls {',
t6+'runTool(i)',
t5+'}',
t4+'}',
'',
t4+'for i := range calls {',
t5+'state.ToolCall = calls[i]',
t5+'state.ToolResult = outs[i]',
t5+'if err := chain.Execute(ctx, middleware.StageAfterTool, state); err != nil && firstMiddlewareErr == nil {',
t6+'firstMiddlewareErr = err',
t5+'}',
t4+'}',
t4+'if firstMiddlewareErr != nil {',
t5+'loopState.lastTransition = transitionError',
t5+'runErr = firstMiddlewareErr',
t5+'return last, firstMiddlewareErr',
t4+'}',
t3+'}'
];
lines.splice(start,end-start,nb.join('\n'));
let s=lines.join('\n').replace(/\nfunc isPromptTooLongError\(err error\) bool \{[\s\S]*?\n\}/,'\n');
fs.writeFileSync(F,s,'utf-8');
console.log("OK t3-indent + del isPromptTooLongError");
