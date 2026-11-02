#!/usr/bin/env bash
set -euo pipefail
KIT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SPEEDIT_LLAMA_HOME="${SPEEDIT_LLAMA_HOME:-$HOME/speeditout-llama-windows}"
LLAMA_LOG_DIR="${LLAMA_LOG_DIR:-$SPEEDIT_LLAMA_HOME/results}"
LLAMA_MODEL_FILE="${LLAMA_MODEL_FILE:-}"
LLAMA_HF_REPO="${LLAMA_HF_REPO:-ggml-org/Qwen3-1.7B-GGUF:Q4_K_M}"
LLAMA_ALIAS="${LLAMA_ALIAS:-speeditout-qwen3-1.7b}"
LLAMA_PORT="${LLAMA_PORT:-18913}"
LLAMA_THREADS="${LLAMA_THREADS:-4}"
LLAMA_CTX="${LLAMA_CTX:-2048}"
fail() { printf 'ERRO: %s\n' "$*" >&2; exit 1; }
info() { printf '[Speedit out] %s\n' "$*" >&2; }
need() { command -v "$1" >/dev/null 2>&1 || fail "Comando ausente: $1"; }
check_windows_gitbash() {
  case "$(uname -s)" in
    MINGW*|MSYS*) ;;
    *) fail 'Este kit foi feito para Git Bash no Windows. Nao execute no WSL/Linux.';;
  esac
}
check_numbers() {
  local x
  for x in LLAMA_PORT LLAMA_THREADS LLAMA_CTX; do
    [[ "${!x}" =~ ^[1-9][0-9]*$ ]] || fail "$x deve ser inteiro positivo"
  done
}
check_numbers
# Use native Windows paths when invoking Windows executables; prevent MSYS2 from
# reinterpreting Hugging Face identifiers as Unix paths.
model_args() {
  MODEL_ARGS=()
  if [[ -n "$LLAMA_MODEL_FILE" ]]; then
    [[ -f "$LLAMA_MODEL_FILE" ]] || fail "GGUF nao encontrado: $LLAMA_MODEL_FILE"
    if command -v cygpath >/dev/null 2>&1; then
      local winmodel
      winmodel="$(cygpath -w "$LLAMA_MODEL_FILE")"
      MODEL_ARGS=(-m "$winmodel")
    else
      MODEL_ARGS=(-m "$LLAMA_MODEL_FILE")
    fi
  else
    MODEL_ARGS=(-hf "$LLAMA_HF_REPO")
  fi
}
winget_command() {
  if command -v winget.exe >/dev/null 2>&1; then printf '%s' winget.exe
  elif command -v winget >/dev/null 2>&1; then printf '%s' winget
  else return 1; fi
}
# Git Bash does not inherit PATH changes made by WinGet while already open.
# Each of our scripts sources common.sh, so refresh known WinGet locations
# inside that script. No terminal restart and no changes to ~/.bashrc.
refresh_winget_path() {
  local win_dir unix_dir
  for win_dir in "${LOCALAPPDATA:-}/Microsoft/WinGet/Links" "${ProgramFiles:-}/WinGet/Links"; do
    [[ -n "$win_dir" ]] || continue
    if command -v cygpath >/dev/null 2>&1; then
      unix_dir="$(cygpath -u "$win_dir" 2>/dev/null || true)"
    else
      unix_dir="$win_dir"
    fi
    if [[ -d "$unix_dir" ]]; then
      case ":$PATH:" in *":$unix_dir:"*) ;; *) PATH="$unix_dir:$PATH" ;; esac
    fi
  done
  export PATH
  hash -r 2>/dev/null || true
}

# Prefer links installed by WinGet, then inspect the official package folder.
# A native Windows executable may be named either llama or llama-server.
find_runner() {
  refresh_winget_path
  if [[ -n "${LLAMA_SERVER_BIN:-}" ]]; then
    [[ -x "$LLAMA_SERVER_BIN" ]] || fail "LLAMA_SERVER_BIN nao executavel: $LLAMA_SERVER_BIN"
    SERVER_COMMAND=("$LLAMA_SERVER_BIN")
  elif command -v llama >/dev/null 2>&1; then
    SERVER_COMMAND=(llama serve)
  elif command -v llama.exe >/dev/null 2>&1; then
    SERVER_COMMAND=(llama.exe serve)
  elif command -v llama-server.exe >/dev/null 2>&1; then
    SERVER_COMMAND=(llama-server.exe)
  elif command -v llama-server >/dev/null 2>&1; then
    SERVER_COMMAND=(llama-server)
  else
    local win_root root pkg_dir found name
    for win_root in "${LOCALAPPDATA:-}/Microsoft/WinGet/Packages" "${ProgramFiles:-}/WinGet/Packages"; do
      if command -v cygpath >/dev/null 2>&1; then
        root="$(cygpath -u "$win_root" 2>/dev/null || true)"
      else
        root="$win_root"
      fi
      [[ -d "$root" ]] || continue
      for pkg_dir in "$root"/ggml.llamacpp*; do
        [[ -d "$pkg_dir" ]] || continue
        found="$(find "$pkg_dir" -maxdepth 5 -type f \( -iname 'llama-server.exe' -o -iname 'llama.exe' \) -print -quit 2>/dev/null || true)"
        [[ -n "$found" ]] || continue
        name="$(basename "$found")"
        if [[ "$name" == 'llama.exe' ]]; then SERVER_COMMAND=("$found" serve)
        else SERVER_COMMAND=("$found")
        fi
        return 0
      done
    done
    fail 'Executavel nao encontrado. Confirme a instalacao do WinGet ou defina LLAMA_SERVER_BIN (veja README).'
  fi
}
