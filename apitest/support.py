"""Shared transport and isolated fixtures; interface assertions live in cases/."""
import base64
import hashlib
import json
import math
import os
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path


class Context:
    def __init__(self):
        import pymysql
        self.base = os.environ.get("APITEST_BASE_URL", "http://127.0.0.1:18085").rstrip("/")
        name = os.environ["APITEST_DB_NAME"]
        if os.environ.get("APITEST_ALLOW_WRITES") != "1" or not name.startswith("campusclaw_test_"):
            raise RuntimeError("仅允许显式授权的 campusclaw_test_ 测试库")
        if urllib.parse.urlsplit(self.base).hostname not in ("localhost", "127.0.0.1"):
            raise RuntimeError("测试目标必须为本地隔离环境")
        self.password = os.environ["APITEST_PASSWORD"]
        self.db = pymysql.connect(host=os.environ["APITEST_DB_HOST"], port=int(os.environ["APITEST_DB_PORT"]), database=name, user=os.environ["APITEST_DB_USER"], password=self.password, autocommit=True, cursorclass=pymysql.cursors.DictCursor)
        self.events = []
        self.teacher, self.student, self.other = [self.login(n) for n in ("teacher_a", "student_a1", "student_b1")]
        self.tokens = [self.teacher, self.student, self.other]
        self.a = self.call("GET", "/api/me", token=self.teacher)[1]["user"]
        self.b = self.call("GET", "/api/me", token=self.other)[1]["user"]
        # A freshly issued random jti proves the API writes to this exact test database.
        payload=self.teacher.split('.')[1]
        claims=json.loads(base64.urlsafe_b64decode(payload+'='*((4-len(payload)%4)%4)))
        session_rows=self.rows('SELECT user_id FROM sessions WHERE token_hash=%s',(hashlib.sha256(claims['jti'].encode()).hexdigest(),))
        assert session_rows and session_rows[0]['user_id']==self.a['id'], "应用未连接本次测试数据库"
        # A normal teacher account in the isolated B fixture, using only this test's bcrypt hash.
        self.execute("INSERT IGNORE INTO users(username,password_hash,role,class_id) SELECT 'apitest_teacher_b',password_hash,'teacher',%s FROM users WHERE username='teacher_a'", (self.b["class_id"],))
        self.teacher_b = self.login("apitest_teacher_b"); self.tokens.append(self.teacher_b)
        assert self.scalar("SELECT class_id FROM users WHERE id=%s", (self.a["id"],)) == self.a["class_id"], "应用与测试数据库不匹配"
        self.suffix = uuid.uuid4().hex[:10]
        self.material = self.upload("数学验收-" + self.suffix, "# 数学方程\n\n方程建模与去括号：乘法应作用于每一项。\n<script>window.lesson5Xss=true</script>\n")
        self.unbound = self.upload("未绑定数学-" + self.suffix, "# 数学方程\n\n方程建模与去括号：不要将此材料发送给只绑定另一份资料的助手。\n")
        self.b_material = self.upload("B数学-" + self.suffix, "方程建模属于 B 班。", self.teacher_b)
        self.source = "名称：数学解题引导" + self.suffix + "\n适用条件：方程,去括号,Equation\n资料：数学\n工具：knowledge_search\n启用：是\n\n检查学生步骤，指出错误并提示方向，不给最终得数。"
        self.skill = self.expect("POST", "/api/skills", {"source": self.source}, 201, self.teacher)
        self.b_skill = self.expect("POST", "/api/skills", {"source": self.source}, 201, self.teacher_b)
        self.agent_input = dict(name="数学助手" + self.suffix, prompt="用简体中文帮助学生理解题目。", enabled=True, skill_ids=[self.skill["id"]], material_ids=[self.material])
        self.agent = self.expect("POST", "/api/agents", self.agent_input, 201, self.teacher)
        self.b_agent = self.expect("POST", "/api/agents", dict(self.agent_input, skill_ids=[self.b_skill["id"]], material_ids=[self.b_material]), 201, self.teacher_b)
        self.conversation = self.expect("POST", "/api/conversations", {"agent_id": self.agent["id"]}, 201, self.student)
        self.wait_ready()

    def call(self, method, path, payload=None, token=None, raw=None, headers=None):
        merged = dict(headers or {})
        if token: merged["Authorization"] = "Bearer " + token
        body = raw
        if payload is not None:
            body = json.dumps(payload, ensure_ascii=False).encode(); merged["Content-Type"] = "application/json"
        request = urllib.request.Request(self.base + urllib.parse.quote(path, safe="/?=&%"), data=body, method=method, headers=merged)
        try: response = urllib.request.urlopen(request, timeout=15)
        except urllib.error.HTTPError as error: response = error
        data = response.read()
        result = json.loads(data) if data and "json" in response.headers.get("Content-Type", "") else data.decode()
        self.events.append({"method": method, "path": path, "status": response.status, "response_fields": sorted(result) if isinstance(result, dict) else "text"})
        return response.status, result, dict(response.headers)

    def expect(self, method, path, payload=None, status=200, token=None, **kw):
        code, data, _ = self.call(method, path, payload, self.teacher if token is None else token, **kw)
        assert code == status, f"{method} {path}: expected {status}, received {code}"
        return data

    def login(self, username):
        code, data, headers = self.call("POST", "/api/login", {"username": username, "password": self.password})
        assert code == 200 and data["token_type"] == "Bearer" and not any(k.lower() == "set-cookie" for k in headers)
        return data["token"]

    def rows(self, sql, args=()):
        with self.db.cursor() as cursor: cursor.execute(sql, args); return cursor.fetchall()
    def scalar(self, sql, args=()): return next(iter(self.rows(sql, args)[0].values()))
    def execute(self, sql, args=()):
        with self.db.cursor() as cursor: return cursor.execute(sql, args)

    def upload(self, title, content, token=None):
        boundary = uuid.uuid4().hex
        raw = (f'--{boundary}\r\nContent-Disposition: form-data; name="title"\r\n\r\n{title}\r\n--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="lesson5.md"\r\nContent-Type: text/markdown\r\n\r\n{content}\r\n--{boundary}--\r\n').encode()
        return self.expect("POST", "/api/materials", status=201, token=token, raw=raw, headers={"Content-Type": "multipart/form-data; boundary=" + boundary})["id"]

    def wait_ready(self):
        for _ in range(100):
            count = self.scalar("SELECT COUNT(*) FROM knowledge_chunks WHERE material_id IN (%s,%s,%s) AND index_status='ready'", (self.material,self.unbound,self.b_material))
            ready = self.scalar("SELECT COUNT(*) FROM knowledge_index_jobs j JOIN knowledge_entries e ON e.id=j.entry_id WHERE e.material_id IN (%s,%s,%s) AND j.status='ready'", (self.material,self.unbound,self.b_material))
            if count >= 3 and ready >= 3: return
            time.sleep(0.2)
        raise AssertionError("测试材料索引未就绪")

    def same404(self, method, cross, missing, token=None, payload=None):
        one = self.call(method, cross, payload, token or self.teacher)
        two = self.call(method, missing, payload, token or self.teacher)
        assert one[0] == two[0] == 404 and one[1] == two[1], "越界与不存在响应不同"

    def stream(self, content, conversation=None, token=None, extra=None, cancel_after=None):
        conversation = conversation or self.conversation["id"]
        request = urllib.request.Request(self.base + f"/api/conversations/{conversation}/messages", data=json.dumps(dict(content=content, **(extra or {})),ensure_ascii=False).encode(), method="POST", headers={"Content-Type":"application/json", "Authorization":"Bearer " + (token or self.student)})
        start=time.monotonic(); events=[]
        with urllib.request.urlopen(request,timeout=20) as response:
            assert response.headers.get("Content-Type", "").startswith("text/event-stream")
            assert response.headers.get("X-Accel-Buffering") == "no"
            kind=None;data=[]
            for raw in response:
                line=raw.decode().rstrip("\r\n")
                if line.startswith("event:"):kind=line[6:].strip()
                if line.startswith("data:"):data.append(line[5:].lstrip(" "))
                if not line and data:
                    events.append((kind,json.loads("\n".join(data)),time.monotonic()-start));kind=None;data=[]
                    if cancel_after and sum(e[0]=="delta" for e in events)>=cancel_after:break
        self.events.append({"method":"POST","path":f"/api/conversations/{conversation}/messages","status":200,"events":[e[0] for e in events]})
        return events

    def assert_saved(self, events):
        assert events[0][0] == "meta" and events[-1][0] == "done"
        text="".join(e[1]["text"] for e in events if e[0]=="delta")
        row=self.rows("SELECT * FROM tutor_messages WHERE id=%s",(events[-1][1]["message_id"],))[0]
        assert row["content"]==text and row["status"]=="complete"
        assert_citations_equal(events[0][1]["citations"], json.loads(row["citations_json"]))
        assert row["skill_name"]==(events[0][1]["skill"]["name"] if events[0][1]["skill"] else "")
        return text

    def stats(self):
        return json.load(urllib.request.urlopen(os.environ["APITEST_MOCK_STATS"],timeout=5))
    def close(self): self.db.close()


def standalone(run):
    c=Context()
    try: run(c); print("PASS")
    finally: c.close()


def assert_citations_equal(received, saved):
    """MySQL JSON can round score doubles by one ULP; evidence remains exact."""
    assert len(received)==len(saved), "引用数量变化"
    for expected,actual in zip(received,saved):
        assert expected.keys()==actual.keys(), "引用字段变化"
        for key,value in expected.items():
            if key in ('keyword_score','vector_score'):
                assert math.isclose(value,actual[key],rel_tol=1e-12,abs_tol=1e-12), "检索分数超出浮点序列化误差"
            else:
                assert value==actual[key], "引用证据或定位信息变化："+key
