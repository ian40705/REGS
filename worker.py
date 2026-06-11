"""
worker.py — REGS 評測 Worker
負責：
  1. 從 Queue 取出任務
  2. 呼叫 Docker 執行 judge.sh
  3. 收集 Log 並搬移到 logs/<op_id>/
  4. 將結果寫回 SQLite
"""

import os
import shutil
import sqlite3
import subprocess
import threading
from datetime import datetime
from pathlib import Path
from queue import Queue

# ── 設定 ──────────────────────────────────────────────────────
BASE_DIR       = Path(__file__).parent
SUBMISSIONS_DIR = BASE_DIR / "submissions"
LOGS_DIR       = BASE_DIR / "logs"
PROBLEMS_DIR   = BASE_DIR / "problems"
JUDGE_SCRIPT   = BASE_DIR / "judge" / "judge.sh"
DB_PATH        = BASE_DIR / "database" / "regs.db"

TIME_LIMIT     = 5          # 秒（可之後從 DB problems.time_limit 讀）
MAX_CONCURRENT = 4          # 最多同時跑幾個 Docker

# 全域 Queue 與 Semaphore
job_queue = Queue()
semaphore = threading.Semaphore(MAX_CONCURRENT)


# ══════════════════════════════════════════════════════════════
# 資料庫工具
# ══════════════════════════════════════════════════════════════

def get_db():
    """取得 SQLite 連線（每個 thread 各自開）"""
    conn = sqlite3.connect(str(DB_PATH))
    conn.row_factory = sqlite3.Row
    return conn


def update_status(op_id: str, status: str):
    """更新 submissions.status"""
    with get_db() as conn:
        conn.execute(
            "UPDATE submissions SET status = ?, finished_at = ? WHERE operator_id = ?",
            (status, datetime.utcnow().isoformat(), op_id)
        )
        conn.commit()


def save_log_paths(op_id: str, configure_path: str, compile_path: str, output_path: str):
    """
    把三個 log 的路徑存進 submission_logs。
    只存路徑，不存內容（log 檔可能很大）。
    """
    with get_db() as conn:
        sub = conn.execute(
            "SELECT id FROM submissions WHERE operator_id = ?", (op_id,)
        ).fetchone()
        if not sub:
            return
        conn.execute(
            """
            INSERT INTO submission_logs
                (submission_id, configure_log_path, compile_log_path, output_log_path)
            VALUES (?, ?, ?, ?)
            """,
            (sub["id"], configure_path, compile_path, output_path)
        )
        conn.commit()


# ══════════════════════════════════════════════════════════════
# Log 工具
# ══════════════════════════════════════════════════════════════

def collect_logs(op_id: str, project_dir: Path) -> dict[str, Path]:
    """
    把 judge.sh 產生的 log 從 project_dir 搬到 logs/<op_id>/。
    回傳 {"configure": Path, "compile": Path, "output": Path}
    """
    dest_dir = LOGS_DIR / op_id
    dest_dir.mkdir(parents=True, exist_ok=True)

    paths = {}
    for name in ("configure.log", "compile.log", "output.log", "user_output.txt"):
        src = project_dir / name
        dst = dest_dir / name
        if src.exists():
            shutil.move(str(src), str(dst))
        else:
            # 確保檔案存在（即使是空的），方便 API 讀取
            dst.touch()
        paths[name.replace(".log", "")] = dst

    return paths


def read_log(op_id: str, log_type: str) -> str:
    """
    讀出指定 log 的文字內容，供 API /submissions/{opId}/logs 使用。

    log_type: "configure" | "compile" | "output"

    若檔案不存在回傳提示字串，不會丟例外。
    """
    log_file = LOGS_DIR / op_id / f"{log_type}.log"
    if not log_file.exists():
        return f"[Log 不存在：{log_type}.log]"
    try:
        return log_file.read_text(encoding="utf-8", errors="replace")
    except Exception as e:
        return f"[讀取 log 失敗：{e}]"


def read_all_logs(op_id: str) -> dict[str, str]:
    """
    一次讀出三個 log，回傳 dict。
    供 API 回傳 JSON 用。
    """
    return {
        "configure_log": read_log(op_id, "configure"),
        "compile_log":   read_log(op_id, "compile"),
        "output_log":    read_log(op_id, "output"),
    }


# ══════════════════════════════════════════════════════════════
# Judge Pipeline（呼叫 Docker + judge.sh）
# ══════════════════════════════════════════════════════════════

def get_testcase_paths(problem_id: int) -> tuple[Path, Path]:
    """
    從 DB 取第一筆測資的 input / answer 路徑。
    （完整版可改成多筆測資逐一跑）
    """
    with get_db() as conn:
        tc = conn.execute(
            "SELECT input_path, answer_path FROM testcases WHERE problem_id = ? LIMIT 1",
            (problem_id,)
        ).fetchone()
    if not tc:
        raise FileNotFoundError(f"找不到 problem_id={problem_id} 的測資")
    return Path(tc["input_path"]), Path(tc["answer_path"])


