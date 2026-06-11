"""
log_api.py — REGS Log 查詢 API
把這段 Blueprint 掛進你的 app.py 即可。

用法（app.py）：
    from log_api import logs_bp
    app.register_blueprint(logs_bp)
"""

from flask import Blueprint, jsonify, abort
from flask_jwt_extended import jwt_required, get_jwt_identity
from worker import read_all_logs, read_log
import sqlite3
from pathlib import Path

logs_bp = Blueprint("logs", __name__)

DB_PATH = Path(__file__).parent / "database" / "regs.db"


def get_db():
    conn = sqlite3.connect(str(DB_PATH))
    conn.row_factory = sqlite3.Row
    return conn


def _get_submission_or_404(op_id: str):
    """從 DB 取提交記錄，不存在就 404"""
    with get_db() as conn:
        row = conn.execute(
            "SELECT * FROM submissions WHERE operator_id = ?", (op_id,)
        ).fetchone()
    if not row:
        abort(404, description=f"找不到 operatorId: {op_id}")
    return row


# ── GET /api/submissions/<op_id> ──────────────────────────────
@logs_bp.route("/api/submissions/<op_id>", methods=["GET"])
@jwt_required()
def get_submission(op_id):
    """
    查詢評測狀態。
    回傳：
        {
            "operatorId": "xxxx",
            "status": "AC",
            "finished_at": "2025-01-01T12:00:00"
        }
    """
    row = _get_submission_or_404(op_id)
    return jsonify({
        "operatorId": row["operator_id"],
        "status":     row["status"],
        "finishedAt": row["finished_at"],
    })


# ── GET /api/submissions/<op_id>/logs ─────────────────────────
@logs_bp.route("/api/submissions/<op_id>/logs", methods=["GET"])
@jwt_required()
def get_logs(op_id):
    """
    回傳完整三份 log。
    回傳：
        {
            "operatorId": "xxxx",
            "status": "CE",
            "logs": {
                "configure_log": "...",
                "compile_log":   "main.cpp:10: error: ...",
                "output_log":    ""
            }
        }
    學生最常用這個 API 來 debug CE / WA。
    """
    row = _get_submission_or_404(op_id)
    logs = read_all_logs(op_id)

    return jsonify({
        "operatorId": op_id,
        "status":     row["status"],
        "logs":       logs,
    })


# ── GET /api/submissions/<op_id>/logs/<log_type> ──────────────
@logs_bp.route("/api/submissions/<op_id>/logs/<log_type>", methods=["GET"])
@jwt_required()
def get_single_log(op_id, log_type):
    """
    只取一份 log（configure / compile / output）。
    適合前端分頁顯示、或只想看 compile 錯誤的情況。

    回傳：
        {
            "operatorId": "xxxx",
            "logType": "compile",
            "content": "main.cpp:5: error: ..."
        }
    """
    if log_type not in ("configure", "compile", "output"):
        abort(400, description="log_type 必須是 configure / compile / output")

    _get_submission_or_404(op_id)   # 確認 op_id 存在

    content = read_log(op_id, log_type)

    return jsonify({
        "operatorId": op_id,
        "logType":    log_type,
        "content":    content,
    })
