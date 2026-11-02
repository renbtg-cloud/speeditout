#!/usr/bin/env python3
"""Run a local Go server, test consent/learning/persistence, then four independent load processes."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time
from urllib.request import Request, urlopen
from urllib.error import HTTPError

ROOT=Path(__file__).resolve().parents[1]
BIN=ROOT/'bin'
RESULTS=ROOT/'results'
URL='http://127.0.0.1:18762'

def post(endpoint,data,expect=200):
    b=json.dumps(data,ensure_ascii=False).encode('utf-8')
    req=Request(URL+endpoint,b,headers={'Content-Type':'application/json'},method='POST')
    try:
        with urlopen(req,timeout=4) as resp:
            code=resp.status; result=json.loads(resp.read())
    except HTTPError as ex:
        code=ex.code;result=json.loads(ex.read())
    assert code==expect,(code,expect,result)
    return result

def start_server(memory_path):
    p=subprocess.Popen([str(BIN/'spititout'),'-listen','127.0.0.1:18762','-memory',str(memory_path)],cwd=ROOT,stdout=subprocess.DEVNULL,stderr=subprocess.PIPE)
    for _ in range(100):
        if p.poll() is not None:
            raise RuntimeError(f'server exited {p.returncode}: {p.stderr.read().decode()}')
        try:
            with urlopen(URL+'/healthz',timeout=.15) as resp:
                if resp.status==200:return p
        except Exception:pass
        time.sleep(.04)
    p.terminate();p.wait(timeout=5)
    raise RuntimeError('server not ready')

def stop_server(p):
    if p.poll() is None:p.terminate()
    try:p.wait(timeout=8)
    except subprocess.TimeoutExpired:p.kill();p.wait()

checks=[]
with tempfile.TemporaryDirectory() as td:
    memory_path=Path(td)/'memory.jsonl'
    server=start_server(memory_path)
    try:
        original={'workspace':'study','user_id':'secretary','text':'Chefe, fica pra segunda','context':{'domain':'work','recipient':'boss'}}
        before=post('/v1/interpret',original)
        assert before['intent']=='defer_work' and before['reply'].startswith('Recebi sua mensagem. ')
        checks.append('HTTP baseline interpretation')
        p=post('/v1/preferences',{'workspace':'study','user_id':'secretary','key':'greeting','value':'omit','recipient':'boss','consent':False},403)
        assert 'consent' in p['error'];checks.append('HTTP no-consent rejection')
        post('/v1/corrections',{'workspace':'study','user_id':'secretary','recipient':'boss','utterance':original['text'],'before':before['reply'],'after':before['reply'].removeprefix('Recebi sua mensagem. '),'consent':True,'generalize':True})
        checks.append('Explicit authorized correction accepted')
        assert not post('/v1/interpret',original)['reply'].startswith('Recebi sua mensagem. ')
        checks.append('Correction applied for boss')
        colleague=dict(original,context={'domain':'work','recipient':'colleague'})
        assert post('/v1/interpret',colleague)['reply'].startswith('Recebi sua mensagem. ')
        checks.append('Recipient isolation')
        dont=post('/v1/interpret',{'workspace':'study','user_id':'secretary','text':'Não responda ao chefe; ele pode esperar até segunda','context':{'domain':'work','recipient':'boss'}})
        assert dont['intent']=='do_not_draft' and dont['decision']=='reply_and_wait'
        checks.append('No-draft instruction honored')
        assert post('/v1/interpret',{'workspace':'study','user_id':'secretary','text':'oi','unknown':3},400)['error']
        checks.append('Unknown JSON field rejected')
    finally:stop_server(server)
    server=start_server(memory_path)
    try:
        assert not post('/v1/interpret',original)['reply'].startswith('Recebi sua mensagem. ')
        checks.append('Durable restart/replay')
        loggens=[]
        for i in range(4):
            path=RESULTS/f'multiprocess-client-{i}.json'
            args=[str(BIN/'loadgen'),'-url',URL+'/v1/interpret','-cases',str(ROOT/'cases/adversarial_cases.json'),'-c','64','-duration','5s','-out',str(path)]
            loggens.append(subprocess.Popen(args,cwd=ROOT,stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True))
        reports=[]
        for i,p in enumerate(loggens):
            sout,serr=p.communicate(timeout=35)
            if p.returncode!=0: raise RuntimeError(f'load process {i} failed: {serr}\n{sout}')
            reports.append(json.loads(sout))
        checks.append('Four independent concurrent load-generator processes completed successfully')
        total=sum(r['requests'] for r in reports)
        failed=sum(r['failures'] for r in reports)
        wall=max(r['duration_sec'] for r in reports)
        run={'kind':'four independent client processes, 64 connections each; same Linux host, 4 CPU quota', 'client_processes':4,'connections_total':256,'mixed_cases':83,'duration_sec_approx':wall,'requests':total,'failures':failed,'passed':total-failed,'aggregate_rps_approx':round(total/wall,1),'client_reports':[str(RESULTS/f'multiprocess-client-{i}.json') for i in range(4)],'e2e_checks':checks,'checks_failed':0,'checks_passed':len(checks)}
        assert failed==0,run
    finally:stop_server(server)
RESULTS.mkdir(parents=True,exist_ok=True)
(RESULTS/'integration-multiprocess.json').write_text(json.dumps(run,indent=2,ensure_ascii=False)+'\n')
print(json.dumps(run,indent=2,ensure_ascii=False))
