# Speedit out v0.15 — evidence provenance, bounded latency and failure behavior

## Failures FIRST

- **6/24 first-run post-code-freeze scenarios failed; 18/24 passed** for v0.15.
  The unchanged v0.14 baseline on those cases failed **8/24** and passed
  **16/24**. Two cases improved, zero became newly incorrect on these
  developer-authored synthetic inputs. The v0.15 code was not retuned after
  inspecting these 24 outcomes.
- **7/28 pre-code cases failed; 21/28 passed** on v0.15, vs **8/28 failed;
  20/28 passed** before changes. These cases were written before implementation
  and then available during development, so treat them as *development* data.
- **0/179 Go tests/subtests failed; 179/179 passed** under
  `go test -race ./... -count=1`. `go vet ./...` passed.
- **0/57,028 real local HTTP requests failed; 57,028/57,028 passed** across
  four independent client processes and 256 configured connections, on one
  shared Linux machine; aggregate throughput observed about **11,319.6 RPS**.
  This is a stress/check run, **not** directly comparable with old throughput
  figures taken under different host contention and code versions.
- **0 newly incorrect individual cases on 9/9 historical sets** against the
  frozen v0.13 Go-only reference; the two authority sets each improved by one
  case. These sets were previously examined and are not prospective evidence.

## Milestones delivered

1. Created and recorded a 28-case baseline before code edits.
2. Inbound HTTP request JSON now rejects duplicate keys anywhere, ambiguous
   escaped duplicates, oversized/unbounded structures and invalid UTF-8.
3. Correct HTTP media-type validation (`application/json; charset=utf-8` is
   accepted, `application/jsonp` is not).
4. Added opt-in per-request local-model inference budget, without changing
   default offline behavior.
5. Added opt-in local-model consecutive-failure cooldown, concurrency-safe
   counters and a read-only metrics snapshot.
6. Added evidence substring provenance: repeated citations and overlap
   between opposing hypotheses are rejected, even if the JSON is valid.
7. Prevented quoted text, including single/curly quotes, being interpreted as
   direct-user evidence in the authority verifier.
8. Added a narrow Go-only quoted-order guard: reported imperatives inside
   literal quotations are not direct-user instructions; independent unquoted
   user commands retain their ordinary approval flow.
9. Added an evaluation JSONL journal, flushed and synced per completed case,
   with corpus SHA-256 and explicit baseline/hybrid phases.
10. Added unit, HTTP, cancellation, concurrency and circuit-recovery tests.
11. Froze and ran 24 *new* synthetic cases after code freeze. Did not retune
    based on their observed failures.
12. Repeated per-case historical regression checks; no new per-case errors.
13. Ran four external concurrent HTTP clients and checked persistence and
    explicit-consent memory isolation across a service restart.
14. Cross-compiled five Windows amd64 Go binaries, checked their PE format,
    and validated the Bash script syntax. Cross-compilation does **not** mean
    the binaries were run or tested under actual Windows in this session.

## What this does NOT demonstrate

- No Qwen GGUF *inference* or llama.cpp execution was performed in this v0.15
  round. Real GGUF availability is unchanged, but model performance is **not**
  evaluated by synthetic/mock HTTP tests.
- No full natural language understanding, true agentic reasoning or learning
  algorithm that mutates its own policy has been implemented.
- No production security claims: the HTTP service still has no authentication
  or TLS and should remain bound to loopback with synthetic data.
- Both 24-case and 28-case sets were authored by the implementer. They are not
  representative samples of people in the real world or a statistical proof
  of improved generalized reasoning.
- A rejected model citation can be a model failure or an evidence-validator
  limitation; it is not necessarily evidence of the user's intent.

## Reproduce the observations

```sh
go test -race ./... -count=1
go vet ./...

# Labelled synthetic Go-only evaluation, no local model endpoint:
go run ./cmd/eval -cases cases/v015-prospective-before-code.json
go run ./cmd/eval -cases cases/v015-postfreeze-new.json \
  -journal results/reproduce-local.jsonl

# Compare historical per-case classifications with frozen v0.13:
go run ./cmd/regress -baseline results/v013-go-regressions.json
```

`results/v015-before-code.json`, `results/v015-after-code.json`,
`results/v015-postfreeze-v014-baseline.json` and
`results/v015-postfreeze-v015-first.json` preserve actual per-case decisions.
The compact Go race and client load summaries are in `results/`.

Next: optimize true Qwen execution with a reference-quality local engine such
as llama.cpp, validate generated citations, and evaluate with human-consented,
genuinely independent samples. Do not move the model into the default path
solely because some synthetic benchmark cases improved.

**Humildade diante das evidências. Acima de tudo.**
