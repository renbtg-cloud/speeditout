#!/usr/bin/env python3
import json,sys,time,re,os
from pathlib import Path
sys.path.insert(0,str(Path(__file__).resolve().parent))
from gguf_qwen3_experimental import GGUF,Store,Tokenizer
from compare_bf16 import RunnerBF16
from eval_choice_model import LABELS,enc_chat
COMPACT='''Identifique a intenção, responda só uma letra: A=rejeitar jogo; B=não entendeu; C=não redigir nem enviar mensagem de trabalho; D=adiar trabalho; E=levou ghosting e se culpa; F=sintoma mecânico; G=outro ou ambíguo. Não confunda fala de terceiros ou citação com intenção do falante.'''
def short(c):
 msg=json.dumps({'c':c.get('context',{}),'t':c['text']},ensure_ascii=False,separators=(',',':'))
 return '<|im_start|>system\n'+COMPACT+'<|im_end|>\n<|im_start|>user\n'+msg+'<|im_end|>\n<|im_start|>assistant\n<think>\n\n</think>\n'
def main():
 g=GGUF('/mnt/data/Qwen3-1.7B-Q4_K_M.gguf');t=Tokenizer(g.meta);s=Store(g,'/mnt/data/spititout-viper-v0.5/runtime/converted')
 allcases=json.load(open('/mnt/data/spititout-viper-v0.6/cases/holdout-v2.json'));cs=[c for c in allcases if c['id'] in ('N01','N02','N03')]
 choices=[t.encode(ch)[0] for ch in LABELS]
 rows=[]
 for c in cs:
  runner=RunnerBF16(g,s);ids=enc_chat(t,short(c));start=time.monotonic();_,logits=runner.forward(ids,0,choices=choices);sec=time.monotonic()-start
  idx=max(range(len(logits)),key=logits.__getitem__);predicted=list(LABELS.values())[idx]
  obj={'id':c['id'],'expected':c['expected_intent'],'predicted':predicted,'correct':predicted==c['expected_intent'],'tokens':len(ids),'seconds':round(sec,3),'margin':round(sorted(logits)[-1]-sorted(logits)[-2],3)}
  rows.append(obj);print('CASE',json.dumps(obj,ensure_ascii=False),flush=True)
  Path('/mnt/data/spititout-viper-v0.6/results/compact-bf16-partial.json').write_text(json.dumps(rows,indent=2,ensure_ascii=False))
 print('SUMMARY',sum(not x['correct'] for x in rows),'failed/',len(rows),flush=True)
if __name__=='__main__':main()
