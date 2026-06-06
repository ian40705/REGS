#!/bin/bash
# =============================================================
# judge.sh — REGS 自動評測管線
# 用法：bash judge.sh <專案目錄> <測資輸入檔> <標準答案檔> [時間限制秒數]
#
# 回傳值（stdout 最後一行）：
#   SE  → Setup Error     (CMake configure 失敗)
#   CE  → Compile Error   (編譯失敗)
#   TLE → Time Limit Exceeded
#   RE  → Runtime Error   (非 0 結束碼)
#   WA  → Wrong Answer
#   AC  → Accepted
# =============================================================

# ── 參數 ──────────────────────────────────────────────────────
PROJECT_DIR="$1"          # 解壓縮後的學生專案根目錄
INPUT_FILE="$2"           # 測資輸入（絕對路徑）
ANSWER_FILE="$3"          # 標準答案（絕對路徑）
TIME_LIMIT="${4:-5}"      # 時間限制（秒），預設 5 秒

# Windows 路徑在 bash / Git Bash 下常會被視為逸出字串；
# 先把所有反斜線替換成正斜線，避免 `C:\Users\...` 變成無效路徑。
PROJECT_DIR="${PROJECT_DIR//\\//}"
INPUT_FILE="${INPUT_FILE//\\//}"
ANSWER_FILE="${ANSWER_FILE//\\//}"

# 若環境提供 cygpath，也可再做一次轉換，作為補強。
if command -v cygpath >/dev/null 2>&1; then
    PROJECT_DIR="$(cygpath -u "$PROJECT_DIR" 2>/dev/null || printf '%s' "$PROJECT_DIR")"
    INPUT_FILE="$(cygpath -u "$INPUT_FILE" 2>/dev/null || printf '%s' "$INPUT_FILE")"
    ANSWER_FILE="$(cygpath -u "$ANSWER_FILE" 2>/dev/null || printf '%s' "$ANSWER_FILE")"
fi

# ── 基本驗證 ─────────────────────────────────────────────────
if [ -z "$PROJECT_DIR" ] || [ -z "$INPUT_FILE" ] || [ -z "$ANSWER_FILE" ]; then
    echo "用法: $0 <project_dir> <input_file> <answer_file> [time_limit]" >&2
    exit 2
fi

if [ ! -d "$PROJECT_DIR" ]; then
    echo "錯誤：專案目錄不存在: $PROJECT_DIR" >&2
    echo "SE"
    exit 1
fi

# ── 切換到專案目錄 ────────────────────────────────────────────
cd "$PROJECT_DIR" || { echo "SE"; exit 1; }

# Log 檔放在專案目錄下（worker.py 之後會把它們搬到 logs/<op_id>/）
LOG_DIR="."
CONFIGURE_LOG="$LOG_DIR/configure.log"
COMPILE_LOG="$LOG_DIR/compile.log"
OUTPUT_LOG="$LOG_DIR/output.log"       # 執行過程的 log 訊息
USER_OUTPUT="$LOG_DIR/user_output.txt" # 程式的真實 stdout（拿來 diff）

# ── 步驟 A：CMake Configure ───────────────────────────────────
echo "[$(date '+%H:%M:%S')] === STEP A: CMake Configure ===" >> "$CONFIGURE_LOG"

# 先確認 CMakeLists.txt 存在
if [ ! -f "CMakeLists.txt" ]; then
    echo "[ERROR] 找不到 CMakeLists.txt" >> "$CONFIGURE_LOG"
    echo "SE"
    exit 1
fi

mkdir -p build

# 執行 cmake；stdout + stderr 都存進 configure.log
cmake -G Ninja -B build >> "$CONFIGURE_LOG" 2>&1
CMAKE_EXIT=$?

if [ $CMAKE_EXIT -ne 0 ]; then
    echo "[ERROR] cmake configure 失敗，exit code: $CMAKE_EXIT" >> "$CONFIGURE_LOG"
    echo "SE"
    exit 1
fi

echo "[OK] CMake configure 成功" >> "$CONFIGURE_LOG"

# ── 步驟 B：Build ─────────────────────────────────────────────
echo "[$(date '+%H:%M:%S')] === STEP B: Build ===" >> "$COMPILE_LOG"

cmake --build build --verbose >> "$COMPILE_LOG" 2>&1
BUILD_EXIT=$?

if [ $BUILD_EXIT -ne 0 ]; then
    echo "[ERROR] 編譯失敗，exit code: $BUILD_EXIT" >> "$COMPILE_LOG"
    echo "CE"
    exit 1
fi

echo "[OK] 編譯成功" >> "$COMPILE_LOG"

# ── 確認執行檔存在 ────────────────────────────────────────────
EXECUTABLE="./build/main"
if [ ! -f "$EXECUTABLE" ]; then
    # 嘗試找其他同名執行檔（Ninja 有時路徑不同）
    EXECUTABLE=$(find ./build -maxdepth 2 -type f -perm /111 | head -n 1)
    if [ -z "$EXECUTABLE" ]; then
        echo "[ERROR] 找不到執行檔" >> "$COMPILE_LOG"
        echo "CE"
        exit 1
    fi
fi

# ── 步驟 C：Run ───────────────────────────────────────────────
echo "[$(date '+%H:%M:%S')] === STEP C: Run (limit=${TIME_LIMIT}s) ===" >> "$OUTPUT_LOG"

# 程式 stdout → user_output.txt；stderr → output.log
timeout "${TIME_LIMIT}s" "$EXECUTABLE" < "$INPUT_FILE" \
    > "$USER_OUTPUT" 2>> "$OUTPUT_LOG"
RUN_EXIT=$?

# exit code 124 = timeout 殺掉
if [ $RUN_EXIT -eq 124 ]; then
    echo "[ERROR] 執行超時（>${TIME_LIMIT}s）" >> "$OUTPUT_LOG"
    echo "TLE"
    exit 1
fi

# 其他非 0 = 程式自己 crash
if [ $RUN_EXIT -ne 0 ]; then
    echo "[ERROR] 程式異常結束，exit code: $RUN_EXIT" >> "$OUTPUT_LOG"
    echo "RE"
    exit 1
fi

echo "[OK] 程式執行完畢" >> "$OUTPUT_LOG"

# ── 步驟 D：Judge（比對答案）────────────────────────────────
echo "[$(date '+%H:%M:%S')] === STEP D: Judge ===" >> "$OUTPUT_LOG"

# diff：忽略結尾空白（-b）與結尾換行差異（-B），避免 Windows 換行誤判
if diff -bB "$USER_OUTPUT" "$ANSWER_FILE" > /dev/null 2>&1; then
    echo "[OK] 答案正確" >> "$OUTPUT_LOG"
    echo "AC"
    exit 0
else
    echo "[DIFF] 答案不符" >> "$OUTPUT_LOG"
    # 把差異記進 output.log，方便學生 debug
    echo "--- 你的輸出 vs 標準答案 ---" >> "$OUTPUT_LOG"
    diff "$USER_OUTPUT" "$ANSWER_FILE" >> "$OUTPUT_LOG" 2>&1 || true
    echo "WA"
    exit 1
fi