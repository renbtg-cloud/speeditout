# Speedit out v0.9 — experimental report

## Failure-first results

| Comparison set | v0.8 failures / total | v0.9 failures / total | v0.9 passes / total | Interpretation |
|---|---:|---:|---:|---|
| **New post-freeze set** | **17/48** | **13/48** | **35/48** | First-run evaluation; 4 net improvements, 0 regressions |
| Earlier 60-case set | 29/60 | 0/60 | 60/60 | Examined and used to debug v0.9; NOT blind |
| Prior v0.8 48-case set | 19/48 | 0/48 | 48/48 | Earlier errors were known; NOT blind |
| Prior 32-case set | 0/32 | 0/32 | 32/32 | Development / regression data |

The 48-case post-freeze set was written by the same developer who wrote the implementation and comprises artificial Portuguese examples. A result on these sentences is **not** evidence of general human intent understanding. Its 13 remaining failures include missing game-rejection/confusion paraphrases, missing blame or ghosting-definition paraphrases, nuanced work deadline constraints and one attributed mechanic account. Avoid promoting by repeatedly fitting its errors.

## Concurrent performance (same local host, 64 clients, four seconds)

| Version | HTTP failures / total | Passes / total | Req/s | p99 |
|---|---:|---:|---:|---:|
| v0.8 | 0/152,878 | 152,878/152,878 | 38,193.6 | 7.15 ms |
| v0.9 | 0/107,876 | 107,876/107,876 | 26,952.7 | 9.87 ms |

The 0.9 evidence parsing/trace reduces observed throughput by about 29% in this local workload. These are sequential local experiments on shared CPU; comparison is illustrative, not a production SLA or a statistically controlled microbenchmark. The HTTP load generator uses the 83 original development scenarios and checks the expected categories. No LLM inference was measured here. An initial comparison attempt failed because the baseline executable path was wrong; that invalid setup attempt was discarded and rerun with the correct binary; this is not silently counted as a successful workload.

## Engineering and provenance

- The owned Go interpretation system continues to run offline, without competitor APIs.
- Existing optional local GGUF/Qwen runtime and adapter are included, **but not rerun for inference in v0.9**. No claim of Qwen improvement is made.
- Tests in `engine/semantic_test.go` exercise trace serialization, attribution, explicit prohibitions, memory personalization, approval, and concurrent calls.
- Final `go test -race ./...` and `go vet ./...` outputs are preserved in `results/`.
- Data, source SHA-256 checksums, per-example evaluation records and both HTTP performance reports are preserved.
- The research HTTP server has no authentication/TLS; bind loopback, use synthetic material only. Do not deploy to the Internet or ingest real private chats.

## Next scientific step

Use an independently assembled human-labeled prospective corpus, including multiple culturally distinct Portuguese varieties, and distinguish inter-annotator disagreement from classifier errors. Add structured Qwen hypotheses with evidence spans; a verifier should contest unsupported claims and abstain on consequential ambiguity. Compare identical underlying Qwen with and without the Speedit out orchestration under matched context and measured latency. Do not tune on the final holdout.
