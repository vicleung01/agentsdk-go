const fs=require('fs');
const F="pkg/api/runloop_v2.go";
let lines=fs.readFileSync(F,'utf-8').split('\n');
let start=-1,end=-1;
for(let i=0;i<lines.length;i++){
  if(lines[i].includes('// --- Tool execution (inline, reuses v1 helpers) ---')) start=i;
  else if(start>=0 && lines[i].includes('return last, firstMiddlewareErr')){end=i+1;break;}
}
if(start<0){console.log("NOT FOUND start");process.exit(1);}
console.log('replace lines '+(start+1)+'-'+end);
const T='\t';
const nb=[
T+T+'// --- Tool execution (origin/main Execute; preserves concurrency + middleware + tracer) ---',
T+T+'if len(resp.Message.ToolCalls) > 0 {',
T+T+T+'calls := resp.Message.ToolCalls',
T+T+T+'var firstMiddlewareErr error',
T+T+T+'outs := make([]*toolpkg.CallResult, len(calls))',
'',
T+T+T+'runTool := func(i int) {',
T+T+T+T+'state.ToolCall = calls[i]',
T+T+T+T+'if err := chain.Execute(ctx, middleware.StageBeforeTool, state); err != nil && firstMiddlewareErr == nil {',
T+T+T+T+T+'firstMiddlewareErr = err',
T+T+T+T+'}',
T+T+T+T+'toolSpan := SpanContext(nil)',
T+T+T+T+'if tracer != nil {',
T+T+T+T+T+'toolSpan = tracer.StartToolSpan(agentSpan, strings.TrimSpace(calls[i].Name))',
T+T+T+T+'}',
T+T+T+T+'res, err := tools.Execute(ctx, calls[i])',
T+T+T+T+'if tracer != nil {',
T+T+T+T+T+'tracer.EndToolSpan(toolSpan, map[string]any{',
T+T+T+T+T+T+'"session_id":  strings.TrimSpace(prep.normalized.SessionID),',
T+T+T+T+T+T+'"request_id":  strings.TrimSpace(prep.normalized.RequestID),',
T+T+T+T+T+T+'"tool_use_id": strings.TrimSpace(calls[i].ID),',
T+T+T+T+T+T+'"tool_name":   strings.TrimSpace(calls[i].Name),',
T+T+T+T+T+'}, err)',
T+T+T+T+'}',
T+T+T+T+'outs[i] = res',
T+T+T+'}',
'',
T+T+T+'parallel := len(calls) > 1',
T+T+T+'if parallel {',
T+T+T+T+'var wg sync.WaitGroup',
T+T+T+T+'for i := range calls {',
T+T+T+T+T+'wg.Add(1)',
T+T+T+T+T+'go func(i int) { defer wg.Done(); runTool(i) }(i)',
T+T+T+T+'}',
T+T+T+T+'wg.Wait()',
T+T+T+'} else {',
T+T+T+T+'for i := range calls {',
T+T+T+T+T+'runTool(i)',
T+T+T+T+'}',
T+T+T+'}',
'',
T+T+T+'for i := range calls {',
T+T+T+T+'state.ToolCall = calls[i]',
T+T+T+T+'state.ToolResult = outs[i]',
T+T+T+T+'if err := chain.Execute(ctx, middleware.StageAfterTool, state); err != nil && firstMiddlewareErr == nil {',
T+T+T+T+T+'firstMiddlewareErr = err',
T+T+T+T+'}',
T+T+T+'}',
T+T+T+'if firstMiddlewareErr != nil {',
T+T+T+T+'loopState.lastTransition = transitionError',
T+T+T+T+'runErr = firstMiddlewareErr',
T+T+T+T+'return last, firstMiddlewareErr',
T+T+T+'}',
T+T+'}'
];
lines.splice(start,end-start,nb.join('\n'));
fs.writeFileSync(F,lines.join('\n'),'utf-8');
console.log("OK replaced");
