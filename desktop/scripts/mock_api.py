"""用于桌面界面开发的零依赖 mock API；不要用于生产环境。"""

from __future__ import annotations

import json
from datetime import date, timedelta
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

TODAY = date.today()
USER = {
    "id": 1,
    "casdoor_id": "local:demo",
    "name": "林默",
    "avatar": "",
    "email": "linmo@example.com",
    "feishu_open_id": "",
    "is_admin": False,
    "created_at": f"{TODAY.isoformat()}T08:00:00Z",
}
USERS = [
    USER,
    {**USER, "id": 2, "name": "沈言", "email": "shenyan@example.com"},
    {**USER, "id": 3, "name": "周屿", "email": "zhouyu@example.com"},
]
PROJECTS = [
    {"id": 1, "name": "基础架构", "description": "", "owner_id": 1, "owner": USER, "status": "active", "created_at": ""},
    {"id": 2, "name": "体验升级", "description": "", "owner_id": 2, "owner": USERS[1], "status": "active", "created_at": ""},
    {"id": 3, "name": "数据平台", "description": "", "owner_id": 3, "owner": USERS[2], "status": "active", "created_at": ""},
]


def task(
    task_id: int,
    title: str,
    project_id: int,
    status: str,
    priority: str,
    due_offset: int | None,
    content: str,
    detail: str = "",
):
    return {
        "id": task_id,
        "title": title,
        "content": content,
        "detail": detail,
        "project_id": project_id,
        "project": PROJECTS[project_id - 1],
        "author_id": 1,
        "author": USER,
        "assignee_id": 1,
        "assignee": USER,
        "participants": USERS[1:],
        "kind": "work",
        "status": status,
        "priority": priority,
        "work_date": TODAY.isoformat(),
        "due_date": (TODAY + timedelta(days=due_offset)).isoformat() if due_offset is not None else None,
        "due_remind": True,
        "start_remind": False,
        "done_at": None,
        "created_at": f"{TODAY.isoformat()}T08:30:00Z",
        "comment_count": task_id % 3,
    }


TASKS = [
    task(
        18,
        "完成告警规则迁移与回归验证",
        1,
        "doing",
        "high",
        0,
        "<p></p><p>迁移剩余服务规则，并验证飞书告警链路。</p>"
        + "".join(f"<p>第 {index} 段回归说明，用于验证长正文的滚动行为。</p>" for index in range(1, 13))
        + "<p></p>",
        "<p>回归范围包含告警触发、飞书送达与恢复通知。</p><ul><li>保留验证截图</li><li>记录异常规则</li></ul>",
    ),
    task(17, "整理新建任务的键盘交互", 2, "doing", "medium", 1, "补齐快捷键说明和焦点状态。"),
    task(16, "补充周报统计口径文档", 3, "todo", "low", 4, "对齐完成日期与工作日期口径。"),
    task(15, "评估数据库归档策略", 1, "todo", "medium", None, "先确认历史数据体量。"),
    task(14, "发布任务详情页优化", 2, "done", "medium", -1, "已完成首轮灰度。"),
    task(13, "跟进链路追踪采样率调整", 1, "doing", "high", -3, "等待运维确认灰度窗口。"),
]

NOTIFICATIONS = [
    {
        "id": 5,
        "user_id": 1,
        "work_item_id": 18,
        "work_item": TASKS[0],
        "type": "comment",
        "title": "沈言评论了你的任务",
        "content": "飞书链路我这边验证通过了，剩下两条规则你看下。",
        "read": False,
        "created_at": f"{TODAY.isoformat()}T09:20:00Z",
    },
    {
        "id": 4,
        "user_id": 1,
        "work_item_id": 13,
        "work_item": TASKS[-1],
        "type": "due",
        "title": "任务已逾期 3 天",
        "content": "跟进链路追踪采样率调整",
        "read": False,
        "created_at": f"{TODAY.isoformat()}T08:00:00Z",
    },
    {
        "id": 3,
        "user_id": 1,
        "work_item_id": 16,
        "work_item": TASKS[2],
        "type": "assign",
        "title": "周屿把任务分配给你",
        "content": "补充周报统计口径文档",
        "read": True,
        "created_at": f"{(TODAY - timedelta(days=1)).isoformat()}T15:30:00Z",
    },
]


class Handler(BaseHTTPRequestHandler):
    def _headers(self, status=200):
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
        self.end_headers()

    def _json(self, body, status=200):
        self._headers(status)
        self.wfile.write(json.dumps(body, ensure_ascii=False).encode())

    def do_OPTIONS(self):
        self._headers(204)

    def do_GET(self):
        path = urlparse(self.path).path
        if path == "/api/auth/me":
            self._json(USER)
        elif path == "/api/projects":
            self._json(PROJECTS)
        elif path == "/api/users":
            self._json(USERS)
        elif path == "/api/work-items":
            self._json(TASKS)
        elif path == "/api/notifications/unread-count":
            self._json({"count": sum(1 for item in NOTIFICATIONS if not item["read"])})
        elif path == "/api/notifications":
            self._json(NOTIFICATIONS)
        elif path.startswith("/api/work-items/"):
            task_id = int(path.rsplit("/", 1)[-1])
            self._json(next((item for item in TASKS if item["id"] == task_id), {}))
        else:
            self._json({"error": "not found"}, 404)

    def do_PATCH(self):
        path = urlparse(self.path).path
        if path.endswith("/status"):
            task_id = int(path.split("/")[-2])
            length = int(self.headers.get("Content-Length", "0"))
            payload = json.loads(self.rfile.read(length) or b"{}")
            item = next(item for item in TASKS if item["id"] == task_id)
            item["status"] = payload.get("status", item["status"])
            self._json(item)
        elif path.startswith("/api/notifications/"):
            target = path.split("/")[-2]
            for item in NOTIFICATIONS:
                if target == "all" or str(item["id"]) == target:
                    item["read"] = True
            self._json({"ok": True})
        else:
            self._json({"error": "not found"}, 404)

    def do_POST(self):
        if urlparse(self.path).path != "/api/work-items":
            self._json({"error": "not found"}, 404)
            return
        length = int(self.headers.get("Content-Length", "0"))
        payload = json.loads(self.rfile.read(length) or b"{}")
        created = task(
            max(item["id"] for item in TASKS) + 1,
            payload["title"],
            payload["project_id"],
            payload.get("status", "doing"),
            payload.get("priority", "medium"),
            0,
            payload.get("content", ""),
            payload.get("detail", ""),
        )
        # 与后端一致：未排期（无开始日期）的任务一律落到「待办」，日期留空
        if not payload.get("work_date"):
            created["status"] = "todo"
            created["work_date"] = None
        if not payload.get("due_date"):
            created["due_date"] = None
        TASKS.insert(0, created)
        self._json(created, 201)

    def log_message(self, *_):
        return


if __name__ == "__main__":
    print("Mock API: http://127.0.0.1:8093", flush=True)
    ThreadingHTTPServer(("127.0.0.1", 8093), Handler).serve_forever()
