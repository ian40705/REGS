"""
conftest.py — pytest 共用設定
讓 tests/ 下的測試都能 import 專案根目錄的模組（worker, log_api）。
"""
import sys
from pathlib import Path

# 把 regs/ 根目錄加入 sys.path
sys.path.insert(0, str(Path(__file__).parent.parent))
