# Speedit out v0.11 — authority and quotation scope

## Failure counts first

This release explores whether the interpreter mistakes **reported speech** for a **user instruction**. All scenarios are synthetic, created by the implementer; none are human-subject evidence. The tests compare the same exact intent/decision labels, not response prose quality.

| Set | v0.10 failures / total | v0.11 failures / total | Independence |
| --- | ---: | ---: | --- |
| Authority development (24) | 7/24 | 0/24 | Used for v0.11 development; **not independent** |
| Prospective set (12) | 5/12 | 1/12 initially | First execution was before tuning to its errors |
| Prospective set (12) after correction | 5/12 | 0/12 | **Contaminated after inspection** |
| Second post-freeze set (16) | 6/16 | 3/16 initially | First execution was before tuning to its errors |
| Second post-freeze set (16) after correction | 6/16 | 0/16 | **Contaminated after inspection** |
| Final post-code-freeze set (16) | **7/16** | **4/16** | New, developer-authored scenarios; no post-result changes |

In the final 16, v0.11 passed **12/16**, compared with **9/16** for v0.10. The four misses are **P11-011, P11-013, P11-014, P11-015**: two unrecognized reporting constructions and two unrecognized user-level refusals. These are not solved and should inform a later, separately evaluated version.

Existing historical corpora were re-run without regression in exact intent/decision counts:

| Historical corpus | v0.11 failures / total |
| --- | ---: |
| holdout-v1 | 7/30 |
| holdout-v2 | 0/32 |
| blind-after-freeze-v07 | 0/64 |
| blind-after-freeze-v08 | 0/48 |
| final-postfreeze-v09 | 13/48 |
| prospective-v07-frozen | 0/48 |
| prospective-v09-frozen | 0/60 |

Go race-detector run: **0/120 tests and subtests failed; 120/120 passed**. All three test-bearing packages passed; `go vet ./...` also passed.

## What changed

- A bounded Go *authority preflight* now checks quoted material, explicitly reported instructions and hypothetical examples before older lexical shortcuts can turn them into proposed drafts.
- A separate and explicit user directive in another clause remains actionable, subject to existing draft approval.
- Explicit questions about whether one should write are not authorization to write.
- Expanded a couple of user-refusal patterns, preserving the distinction between *talking about* a message and *asking to draft* it.
- Returns a structured `SemanticTrace` with a reason when an apparent instruction is blocked.

## Known limits

- **This is a scoped rule-based parser, not generalized natural-language comprehension.** The final 4/16 failure result is direct evidence of this limitation.
- Reported-speech detection uses finite vocabulary and limited clause splitting. Indirect speech, complex nesting, non-standard slang, and ambiguous speaker changes remain vulnerable to misclassification.
- An LLM-assisted navigator is optional in the repository but was **not executed** or benchmarked in this v0.11 study. These are Go-only results.
- No automatic email sending is implemented. An internal `propose_draft` still requires user approval; even this weaker mistake must be measured.
- Test labels are manually authored assumptions about intended intent, not a validated benchmark from independent users.

## Reproduce

```sh
go test -race ./... -count=1
go vet ./...
go run ./cmd/eval -cases cases/authority-scope-v011-dev.json
go run ./cmd/eval -cases cases/authority-scope-v011-prospective.json
go run ./cmd/eval -cases cases/authority-scope-v011-final-probe.json
```

A bigger model is not a substitute for identifying **who actually authorized an action**. The next investigation should test more expressive evidence structures and independent language samples without simply adding keywords until a known dataset reaches 100%.
