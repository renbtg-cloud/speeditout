# Speedit out v0.17 — structured user feedback without automatic learning

## Failures first

- **0/189 Go tests/subtests failed; 189/189 passed** with `-race -short`, in four test-bearing packages. `go vet ./...` passed.
- **0/8 TypeScript transport tests failed; 8/8 passed** under Node 22. This includes consent rejection and dishonest server confirmation rejection.
- **0/9 tracked historical corpora showed any new case-level regression** versus frozen v0.13 decisions; those corpora still contain previously documented errors.
- API and memory tests include concurrent submissions, process restart replay, cross-user isolation and attempts to delete another user's entry.

## Changes

1. `POST /v1/feedback`: explicit opt-in recording of one of six categories: `misheard`, `wrong_intent`, `rejected_suggestion`, `insufficient_evidence`, `uncertain`, `helpful`.
2. Strict category, per-field and note-length checks. The client does **not** attach transcripts or audio by default. Optional notes are plaintext; synthetic inputs only.
3. Feedback records appended and fsynced to the existing experimental JSONL memory journal. Reload preserves categories and event IDs.
4. `POST /v1/feedback/summary` returns counts only; `POST /v1/feedback/delete` records a tombstone so the record is no longer active. Cross-user deletion is rejected according to the *supplied* scope, but **there is no authenticated identity**.
5. React Native/web TypeScript research transport handles consent explicitly and refuses server confirmations implying automatic learning or pretending the journal was securely erased.
6. Feedback is **not used for classification, personalization, model training or rule updates**. This is an observation and human-control substrate, not evidence of learning capability.

## Security and privacy limitations

- Research server is bound to loopback only. No TLS, sessions or real identity; therefore this is NOT a secure multiuser API.
- Tombstones are **logical removal**, not physical erasure. Original plaintext bytes can remain on disk or in backups. Do not submit real private notes or recordings.
- A malicious local process can impersonate any supplied user ID. Future mobile and web apps require separately designed and tested identity/authorization and secure retention/deletion.
- No inference or performance benchmark of the Qwen was performed in this release.

## Reproduce

```sh
go test -race -short ./... -count=1
go vet ./...
go run ./cmd/regress
tsc -p clients/react-native-web
node --test clients/react-native-web/tests/*.cjs
```

## Next engineering decisions

Establish authenticated identity before multiuser remote access, implement atomic journal compaction/secure retention (and state backup limitations), and validate a feedback-to-learning pipeline only after supervised, consent-based evaluation. Continued classification improvements require genuinely new samples instead of tuning solely to known cases.
