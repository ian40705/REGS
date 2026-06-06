#!/bin/bash
# =============================================================
# judge_catch2.sh (stable version)
# =============================================================

STUDENT_DIR="$1"
TEMPLATE_DIR="$2"
SCORES_FILE="$3"
TIME_LIMIT="${4:-5}"

# ── 基本驗證 ─────────────────────────────
if [ -z "$STUDENT_DIR" ] || [ -z "$TEMPLATE_DIR" ] || [ -z "$SCORES_FILE" ]; then
    echo "用法: $0 <student_dir> <template_dir> <scores_file> [time_limit]"
    exit 2
fi

[ ! -d "$STUDENT_DIR" ] && { echo "[ERROR] student_dir not found"; exit 1; }
[ ! -d "$TEMPLATE_DIR" ] && { echo "[ERROR] template_dir not found"; exit 1; }
[ ! -f "$SCORES_FILE" ] && { echo "[ERROR] scores_file not found"; exit 1; }

PROBLEM_ROOT="$(cd "$TEMPLATE_DIR/.." && pwd)"
OJ_DIR="$PROBLEM_ROOT/online-judge"

[ ! -d "$OJ_DIR" ] && { echo "[ERROR] online-judge missing"; exit 1; }

# ── logs ─────────────────────────────
LOG_DIR="$STUDENT_DIR"
OUTPUT_LOG="$LOG_DIR/output.log"

# ── 讀 scores.txt（修正：只取數字） ─────────────────────────────
declare -a CASE_NAMES
declare -A CASE_SCORES

MAX_SCORE=0

while read -r case_name score; do
    [ -z "$case_name" ] && continue

    # 去掉 comment / 非數字
    score="${score%%#*}"
    score="$(echo "$score" | grep -oE '[0-9]+')"

    CASE_NAMES+=("$case_name")
    CASE_SCORES["$case_name"]="$score"

    MAX_SCORE=$((MAX_SCORE + score))
done < "$SCORES_FILE"

TOTAL_SCORE=0

echo "=== start judge ===" >> "$OUTPUT_LOG"
echo "cases: ${#CASE_NAMES[@]}, max: $MAX_SCORE" >> "$OUTPUT_LOG"

# ── build dir ─────────────────────────────
BUILD_DIR="$(mktemp -d /tmp/regs-build-XXXXXX)"

# ── cmake configure ─────────────────────────────
cmake -G Ninja \
    -S "$TEMPLATE_DIR" \
    -B "$BUILD_DIR" \
    -D SOURCE_DIR="$STUDENT_DIR" \
    -D SPEC_DIR="$OJ_DIR" \
    >> "$OUTPUT_LOG" 2>&1

if [ $? -ne 0 ]; then
    echo "SE"
    exit 1
fi

# ── build ─────────────────────────────
cmake --build "$BUILD_DIR" --parallel >> "$OUTPUT_LOG" 2>&1

if [ $? -ne 0 ]; then
    echo "CE"
    exit 1
fi

# ── run tests ─────────────────────────────
for CASE_NAME in "${CASE_NAMES[@]}"; do
    SCORE="${CASE_SCORES[$CASE_NAME]}"
    EXEC="$BUILD_DIR/$CASE_NAME"
    EXPECTED="$OJ_DIR/$CASE_NAME"

    # find fallback
    if [ ! -x "$EXEC" ]; then
        EXEC=$(find "$BUILD_DIR" -type f -name "$CASE_NAME" -perm /111 2>/dev/null | head -n 1)
    fi

    if [ -z "$EXEC" ]; then
        echo "$CASE_NAME 0/$SCORE (NO EXEC)" >> "$OUTPUT_LOG"
        continue
    fi

    if [ ! -f "$EXPECTED" ]; then
        echo "$CASE_NAME 0/$SCORE (NO EXPECTED)" >> "$OUTPUT_LOG"
        continue
    fi

    USER_OUT="$LOG_DIR/out_${CASE_NAME}.txt"

    timeout "${TIME_LIMIT}s" "$EXEC" > "$USER_OUT" 2>>"$OUTPUT_LOG"
    EXIT_CODE=$?

    # timeout / crash handling
    if [ $EXIT_CODE -eq 124 ]; then
        echo "$CASE_NAME 0/$SCORE (TLE)" >> "$OUTPUT_LOG"
        continue
    elif [ $EXIT_CODE -ge 128 ]; then
        echo "$CASE_NAME 0/$SCORE (RE)" >> "$OUTPUT_LOG"
        continue
    elif [ $EXIT_CODE -ne 0 ]; then
        echo "$CASE_NAME 0/$SCORE (RE)" >> "$OUTPUT_LOG"
        continue
    fi

    # compare
    if diff -bB "$USER_OUT" "$EXPECTED" >/dev/null 2>&1; then
        TOTAL_SCORE=$((TOTAL_SCORE + SCORE))
        echo "$CASE_NAME $SCORE/$SCORE (AC)" >> "$OUTPUT_LOG"
    else
        echo "$CASE_NAME 0/$SCORE (WA)" >> "$OUTPUT_LOG"
    fi
done

rm -rf "$BUILD_DIR"

echo "FINAL: $TOTAL_SCORE/$MAX_SCORE"
exit 0