#!/usr/bin/env python3
"""Research-only loopback bridge, REAL GGUF inference with constrained next-token.
It provides the v0.4 localhost OpenAI-style wire format, but does NOT implement
full OpenAI/llama.cpp semantics. Evidence excerpts come from original input,
not the model. Never expose to a network or use personal user data.
"""
import argparse, http.server, json, logging, sys, threading, time
from gguf_qwen3_experimental import GGUF,Store,Tokenizer,Runner
from eval_choice_model import LABELS,wrap,enc_chat
from compute_backends import RunnerBF16

class App:
 def __init__(self,model,cache,mode="fp32"):
  self.mode=mode
  self.g=GGUF(model); self.t=Tokenizer(self.g.meta); self.r=(RunnerBF16 if mode=="bf16" else Runner)(self.g,Store(self.g,cache));self.ids=[self.t.encode(ch)[0] for ch in LABELS];self.lock=threading.Lock();self.count=0
 def classify(self,req):
  messages=req.get('messages',[])
  if not isinstance(messages,list) or len(messages)<2:raise ValueError('messages required')
  data=json.loads(messages[-1]['content']);text=data['text']; context=data.get('context',{})
  if not isinstance(text,str) or not isinstance(context,dict):raise ValueError('invalid payload')
  with self.lock:
   self.r.cache=[];tokens=enc_chat(self.t,wrap(self.t,{'text':text,'context':context}));start=time.monotonic();_,scores=self.r.forward(tokens,0,choices=self.ids);idx=max(range(len(scores)),key=scores.__getitem__)
   intent=list(LABELS.values())[idx]
   # These are deterministic, verbatim user quotes, not model-produced explanations.
   evidence='' if intent=='unknown' else text[:120]
   ret={'intent':intent,'evidence':evidence,'abstain':intent=='unknown'}
   duration=round(time.monotonic()-start,3);self.count+=1
   print('INFER',json.dumps({'count':self.count,'intent':intent,'seconds':duration,'n_tokens':len(tokens),'text':text[:60],'letter_scores':dict(zip(LABELS,[round(x,2) for x in scores]))},ensure_ascii=False),flush=True)
   return ret

class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_GET(self):
  if self.path=='/healthz':return self.sendjson(200,{'ok':True,'model':'Qwen3-1.7B GGUF real constrained-letter logits','compute':self.server.app.mode,'requests':self.server.app.count})
  return self.sendjson(404,{'error':'not found'})
 def do_POST(self):
  if self.path!='/v1/chat/completions':return self.sendjson(404,{'error':'not found'})
  try:
   n=int(self.headers.get('Content-Length','0'))
   if n<1 or n>131072:raise ValueError('bad input length')
   obj=json.loads(self.rfile.read(n)); ret=self.server.app.classify(obj)
   body={'id':'qwen-choice-'+str(self.server.app.count),'object':'chat.completion','choices':[{'index':0,'message':{'role':'assistant','content':json.dumps(ret,ensure_ascii=False)},'finish_reason':'stop'}]}
   self.sendjson(200,body)
  except Exception as e:
   print('ERROR',repr(e),flush=True);self.sendjson(500,{'error':str(e)})
 def sendjson(self,code,obj):
  blob=json.dumps(obj,ensure_ascii=False).encode();self.send_response(code);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(blob)));self.end_headers();self.wfile.write(blob)
class HTTPServer(http.server.ThreadingHTTPServer):
 daemon_threads=True
 def __init__(self,addr,app):super().__init__(addr,Handler);self.app=app

if __name__=='__main__':
 ap=argparse.ArgumentParser();ap.add_argument('--port',type=int,default=18080);ap.add_argument('--model',default='/mnt/data/Qwen3-1.7B-Q4_K_M.gguf');ap.add_argument('--cache',default='/mnt/data/spititout-viper-v0.5/runtime/converted');ap.add_argument('--compute',choices=['fp32','bf16'],default='fp32');a=ap.parse_args()
 print('Starting real GGUF constrained-choice local bridge. Not a production inference server.',flush=True)
 HTTPServer(('127.0.0.1',a.port),App(a.model,a.cache,a.compute)).serve_forever()
