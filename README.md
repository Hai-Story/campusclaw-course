# CampusClaw · 教学 AI 应用与可追溯知识库

CampusClaw 是面向中小学的教研智能体，内置学习应用、提示词模板、可上传技能及 MCP 工具接入。本仓库提供账号或 Token 登录、教师/学生权限、班级隔离、`.txt` / `.md` 材料管理，以及本班知识切片的关键字、向量、混合检索和带出处的简短问答。前端为 React，API 为 Go；MySQL 保存材料与切片，Qdrant 保存向量。

身份认证、权限、知识入库、检索与问答的代码流程和实现边界，见 [身份认证与知识库问答实现方案](docs/AUTH-AND-KNOWLEDGE.md)。

第五课已新增智能体配置、技能文件、私有会话、真实 SSE 流式链路、Swagger 与逐接口数据库验收；使用与验证方法见 [第五课实现说明](docs/LESSON-5.md)。下文单次问答的契约继续保留，新会话无资料时仍可回答。

第六课新增同仓 `mcp-server/`、教师本班服务器管理、发现与版本批准、技能工具引用、模型多轮调用和审计；使用方法见 [第六课实现说明](docs/LESSON-6.md)，本次验证状态见 [第六课验证记录](docs/LESSON-6-VALIDATION.md)。启动前还需设置独立的 `MCP_AUTH_TOKEN`，教师登记时输入同一值。

## 五项 AI 能力

| 能力 | 入口与行为 |
| --- | --- |
| 内置多个 AI 应用 | “解题会话”优先展示数学解题助手、语文阅读助手、英语学习助手，点击卡片直接创建会话。教师可新增或调整本班助手。 |
| 提示词模板 | “助手配置 → 提示词模板库”提供三个只读内置模板，可复制为本班自定义模板并创建、编辑、删除。预览后应用到角色提示词，保存助手生效。 |
| 知识库检索 | “知识检索”支持关键词、向量、混合检索；助手配置可绑定本班材料，回答携带材料及切片引用。 |
| 内置和上传 Skill | 预置数学解题引导、语文阅读分析、英语学习指导；教师可导入课程 `.txt/.md` 或标准单文件 `SKILL.md`，预览、编辑、启停、删除并绑定助手。 |
| 外部工具与 MCP | “助手配置 → 本班工具服务”登记、连接并发现获准的 HTTP MCP 工具，通过技能批准并参与模型多轮调用，保存调用记录和审计。 |

启动时会对所有现有班级初始化默认技能、模板和应用；以后新增的班级在下一次启动时补齐。初始化有独立班级版本记录，重复启动不覆盖教师修改、不重新启用停用助手、不恢复已删除技能。同名用户对象保持原样，不自动修改其绑定。

默认应用不绑定材料或外部服务，无材料也能正常对话，此时没有资料引用。默认技能分别按标题中的“数学”“语文”“英语”筛选已绑定材料，可由教师调整资料范围。详细步骤见 [五项能力操作说明](docs/AI-CAPABILITIES.md)，本轮实际结果见 [验证记录](docs/AI-CAPABILITIES-VALIDATION.md)。

### 提示词模板

教师选择模板可预览正文，点击“应用到角色提示词”；替换已有提示词前需要确认，再点击“保存助手”。应用模板只复制正文，后续修改或删除模板不会影响已经保存的助手。

模板管理接口为教师专用：`GET/POST /api/prompt-templates`、`PUT/DELETE /api/prompt-templates/{id}`。自定义模板按班级隔离，内置模板只读；本期采用固定文本，不支持变量填充。

### 标准 SKILL.md 导入

例如创建以下 UTF-8 文件：

```markdown
---
name: math-step-guide
description: 检查数学解题步骤，提供下一步提示。
---
先判断学生已经写出的步骤，指出错误与检查方向，鼓励学生自行继续推导。
使用提供的资料时准确标注引用，不编造来源。
```

教师在“技能文件”选择文件，查看解析预览，再填写触发关键词（例如“方程，移项”）、资料标题范围、工具引用及启用状态。保存后勾选绑定该技能的助手并保存助手。编辑原文后应重新预览；文件的原始正文会完整保留，触发与工具配置单独保存。

