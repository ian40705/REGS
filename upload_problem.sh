#!/bin/bash

# =====================================================
# REGS OJ - One Click Problem Upload
# usage:
#   bash upload_problem.sh 114FinalQ001
# =====================================================

BASE_DIR="/mnt/c/Users/USER/Desktop/114-framework"
PROBLEM_ID="$1"

if [ -z "$PROBLEM_ID" ]; then
    echo "Usage: $0 <ProblemID>"
    exit 1
fi

PROBLEM_DIR="$BASE_DIR/$PROBLEM_ID"

TEMPLATE_DIR="$PROBLEM_DIR/template"
TESTCASE_DIR="$PROBLEM_DIR/online-judge"
SETTINGS_FILE="$PROBLEM_DIR/settings.yaml"

if [ ! -d "$PROBLEM_DIR" ]; then
    echo "[ERROR] Problem not found: $PROBLEM_DIR"
    exit 1
fi

echo "======================================"
echo "Uploading Problem: $PROBLEM_ID"
echo "======================================"

# =====================================================
# 1. Create / Update Problem
# =====================================================

curl -s -X PUT http://localhost:8080/api/problems \
  -H "Content-Type: application/json" \
  -d "{
    \"id\": \"$PROBLEM_ID\",
    \"title\": \"$PROBLEM_ID\",
    \"template_dir\": \"$TEMPLATE_DIR\",
    \"testcase_dir\": \"$TESTCASE_DIR\"
  }" | jq .

echo ""
echo "✔ Problem uploaded"

# =====================================================
# 2. Show summary
# =====================================================

echo ""
echo "Template: $TEMPLATE_DIR"
echo "Testcases: $TESTCASE_DIR"
echo "Settings: $SETTINGS_FILE"

echo ""
echo "✔ Done"
echo "Now you can submit:"
echo "POST /api/submissions"