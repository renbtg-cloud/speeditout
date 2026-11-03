# Local-model resilience (v0.15 research implementation)

Speedit out's Go interpreter works without a neural model. Local model
contributions can veto an insufficiently authorized draft, but **cannot**
authorize sending messages or memory changes. No sending capability exists.

## Response-time budget

The HTTP service now supports `-model-budget 750ms` or `1s`. This is the maximum
duration available to optional inference **after** the deterministic result has
been computed. A canceled or timed-out model preserves the existing Go result.
An internal draft proposal still requires explicit user approval.

Zero (the default) preserves the historical `-model-timeout` budget and allows
slow research runs. Set `-model-budget` for speed-constrained UX experiments.
This does not make the Qwen capable of meeting the deadline; measure timeouts.

## Repeated-error cooldown (opt-in)

`-model-failures 3 -model-cooldown 30s` opens a local circuit after three
consecutive errors from the optional provider. While open, further inference is
skipped; after cooldown one request can be attempted. Successful completion
resets the consecutive failure counter. Errors in model payload validation are
counted, while explicit `context.Canceled` is not counted as a failure. This is
not a global distributed circuit breaker and the first post-cooldown request
is not a single exclusive probe under concurrent load.

These flags are disabled by default. The model URL must remain numeric loopback,
no redirects/proxy routing; the service is not meant for public exposure.
`GET /metrics` includes counters for invocations, failures, skipped calls and
whether the circuit is currently open. They are technical counters, not a
measure of model accuracy.

## Evidence trust boundary

Model-selected quotes must be byte-for-byte excerpts of the actual user text;
repeated or overlapping opposing snippets are rejected as ambiguous. A quoted
imperative reported by someone else does not, by itself, authorize drafting.
The parser still has limited grammar; simple evidence location does not prove
meaning. A concrete independent user request still needs existing approval.

## Preserve interrupted experiments

`cmd/eval -journal /path/to/run.jsonl` writes an opening record containing the
corpus SHA-256 and then syncs every case to disk. The baseline and hybrid phases
are labeled separately. The JSONL file is deliberately *not* an automatic
accuracy score: a truncated journal must be labeled **incomplete**, including
its completed count and total. Never fill in missing cases by inference.

An example (only if your local Qwen server is already running):

```sh
go run ./cmd/eval \
  -cases cases/v015-postfreeze-new.json \
  -local-model-url http://127.0.0.1:18913/v1/chat/completions \
  -local-model-name qwen3-1.7b -model-profile qwen3-1.7b-evidence \
  -model-timeout 90s -journal results/local-private-run.jsonl \
  -out results/local-private-summary.json -progress
```

The example is a command only; no Qwen run of this v0.15 corpus is claimed.
Do not commit logs containing actual human utterances. Consent and data
retention policies must be designed before deploying to real users.
