# Journal compaction — local research tooling (v0.18)

The feedback endpoint writes plaintext into an append-only JSONL journal. Deleting an entry writes a tombstone: active APIs stop returning it, but **the old bytes stay in the journal** until the operator performs compaction.

## Offline maintenance workflow

1. Stop all processes using the journal. The journal now has an exclusive `<filename>.lock` owner file. The tool will refuse to open the journal while another owner exists.
2. Before and after compaction, separately handle backups and snapshots as required by your storage policy. **Never interpret compaction as secure erasure.**
3. From the Speedit out source directory, run `go run ./cmd/memorycompact -path /absolute/path/journal.jsonl` to preview counts without changing the file.
4. Run `go run ./cmd/memorycompact -path /absolute/path/journal.jsonl -apply` to compact. This writes a same-directory temporary file, syncs it and replaces the active journal. The process holds its mutex during the operation. Reopening it and appending further feedback has dedicated tests.
5. Repeat the dry run to check the new file length. Restart the server only after the compaction tool exits.

The lock marker is created with O_EXCL and is **not automatically stolen**, including if the recorded PID no longer exists: PID reuse, cross-host filesystems and partially alive processes make automatic recovery unsafe. After a crash, an operator must independently verify that no writer still exists before removing `<filename>.lock`. This is **not** a general distributed lease or a guarantee against processes that ignore the lock file.

For Windows/Git Bash, use `go run ./cmd/memorycompact -path 'C:/path/to/journal.jsonl' [-apply]`. It requires Go 1.23+ and should run without Cygwin tools. The Windows-specific filesystem replacement semantics have not been validated in a live Windows run; test with disposable data first.

## Guarantees and non-guarantees

- Expected: active preferences, approved examples, active feedback and a monotonic ID counter survive the rewrite, and logical deletions vanish from the **active file**.
- Expected: each single-file journal has at most one cooperating process writer. Concurrent goroutines within a process are serialized by a mutex.
- Not guaranteed: secure disk-sector erasure, removing data from backups, authenticated user identity, encryption at rest, durable atomicity across every network or Windows filesystem, protection from noncooperating processes, recovery from arbitrary corruption, or a full ACID transaction system.
- Do not compact live production or user data: this is experimental local storage and is **not** an operational privacy/erasure solution.