- `name`、`description` 与 Markdown 正文不能为空；标准技能名称最多 64 字，文件总计最多 8000 个 Unicode 字符。标准正文不受旧课程格式 2000 字说明限制。
- 中文课程格式继续兼容：名称最多 32 字、说明最多 2000 字；原有仅提交 `source` 的 API 调用保留。
- 标准格式创建/修改使用 `source` 加 `routing: {trigger, subject, tools, enabled}`；`POST /api/skills/preview` 供教师预览，不写入数据库。
- 触发关键词沿用现有匹配机制。文件中的 `allowed-tools` 等附加元数据不授予任何工具权限。
- 只导入单个指令文件，不加载附属文件、不安装依赖、不执行上传脚本，不支持 ZIP 技能包。

### MCP 接入与范围

部署者先允许服务根地址及只读工具（`MCP_ALLOWED_BASE_URLS`、`MCP_ALLOWED_TOOLS`），教师再登记独立 MCP Bearer 令牌，点击“连接并发现”。同仓默认服务提供 `formula_hint` 和 `unit_convert`；在技能工具字段填写 `工具名@服务器引用名`，核对声明后保存，表示批准当前版本。

技能绑定助手且问题匹配时，模型自行选择是否调用；界面显示轮次与成功、出错、被拒状态，刷新后可以回放。重新发现引起声明变化时，旧批准失效，须重新核对并保存相关技能。默认初始化不会连接或批准外部服务。

当前兼容 MCP `2025-06-18` 的 HTTP 工具子集，不支持 stdio、资源、提示模板、会话恢复或通用写入工具。应用登录 Token 与 MCP 令牌用途独立。

## 范围

- 教师：查看、搜索、下载、上传和删除本班材料，并可重新建立材料索引；删除预置材料后重启也不会恢复。
- 学生：查看、搜索和下载本班材料；上传接口由服务端返回 403。
- 班级：只取 Token 对应用户在服务端数据库中的 `class_id`。列表强制按班过滤；详情和文件先取行再核对归属。
- 跨班：与不存在资源返回完全相同的 404，避免泄露资源是否存在。
- 知识库：MySQL 保留原文与切片；Qdrant 只保存向量和标识。检索及问答都由 Token 对应的班级限定，结果可返回材料、切片序号与字符区间。

## 不做的内容

本版本不包含作业和成绩、超级管理员、OAuth/SSO、注册改密、PDF/Word/图片解析、材料在线编辑、Kubernetes、CI、公网域名、HTTPS 或多副本高可用。

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

模型名与嵌入维度由 `.env` 决定，仓库中的示例值只是占位符。网页分别提供“知识检索”和“知识问答”入口；检索中的“语义”模式调用嵌入模型，没有单独的浏览器 embedding 接口。

桌面左侧的“知识问答”进入聊天界面，手机通过同名导航进入。可连续提问，每轮显示用户问题、助手回答和可打开原文的来源。输入支持 Enter 发送、Shift+Enter 换行及中文输入法，问题上限为 200 个 Unicode 字符。后续提问仅向后端发送最近六条已完成历史消息，每条最多 1000 字符；检索仍以当前问题和服务端班级为准。回答期间可以停止，失败或停止后可重试最后一题，也可清空对话。切换页面保留本次已完成聊天并停止待处理请求，清空、退出登录或刷新页面会丢弃聊天。对话没有写入浏览器持久存储。

## 预置账号

口令是你在 `.env` 中填写的值；应用启动时只把 bcrypt 哈希写入数据库。

账号密码登录成功后，`POST /api/login` 返回 `token`、`token_type: "Bearer"` 和 `expires_in`，不设置会话 Cookie。网页会把 Token 保存到当前标签的 `sessionStorage`，后续材料、知识检索、上传、下载等请求都发送 `Authorization: Bearer <token>`，并禁用请求携带 Cookie。也可以在登录页选择「Token 登录」，粘贴本应用签发且尚未过期的 Token；服务端会先通过 `/api/me` 校验身份。Token 不包含角色和班级授权信息，服务端仍以数据库中的当前用户信息为准。

脚本或 API 客户端可在登录响应中获取 `token`，然后按下例访问受保护接口（请替换占位符，不要把真实 Token 提交到仓库）：

```sh
curl -H 'Authorization: Bearer <token>' http://localhost:8080/api/materials
```

Token 使用 `SESSION_SECRET` 签名，有效期由 `SESSION_TTL_MINUTES` 决定。退出登录会撤销对应数据库会话，因此同一 Token 随即失效。受保护接口只接受 Bearer Token；旧版 Cookie 不再能登录。

