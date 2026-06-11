#!/bin/bash

QUESTION_DIR="$1"

if [ -z "$QUESTION_DIR" ] || [ ! -f "$QUESTION_DIR/settings.yaml" ]; then
    echo "用法: $0 <114FinalQxxx目錄>"
    exit 1
fi

SCORES_FILE="$QUESTION_DIR/scores.txt"

mapfile -t SCORES < <(awk -F': *' '/score/ {gsub(/\r/,""); print $2}' "$QUESTION_DIR/settings.yaml")

> "$SCORES_FILE"

TOTAL=0
COUNT=${#SCORES[@]}

for ((i=0;i<COUNT;i++)); do
    SCORE=${SCORES[$i]}
    echo "case$((i+1)) $SCORE" >> "$SCORES_FILE"
    TOTAL=$((TOTAL + SCORE))
done

echo "產生 $SCORES_FILE："
cat "$SCORES_FILE"
echo "滿分：$TOTAL"