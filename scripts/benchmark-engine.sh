#!/usr/bin/env bash
# Reproducible CPU-only microbenchmark (no LLM, HTTP, network or production claims).
set -euo pipefail
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo"
: "${GOMAXPROCS:=4}"
export GOMAXPROCS
if [[ -z "${SPEEDITOUT_BENCH_OUTPUT:-}" ]]; then
    SPEEDITOUT_BENCH_OUTPUT="$repo/results/local-engine-benchmark.txt"
fi
mkdir -p "$(dirname "$SPEEDITOUT_BENCH_OUTPUT")"
go test -run '^$' -bench '^BenchmarkInterpret' -benchtime=500ms -count=3 ./engine | tee "$SPEEDITOUT_BENCH_OUTPUT"
printf '\nInterpretation quality is NOT measured by this benchmark. Shared CPU load may dominate the result.\n'