| 账号 | 角色 | 班级 | 权限 |
| --- | --- | --- | --- |
| `teacher_a` | 教师 | A 班 | 上传、查看、搜索、下载、删除、重建索引 |
| `student_a1` | 学生 | A 班 | 查看、搜索、下载 |
| `student_b1` | 学生 | B 班 | 查看、搜索、下载 |

两班各有一份标题可区分的种子材料。初始化幂等，重启不会复制种子，也不会覆盖教师上传的内容。

## 知识检索与溯源

`GET /api/knowledge/search?q=问题&mode=hybrid&limit=10` 需要 Bearer Token。`q` 为 1–200 个字符，`limit` 默认为 10、可设为 1–20；`mode` 可为 `keyword`、`vector` 或 `hybrid`（默认）。关键字模式只查 MySQL 的二元 ngram 全文索引；向量模式调用嵌入网关和 Qdrant，并剔除余弦相似度低于 0.35 的候选；混合模式以 RRF `k=60` 融合两路通过阈值的名次。结果包含 MySQL 切片摘录、材料 ID/标题、原文件名、切片序号、字符区间及偏移基准。请求中的 `class_id` 不参与授权。

`POST /api/ask` 接收 `{"question":"..."}`，问题同样限 1–200 个字符，以本班混合检索的前四条切片作为依据。可选 `history` 最多六条；只接受 `user`、`assistant` 角色，每条最多 1000 字符。无命中时返回 `资料中未找到相关内容`、空 `citations`，不调用对话网关；有命中时返回带 `[1]` 等引用的简短回答。响应同时返回本班 `index_state`，聊天中可显示索引中或索引失败的提示。两接口均要求 Bearer Token，客户端不能提供 system 指令覆盖服务端提示。

教师可在材料列表或详情中确认删除本班材料。`DELETE /api/materials/{id}` 成功返回 204；学生返回 403，跨班与不存在材料返回同形 404，索引处理中返回 409 以便稍后重试。删除会清理原文、私有文件、切片、索引任务和向量；预置材料的删除记录会阻止服务重启时重新播种。此操作不可撤销。

教师上传可在表单中选择切分策略：`auto` 为最多 800 个 Unicode 字符、重叠 80 字；`custom` 支持 100–2000 字、0–50% 重叠、换行/空行/句号分隔和可选预处理；`hierarchy` 按 Markdown 一级至三级标题分章，较长章节仍会继续切分。原文始终保留。预处理过的切片偏移相对于处理后文本，界面会明确标注。教师也可在材料详情中按新策略请求 `POST /api/materials/{id}/reindex`，成功返回 202，表示索引任务已排队；跨班材料与未知 ID 同形 404。

检索响应的 `index_state` 为 `building`、`degraded` 或 `ready`。检索正常但无依据时是 HTTP 200、空 `hits` 和固定文案；Qdrant 或嵌入网关故障时，向量/混合模式返回 503，关键字模式仍可读取已就绪切片。现有 `/api/materials?q=...` 始终保留材料列表搜索语义。

## 验收

本地验证需要 Go 1.22、Node.js/npm 和 OpenSpec CLI。先执行测试、前端构建与规约校验：

```sh
cd backend && go test ./...
cd ../frontend && npm ci && npm test && npm run build
cd .. && openspec validate add-auth-rbac-class-knowledge --strict
openspec validate add-class-scoped-knowledge-retrieval --strict
```

Compose 启动后运行关键行为验收：

```sh
sh scripts/verify.sh
```

脚本验证公开 health、未登录及旧 Cookie 401、登录响应不设置 Cookie、Bearer 身份查询、篡改 Token 401、教师上传 201、学生上传 403、跨班 404、登出后 Token 401，并比较跨班与不存在资源的响应体完全一致。脚本会向 A 班上传一份验收材料，持久卷中会保留该数据。

检索的本地集成验收可使用 `scripts/mock_gateway.py`（仅测试，提供确定性向量和回答）及 `scripts/verify_knowledge.py`。将 Compose 的 `GATEWAY_BASE_URL` 指向可从 API 容器访问的模拟网关 `/v1`，并将 `EMBEDDING_DIMENSION` 设为模拟网关默认的 `8`（或与 `MOCK_EMBEDDING_DIMENSION` 一致）。运行验收脚本时设置 `BASE_URL`、`TEST_TEACHER_PASSWORD`、`TEST_STUDENT_A_PASSWORD`、`TEST_STUDENT_B_PASSWORD`；如模拟网关统计地址与默认的 `http://127.0.0.1:18765/stats` 不同，还需设置 `MOCK_GATEWAY_STATS`。脚本会上传测试材料并重建索引。模拟网关只验证链路、班级隔离与失败路径，不能衡量真实模型的语义质量。

