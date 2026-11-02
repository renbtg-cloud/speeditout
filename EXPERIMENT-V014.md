# Speedit out v0.14 — quantized CPU decode, authority evidence and defensive integration

This is a **research** release. The official v0.13 evidence profile remains
optional; v0.14 does **not** turn Qwen's output into authorization to act.

## Failures and denominator FIRST

- **8/20 failed; 12/20 passed** in the **completed, live Qwen GGUF** run on
  the existing v0.13 authority corpus. Go-only baseline on the exact same set:
  **9/20 failed; 11/20 passed**. One correction, zero newly incorrect
  classifications **on this already-seen, developer-authored set**.
- **2/14 model-gate attempts were unavailable**, and 12/14 produced an
  evidence-gate outcome. The latter split into 1 veto applied and 11 rejections
  of insufficient/contradictory evidence. **6/20 cases never required the
  evidence gate**, so no inference was needed for them.
- **The model still missed 8/20**. p50 hybrid latency **25.23 seconds** and p95
  **100.14 seconds** across all 20 cases, including fast deterministic paths.
  **Not interactive.** A model invocation timing out is a model-availability
  failure, not evidence of an incorrect generated label.
- **0/153 Go tests/subtests failed; 153/153 passed** in the normal final run.
  A separate complete `go test -race ./... -count=1` and `go vet ./...`
  both passed. The 153 figure counts tests/subtests, not synthetic corpora.
- **0/7 Python tests failed; 7/7 passed** (GGUF/tokenizer plus cache isolation).
- **0/6 direct-matvec numerical checks failed; 6/6 passed** versus
  explicitly dequantized float32 references. Relative L2 error per matrix
  was approximately 4–6 × 10⁻⁷ for the six chosen tensors.
- **0 newly incorrect cases in 9/9 frozen historical Go-only corpora**,
  compared *case-by-case* with a v0.13 executable snapshot. Aggregate failure
  counts stayed unchanged. This is a regression check against **seen cases**,
  not independent evidence of generalized capability.

## CPU runtime experiment

The reference model was reconstructed from previously uploaded fragments and
SHA-256 verified before any inference:

- File: `Qwen3-1.7B-Q4_K_M.gguf`
- Bytes: `1,282,439,264`
- SHA-256: `d2387ca2dbfee2ffabce7120d3770dadca0b293052bc2f0e138fdc940d9bc7b5`

The existing Python/PyTorch GGUF runner was extended by `runtime/compute_native.py`
and `runtime/native_matvec.cpp`. A local OpenMP CPU kernel executes single-token
Q4_K/Q6_K matrix multiplications directly on **quantized GGUF bytes**. Multi-token
prefill remains in the **experimental BF16/PyTorch** path; this is *not*
llama.cpp and has not been reference-validated against llama.cpp outputs.

One matched, 16-generated-token CPU trial gave:

| Backend | Total | Prompt prefill (layer timings) | Median subsequent layer pass | Output |
| --- | ---: | ---: | ---: | --- |
| Native single-token decode | 91.345 s | 35.3 s | 0.8 s | Incomplete JSON |
| Previous BF16 | 131.720 s | 12.8 s | 4.0 s | Identical incomplete JSON |

The observed total walltime was **30.65% lower** in this single comparison.
This is **not** a statistically validated throughput gain. The runs had
significant CPU stalls; prefill/IO effects differed. Both were artificially
capped at 16 generated tokens, not enough to complete their JSON. Their matching
prefixes do not establish general numerical equivalence.

See `results/v014-native-speed-summary.json` and
`results/v014-native-matvec.json`. Raw generation logs were kept locally
for troubleshooting, but are not committed as repository artifacts.

## Completed real-model authority evaluation

To remedy the incomplete v0.13 evaluation, all **20 cases** of
`cases/authority-v013-frozen.json` were run with the **actual local Qwen
weights** in the v0.13 evidence-span selection mode. Each model request ran
against the experimental PyTorch forward pass with native single-token support
and Go verified each cited span. The 20-case score is *not* a replay: the
inference requests were executed live. The earlier v0.13 set is now **seen**,
so improvements do not constitute a new independent holdout.

- Go-only baseline: **9/20 wrong, 11/20 correct**.
- Go + real Qwen: **8/20 wrong, 12/20 correct**.
- 1 improvement, 0 classification regressions, 1 change.
- 14 evidence-gate considered; 1 applied, 11 unproven, 2 unavailable.
- At least two long/failed responses show continued unreliability of the CPU
  runtime under concurrent environment load. Inference latency is unacceptable
  for the user's intended rapid conversational experience.

Raw per-case data: `results/v014-qwen20-complete.json`; model-selected
evidence and per-request selection timings:
`results/v014-qwen20-model-selections.json`. This server-side log export
can include responses completed *after* a caller timeout; the per-case Go
report is authoritative for what the application actually received.
No synthetic provider was substituted for Qwen in this specific run. No other
LLM or frontier-model comparison was performed.

## Additional milestones

1. `engine/strictjson.go`: reject duplicate keys at any nesting level,
   invalid UTF-8, oversized responses, excess depth and trailing JSON.
2. `engine/model_envelope.go`: read and validate the **entire** local provider
   HTTP envelope; no acceptance of a valid JSON prefix followed by junk.
3. Require a constrained JSON output format for the llama.cpp-compatible
   evidence endpoint, plus tests of the HTTP request contract. Actual
   llama.cpp could **not** be downloaded/started in this Linux environment
   because DNS to GitHub was unavailable; the contract tests use a mock.
4. `cmd/eval`: per-case progress on stderr, gate-applied/unproven/unavailable
   counters, and latency and correctness evidence for a complete experiment.
5. `cmd/regress`: compare frozen corpora against v0.13 both in aggregate and
   **per case**, so an improvement cannot conceal a separate regression.
   Historical case labels are not claimed to be independent.
6. Python GGUF `Store` now includes full **model SHA-256** in its cache path,
   preventing silent reuse of stale BF16 weights with a different GGUF.
7. Added the Windows/Git Bash `llama.cpp` launcher and benchmarking kit under
   `tools/windows-llama`, local verification scripts and a Linux/Windows Go CI
   definition. Cross-compiled five Windows x86-64 executable commands from
   Linux; **did not execute them on a real Windows host**. CI workflow
   execution is not claimed until GitHub confirms it.

## Reproduction

```sh
bash scripts/verify.sh
bash scripts/build-native-linux.sh
python3 runtime/bench_native_decode.py \
  --model /path/to/Qwen3-1.7B-Q4_K_M.gguf \
  --out results/native-recheck.json
```

Live model requires **the exact GGUF**, the conversion cache (or disk space to
rebuild it), local CPU/memory, PyTorch/NumPy, and an extended timeout. It is not
part of the default Go workflow. See [docs/CPU-INFERENCE.md](docs/CPU-INFERENCE.md).

## Next priority

Obtain a reference-quality optimized `llama.cpp` runtime (already prepared for
Windows/Git Bash), test true generated JSON under constrained sampling, validate
numerical agreement and prompt semantics, measure throughput/latency across
multiple runs, then evaluate on fresh genuinely unseen and—ideally—consented
human inputs. Do not enable long-latency model contestation by default.

**Humildade diante das evidências. Acima de tudo.**
