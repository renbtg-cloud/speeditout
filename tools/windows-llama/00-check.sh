#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "$0")/common.sh"
check_windows_gitbash
refresh_winget_path
printf 'Terminal: Git Bash on Windows (%s)\n' "$(uname -s)"
printf 'Architecture: %s\n' "$(uname -m)"
printf 'Home: %s\n' "$HOME"
printf 'Results: %s\n' "$LLAMA_LOG_DIR"
printf 'Threads: %s; context: %s; port: %s\n' "$LLAMA_THREADS" "$LLAMA_CTX" "$LLAMA_PORT"
for cmd in curl git cygpath; do
  if command -v "$cmd" >/dev/null 2>&1; then printf '%-12s OK\n' "$cmd"
  else printf '%-12s MISSING\n' "$cmd"; fi
done
if wc="$(winget_command)"; then printf 'winget       %s\n' "$wc"; else printf 'winget       MISSING\n'; fi
if command -v llama >/dev/null 2>&1; then printf 'llama        OK (%s)\n' "$(command -v llama)"
elif command -v llama.exe >/dev/null 2>&1; then printf 'llama.exe    OK\n'
elif command -v llama-server.exe >/dev/null 2>&1; then printf 'llama-server.exe OK\n'
else printf 'llama        not on PATH yet\n'; fi
if [[ -n "$LLAMA_MODEL_FILE" ]]; then
  [[ -f "$LLAMA_MODEL_FILE" ]] || fail "GGUF ausente: $LLAMA_MODEL_FILE"
  printf 'Modelo local: %s\n' "$LLAMA_MODEL_FILE"
  command -v sha256sum >/dev/null 2>&1 && sha256sum "$LLAMA_MODEL_FILE" || true
else
  printf 'Modelo no primeiro uso: Hugging Face %s\n' "$LLAMA_HF_REPO"
fi
if command -v powershell.exe >/dev/null 2>&1; then
  printf '\nRAM do Windows, em GiB:\n'
  powershell.exe -NoProfile -Command '$os=Get-CimInstance Win32_OperatingSystem; Write-Output ("Total: {0:N2}; livre: {1:N2}" -f ($os.TotalVisibleMemorySize/1MB),($os.FreePhysicalMemory/1MB))' | tr -d '\r'
fi
printf '\nProximo: ./01-install.sh (se ainda nao instalou)\n'
