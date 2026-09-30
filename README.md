# CampusClaw · 可追溯知识库检索

CampusClaw 是面向中小学的教研智能体。本仓库包含登录、教师/学生权限、班级隔离、`.txt` / `.md` 材料入库，以及本班知识切片的关键字、向量、混合检索和带出处的简短问答。

## 范围

- 教师：查看、搜索、下载和上传本班材料。
- 学生：查看、搜索和下载本班材料；上传接口由服务端返回 403。
- 班级：只取服务端会话中的 `class_id`。列表强制按班过滤；详情和文件先取行再核对归属。
- 跨班：与不存在资源返回完全相同的 404，避免泄露资源是否存在。
- 知识库：MySQL 保留原文与切片；Qdrant 只保存向量和标识。检索及问答都由会话班级限定，结果可返回材料、切片序号与字符区间。

## 不做的内容

本版本不包含流式长对话、作业和成绩、超级管理员、JWT/OAuth/SSO、注册改密、PDF/Word/图片解析、在线编辑、Kubernetes、CI、公网域名、HTTPS 或多副本高可用。

这是单实例 Compose 版本。登录失败限流保存在 API 进程内；在引入共享限流器和相应规约之前不要横向扩容 API。

## 启动

需要 Docker Desktop（含 `docker compose`）。仓库不会提供可用密钥或口令。

```sh
cp .env.example .env
# 编辑 .env：替换 DB_PASSWORD、SESSION_SECRET、三个 SEED_*_PASSWORD，
# 再设置可信模型网关的 URL、密钥、模型名及真实嵌入维度。
docker compose up --build -d
docker compose ps
```

