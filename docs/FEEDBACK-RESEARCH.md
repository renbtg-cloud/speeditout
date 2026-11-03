# User feedback, consent, and evidence boundaries — research v0.17

The interpreter must distinguish a rejected interpretation from insufficient
evidence, uncertain user intent, and audio that was potentially misheard.
These are **user-reported assessments**, not factual verification.

The loopback-only research API accepts explicit consent for
`POST /v1/feedback`, with categories `misheard`, `wrong_intent`,
`rejected_suggestion`, `insufficient_evidence`, `uncertain`, and `helpful`.
An optional short `reference` can match an app-local request ID; notes are
optional and never inserted automatically from audio/transcripts. A new record
is fsynced to the same append-only JSONL memory file used by the prototype.

The scoped `POST /v1/feedback/summary` endpoint returns category counts without
notes. `POST /v1/feedback/delete` appends a tombstone and removes the item from
active indexes, with an explicit `journal_tombstone_only` response. **This is
not secure deletion:** prior plaintext remains in the file until a separate
compaction/erasure mechanism exists, and may remain in filesystem backups.

These endpoints are **not suitable for production**. There is no session
identity, authentication, or TLS, so user IDs are not enforceable authorization.
The server now refuses binding to non-loopback addresses. Do not use real
personal data in the current research implementation.

Important: `FeedbackSummary` does not train the model or alter interpretation.
That must remain a separate, empirically validated, user-authorized workflow.

Example body:

```json
{"workspace":"synthetic","user_id":"local-u","category":"insufficient_evidence","reference":"req-42","consent":true}
```
