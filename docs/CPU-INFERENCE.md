# Local CPU inference: reproducibility and boundaries (v0.14)

Speedit out's Go engine **does not require a neural network**. Local Qwen inference
is optional, and any model proposal remains subject to the independent Go
authorization and evidence gates. The model never sends an email or writes
memory directly.

## Three different mechanisms (do not confuse them)

1. **BF16 research backend:** `runtime/compute_backends.py` converts GGUF weights
   into a BF16-friendly cache, computes most operations using PyTorch, and
   executes a custom Qwen3 forward pass. It is not a production-quality GGUF
   reference implementation.
2. **Native research decode:** `runtime/compute_native.py` uses an own C++
   OpenMP matvec kernel reading Q4_K / Q6_K matrices *directly* from the GGUF
   for single-token inference. Multi-token prefill remains BF16/PyTorch.
   Enable with `SPEEDIT_NATIVE_DECODE=1`. On Linux, compile with
   `bash scripts/build-native-linux.sh`. This is **not llama.cpp** and the
   source does not contain `llama.cpp`.
3. **llama.cpp (recommended to evaluate on Windows):** use the
   `tools/windows-llama` Bash scripts from Git Bash to install an optimized
   executable, start it locally and record timings. A separate live test on
   real Windows hardware is required before making any speed/quality claims.

## GGUF identity and privacy

The reference research artifact is `Qwen3-1.7B-Q4_K_M.gguf`, exactly
1,282,439,264 bytes, SHA-256
`d2387ca2dbfee2ffabce7120d3770dadca0b293052bc2f0e138fdc940d9bc7b5`.
The model is **NOT committed**. Do not commit model fragments, conversion cache,
logs containing user conversations, secrets or local memory journals.

In v0.14, dequantization cache is keyed by the **full model SHA-256 and converter
version**. Do not rename/reuse an old unkeyed converted cache silently; stale
weights can produce valid-looking, incorrect results.

## Reproducing native numerical checks (Linux)

```sh
bash scripts/build-native-linux.sh
python3 runtime/bench_native_decode.py \
  --model /path/to/Qwen3-1.7B-Q4_K_M.gguf \
  --out results/native-matvec-reproduced.json
```

For the slower experimental evidence selector:

```sh
SPEEDIT_NATIVE_DECODE=1 python3 runtime/qwen_evidence_server.py \
  --model /path/to/Qwen3-1.7B-Q4_K_M.gguf --cache /tmp/speeditout-cache --port 18913
```

On a second terminal (choose a timeout that fits your CPU, not production latency):

```sh
go run ./cmd/eval -cases cases/authority-v013-frozen.json \
  -local-model-url http://127.0.0.1:18913/api/chat \
  -local-model-name qwen3-1.7b -model-profile qwen3-1.7b-evidence \
  -model-timeout 100s -progress
```

The model runs at numeric loopback only. Results must distinguish actual
inference from captured/replayed responses, deterministic Go decisions and
cases where inference timed out. `-progress` reports case IDs, not utterance
contents; still treat saved JSON reports as research data.

## Operational caveats

- Inference startup has substantial CPU, memory and disk costs; the optional
  native kernel is a prototype, not production-grade or reference-validated.
- A valid JSON object and an exact substring citation do **not** prove that
  the interpretation of that citation is correct.
- The HTTP adapter checks response size, encoding and duplicate JSON keys; it
  does not make language-model output trusted.
- No real-world human ASR or conversational success has been measured here.