打开 [http://localhost:8080](http://localhost:8080)。存活检查地址为 [http://localhost:8080/health](http://localhost:8080/health)。若 8080 已占用，修改 `.env` 的 `WEB_PORT`。

首次启动 MySQL 可能需要几十秒；API 除了等待 Compose 健康状态，还会在进程内重试数据库连接。数据库 3306、API 8081、Qdrant 6333 与上传目录都不会映射到宿主机。API 启动后幂等补录旧材料的索引任务，后台逐条建立切片与向量。

模型网关使用 OpenAI 兼容 `/embeddings` 与 `/chat/completions` 接口。`GATEWAY_BASE_URL` 填到 `/v1`，`GATEWAY_API_KEY` 仅由 Go API 持有。`EMBEDDING_MODEL` 和 `EMBEDDING_DIMENSION` 必须与网关实际输出一致；更换模型或维度后需重建向量索引。**切片正文会发送到所配置的嵌入网关；仅在问答命中本班切片时，这些切片才会发送到对话网关。**部署前应确认网关的数据处理边界。示例文件只含占位值，不是可用凭据。

## 预置账号

口令是你在 `.env` 中填写的值；应用启动时只把 bcrypt 哈希写入数据库。

| 账号 | 角色 | 班级 | 权限 |
| --- | --- | --- | --- |
| `teacher_a` | 教师 | A 班 | 上传、查看、搜索、下载 |
| `student_a1` | 学生 | A 班 | 查看、搜索、下载 |
| `student_b1` | 学生 | B 班 | 查看、搜索、下载 |

两班各有一份标题可区分的种子材料。初始化幂等，重启不会复制种子，也不会覆盖教师上传的内容。

## 知识检索与溯源

`GET /api/knowledge/search?q=问题&mode=hybrid&limit=10` 需要登录，`mode` 可为 `keyword`、`vector` 或 `hybrid`（默认）。关键字模式只查 MySQL 的二元 ngram 全文索引；向量模式调用嵌入网关和 Qdrant，并剔除余弦相似度低于 0.35 的候选；混合模式以 RRF `k=60` 融合两路通过阈值的名次。结果包含 MySQL 切片摘录、材料 ID/标题、原文件名、切片序号、字符区间及偏移基准。请求中的 `class_id` 不参与授权。

`POST /api/ask` 接收 `{"question":"..."}`，以本班混合检索的前四条切片作为依据。无命中时返回 `资料中未找到相关内容`、空 `citations`，不调用对话网关；有命中时返回带 `[1]` 等引用的简短回答。两接口均要求会话，客户端不能提供 system 指令覆盖服务端提示。

教师上传可在表单中选择切分策略：`auto` 为最多 800 个 Unicode 字符、重叠 80 字；`custom` 支持 100–2000 字、0–50% 重叠、换行/空行/句号分隔和可选预处理；`hierarchy` 按 Markdown 一级至三级标题分章。原文始终保留。预处理过的切片偏移相对于处理后文本，界面会明确标注。教师也可在材料详情中按新策略请求 `POST /api/materials/{id}/reindex`；跨班材料与未知 ID 同形 404。

检索响应的 `index_state` 为 `building`、`degraded` 或 `ready`。检索正常但无依据时是 HTTP 200、空 `hits` 和固定文案；Qdrant 或嵌入网关故障时，向量/混合模式返回 503，关键字模式仍可读取已就绪切片。现有 `/api/materials?q=...` 始终保留材料列表搜索语义。

## 验收

先执行静态构建与规约校验：

```sh
cd backend && go test ./...
cd ../frontend && npm ci && npm run build
cd .. && openspec validate add-auth-rbac-class-knowledge --strict
openspec validate add-class-scoped-knowledge-retrieval --strict
```

Compose 启动后运行关键行为验收：

```sh
sh scripts/verify.sh
```

原有脚本验证公开 health、未登录 401、教师上传 201、学生上传 403、跨班 404，并比较跨班与不存在资源的响应体完全一致。检索的本地集成验收可使用 `scripts/mock_gateway.py`（仅测试，确定性向量和回答）及 `scripts/verify_knowledge.py`。后者需要 `BASE_URL`、三个 `TEST_*_PASSWORD` 和可选 `MOCK_GATEWAY_STATS` 环境变量；它会上传测试材料并重建索引。模拟网关只验证链路、班级隔离与失败路径，不能衡量真实模型的语义质量。

可在 API 容器内执行 `go test ./...`、`go vet ./...`，并在前端目录运行 `npm run build`。完整 Compose 验收应在填入真实、获准使用的网关配置后执行；本仓库不附带网关密钥。

## 持久化与重置

```sh
docker compose down
docker compose up -d
```

上面的操作会保留数据库、上传文件与 Qdrant 向量。若切分算法、嵌入模型或向量维度改变，可在停止 API 后执行以下可重建索引操作，再启动服务：

```sh
docker compose stop api
docker compose run --rm api reindex-all
docker compose up -d api web
```

该命令仅清空并重建派生的 Qdrant 集合，重排所有当前版本索引任务，不删除材料原文。请在重建完成前将 `index_state=building` 视为结果尚不完整。回滚到旧应用时可保留新增表和向量卷，旧材料读写路径仍使用 MySQL 与上传卷。

只有明确要清空所有本地数据时才执行：

```sh
docker compose down -v
```

## 安全边界

- Cookie 是带签名的不透明随机会话标识，使用 `HttpOnly` 与 `SameSite=Lax`；角色和班级不写入 `localStorage`。
- 登录成功换发会话，登出删除服务端会话行。用户名不存在、密码错误和锁定期使用同一失败响应。
- 文件名只用于展示；磁盘存储名由服务端随机生成。仅允许非空 UTF-8 的 `.txt` / `.md`，大小上限来自环境变量。
- 材料、知识库行与待索引任务处于同一事务；失败时回滚数据库并删除已写文件。向量索引异步重试，不影响原文保存。
- 关键字 SQL 与 Qdrant 查询都用会话班级过滤；向量命中再回 MySQL 按班级、版本、状态和原文哈希核对。Qdrant 不保存切片正文。
- Nginx 不提供 `/uploads` 静态目录，文件下载必须通过鉴权 API。

## 规约流程

基础迭代的 OpenSpec change 是 `add-auth-rbac-class-knowledge`；检索与问答变更是 `add-class-scoped-knowledge-retrieval`。两者的 `proposal.md`、`design.md`、delta spec 和 `tasks.md` 是对应实现依据。基础变更仍有独立的 Compose 发布门禁，不能因本变更通过静态测试而自动归档。
