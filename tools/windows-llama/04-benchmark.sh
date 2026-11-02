#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "$0")/common.sh"
check_windows_gitbash
need curl
mkdir -p "$LLAMA_LOG_DIR"
[[ -f "$LLAMA_LOG_DIR/request.json" ]] || fail 'Rode ./03-test.sh primeiro.'
BASE="http://127.0.0.1:$LLAMA_PORT"
info 'Tres repeticoes sequenciais no mesmo servidor (nenhuma simultaneidade).'
printf 'run,wall_seconds,response_file\n' > "$LLAMA_LOG_DIR/benchmark.csv"
for i in 1 2 3; do
  answer="$LLAMA_LOG_DIR/bench-$i.json"
  elapsed="$(curl --noproxy '*' -fsS --connect-timeout 5 --max-time 240 \
    -H 'Content-Type: application/json' \
    --data-binary "@$LLAMA_LOG_DIR/request.json" \
    -o "$answer" -w '%{time_total}' "$BASE/v1/chat/completions")" || fail "Erro na repeticao $i"
  printf '%s,%s,%s\n' "$i" "$elapsed" "$answer" >> "$LLAMA_LOG_DIR/benchmark.csv"
  printf 'Rodada %s: %s s\n' "$i" "$elapsed"
done
info "CSV: $LLAMA_LOG_DIR/benchmark.csv"
info 'Para tokens/s, veja os campos timings dos arquivos JSON (quando fornecidos pelo servidor).'