完整 Compose 验收应在填入真实、获准使用的网关配置后执行；本仓库不附带网关密钥。

聊天界面的浏览器验收脚本为 `scripts/verify_chatbot.py`，只使用模拟 API、测试账号和测试 Token，不读取 `.env`、不调用真实网关，也不写入数据库。它检查连续聊天和历史上限、来源定位、无依据/索引提示、输入法与 Unicode、失败重试、停止/清空后迟到响应、页面切换、主题、手机布局及退出登录。安装 Playwright 时使用项目虚拟环境：

```sh
python3 -m venv .venv
.venv/bin/python -m pip install playwright
# 若已有 Chromium，可设置 PLAYWRIGHT_CHROMIUM_EXECUTABLE 指向其可执行文件；
# 否则把浏览器也安装在项目目录内。
PLAYWRIGHT_BROWSERS_PATH="$PWD/.tools/playwright" .venv/bin/python -m playwright install chromium
# 在另一个终端运行：cd frontend && npm run dev -- --host 127.0.0.1 --port 5178
PLAYWRIGHT_BROWSERS_PATH="$PWD/.tools/playwright" .venv/bin/python scripts/verify_chatbot.py
```

可用 `CHATBOT_BASE_URL` 覆盖验收地址，截图写入 `output/playwright/`。该模拟验收不能代替真实 `.env` 网关、MySQL 和 Qdrant 的 Compose 联调。

## 本轮功能验收

2026-10-21 已完成：后端全量竞态测试、独立 MCP 服务竞态测试、前端 12 项测试及构建、OpenAPI 校验、全部 5 个 OpenSpec 变更严格校验、41/41 逐接口数据库验收、重复迁移指纹检查，以及新旧真实浏览器端到端验收。

浏览器已验证模板复制、覆盖确认与独立快照、两种技能格式导入、MCP 发现/批准/两轮调用/刷新回放、应用卡片、移动深色主题、原有材料引用和 Swagger Bearer 试调。隔离环境使用真实 MySQL、Qdrant 和独立 MCP 服务；模型和嵌入来自确定性模拟网关，**真实模型的技能遵循、工具选择与回答质量仍待验证**。

启动独立测试环境后可运行：

```sh
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/run_all.py
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/verify_builtin_apps.py
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/verify_builtin_knowledge.py
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/verify_standard_mcp.py
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/browser_capabilities.py
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/browser_check.py
```

这些脚本只允许显式授权的本机 `campusclaw_test_` 数据库，会写入测试数据。报告、截图和指纹位于 Git 忽略的 `apitest/reports/`。本次使用缓存基础镜像构建最新程序完成 Docker 验收；不代表生产 Dockerfile 的远程镜像拉取或生产部署已通过。

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

- Bearer Token 是 HMAC-SHA256 签名的 JWT，并与可撤销的服务端会话关联。网页只在当前标签的 `sessionStorage` 保存 Token，不在浏览器存储角色或班级。
- 登录成功创建 Token 对应的服务端会话，登出删除该会话行。用户名不存在、密码错误和锁定期使用同一失败响应。
- 文件名只用于展示；磁盘存储名由服务端随机生成。仅允许非空 UTF-8 的 `.txt` / `.md`，大小上限来自环境变量。
- 材料、知识库行与待索引任务处于同一事务；失败时回滚数据库并删除已写文件。向量索引异步重试，不影响原文保存。
- 关键字 SQL 与 Qdrant 查询都用会话班级过滤；向量命中再回 MySQL 按班级、版本、状态和原文哈希核对。Qdrant 不保存切片正文。
- Nginx 不提供 `/uploads` 静态目录，文件下载必须通过鉴权 API。

## 规约流程

基础迭代的 OpenSpec change 是 `add-auth-rbac-class-knowledge`；检索与问答变更是 `add-class-scoped-knowledge-retrieval`。两者的 `proposal.md`、`design.md`、delta spec 和 `tasks.md` 是对应实现依据。基础变更仍有独立的 Compose 发布门禁，不能因本变更通过静态测试而自动归档。

本轮五项能力的 OpenSpec 变更为 `add-built-in-ai-capabilities`，包含 proposal、design、三份新增能力规格和按提交划分的 tasks。前置教学/MCP 规约仍在途，本轮未同步主规约或归档变更。
