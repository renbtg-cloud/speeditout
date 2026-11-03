# Speedit out

**Dodge Viper — experimental Go human↔AI communication interpreter.**

*Speedit out* is the product name (previously "Spititout"). Its goal is to
help people express what they mean, and understand what the machine means,
without requiring polished language or prompt-engineering skills.

This repository holds the working prototype up through **v0.18.0**. It is
research software, **not** a production service or a deployed React Native app.

## What exists

- Go HTTP interpretation engine with deterministic fallback, bounded semantic
  evidence frames, attribution/quotation guards, and explicit limits on what the model can decide.
- Optional **v0.13 evidence-hypothesis comparator**, disabled by default;
  selects supporting source spans for two rival authority hypotheses. The
  CPU experimental backend remains slow and unreliable (see EXPERIMENT-V013.md).
- Optional experimental Qwen3 1.7B GGUF inference bridge; the weights are not
  bundled, and the experimental Python/PyTorch runtime is not optimized.
- Per-user communication preferences and correction handling.
- Conservative audio-quality assessment (`POST /v1/audio/assess`), with a
  TypeScript transport contract for future React Native and web clients.
- Adversarial test corpora and experimental reports, including unsuccessful
  trials and documented latency/quality regressions.

## Build and check

Requires Go 1.23+.

```sh
go test -race ./... -count=1
go vet ./...
go run ./cmd/speeditout -listen 127.0.0.1:18761
```

In a separate shell:

```sh
curl -fsS http://127.0.0.1:18761/v1/interpret \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo","workspace":"synthetic","text":"Não faça rascunho. Só quero desabafar sobre o chefe.","context":{"domain":"work"}}'
```

For audio assessment, see [docs/AUDIO-ADAPTATION.md](docs/AUDIO-ADAPTATION.md)
and [EXPERIMENT-V010.md](EXPERIMENT-V010.md).
See [EXPERIMENT-V011.md](EXPERIMENT-V011.md) for the rule-based authority trial and [EXPERIMENT-V012.md](EXPERIMENT-V012.md) for real local Qwen authority ranking (veto-only) and its unsolved failures.

## Experimental integrity

The published results are **synthetic** and do not establish general human
communication understanding or superiority over a frontier language model.
Earlier case sets were examined during iteration; see the historical
experiment reports for proper baselines. No GGUF model, compiled binaries,
secret keys or local databases belong in this repository.

**Security:** no authentication or TLS; bind to loopback and use synthetic
inputs only. Not suitable for deployment on a public interface.

Guiding principle: *Humildade diante das evidências. Acima de tudo.*

## v0.13 research status

Two-source evidence comparison is opt-in via `qwen3-1.7b-evidence`. This is
research, not general reasoning; the Qwen selected span IDs rather than writing
free text, and the complete live model trial timed out.
See [EXPERIMENT-V013.md](EXPERIMENT-V013.md) for failures, reproducibility and
the partially recorded real-model trial.

## v0.14 research status

This iteration adds optional CPU direct-quantized matvec for single-token GGUF
inference, model-SHA-keyed conversion caches, bounded/strict JSON from local
model endpoints, per-case regression snapshots, and evaluation observability.
It **does not** replace the v0.13 default model profile or claim a new general
reasoning model. See [EXPERIMENT-V014.md](EXPERIMENT-V014.md) for measured
results and limitations, and [docs/CPU-INFERENCE.md](docs/CPU-INFERENCE.md)
for CPU/Windows reproduction.

For quick Go-only checks on Linux or Git Bash:

```sh
bash scripts/verify.sh
```

The Windows/Git Bash llama.cpp installer, launcher and benchmark scripts live
in [`tools/windows-llama/`](tools/windows-llama/). They have not been executed
on an actual Windows host in this experiment. CI includes Linux and Windows
build/test jobs; a committed workflow is not evidence that GitHub Actions
has executed successfully.

## v0.15 research status

This release tightens quoted-instruction attribution, literal evidence
provenance and **incoming** JSON ambiguity checks. The optional local model
can now use a per-request time budget and an opt-in failure cooldown, both
controlled by the Go process. The evaluator supports a durable JSONL journal
so an interrupted Qwen benchmark preserves completed cases.

The default remains **Go-only**, with no network model required; no new
Qwen or llama.cpp inference speed claim was made in v0.15. These controls do
not create production authentication or permission to send messages.
See [EXPERIMENT-V015.md](EXPERIMENT-V015.md) and
[docs/LOCAL-MODEL-RESILIENCE.md](docs/LOCAL-MODEL-RESILIENCE.md).

Example when an optional model is available on local loopback:

```sh
go run ./cmd/speeditout -listen 127.0.0.1:18761 \
  -local-model-url http://127.0.0.1:18913/v1/chat/completions \
  -local-model-name qwen3-1.7b -model-profile qwen3-1.7b-evidence \
  -model-budget 1s -model-timeout 30s \
  -model-failures 3 -model-cooldown 30s
```

A 1-second budget is a UX tradeoff and may cause all CPU-only inferences to
time out; it is not a recommendation for evaluating model quality. Use no
`-model-budget` for an offline, CPU-bound research benchmark.

## v0.16: policy, isolation and client contract

The Go engine applies a central, **fail-closed** decision policy; the research
HTTP server refuses all non-loopback listener addresses. Both model-assisted
and deterministic outputs include a bounded `decision_audit` that cannot grant
permission to send a message. A strict TypeScript prototype for future mobile
and web clients validates the audit before presenting a proposed draft.

See [EXPERIMENT-V016.md](EXPERIMENT-V016.md) for test counts and limitations.
The client transport is not a real React Native app; the server has no
network-facing authentication or TLS and is unsuitable for a public service.


## v0.17: feedback with evidence categories

User feedback now distinguishes an apparently misheard message, wrong intent,
rejected suggestion, insufficient evidence, uncertainty, and helpful responses.
Recording needs explicit consent; categories and optional notes are written to
a local append-only journal. Feedback **does not update inference rules**.

The deletion endpoint uses a tombstone only: it removes an item from active
indexes but DOES NOT erase historical plaintext bytes. The local research API
has no user authentication or TLS. Do not use real personal data. See
[EXPERIMENT-V017.md](EXPERIMENT-V017.md) and
[docs/FEEDBACK-RESEARCH.md](docs/FEEDBACK-RESEARCH.md).

## v0.18: compact the local feedback journal

An **operator-only** `cmd/memorycompact` tool can rewrite the research JSONL
journal, removing tombstoned feedback from the current file while preserving
remaining preferences, examples and active feedback. This is opt-in and
not exposed over HTTP. It does **not** securely erase backups or old SSD
sectors. See [EXPERIMENT-V018.md](EXPERIMENT-V018.md) for tests and limits.
