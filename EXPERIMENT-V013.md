# Speedit out v0.13 — competing evidence, CPU limits, and failed scale-up

## Failures first

- **9/20 failed, 11/20 passed**: v0.12's Go-only baseline on 20 scenarios written and frozen before this implementation (see `cases/authority-v013-frozen.json`).
- **5/6 failed, 1/6 passed**: v0.13 Go gate when **replaying six genuinely model-selected span pairs**; the same 6 Go-only cases failed **6/6**. This is one correction, zero classification regressions **on this non-independent, truncated, development subset**. It is **not a live end-to-end evaluation**; the six model responses were captured during an interrupted earlier live run and then replayed after gate modifications.
- The planned **20/20** live-Qwen evaluation did **not finish** within the execution budget. **No full 20-case hybrid score exists.** Do not extrapolate 1/6 improvement to 20 cases.
- The free-text-generative Qwen trial ran for roughly **200 seconds without completing a 100-token response**, on this CPU-only custom GGUF runtime. No completed model-written competing hypotheses or model-written evidence were produced. That approach **failed the practicality test** here.
- The older v0.12 single-letter model classifier was also attempted on the new 20, but the trial was interrupted before completion. No v0.12-v0.13 real-model comparative quality claim follows from these incomplete runs.
- **No Go test package failed**, `go test -race ./... -count=1` passed and `go vet ./...` passed. No Go-only regression was observed on the nine tracked historical case sets; their existing failures remain (not hidden).

## What is implemented (and what isn't)

1. `runtime/qwen_evidence_server.py` submits **two independent questions** to the **actual local Qwen3-1.7B GGUF Q4_K_M**, asking it to select the best existing span to support each of two competing explanations: (a) a reported/quoted third-party instruction and (b) a direct instruction of the current user.
2. It uses true next-token logits over span **letter IDs**, in **two separate model passes**. **The host splits the source into up to eight spans and resolves selected IDs to exact substrings.** These are **model-selected** snippets, **not model-written text**. This is more informative than v0.12's single class selection, but is emphatically **not free-form semantic reasoning or agentic deliberation**.
3. `engine/authority_evidence.go` performs a strict, bounded JSON parse. It rejects any model output claiming an excerpt not present **byte-for-byte** in the user's original message, extra fields, malformed responses, and ambiguous/exactly duplicated competing evidence.
4. The Go core applies its own authority checks. Model-selected evidence may **veto** a previously proposed draft when it is sufficiently grounded in the source and no independent user instruction is detected. It cannot create a draft, transmit a message, change memory, or override an independently clear user directive.
5. This behavior is **OPT-IN**: use `-model-profile qwen3-1.7b-evidence`. Default/no-model core behavior, and the existing v0.12 profile `qwen3-1.7b`, remain unchanged.
6. If the local model is unavailable, slow, overloaded, cancels, or produces unacceptable evidence, the original draft-proposal flow survives; **proposed drafts still require explicit user approval**. That fallback is not sufficient for production automatic actions: there are none.

## A precise account of actual inference

The real GGUF trial selected evidence for six of the 20 initial cases before timeout. Median successful two-pass inference elapsed time was roughly 6–8 seconds at this stage (individual timings are recorded). The Qwen sometimes selected an obviously invalid direct-user-evidence span such as `a instrutora falou:`. The Go cross-check rejected the supposed authorization. See:

- `results/v013-first-six-qwen.json`: selections and actual logits, recorded from genuine Qwen runs.
- `results/v013-first-six-gate-replay.json`: Go correctness evaluation with a loopback **replay server** returning precisely those selections. No Qwen inference occurred during this replay.
- `results/v013-go-regressions.json`: Go-only classifications across existing case suites.

Only one of the first six incorrect draft proposals was successfully held for clarification. This **does not justify adopting the new method by default**, given its reliability and latency costs.

## Reproduction

```sh
go test -race ./... -count=1
go vet ./...
go run ./cmd/eval -cases cases/authority-v013-frozen.json

# Optional CPU-bound experiment. Real weights/cache are NOT shipped.
python3 runtime/qwen_evidence_server.py --model /path/to/Qwen3-1.7B-Q4_K_M.gguf --cache /path/to/conversion-cache --port 18913
# Another shell; can take a long time, and may time out:
go run ./cmd/eval -cases cases/authority-v013-frozen.json \
  -local-model-url http://127.0.0.1:18913/api/chat \
  -local-model-name qwen3-1.7b -model-profile qwen3-1.7b-evidence \
  -model-timeout 30s
```

Security: research-only, not production-hardened. No arbitrary remote inference endpoints: loopback only. No weights or generated data from users are included.

## Next engineering decision

The bottleneck is not just the number of output tokens: the current experimental GGUF runner is badly inefficient and fails to achieve interactive latency or reliable selection. Next compare against an optimized, reference-quality in-house-controlled llama.cpp runtime (when obtainable), with true generated citations and structured validation. Obtain independent, consented human language samples before claims of broad advantage. Don't make the Qwen a mandatory dependency, or tune merely to known samples until they report perfect accuracy.
