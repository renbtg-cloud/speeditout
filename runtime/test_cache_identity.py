#!/usr/bin/env python3
import pathlib
import tempfile
import unittest
from types import SimpleNamespace
from gguf_qwen3_experimental import Store

class CacheIsolation(unittest.TestCase):
 def test_distinct_source_bytes_separate_weight_caches(self):
  with tempfile.TemporaryDirectory() as root:
   p=pathlib.Path(root)/'model.gguf';p.write_bytes(b'one')
   a=Store(SimpleNamespace(path=p),pathlib.Path(root)/'cache')
   p.write_bytes(b'two')
   b=Store(SimpleNamespace(path=p),pathlib.Path(root)/'cache')
   self.assertNotEqual(a.model_sha256,b.model_sha256)
   self.assertNotEqual(a.cache,b.cache)
   self.assertTrue(a.cache.is_dir())
   self.assertTrue(b.cache.is_dir())

if __name__ == '__main__':unittest.main(verbosity=2)
