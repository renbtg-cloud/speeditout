"""Optional low-overhead CPU decode path for *single-token* Qwen3 inference.

This is NOT llama.cpp. It reuses the previous experimental forward pass, but
multiplies quantized GGUF matrices directly through the bundled C++ CPU kernel.
Prefill (>1 token) still uses the experimental BF16 path.

Build: g++ -O3 -march=native -fopenmp -shared -fPIC -o runtime/libviper_matvec.so runtime/native_matvec.cpp
"""
import ctypes
from pathlib import Path

import numpy as np
import torch
import torch.nn.functional as F

from compute_backends import RunnerBF16


class RunnerNativeDecode(RunnerBF16):
    def __init__(self, gguf, store, threads=4, library=None):
        super().__init__(gguf, store)
        self.threads = max(1, int(threads))
        self.library_path = Path(library or Path(__file__).with_name('libviper_matvec.so'))
        self.lib = ctypes.CDLL(str(self.library_path))
        self.kernel = self.lib.viper_matvec
        self.kernel.restype = ctypes.c_int
        self.kernel.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.c_int64, ctypes.c_int64,
                                ctypes.c_void_p, ctypes.c_void_p, ctypes.c_int]
        self._raw = {}
        self._single = False

    def _matrix(self, x, name):
        dims, qtype, _ = self.g.tensors[name]
        if qtype not in (12, 14) or len(dims) != 2 or dims[0] % 256:
            return None
        cols, rows = dims
        if x.numel() != cols:
            return None
        if name not in self._raw:
            self._raw[name] = self.g.get_raw(name)[2]
        w = self._raw[name]
        vec = x.detach().float().contiguous().view(-1).numpy()
        ans = np.empty(rows, dtype=np.float32)
        rc = self.kernel(w.ctypes.data, qtype, rows, cols, vec.ctypes.data,
                         ans.ctypes.data, self.threads)
        if rc:
            raise RuntimeError(f'native GGUF kernel failed for {name} ({rc})')
        return torch.from_numpy(ans).view(1, -1)

    def mat(self, x, name):
        if self._single:
            ans = self._matrix(x, name)
            if ans is not None:
                return ans
        return super().mat(x, name)

    def choice_logits(self, x, choices):
        """Avoid dequantizing the entire 151936-row output matrix for choices."""
        name = 'output.weight'
        dims, qtype, _ = self.g.tensors[name]
        if qtype not in (12, 14):
            raise RuntimeError('unexpected quantization type for output matrix')
        cols, rows = dims
        if any(i < 0 or i >= rows for i in choices):
            raise ValueError('choice id outside tokenizer vocabulary')
        raw = self.g.get_raw(name)[2]
        stride = (cols // 256) * (144 if qtype == 12 else 210)
        vec = x.detach().float().contiguous().view(-1).numpy()
        result = []
        for idx in choices:
            one = np.ascontiguousarray(raw[idx * stride:(idx + 1) * stride])
            out = np.empty(1, dtype=np.float32)
            rc = self.kernel(one.ctypes.data, qtype, 1, cols,
                             vec.ctypes.data, out.ctypes.data, 1)
            if rc:
                raise RuntimeError('native output row failed')
            result.append(float(out[0]))
        return int(np.argmax(result)), result

    def forward(self, tokens, position, choices=None):
        self._single = len(tokens) == 1
        try:
            return super().forward(tokens, position, choices=choices)
        finally:
            self._single = False
