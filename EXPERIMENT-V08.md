# Speedit out — v0.8, scoped interpretation and generalization audit

## Results — failures first

| Corpus | Failed | Passed | Validity / interpretation |
|---|---:|---:|---|
| Earlier 32-case development | 0/32 | 32/32 | Inspected and tuned; not blind |
| Previously frozen 48 | 0/48 | 48/48 | Inspected and tuned during v0.7; not blind anymore |
| Additional 64 cases written after initial v0.7 fixes | 0/64 | 64/64 | Used to develop v0.8; not blind anymore |
| **New 48-case post-v0.8 freeze, v0.7** | **20/48** | **28/48** | Fresh comparison for both versions |
| **New 48-case post-v0.8 freeze, v0.8** | **19/48** | **29/48** | Fresh comparison for both versions |

The fresh 48 had exactly **one net improvement and no regressions** when running v0.8 instead of v0.7, on exact intent/decision category agreement. They were authored by the same developer and are still artificial paraphrases, not an independent human study. The v0.8 source was frozen before that evaluation; subsequently only a scoped-draft personalization pass-through and unit test were added (no rule classifier edits). All final categorical evaluation files were regenerated after this bug fix. Hashes of the scenario file and final source are in `results/`.

The 29/48 result is the critical counterweight to the apparently perfect development corpora. It demonstrates substantial lexical brittleness and limited generalization. **Do not use the 32/32, 48/48, or 64/64 results as evidence of broad human-intent understanding.**

## What was implemented

- V0.7 scope-aware preflight retained and repaired; N07 second-person attribution, third-party deadline reports, questionnaire-vs-action, negated dates, reported car symptoms.
- V0.8 extended coverage for explicit self-rejection and confusion, correct assignment of third-party views, additional first-person ghosting patterns, work refusals and explicit deadline caveats, mechanic reporting and resolved symptoms.
- Specific bug fixed: the preflight previously bypassed consented per-recipient greeting/avoid-phrase preferences; added a regression test to keep preferences and approval semantics.
- No external API was called; the deterministic Go core remains operational without the Qwen bridge. Existing optional GGUF PyTorch/BF16 research runtime is still provided, but **was not rebenchmarked with the v0.8 classifier**.

## Engineering checks

- `go test -race ./... -count=1` passed, see `results/v08-go-test-race.txt`.
- `go vet ./...` passed, see `results/v08-go-vet.txt`.
- Six GGUF tensor/tokenizer/architecture checks passed, see `results/v08-gguf-tests.txt`. These are *not* full-model numerical equivalence against llama.cpp.
- Updated Linux amd64 binaries and their hashes are included. These checks are neither production load testing nor a security audit.

## Reproduction

```sh
go test -race ./... -count=1
go vet ./...
python3 -m unittest -v runtime/test_gguf_runtime.py
go run ./cmd/eval -cases cases/holdout-v2.json -out results/holdout-v2-v08-final.json
go run ./cmd/eval -cases cases/blind-after-freeze-v08.json -out results/blind-after-freeze-v08-final.json
```

## Next critical engineering step

Do not keep endlessly adding synonym lists to improve a synthetic corpus. Add a typed intermediate hypothesis representation (speaker, target, action, negation, source, quotation, temporal commitment and explicit constraints), plus a small *local* model as optional hypothesis generator. Check for contradicting evidence and ask the human when ambiguity is consequential. Use new paraphrases and real consented human corrections in a clean, prospective holdout. Compare against the SAME Qwen working alone and Qwen+Speedit out; separate input-context advantage, orchestration benefit, cost and latency. Never infer evidence from mere confidence of an LLM.
