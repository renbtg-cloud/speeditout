const {test} = require('node:test');
const assert = require('node:assert/strict');
const net = require('node:net');
const {spawn} = require('node:child_process');
const {setTimeout: sleep} = require('node:timers/promises');
const {interpretText,canOfferDraft} = require('../dist/interpretationClient.js');
const {submitFeedback,deleteFeedback} = require('../dist/feedbackClient.js');
const binary=process.env.SPEEDITOUT_SERVER_BINARY;

async function availablePort() {
  const srv=net.createServer();
  await new Promise((resolve,reject)=>srv.once('error',reject).listen(0,'127.0.0.1',resolve));
  const p=srv.address().port;
  await new Promise(resolve=>srv.close(resolve));
  return p;
}

test('real Go <-> TypeScript contract and feedback lifecycle', {skip:!binary,timeout:20000},async t=>{
  const port=await availablePort();
  const url=`http://127.0.0.1:${port}`;
  const child=spawn(binary,['-listen',`127.0.0.1:${port}`],{stdio:'ignore'});
  t.after(()=>{if(child.exitCode===null)child.kill()});
  let ready=false;
  for(let k=0;k<80;k++) {
    if(child.exitCode!==null)break;
    try {const r=await fetch(url+'/healthz');if(r.ok){ready=true;break}}catch(_){}
    await sleep(75);
  }
  assert.ok(ready,'Go service did not become available');
  const got=await interpretText(url, {
    workspace:'integration-synthetic',user_id:'one',
    text:'Escreva ao chefe que volto segunda.',context:{domain:'work'},
  });
  assert.equal(got.decision,'propose_draft');
  assert.equal(canOfferDraft(got),true);
  assert.equal(got.external_calls,0);
  assert.equal(got.decision_audit.external_action_authorized,false);

  const feedback=await submitFeedback(url,{
    workspace:'integration-synthetic',user_id:'one',category:'uncertain',reference:'e2e-001',
  },true);
  assert.ok(feedback.feedback_id>0);
  const summary=await fetch(url+'/v1/feedback/summary',{
    method:'POST',headers:{'Content-Type':'application/json'},
    body:JSON.stringify({workspace:'integration-synthetic',user_id:'one'}),
  });
  assert.equal(summary.status,200);
  assert.equal((await summary.json()).counts.uncertain,1);
  const deletion=await deleteFeedback(url,'integration-synthetic','one',feedback.feedback_id,true);
  assert.equal(deletion.journal_tombstone_only,true);
});
