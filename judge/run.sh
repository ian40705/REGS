#!/bin/bash

# =========================
# 使用方式:
#   ./run_judge.sh 114FinalQ001
# =========================

PROBLEM_ID="$1"

if [ -z "$PROBLEM_ID" ]; then
    echo "用法: $0 <題號，例如114FinalQ001>"
    exit 1
fi

BASE_DIR=/mnt/c/Users/USER/Desktop/114-framework
PROBLEM_DIR="$BASE_DIR/$PROBLEM_ID"

STUDENT_DIR="$PROBLEM_DIR/solution"
TEMPLATE_DIR="$PROBLEM_DIR/template"
SCORES_FILE="$PROBLEM_DIR/scores.txt"

JUDGE_SCRIPT=/mnt/c/Users/USER/Desktop/REGS-final/judge/judge_catch2.sh

# ── 檢查檔案 ─────────────────────────────
if [ ! -d "$STUDENT_DIR" ]; then
    echo "[ERROR] 找不到 student dir: $STUDENT_DIR"
    exit 1
fi

if [ ! -d "$TEMPLATE_DIR" ]; then
    echo "[ERROR] 找不到 template dir: $TEMPLATE_DIR"
    exit 1
fi

if [ ! -f "$SCORES_FILE" ]; then
    echo "[ERROR] 找不到 scores file: $SCORES_FILE"
    exit 1
fi

# ── 執行 judge ─────────────────────────────
bash "$JUDGE_SCRIPT" \
  "$STUDENT_DIR" \
  "$TEMPLATE_DIR" \
  "$SCORES_FILE" \
  5

# ── 輸出結果 ─────────────────────────────
echo "--- output.log ---"
cat "$STUDENT_DIR/output.log" 2>/dev/null || echo "(no output.log)"