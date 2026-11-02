#!/usr/bin/env bash
# Only for experimental Linux x86_64 CPU direct-GGUF matvec, NOT Windows.
set -euo pipefail
cd "$(dirname "$0")/.."
if [[ "$(uname -s)" != "Linux" ]]; then
    echo 'Native C++ kernel recipe is Linux only; use llama.cpp on Windows.' >&2
    exit 2
fi
command -v g++ >/dev/null || { echo 'g++ required' >&2; exit 2; }
g++ -O3 -march=native -fopenmp -shared -fPIC \
    -o runtime/libviper_matvec.so runtime/native_matvec.cpp
printf 'Built runtime/libviper_matvec.so (local experimental kernel)\n'
