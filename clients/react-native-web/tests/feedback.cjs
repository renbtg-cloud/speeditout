const {test} = require('node:test');
const assert = require('node:assert/strict');
const {submitFeedback,deleteFeedback} = require('../dist/feedbackClient.js');
const sample={workspace:'w',user_id:'u',category:'insufficient_evidence'};
test('feedback cannot transmit without affirmative consent', async () => {
 const saved=global.fetch;let called=0;
 global.fetch=async () => {called++;throw Error('should never be called')};
 try {
   await assert.rejects(submitFeedback('http://127.0.0.1:18761',sample,false));
   await assert.rejects(deleteFeedback('http://127.0.0.1:18761','w','u',1,false));
   assert.equal(called,0);
 } finally {global.fetch=saved}
});
test('feedback confirmation must explicitly say no automatic learning',async () => {
 const saved=global.fetch;
 global.fetch=async () => ({ok:true,json:async()=>({saved:true,feedback_id:7,automatically_learned:true})});
 try { await assert.rejects(submitFeedback('http://127.0.0.1:18761',sample,true)); }
 finally {global.fetch=saved}
});
test('valid feedback does not send transcript or audio implicitly', async()=>{
 const saved=global.fetch;
 global.fetch=async (_,opt)=>{
  const body=JSON.parse(opt.body);
  assert.deepEqual(Object.keys(body).sort(),['category','consent','user_id','workspace']);
  return {ok:true,json:async()=>({saved:true,feedback_id:11,automatically_learned:false})};
 };
 try{ const result=await submitFeedback('http://127.0.0.1:18761',sample,true);assert.equal(result.feedback_id,11); }
 finally{global.fetch=saved}
});
test('deletion response must expose persistent journal tombstone limitation',async()=>{
 const saved=global.fetch;
 global.fetch=async () => ({ok:true,json:async()=>({deleted_from_active_index:true,journal_tombstone_only:false})});
 try {await assert.rejects(deleteFeedback('http://127.0.0.1:18761','w','u',11,true)); }
 finally {global.fetch=saved}
});
