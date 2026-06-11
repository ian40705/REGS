#!/bin/bash
# ==============================================================================
# build_judge_image.sh
# 在執行 test.sh 之前必須先跑這支 script，確保 regs-judge:latest image 存在。
# Usage: ./build_judge_image.sh
# ==============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
JUDGE_DIR="$SCRIPT_DIR/judge"

echo "[build_judge_image] Building regs-judge:latest from $JUDGE_DIR ..."

docker build -t regs-judge:latest "$JUDGE_DIR"

echo "[build_judge_image] ✅ Done. Image regs-judge:latest is ready."
echo "[build_judge_image] You can now run: ./test.sh"
