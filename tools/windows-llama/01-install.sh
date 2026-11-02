#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "$0")/common.sh"
check_windows_gitbash
refresh_winget_path
if command -v llama >/dev/null 2>&1 || command -v llama.exe >/dev/null 2>&1 || command -v llama-server.exe >/dev/null 2>&1; then
  info 'llama ja encontrado. Nao vou reinstalar.'
  exit 0
fi
winget_cmd="$(winget_command)" || fail 'winget nao encontrado. Veja README: instalacao manual via GitHub Releases.'
info 'Instalando llama.cpp precompilado para Windows (sem Visual Studio e sem compilar).'
"$winget_cmd" install --exact --id ggml.llamacpp --source winget --accept-source-agreements --accept-package-agreements
info 'Instalacao concluida. Nao precisa fechar o Git Bash: 02-serve.sh localiza o executavel.'
