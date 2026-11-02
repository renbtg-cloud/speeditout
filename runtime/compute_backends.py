"""Optional experimental BF16 execution of linear layers; keep reference FP32 default.

Uses torch CPU BF16 GEMM where supported; RMSNorm, RoPE, attention and
residuals remain FP32. This is a numerical APPROXIMATION, not an exact
replacement for the FP32 research path. Must be checked against regression cases.
"""
import torch
import torch.nn.functional as F
from gguf_qwen3_experimental import Runner

class RunnerBF16(Runner):
    def mat(self, x, name):
        # FP16 dequantization cache -> BF16 GEMM -> FP32 residual path.
        weights = self.read(name).to(torch.bfloat16)
        return F.linear(x.to(torch.bfloat16), weights).float()
