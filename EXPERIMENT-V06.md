# Speedit out v0.6 — CPU inference optimization research

## Fundamental limitation

An optimized reference `llama.cpp` runtime **was not available** and the container's DNS could not resolve GitHub/PyPI/Hugging Face. The CPU-only native C++ implementation in this archive is our own *tensor-level research kernel*, **not** a substitute for the full ggml/llama.cpp inference engine and not independent numerical validation against it.

We measured alternatives rather than proclaiming a new language/runtime faster without data.

## 1. Native C++ Q4_K/Q6_K matvec experiment

`runtime/native_matvec.cpp` operates directly on raw quantized GGUF superblocks (no extraction of full FP16 matrices). It supports Q4_K and Q6_K matvec only, with OpenMP optional; it does not implement attention, tokenization or model serving. It can be built independently with `g++ -O3 -march=native -fopenmp -shared -fPIC`.

Numerical comparisons: **0 failures / 6**, 6/6 tensor outputs matched the existing Python/PyTorch FP16-cache reference within the documented tolerance (`rtol=0.02, atol=0.07`); observed relative RMS errors about 0.020%–0.022%, largely explained by comparison to the FP16-cached dequantized matrices and floating-point accumulation differences. Exact figures and timing are in `results/native-matvec-verification.json`.

**Performance: native kernel is SLOWER than PyTorch F32 on these tested matrices. DO NOT deploy as a speed optimization.** It is useful as a minimal independent quantized storage/matvec POC, not a fast production inferencer.

## 2. BF16 accelerated matrix multiplication (CPU)

`runtime/compute_backends.py` offers an optional BF16 linear-layer path. Matrix multiplies use BF16; normalization, attention, rotary embeddings, residual additions and chosen-token logits stay FP32. The default path remains the previous FP32 implementation. This is an approximation, **not a reference implementation**.

On four synthetic cases of about 237–249 prompt tokens, the same first-token choice was obtained with BF16 and FP32 (0/4 prediction changes). Representative timings (full model prefill, 28 layers):

| Case | FP32 s | BF16 s | Same decision? |
| --- | ---: | ---: | --- |
| N01 | 6.804 | 5.339 | Yes |
| N02 | 6.200 | 4.897 | Yes |
| N03 | 6.433 | 4.395 | Yes |
| N04 | 5.387 | 4.381 | Yes |

This is a small non-blind synthetic sample on a shared CPU, not a guarantee of speedup across machines, model types or workloads.

## 3. Prompt-compression experiment

A shorter classification prompt reduced context from around 240–250 tokens to 130–142 tokens, but **2/3 cases failed** (1/3 passed) with BF16 on the three tested cases: N02 misattributed a rejection, N03 missed lack of comprehension. It was **rejected** rather than promoted as an optimization. Results: `results/compact-bf16-partial.json`.

## 4. Integration

`runtime/qwen_choice_server.py` now supports `--compute fp32` (default) or `--compute bf16`; `GET /healthz` reports the chosen mode. This adapter remains research-only, bound to 127.0.0.1, and sends a constrained seven-letter classification to the existing Go controller. It does NOT generate a free-form JSON answer; excerpt evidence is still extracted mechanically from the input and **is not independently verified by Qwen**.

The Qwen model is real and quantized, but the classifier is not a calibrated intent estimator. All examples are artificial and developer-visible, so scores do not establish any improvement over frontier models, nor an unbiased generalization benefit.

## Remaining needs

- Obtain a known version of `llama.cpp` or another verified reference inference engine in a future environment, then compare logits/tokenization/numerical differences; never claim reference equivalence until done.
- Replace experimental BF16 and native matvec kernels only after a proper test of throughput, latency, quality and concurrency.
- Validate on a fresh blind holdout independent of prompt editing and rule tuning.
- Do not use data from real children, companies or private emails in this prototype.

## 5. Completed 32-case Go hybrid A/B comparison

The local loopback bridge with BF16 ran **21 actual Qwen inferences** for the full 32-case Go evaluator. FP32 results were retained from the prior v0.5 test using the same case set and Qwen GGUF.

| Metric | FP32 hybrid | BF16 hybrid |
| --- | ---: | ---: |
| Failed / total | 17/32 | 17/32 |
| Passed / total | 15/32 | 15/32 |
| Latency p50 | 5867.395 ms | 4023.887 ms |
| Latency p95 | 6985.942 ms | 4747.914 ms |
| Final decision changes | — | 0/32 |

This is a **31.42% lower observed p50, 32.04% lower observed p95**, with no changes to the 32 final classification/policy outcomes. The reduction is not proof of universal speedup; the conditions were local and non-blind, runs were sequential rather than randomized repetitions, and throughput at model concurrency >1 was not tested.

`results/holdout-v2-bf16-hybrid.json` contains per-case decisions; `results/hybrid-fp32-vs-bf16-diff.json` has the exact comparison; `results/v06-summary.json` has machine-readable summary. The benchmark does not repair the 17/32 existing errors; quality remains the primary obstacle.

## 6. Verification before packaging

- Go regression suite with `-race`: completed successfully (raw log in `results/go-race-v06.txt`).
- `go vet ./...`: completed successfully (log in `results/go-vet-v06.txt`).
- GGUF runtime unittest: 0/6 failures, 6/6 passes (`results/gguf-unit-v06.txt`).
- Python syntax compilation for all experimental scripts: passed.
- For native matvec, 0/6 failed and 6/6 passed at documented tolerances, yet it is inferior in performance to the torch F32 path and stays a standalone POC.

The priority for the next version is a verified optimized GGUF reference engine such as `llama.cpp`, a truly blind fresh evaluation set, and measuring *quality first*, not claiming a Viper is already faster than optimized GGUF runtimes.
