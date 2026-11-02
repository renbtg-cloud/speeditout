#!/usr/bin/env python3
"""Experimental real-Qwen GGUF authority ranking bridge. NOT a production server.

The model chooses one of four *letter tokens* (ranked by next-token logits).
The host makes a literal, deterministic citation from the original text; the
model DOES NOT generate or verify the citation or a JSON response. No claims of
full model-generated semantic reasoning should be based on this experiment.
This script intentionally works without llama.cpp or a network connection.
"""
import argparse
import http.server
import json
import os
import logging
import re
import threading
import time
from gguf_qwen3_experimental import GGUF, Store, Tokenizer
from eval_choice_model import enc_chat
from compute_backends import RunnerBF16
from compute_native import RunnerNativeDecode

ROLES = {"A": "reported", "B": "direct", "C": "refusal", "D": "uncertain"}
INSTRUCTIONS = """Classifique a fonte de autorização na fala da pessoa. Responda APENAS UMA LETRA.
A = a instrução é CITAÇÃO, exemplo hipotético ou fala de terceiro, não pedido da pessoa.
B = a própria pessoa dá instrução DIRETA para redigir/responder, mesmo se mencionar terceiros.
C = a pessoa NEGA autorização, diz que não quer mensagem/rascunho, só quer falar.
D = não é possível determinar ou não há instrução.
Não confunda relatos de pedidos de chefes com pedidos da pessoa. Se houver ordem da própria pessoa em outra frase, escolha B. Não infira intenção de um prazo isolado.
Responda só A, B, C ou D."""

def make_prompt(data):
    metadata = json.dumps({'context': data.get('context') or {}, 'text': data['text']}, ensure_ascii=False)
    return '<|im_start|>system\n'+INSTRUCTIONS+'<|im_end|>\n<|im_start|>user\n'+metadata+'<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n'

def excerpt(text, role):
    if role == 'uncertain': return ''
    patterns = {
      'reported': [r'(?i)(?:\b(?:o|a|meu|minha|um|uma)\s+\w+(?:\s+\w+){0,2}\s+(?:disse|falou|comentou|pediu|ordenou|mandou|relatou|afirmou)|\b(?:foi dito|falaram|pediram|no workshop|no treinamento|no exemplo|no curso|no seminário|no podcast))'],
      'direct': [r'(?i)\b(?:agora\s+)?(?:escreva|redija|mande|responda|avise|diga|fale)\b'],
      'refusal': [r'(?i)(?:não\s+quero|nao\s+quero|não\s+prepare|nao\s+prepare|só\s+quero|so\s+quero)'],
    }
    for pattern in patterns.get(role, []):
        match = re.search(pattern, text)
        if match: return text[match.start():min(match.end(),match.start()+120)]
    return text[:min(100,len(text))]

def compose(choice, text):
    if choice == 'reported': kinds=('reported','direct')
    elif choice == 'direct': kinds=('direct','reported')
    elif choice == 'refusal': kinds=('refusal','reported')
    else: kinds=('uncertain','direct')
    return {'hypotheses':[{'kind':k,'evidence':excerpt(text,k)} for k in kinds], 'choice':choice, 'abstain': choice=='uncertain'}

class QwenAuthority:
    def __init__(self, model, cache):
        self.g=GGUF(model); self.t=Tokenizer(self.g.meta)
        self.r=(RunnerNativeDecode if os.environ.get("SPEEDIT_NATIVE_DECODE") == "1" else RunnerBF16)(self.g,Store(self.g,cache))
        self.ids=[self.t.encode(ch)[0] for ch in ROLES]
        self.lock=threading.Lock(); self.count=0
    def rank(self, body):
        msgs=body.get('messages')
        if not isinstance(msgs,list) or len(msgs)<2 or 'MODE_AUTHORITY_V012' not in msgs[0].get('content',''):
            raise ValueError('authority prompt required')
        user=json.loads(msgs[-1]['content'])
        sentence=user['text']
        if not isinstance(sentence,str) or len(sentence)>4000: raise ValueError('input text invalid')
        with self.lock:
            self.r.cache=[]
            tokens=enc_chat(self.t,make_prompt(user))
            started=time.monotonic()
            _, logits=self.r.forward(tokens,0,choices=self.ids)
            selected=list(ROLES.values())[max(range(len(logits)),key=logits.__getitem__)]
            duration=time.monotonic()-started
            self.count+=1
            print('AUTHORITY_QWEN',json.dumps({'case':self.count,'selection':selected,'duration_sec':round(duration,3),'token_count':len(tokens),'letter_logits':{ch:round(float(log),3) for ch,log in zip(ROLES,logits)}},ensure_ascii=False),flush=True)
            return compose(selected,sentence)

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self,*args): pass
    def do_GET(self):
        if self.path == '/healthz': return self.send(200,{'ready':True,'type':'real GGUF constrained letter logits; host-cited evidence, not model generated'})
        self.send(404,{'error':'not found'})
    def do_POST(self):
        if self.path not in ('/api/chat','/v1/chat/completions'): return self.send(404,{'error':'not found'})
        try:
            size=int(self.headers.get('Content-Length','0'))
            if size<1 or size>32768: return self.send(413,{'error':'request too big'})
            payload=json.loads(self.rfile.read(size))
            result=self.server.qwen.rank(payload)
            content=json.dumps(result,ensure_ascii=False)
            response={'choices':[{'message':{'content':content}}]} if self.path.endswith('/completions') else {'message':{'content':content}}
            self.send(200,response)
        except Exception as e:
            print('ERROR',repr(e),flush=True)
            self.send(500,{'error':str(e)})
    def send(self, status, obj):
        raw=json.dumps(obj,ensure_ascii=False).encode()
        self.send_response(status); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
class Server(http.server.ThreadingHTTPServer):
    daemon_threads=True

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--model',default='/mnt/data/Qwen3-1.7B-Q4_K_M.gguf');p.add_argument('--cache',default='/mnt/data/speeditout-qwen-converted');p.add_argument('--port',type=int,default=18891);a=p.parse_args()
    server=Server(('127.0.0.1',a.port),Handler)
    server.qwen=QwenAuthority(a.model,a.cache)
    print('Qwen Authority bridge listening on 127.0.0.1:'+str(a.port),flush=True)
    server.serve_forever()
