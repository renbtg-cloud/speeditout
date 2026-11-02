"""Experimental BF16 fast inference: in-use conversions, choice scoring stays FP32."""
import argparse,sys,json,time,os,statistics
from pathlib import Path
import torch
sys.path.insert(0,str(Path(__file__).resolve().parent))
from gguf_qwen3_experimental import GGUF,Store,Tokenizer,Runner
from eval_choice_model import enc_chat,wrap,LABELS
from compute_backends import RunnerBF16

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--ids',default='N01,N02,N03,N04');ap.add_argument('--maxcases',type=int,default=4);a=ap.parse_args()
 g=GGUF('/mnt/data/Qwen3-1.7B-Q4_K_M.gguf');t=Tokenizer(g.meta);s=Store(g,'/mnt/data/spititout-viper-v0.5/runtime/converted')
 cases=json.load(open('/mnt/data/spititout-viper-v0.5/cases/holdout-v2.json'));cs=[c for c in cases if c['id'] in a.ids.split(',')][:a.maxcases]
 choices=[t.encode(k)[0] for k in LABELS];results=[]
 for c in cs:
  tokens=enc_chat(t,wrap(t,c));out=[]
  for label,cls in [('float32',Runner),('bf16-matmul',RunnerBF16)]:
   runner=cls(g,s);start=time.perf_counter();i,logits=runner.forward(tokens,0,choices=choices);time_s=time.perf_counter()-start
   choice=list(LABELS.values())[i];out.append({'mode':label,'intent':choice,'time_s':round(time_s,3),'logits':[round(x,4) for x in logits]})
  results.append({'id':c['id'],'tokens':len(tokens),'expected':c['expected_intent'],'runs':out,'prediction_same':out[0]['intent']==out[1]['intent'],'speedup':round(out[0]['time_s']/out[1]['time_s'],2)})
  print('CASE',json.dumps(results[-1],ensure_ascii=False),flush=True)
 res={'method':'float32 vs bf16 matrix multiplication, unchanged RMSnorm/softmax/rotary, GGUF dequant f16 cache','cases':results,'count':len(results),'same_predictions':sum(r['prediction_same'] for r in results),'avg_speedup':round(statistics.mean(r['speedup'] for r in results),2)}
 Path('/mnt/data/spititout-viper-v0.6/results/bf16-a-b.json').write_text(json.dumps(res,indent=2,ensure_ascii=False))
 print('SUMMARY',json.dumps({'same':res['same_predictions'],'total':res['count'],'avg_speedup':res['avg_speedup']}),flush=True)
if __name__=='__main__':main()
