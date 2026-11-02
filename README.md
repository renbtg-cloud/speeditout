# Speedit out

**Dodge Viper — experimental Go human↔AI communication interpreter.**

*Speedit out* is the product name (previously "Spititout"). Its goal is to
help people express what they mean, and understand what the machine means,
without requiring polished language or prompt-engineering skills.

This repository holds the working prototype up through **v0.11.0**. It is
research software, **not** a production service or a deployed React Native app.

## What exists

- Go HTTP interpretation engine with deterministic fallback, bounded semantic
  evidence frames, attribution/quotation guards, and explicit limits on what the model can decide.
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
See [EXPERIMENT-V011.md](EXPERIMENT-V011.md) for the new authority-scope trial and its unsolved failures.

## Experimental integrity

The published results are **synthetic** and do not establish general human
communication understanding or superiority over a frontier language model.
Earlier case sets were examined during iteration; see the historical
experiment reports for proper baselines. No GGUF model, compiled binaries,
secret keys or local databases belong in this repository.

**Security:** no authentication or TLS; bind to loopback and use synthetic
inputs only. Not suitable for deployment on a public interface.

Guiding principle: *Humildade diante das evidências. Acima de tudo.*
