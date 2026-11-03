# Speedit out v0.19 — live Go/TypeScript contract and benchmark instrumentation

## Failures first

- **0/9 TypeScript checks failed; 9/9 passed**, including the NEW integration test which starts the real local Go HTTP server, interprets a synthetic request, records user-consented feedback and deletes it logically.
- The Go runtime and HTTP approval audit were exercised in the same test. `external_calls` stayed zero and the draft still required approval.
- The new CPU microbenchmarks complete, but results are **noisy** on the shared host. This release does not claim that default performance improved or regressed.

## Storage-hardening integrated from the local v0.18 follow-on work

- Single cooperating writer per journal using an exclusive local `.lock`
  marker; refuses concurrent process opens and deliberately does not auto-steal
  a stale marker after crashes.
- Strict journal record validation: unknown entry types and malformed envelope
  combinations fail startup instead of being silently discarded by compaction.
- Failure injection proves that a failed atomic rename retains the original
  journal and recovers the open writer; tests reject pretending a disconnected
  file journal was persisted. These are bounded tests, not a full filesystem
  fault-injection campaign.
- These improvements were present in a newer local v0.18 tree but were **not
  included in the initial v0.18 GitHub upload**. This release includes and
  tests them explicitly.

## New milestones

1. Client integration is no longer limited to stubs. The Node test launches the **actual Go service** on a free loopback port and tests the request/response protocol, required approval, feedback categories, and tombstone result.
2. Added a CI client contract job that compiles TypeScript, builds the Go backend and runs the same live integration checks on Ubuntu.
3. Added `BenchmarkInterpretFixed`, `BenchmarkInterpretReported`, and `BenchmarkInterpretParallel` to compare deterministic Go paths without network or model inference.
4. Added `scripts/benchmark-engine.sh` for repeatable runs with a fixed `GOMAXPROCS` and multiple samples. The measurements are descriptive only; CI does not gate on unstable timing thresholds.

## Measurement caution

A preliminary three-repeat run on the shared Linux host showed wide timing variation even on identical code. The v0.15 fixed-prompt path ranged roughly **40–67 µs/op** and the v0.19 counterpart **42–49 µs/op**. Other cases ranged more widely. This is insufficient to prove a consistent 17% HTTP throughput regression or any opposite improvement. The historical HTTP figures remain observations of separate short tests, not a controlled throughput comparison.

## Security limitations

- The integration test uses synthetic data only and runs on loopback. The real mobile/web app is not implemented.
- No actual phone, web browser session, user authentication or TLS has been tested.
- Tombstone-only feedback removal is still not secure erasure from backups or disks.
- No Qwen/llama.cpp model inference was used here; no model-quality claim follows.

## Reproduction

```sh
go test -race -short ./... -count=1
go vet ./...
go run ./cmd/regress
tsc -p clients/react-native-web
go build -o /tmp/speeditout-service ./cmd/speeditout
SPEEDITOUT_SERVER_BINARY=/tmp/speeditout-service node --test clients/react-native-web/tests/*.cjs
bash scripts/benchmark-engine.sh
```
