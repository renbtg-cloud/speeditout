#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "$0")/common.sh"
check_windows_gitbash
find_runner
model_args
mkdir -p "$LLAMA_LOG_DIR"
info "API local: http://127.0.0.1:$LLAMA_PORT (Ctrl+C para parar)"
if [[ -z "$LLAMA_MODEL_FILE" ]]; then
  info "Sem GGUF local. Primeira execucao baixara $LLAMA_HF_REPO (internet necessaria)."
else
  info "GGUF local: $LLAMA_MODEL_FILE"
fi
# -ngl 0 explicitly chooses CPU; -np 1 prevents simultaneous large contexts.
# No exposure of model service to the LAN.
MSYS_NO_PATHCONV=1 exec "${SERVER_COMMAND[@]}" "${MODEL_ARGS[@]}" \
  --host 127.0.0.1 --port "$LLAMA_PORT" \
  --alias "$LLAMA_ALIAS" -c "$LLAMA_CTX" -np 1 -t "$LLAMA_THREADS" -ngl 0
