#!/usr/bin/env python3
"""Reproducible per-matrix checks for the optional raw-GGUF CPU matvec.

These numerical unit benchmarks do not imply end-to-end token generation speed.
"""
import argparse
import json
import os
from pathlib import Path
import statistics
import sys
import time

import numpy as np
import torch

sys.path.insert(0, str(Path(__file__).resolve().parent))
from compute_native import RunnerNativeDecode
from gguf_qwen3_experimental import GGUF, Store, dequant_q4k, dequant_q6k

SAMPLES = [
    'blk.0.attn_q.weight', 'blk.0.attn_k.weight',
    'blk.0.ffn_gate.weight', 'blk.0.ffn_down.weight',
    'blk.13.attn_q.weight', 'blk.27.ffn_up.weight',
]


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--model', required=True)
    p.add_argument('--cache', default='/tmp/speeditout-viper-matvec')
    p.add_argument('--reps', type=int, default=4)
    p.add_argument('--out')
    args = p.parse_args()
    if not 1 <= args.reps <= 100:
        p.error('reps must be 1..100')
    g = GGUF(args.model)
    engine = RunnerNativeDecode(g, Store(g, args.cache))
    report = {'model_bytes': os.stat(args.model).st_size, 'total': len(SAMPLES), 'failed': 0, 'passed': 0, 'items': []}
    for j, name in enumerate(SAMPLES):
        dims, kind, raw = g.get_raw(name)
        cols, rows = dims
        v = np.random.default_rng(300+j).standard_normal(cols).astype('f4')
        d = dequant_q4k(np.asarray(raw)) if kind == 12 else dequant_q6k(np.asarray(raw))
        reference = d.reshape(rows, cols) @ v
        elapsed = []
        for _ in range(args.reps):
            start = time.perf_counter()
            y = engine._matrix(torch.from_numpy(v), name).numpy().reshape(-1)
            elapsed.append(1000*(time.perf_counter()-start))
        rel = float(np.linalg.norm(y-reference)/(np.linalg.norm(reference)+1e-20))
        maxerr = float(np.max(np.abs(y-reference)))
        passed = bool(np.allclose(y, reference, rtol=.002, atol=.0001))
        report['passed' if passed else 'failed'] += 1
        report['items'].append({'tensor': name, 'kind': kind, 'rows': rows, 'cols': cols,
                               'passed': passed, 'max_absolute_error': maxerr,
                               'relative_l2_error': rel, 'median_ms': statistics.median(elapsed)})
        print(('PASS' if passed else 'FAIL'), name, f'median={statistics.median(elapsed):.2f}ms',
              f'rel_L2={rel:.2g}', flush=True)
    if args.out:
        Path(args.out).parent.mkdir(parents=True, exist_ok=True)
        Path(args.out).write_text(json.dumps(report, indent=2)+'\n')
    print(f"SUMMARY: {report['failed']}/{report['total']} failed, {report['passed']}/{report['total']} passed", flush=True)
    return 1 if report['failed'] else 0


if __name__ == '__main__':
    raise SystemExit(main())
