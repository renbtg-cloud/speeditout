# Speedit out Navigator: Qwen3-1.7B, under control of Go

## Roles

- **Driver (owned, Go):** decides whether to ask, propose a draft, preserve a deadline or remain silent. Never sends a message automatically.
- **Navigator (optional, locally hosted Qwen):** labels an unfamiliar utterance and cites verbatim evidence from that utterance. It has no authority to send messages or choose outgoing text.
- **Failure behavior:** if inference is absent, overloaded, canceled, malformed or unauthorized by the admission gate, Go falls back to its own clarification path.
- **Trade-off:** strict admission rules intentionally reject some semantically valid Qwen labels; improving semantic recall without fabricating user intent is an open research problem.

## What is implemented

Two local protocols are supported on **numeric loopback only**:

1. `http://127.0.0.1:11434/api/chat` — Ollama-compatible API; Qwen profile uses `think:false` and JSON Schema.
2. `http://127.0.0.1:8080/v1/chat/completions` — llama.cpp server; Qwen profile requests JSON Schema and `chat_template_kwargs.enable_thinking=false`.

Both implement a strict JSON contract: `{ "intent": <one of seven labels>, "evidence": <quoted excerpt or empty>, "abstain": <boolean> }`.

Speedit out rejects unexpected keys, trailing JSON, unknown labels, inconsistent abstention, evidence absent from the user utterance, unsafe recipient assumptions and unauthorized dates. Evidence matching is a **limited syntactic safeguard**, not validation of a model's reasoning.

The adapter specializes the Qwen3-1.7B prompt, decoding budget and inference format. It deliberately does not require all optimizations to be universal across LLMs.

## Preparation: weights and runtime

**Not included:** Qwen GGUF weights and llama.cpp/Ollama binaries. This Linux had neither weights nor internet DNS access. No actual LLM inference was measured.

Recommended source files to provide separately:

- Qwen3-1.7B GGUF Q4_K_M (approximately 1.28 GB): https://huggingface.co/ggml-org/Qwen3-1.7B-GGUF
- llama.cpp source ZIP (to compile locally if a runtime is not available): https://github.com/ggml-org/llama.cpp

If the ChatGPT file uploader cannot accept a single GGUF, use `tools/split_spititout_model.py` on your PC. Preserve the manifest and verify SHA-256 after reconstruction.

## Example with llama.cpp server (after source and weights are available)

From a current llama.cpp checkout, **verify supported flags in `llama-server --help` before use**:

```bash
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build -j 4 --target llama-server
./build/bin/llama-server -m /path/Qwen3-1.7B-Q4_K_M.gguf -c 2048 -t 4 -np 1 --host 127.0.0.1 --port 8080 --jinja --chat-template-kwargs '{"enable_thinking": false}'
```

From Speedit out's directory:

```bash
./bin/speeditout -listen 127.0.0.1:18761 \
  -local-model-url http://127.0.0.1:8080/v1/chat/completions \
  -local-model-name qwen3:1.7b \
  -model-profile qwen3-1.7b \
  -model-parallel 1 -model-timeout 30s
```

To compare same evaluation cases without vs with a running model:

```bash
./bin/eval -cases cases/holdout-v2.json \
  -local-model-url http://127.0.0.1:8080/v1/chat/completions \
  -local-model-name qwen3:1.7b \
  -model-profile qwen3-1.7b -model-parallel 1 -model-timeout 30s \
  -out results/ab-qwen-real.json
```

With Ollama instead, use `http://127.0.0.1:11434/api/chat`, and confirm the exact local model tag installed on that runtime.

## Benchmarks and constraints

- The original 83-case corpus is a regression set known to the deterministic engine, not an independent model-quality benchmark.
- `holdout-v1` has 30 developer-authored cases, **8/30 failed** on the deterministic core.
- `holdout-v2` has 32 additional developer-authored cases, **18/32 failed** on the deterministic core. It is reserved for future experiments: do not tune prompts/rules on it and then present it as blind evidence.
- HTTP throughput in `results/` measures the deterministic path ONLY. The model path has not yet been benchmarked.
- A Qwen GGUF file is not an in-house-trained model. It is a third-party open-weight model operated locally; Speedit out's own classifier, memory and policy gate remain functional independently.
- For 4 GiB RAM and 4 CPU quota, begin with a single model generation slot and context length ~2048, then measure RSS, tail latency, tokens per second and out-of-memory risk.
- Reproduction of results with a real LLM will require fixing model version, quantization, runtime revision, prompt profile, temperature/seed, dataset and hardware limits.

## Practical safety and privacy scope

Synthetic examples only. Research server has **no login, TLS, data deletion, PII scrubbing, confidentiality controls or authorization for real corporate/children's messages**. Run only on loopback. The optional inference endpoint must also be loopback-bound; do not expose it to the Internet. Personalization uses existing consented preferences and scoped memory, not inferred demographic stereotypes.
