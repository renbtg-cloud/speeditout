# Speedit out v0.12 — real Qwen authority ranking, bounded by Go

## Failures first

On the **32 developer-authored cases frozen before implementation**, the v0.11 Go baseline had **8/32 failures** (24/32 passed), while the v0.12 hybrid had **6/32 failures** (26/32 passed). The hybrid corrected **2/32** cases with **0/32 regressions** in exact intent/decision matching. It did **not** solve the other six cases. These cases are synthetic, not independently human-validated.

The 10-case model smoke subset, selected from the same frozen 32 after inspecting the baseline, had **5/10 failures** Go-only and **3/10 failures** hybrid; it is exploratory, **not an independent second sample**.

### What's real and what's not

- We **ran the actual Qwen3-1.7B GGUF Q4_K_M weights** locally. Verified model SHA-256: `d2387ca2dbfee2ffabce7120d3770dadca0b293052bc2f0e138fdc940d9bc7b5`.
- The experimental PyTorch BF16 bridge asks the model to choose between four **single-letter answer tokens**: reported, direct, refusal, uncertain, and obtains true next-token logits. This is *constrained-choice inference*, **not full language generation, multiagent deliberation, calibrated probabilities, or Qwen-generated JSON**.
- The bridge converts the winning class into a JSON contract and supplies a **host-extracted** verbatim snippet. The model does **not generate, choose, or validate that evidence snippet**; the Go core validates the presence of the supplied phrase in the original utterance and requires a reporting cue before a veto.
- Only a pre-existing Go `propose_draft` on `defer_work` is challenged. The model can **veto by changing to `clarify`**, not issue an authorization or send email. Direct independent user instructions override an alleged third-party report; the Go core is the authority boundary.
- The Go engine remains operational with **no model, failed inference, malformed JSON, absent evidence, overload, cancellations and timeouts**. Its original no-model behavior is unchanged on the frozen 32.
- In 22 local Qwen inference calls across the smoke and full evaluation, observed median model-call duration was **3.176 seconds**, with **25.938 seconds maximum** including first-run setup/dequantization. Not interactive-fast in this environment.
- The model sometimes chose `direct` even for reported instructions; **four of the six remaining errors** are not necessarily from the same failure category. Inspect the raw data, don't claim broad generalization.

### Artifacts

- `engine/authority_model.go`: ranking schema, strict JSON validation, grounded evidence checks, optional loopback inference, veto-only policy.
- `engine/authority_model_test.go`: adversarial payload, refusal, provenance, racing and failure-mode tests.
- `runtime/qwen_authority_server.py`: local real-GGUF constrained-letter ranking bridge (uses original experimental GGUF runner; **not** a production runtime).
- `cases/authority-v012-frozen.json`: frozen evaluation cases.
- `cases/authority-v012-model-smoke.json`: cases chosen for the smoke trial.
- `results/v012-real-qwen-summary.json`: full summary; per-case raw report `results/v012-32-real.json`.

### Run

```sh
go test -race ./... -count=1
go vet ./...
# Qwen GGUF model and numpy/torch required, not bundled:
python3 runtime/qwen_authority_server.py --model /path/to/Qwen3-1.7B-Q4_K_M.gguf --cache /path/to/converted --port 18891
# in a second terminal:
go run ./cmd/eval -cases cases/authority-v012-frozen.json -local-model-url http://127.0.0.1:18891/api/chat -local-model-name qwen3:1.7b -model-profile qwen3-1.7b -model-timeout 40s -out results/reproduced-v012.json
```

### Engineering caution

Inference is CPU-intensive and has high end-to-end latency. Containing the interface to numeric loopback is necessary but not sufficient for production security. This is research software, without auth, multi-tenant isolation or production deployment suitability. Don't connect it to untrusted remote network clients.

**Next:** stronger authority evidence representations and an independent held-out corpus from real opt-in language samples. Avoid scoring model-produced citations as genuine model reasoning when the host extracted them.
