#!/usr/bin/env bash
# Works in Git Bash and Linux. No jq, sudo, Docker, or local Qwen required.
set -euo pipefail
cd "$(dirname "$0")/.."
command -v go >/dev/null || { printf >&2 'Go 1.23+ required\n'; exit 2; }
printf 'Go tests:\n'
if [[ "${SPEEDIT_RACE:-0}" == "1" ]]; then
    go test -race ./... -count=1
else
    go test ./... -count=1
fi
printf 'Static analysis:\n'
go vet ./...
printf 'Frozen historical regressions:\n'
go run ./cmd/regress -baseline results/v013-go-regressions.json -out "${SPEEDIT_REPORT:-/dev/null}"