def judge_submission(op_id: str):
    """
    完整評測流程：
      1. 找到解壓縮後的專案目錄
      2. 取得測資路徑
      3. 呼叫 judge.sh（透過 Docker 或直接 subprocess）
      4. 收集 log
      5. 更新 DB
    """
    # ── 0. 找專案目錄 ─────────────────────────────────────────
    project_dir = SUBMISSIONS_DIR / op_id / "extracted"

    if not project_dir.exists():
        _finish(op_id, "SE", project_dir)
        return

    # ── 1. 從 DB 取 problem_id ────────────────────────────────
    with get_db() as conn:
        row = conn.execute(
            "SELECT problem_id FROM submissions WHERE operator_id = ?", (op_id,)
        ).fetchone()
    if not row:
        return
    problem_id = row["problem_id"]

    # ── 2. 取測資路徑 ─────────────────────────────────────────
    try:
        input_path, answer_path = get_testcase_paths(problem_id)
    except FileNotFoundError:
        _finish(op_id, "SE", project_dir)
        return

    # ── 3. 更新狀態：CONFIGURING ──────────────────────────────
    update_status(op_id, "CONFIGURING")

    # ── 4. 執行 judge.sh ──────────────────────────────────────
    #
    #   直接用 subprocess（開發 / 測試用）
    #   正式版請改用 Docker SDK（見下方 run_in_docker）
    #
    status = _run_judge_subprocess(project_dir, input_path, answer_path)

    # ── 5. 收集 log → logs/<op_id>/ ──────────────────────────
    log_paths = collect_logs(op_id, project_dir)

    # ── 6. 存 log 路徑進 DB ───────────────────────────────────
    save_log_paths(
        op_id,
        str(log_paths["configure"]),
        str(log_paths["compile"]),
        str(log_paths["output"]),
    )

    # ── 7. 最終狀態寫回 DB ────────────────────────────────────
    update_status(op_id, status)
    print(f"[Worker] {op_id} → {status}")


def _run_judge_subprocess(project_dir: Path, input_path: Path, answer_path: Path) -> str:
    """用 subprocess 直接跑 judge.sh（本機測試用）"""
    try:
        result = subprocess.run(
            [
                "bash", str(JUDGE_SCRIPT),
                str(project_dir),
                str(input_path),
                str(answer_path),
                str(TIME_LIMIT),
            ],
            capture_output=True,
            text=True,
            timeout=TIME_LIMIT + 10,  # 給 judge.sh 本身一點緩衝
        )
        # judge.sh 最後一行 stdout 就是狀態碼
        status = result.stdout.strip().splitlines()[-1] if result.stdout.strip() else "SE"
        return status if status in ("AC", "WA", "CE", "RE", "SE", "TLE") else "SE"
    except subprocess.TimeoutExpired:
        return "TLE"
    except Exception as e:
        print(f"[Worker] subprocess 錯誤: {e}")
        return "SE"


def _run_judge_docker(op_id: str, project_dir: Path,
                      input_path: Path, answer_path: Path) -> str:
    """
    用 Docker SDK 跑 judge.sh（正式版）。
    需要 `pip install docker`。
    """
    import docker  # 只在需要時 import

    client = docker.from_env()

    # 把 project_dir 掛進 container 的 /workspace
    host_project = str(project_dir.resolve())
    host_input   = str(input_path.resolve())
    host_answer  = str(answer_path.resolve())

    cmd = (
        f"bash /workspace/judge.sh "
        f"/workspace /input/input.txt /input/answer.txt {TIME_LIMIT}"
    )

    try:
        logs = client.containers.run(
            "yhlib/cs3060701",          # Docker image
            command=cmd,
            volumes={
                host_project: {"bind": "/workspace", "mode": "rw"},
                str(input_path.parent.resolve()): {"bind": "/input", "mode": "ro"},
            },
            network_disabled=True,      # --network none
            mem_limit="256m",           # 記憶體上限
            cpu_quota=100000,           # 1 核 CPU
            remove=True,                # --rm
        )
        output = logs.decode("utf-8").strip()
        last_line = output.splitlines()[-1] if output else "SE"
        return last_line if last_line in ("AC", "WA", "CE", "RE", "SE", "TLE") else "SE"
    except Exception as e:
        print(f"[Worker] Docker 錯誤: {e}")
        return "SE"


def _finish(op_id: str, status: str, project_dir: Path):
    """快速結束（SE 等不需要跑的情況）"""
    collect_logs(op_id, project_dir)
    update_status(op_id, status)
    print(f"[Worker] {op_id} → {status}")


# ══════════════════════════════════════════════════════════════
# Worker Thread
# ══════════════════════════════════════════════════════════════

def worker_loop():
    """
    無限迴圈，從 job_queue 取任務執行。
    Semaphore 限制最多 MAX_CONCURRENT 個同時跑。
    """
    while True:
        op_id = job_queue.get()
        try:
            with semaphore:
                judge_submission(op_id)
        except Exception as e:
            print(f"[Worker] 未預期錯誤 ({op_id}): {e}")
            update_status(op_id, "SE")
        finally:
            job_queue.task_done()


def start_workers(num_threads: int = MAX_CONCURRENT):
    """
    啟動背景 Worker threads，在 Flask app 啟動時呼叫一次。
    """
    for _ in range(num_threads):
        t = threading.Thread(target=worker_loop, daemon=True)
        t.start()
    print(f"[Worker] 已啟動 {num_threads} 個 Worker threads")