# Speedit out v0.16 — decision invariants, network isolation, typed client

## Failures first; test provenance

- **0/186 Go tests/subtests failed; 186/186 passed**, across four test-bearing packages, using `go test -race -short -json ./... -count=1`. The counts include subtests. Other build-only packages passed compilation. `go vet ./...` passed.
- **0/4 TypeScript client contract tests failed; 4/4 passed** under Node 22 and TypeScript; these test parsing, contradictory approval claims, invalid response structures and HTTP transport rejection.
- **0/28,620 live local HTTP requests failed; 28,620/28,620 passed**, 32 concurrent clients, three seconds, fixed synthetic workload, Go-only, median 1.94 ms, p99 14.76 ms, ~9,537 RPS.
- **0/9 historical corpora showed a case-level regression** compared to frozen v0.13 outputs. Two corpora gained one correction each; pre-existing mistakes remain, including 13/48 failures in the v0.9 post-freeze set.
- 10,000 seeded synthetic mutations check invariants; these are **not 10,000 independent semantic accuracy examples**.

## What was built

1. A central fail-closed `DecisionAudit` and decision policy: proposals require explicit approval; the engine cannot claim to have externally sent messages. A rejected policy result becomes a clarification. Audit describes the decision status, **not real-world authorization**.
2. Caller cancellation and optional inference budget now cover both navigation and evidence-verification stages. Optional model failure must not prevent the independent Go core from returning.
3. The HTTP research server refuses non-loopback listener addresses (including `0.0.0.0` and `::`). No authentication or TLS exists; this is **not a deployable public/mobile backend**.
4. Strict TypeScript response parsing and four contract tests for eventual React Native/web clients. Malformed permissions and fabricated model statuses cause an exception, never a draft.
5. CI configuration for Linux/Windows Go tests, plus a pinned TypeScript check. Local test success is not proof a newly changed remote CI workflow completed.
6. The Go load generator reports v0.16 instead of its stale v0.9 version label.

## Throughput caveat

A separate run of the unmodified v0.15 Go server with the same load generator and 32 clients, three seconds, achieved **0/34,544 failed; 34,544/34,544 passed**, ~11,512 RPS. The v0.16 run achieved ~9,537 RPS, about **17% less throughput** in these two short, non-isolated trials. The CPU was shared and results were not replicated; do not attribute this difference precisely to the decision policy. Neither run used model inference or measured human language quality.

## Remaining problems

- This is a bounded heuristic system. It still misreads many indirect speech constructions. No new unseen human-user benchmark has been conducted.
- The TypeScript client is a transport prototype, not a deployable React Native application and not a user approval UI.
- The optional Qwen inference runtime is still very slow on CPU; v0.16 makes **no new Qwen speed or reasoning-quality claim**.
- The endpoint must stay on loopback until authentication, TLS and identity/consent architecture are implemented and tested.

## Reproduce

```sh
go test -race -short ./... -count=1
go vet ./...
go run ./cmd/regress -out results/local-regressions.json
tsc -p clients/react-native-web
node --test clients/react-native-web/tests/*.cjs
```

The complete raw metrics are in `results/v016-race-summary.json`, `results/v016-final-regressions.json`, `results/v016-http-32clients.json` and `results/v016-v015-http-comparison.json`.
