# Speedit out v0.18 — local journal compaction, logical deletion, and recovery

## Failures first

- **0/192 Go tests/subtests failed; 192/192 passed** with `go test -race -short -json ./... -count=1` in four test-bearing packages. `go vet ./...` passed.
- **0/9 historical corpus regressions**, compared per case against the frozen v0.13 outputs (prior failures remain).
- **0/8 TypeScript client tests failed; 8/8 passed** (same client contracts as v0.17; no new UI).
- The end-to-end CLI smoke test successfully rewrote a synthetic two-entry JSONL journal from **227 bytes to 31 bytes** and removed the tombstoned synthetic note from the active file.
- A concurrent compaction test interleaved journal rewrites with **400/400** consented synthetic feedback submissions, recovered all 400 after restart, with **0/400 lost**. This is not a durability guarantee under power failure, filesystem corruption or hostile processes.

## What was implemented

1. An explicit, operator-only `cmd/memorycompact` CLI with a safe dry-run default and `-apply` for a real atomic replacement of the ACTIVE JSONL journal. It does not open an HTTP endpoint.
2. Journal rewrite with `0600` permissions, deterministic record order, `fsync` of the replacement file and a best-effort parent directory sync. In-memory locking prevents concurrent appends during replacement.
3. Preservation of current preferences, approved examples, active feedback, and a monotonic ID counter. A test verifies all survive a restart even when the highest previous feedback ID was deleted.
4. A compacted journal no longer contains logically deleted feedback or its note in the ACTIVE filesystem path. The old journal is replaced atomically; the tool does not deliberately retain a backup copy.
5. The normal feedback endpoint still only adds tombstones. Operators must run compaction explicitly if they want to eliminate those records from the active file.

## Crucial limitations

- **Not secure erasure.** Backups, snapshots, SSD wear-leveling, journaled filesystem blocks or open file handles can retain bytes. The tool makes no guarantee about those physical copies.
- Recovery from a mid-write hard power loss or corrupt filesystem was not tested; the replacement strategy reduces risk but is not a formally verified storage engine.
- Windows binaries were cross-compiled; the atomic replacement path on a real Windows filesystem has not been exercised here.
- This remains a single-node, loopback-only, unauthenticated research process. Do not store real user data.
- No new inference improvement or Qwen latency claim was made in v0.18.

## Reproduce

```sh
go test -race -short ./... -count=1
go vet ./...
go run ./cmd/regress
# Operator must provide an EXISTING JSONL file, backed up according to their policy:
go run ./cmd/memorycompact -path /local/path/to/memory.jsonl
# Explicit destructive rewrite of the active file, not a secure physical erase:
go run ./cmd/memorycompact -path /local/path/to/memory.jsonl -apply
```
