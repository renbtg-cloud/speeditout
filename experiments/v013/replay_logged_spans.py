#!/usr/bin/env python3
"""Replays previously recorded real Qwen span picks. NO model inference here.
Used to inspect the Go gate reproducibly after a CPU timeout, not as a fresh
benchmark or evidence of generalization. Do not mislabel these as live runs.
"""
import http.server,json,sys
from pathlib import Path

def samples():
    log=Path(sys.argv[1]).read_text().splitlines()
    rows=[json.loads(x[len('EVIDENCE '):]) for x in log if x.startswith('EVIDENCE ')]
    # First log record was a manual smoke, subsequent records belong to the
    # timed-out full evaluation, whose first six completed Qwen inferences.
    rows=rows[1:7]
    cases=json.loads(Path(sys.argv[2]).read_text())
    mapping={c['text']:row['selected'] for c,row in zip(cases,rows)}
    return mapping

class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self,*_):pass
    def do_POST(self):
        try:
            n=int(self.headers['Content-Length'])
            q=json.loads(self.rfile.read(n))
            text=json.loads(q['messages'][-1]['content'])['text']
            result=self.server.mapping.get(text,{'reported':'','direct':''})
            b=json.dumps({'message':{'content':json.dumps(result,ensure_ascii=False)}},ensure_ascii=False).encode()
            self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b)
        except Exception as e:
            self.send_error(500,str(e))

if __name__=='__main__':
    server=http.server.ThreadingHTTPServer(('127.0.0.1',18914),Handler)
    server.mapping=samples(); print('REPLAY original Qwen outputs only, not new inference',len(server.mapping),flush=True)
    server.serve_forever()
