#!/usr/bin/env python3
"""Run Go engine race tests in isolated groups to avoid one memory-heavy process.
This validates race-free behavior but is not identical to a single monolithic run.
"""
import json, re, subprocess, sys
from pathlib import Path
root=Path(__file__).resolve().parents[1]
listing=subprocess.check_output(['go','test','-list','^Test','./engine'],cwd=root,text=True)
names=[x.strip() for x in listing.splitlines() if x.startswith('Test')]
summary={'test_functions':len(names),'batches':0,'failed':0,'passed':0,'total':0,'timed_out_batches':0}
for i in range(0,len(names),15):
    batch=names[i:i+15]
    pattern='^('+ '|'.join(re.escape(x) for x in batch) +')$'
    try:
        proc=subprocess.run(['go','test','-race','-short','-count=1','-run',pattern,'-json','./engine'],cwd=root,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=30)
    except subprocess.TimeoutExpired:
        summary['timed_out_batches']+=1
        print(f'BATCH {i//15+1}: TIMEOUT',flush=True)
        continue
    events=[json.loads(x) for x in proc.stdout.splitlines() if x.startswith('{')]
    f=sum(e.get('Action')=='fail' and 'Test' in e for e in events)
    p=sum(e.get('Action')=='pass' and 'Test' in e for e in events)
    summary['batches']+=1;summary['failed']+=f;summary['passed']+=p
    summary['total']+=f+p
    if proc.returncode!=0 and f==0:
        summary['failed']+=1;summary['total']+=1
    print(f'BATCH {i//15+1}: failures={f}/{f+p}, passes={p}/{f+p}, exit={proc.returncode}',flush=True)
summary['success']= summary['failed']==0 and summary['timed_out_batches']==0 and len(names)<=summary['passed']
(root/'results/v016-engine-race-batched.json').write_text(json.dumps(summary,indent=2)+'\n')
print(json.dumps(summary,indent=2),flush=True)
sys.exit(0 if summary['success'] else 1)
