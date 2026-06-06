#!/bin/bash
# tools/prepare_testdata.sh
# 將 114Final 題目資料夾轉換為 REGS catch2 testdata 格式
# 用法：bash tools/prepare_testdata.sh <114FinalQxxx目錄> <輸出目錄>

QUESTION_DIR="$1"
OUTPUT_DIR="$2"
if [ -z "$QUESTION_DIR" ] || [ -z "$OUTPUT_DIR" ]; then
    echo "用法: $0 <114FinalQxxx目錄> <輸出目錄>"; exit 1
fi
if [ ! -f "$QUESTION_DIR/settings.yaml" ]; then
    echo "找不到 settings.yaml"; exit 1
fi

TEMPLATE_DIR="$QUESTION_DIR/template"
OJ_DIR="$QUESTION_DIR/online-judge"
mkdir -p "$OUTPUT_DIR/cases"

echo "複製 OJ 固定檔..."
cp "$TEMPLATE_DIR/CMakeLists.txt" "$OUTPUT_DIR/"
cp "$TEMPLATE_DIR/entrypoint.cpp" "$OUTPUT_DIR/"
cp "$TEMPLATE_DIR/test.h"         "$OUTPUT_DIR/"

for f in "$TEMPLATE_DIR"/*.h; do
    fname=$(basename "$f")
    if [ "$fname" != "case.h" ] && [ "$fname" != "test.h" ]; then
        cp "$f" "$OUTPUT_DIR/" && echo "  header: $fname"
    fi
done

SCORES_FILE="$OUTPUT_DIR/scores.txt"
> "$SCORES_FILE"
SCORES=($(grep '^\s*score:' "$QUESTION_DIR/settings.yaml" | awk '{print $2}'))
TOTAL=0

for i in 1 2 3 4 5; do
    [ -f "$OJ_DIR/case${i}.h" ] && [ -f "$OJ_DIR/case${i}" ] || continue
    cp "$OJ_DIR/case${i}.h" "$OUTPUT_DIR/cases/case${i}.h"
    cp "$OJ_DIR/case${i}"   "$OUTPUT_DIR/cases/case${i}"
    SCORE="${SCORES[$(( i-1 ))]:-2}"
    echo "case${i} ${SCORE}" >> "$SCORES_FILE"
    TOTAL=$(( TOTAL + SCORE ))
    echo "  case${i}: ${SCORE} 分"
done

echo ""
echo "完成！滿分：$TOTAL"
echo "下一步："
echo "  1. zip -r testdata_Q0xx.zip $OUTPUT_DIR/"
echo "  2. 用 API 建立題目（judge_type: catch2, max_score: $TOTAL）"
