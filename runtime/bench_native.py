#!/usr/bin/env python3
"""Correctness + wall time comparing direct GGUF quantized matvec with FP16-cache torch F.linear."""
import ctypes, json, time, statistics, os, sys
from pathlib import Path
import numpy as np
import torch
sys.path.insert(0,str(Path(__file__).resolve().parent))
from gguf_qwen3_experimental import GGUF,Store

def main():
 torch.set_num_threads(4)
 g=GGUF('/mnt/data/Qwen3-1.7B-Q4_K_M.gguf'); s=Store(g, '/mnt/data/spititout-viper-v0.5/runtime/converted')
 lib=ctypes.CDLL(str(Path(__file__).with_name('libviper_matvec.so')))
 lib.viper_matvec.argtypes=[ctypes.c_void_p,ctypes.c_int,ctypes.c_longlong,ctypes.c_longlong,ctypes.c_void_p,ctypes.c_void_p,ctypes.c_int]
 lib.viper_matvec.restype=ctypes.c_int
 rows=[]
 samples=['blk.0.attn_q.weight','blk.0.attn_k.weight','blk.0.ffn_down.weight','blk.0.ffn_gate.weight','blk.13.attn_q.weight','blk.27.ffn_up.weight']
 for j,name in enumerate(samples):
  dims,ty,w=g.get_raw(name); cols, nrows=dims; x=np.random.default_rng(2026+j).standard_normal(cols).astype('f4'); y=np.empty(nrows,dtype='f4')
  a=torch.from_numpy(x)
  r=torch.from_numpy(np.asarray(s.get(name)).copy()).float()
  def native(threads=4):
   assert lib.viper_matvec(w.ctypes.data,ty,nrows,cols,x.ctypes.data,y.ctypes.data,threads)==0
   return y
  def baseline():return torch.mv(r,a)
  ref=baseline().numpy(); native()
  abs_err=np.abs(y-ref)
  err_max=float(abs_err.max()); rms=float(np.sqrt(np.mean(abs_err**2))); ref_rms=float(np.sqrt(np.mean(ref**2)))
  print('VERIFY',name,'dims',dims,'ty',ty,'abs_max',err_max,'rel_rms',rms/(ref_rms+1e-20),flush=True)
  if not np.allclose(y,ref,rtol=0.02,atol=0.07):
   bad=np.where(~np.isclose(y,ref,rtol=0.02,atol=0.07))[0][:5]; print('WRONG',[(int(i),float(y[i]),float(ref[i])) for i in bad],flush=True);raise RuntimeError('Numerical mismatch')
  bench={}
  for thread in (1,4):
   native(thread); ts=[]
   for _ in range(5):
    start=time.perf_counter();native(thread);ts.append(time.perf_counter()-start)
   bench['native_'+str(thread)]=round(statistics.median(ts)*1000,3)
  ts=[]
  for _ in range(5):
   start=time.perf_counter();baseline();ts.append(time.perf_counter()-start)
  bench['torch_float32']=round(statistics.median(ts)*1000,3)
  print('BENCH',name,bench,flush=True)
  rows.append({'tensor':name,'qtype':ty,'dims':dims,'max_abs_error_vs_torch_cache':err_max,'relative_rms_error_vs_torch_cache':rms/(ref_rms+1e-20),**bench})
 path=Path('/mnt/data/spititout-viper-v0.6/results/native-matvec-verification.json');path.write_text(json.dumps({'checks':len(rows),'failed':0,'passed':len(rows),'results':rows},indent=2))
 print('SUMMARY',path,flush=True)
if __name__=='__main__':main()
