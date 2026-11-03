const { test } = require('node:test');
const assert = require('node:assert/strict');
const { parseInterpretationResult, canOfferDraft, interpretText } = require('../dist/interpretationClient.js');
const sample = () => ({
  decision: 'propose_draft', intent: 'defer_work', reply: 'Rascunho',
  confidence: 'medium', reasons: [], requires_approval: true, external_calls: 0,
  decision_audit: {
    policy_version: '0.16', approval_required: true,
    model_outcome: 'not_needed', external_action_authorized: false,
  },
});
test('valid draft still needs human approval', () => {
  const result = parseInterpretationResult(sample());
  assert.equal(canOfferDraft(result), true);
  assert.equal(result.decision_audit.external_action_authorized, false);
});
test('reject inconsistent authorization variants', () => {
  const variants = [
    { decision_audit: {...sample().decision_audit, external_action_authorized: true} },
    { decision_audit: {...sample().decision_audit, approval_required: false} },
    { decision_audit: {...sample().decision_audit, model_outcome: 'hacked'} },
    { external_calls: 1 }, { requires_approval: false },
    { decision: 'send_email' }, { reasons: ['ok', 7] },
    { reply: '' }, { decision_audit: null },
  ];
  for (const variant of variants) {
    assert.throws(() => parseInterpretationResult({...sample(), ...variant}));
  }
});
test('invalid top-level values are rejected', () => {
  for (const x of [null, true, [], 'text', 42]) {
    assert.throws(() => parseInterpretationResult(x));
  }
});
test('transport validates before returning a result', async () => {
  const saved = global.fetch;
  global.fetch = async (_url, options) => {
    assert.equal(options.method, 'POST');
    return { ok: true, json: async () => ({...sample(), external_calls: 7}) };
  };
  try {
    await assert.rejects(interpretText('http://127.0.0.1:18761', {
      workspace:'testing',user_id:'alice',text:'test',
    }));
  } finally { global.fetch = saved; }
});
