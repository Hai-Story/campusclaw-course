# CampusClaw

面向中小学课堂的 AI 学习助手，「互联网软件开发」课程实践项目。

教师上传讲义、配置助手，学生可以查资料、问问题，也可以带着自己的解题步骤继续追问。回答用到班级资料时会附上原文引用，方便核对。项目内置数学、语文、英语三个助手，支持保存会话，并可通过 MCP 接入教学工具。

[本地运行](#本地运行) · [第一次使用](#第一次使用) · [开发](#开发) · [文档](#文档)

## 功能

- **班级知识库**：上传文本或 Markdown 材料，按关键词、向量或混合方式检索，查看回答引用的原文。
- **学习助手**：流式回答、连续追问、停止生成和历史会话恢复。教师可以调整提示词、绑定材料，也可以导入单文件 `SKILL.md`。
- **教学工具**：附带公式步骤提示、单位换算方向两个 MCP 示例工具。教师连接服务、确认工具版本后，助手才能调用；调用过程有记录可查。

材料和助手按班级隔离，会话仅本人可见，教师也不能查看学生的私有会话。页面支持手机访问和深浅主题。

## 本地运行

准备好 Docker Desktop（或 Docker Engine + Compose 插件）和模型网关。网关需要兼容 OpenAI 的 `/embeddings`、`/chat/completions` 接口，支持 SSE 流式输出；使用 MCP 时，对话模型还需要支持工具调用。

### 1. 获取代码

```sh
git clone https://github.com/Hai-Story/campusclaw-course.git
cd campusclaw-course
cp .env.example .env
```

如果已有 `.env`，保留原文件，对照 [.env.example](.env.example) 补齐配置。

### 2. 配置环境

编辑 `.env`，替换这些示例值，其余项可以先用默认值：

| 配置 | 说明 |
| --- | --- |
| `DB_PASSWORD` | 本地数据库密码 |
| `SESSION_SECRET` | 至少 32 个随机字符，用于签名登录 Token |
| `SEED_TEACHER_A_PASSWORD` | 教师账号初始密码 |
| `SEED_STUDENT_A1_PASSWORD`、`SEED_STUDENT_B1_PASSWORD` | 两个学生账号的初始密码 |
| `GATEWAY_BASE_URL`、`GATEWAY_API_KEY` | 模型网关地址（通常以 `/v1` 结尾）和密钥 |
| `CHAT_MODEL` | 对话模型名称 |
| `EMBEDDING_MODEL`、`EMBEDDING_DIMENSION` | 嵌入模型名称及其实际输出维度，二者必须匹配 |
| `MCP_AUTH_TOKEN` | MCP 服务独立令牌，不使用应用登录 Token |

默认网页端口为 `8080`，占用时修改 `WEB_PORT`。Compose 中的 `API_ADDR` 保持 `:8081`，与 Nginx 和健康检查一致。

材料切片、提问及相关历史会发送到配置的模型网关。请选用适合处理这些内容的服务，不要提交 `.env`。

### 3. 启动

```sh
docker compose up --build -d
docker compose ps
```

首次启动会构建镜像，等待 MySQL 就绪，并创建示例账号、两班材料和默认助手。启动后访问 [localhost:8080](http://localhost:8080)，API 文档在 [Swagger UI](http://localhost:8080/swagger/)。如果改过 `WEB_PORT`，地址中的端口也要对应修改。

| 账号 | 身份 | 密码来源 |
| --- | --- | --- |
| `teacher_a` | A 班教师 | `SEED_TEACHER_A_PASSWORD` |
| `student_a1` | A 班学生 | `SEED_STUDENT_A1_PASSWORD` |
| `student_b1` | B 班学生 | `SEED_STUDENT_B1_PASSWORD` |

初始密码只在创建账号时使用，之后修改 `.env` 不会重置已有密码。启动失败时，先看 `docker compose logs --tail=100 api`。

## 第一次使用

用 `student_a1` 登录，打开「解题会话」，选择「数学解题助手」。可以先问一道题，例如：

> 我解 2(x + 3) = 14 时，第一步写成了 2x + 3 = 14。哪一步有问题？先给我一个提示。

助手不绑定材料也能对话。要试试带资料引用的回答，可以走一遍教师和学生的流程：

1. 用 `teacher_a` 登录，在「材料库」上传一份 UTF-8 的 `.txt` 或 `.md`，标题中写上学科，例如「数学：一元一次方程」，等待索引就绪。
2. 在「助手配置」选中「数学解题助手」，勾选刚上传的材料并保存。
3. 切回 `student_a1`，围绕材料内容提问，点击回答中的引用查看原文。

默认技能按材料标题里的「数学」「语文」「英语」筛选资料；教师也可以调整技能的资料范围。助手配置保存后，已有会话的下一轮就会使用新配置。

页面里有三个相关入口：

| 入口 | 用途 | 记录 |
| --- | --- | --- |
| 知识检索 | 查找本班材料中的片段 | 不创建会话 |
| 知识问答 | 根据本班资料回答；没有命中资料时不调用对话模型 | 刷新页面后清空 |
| 解题会话 | 与选定的助手连续对话；未命中资料时也能回答，但不附资料引用 | 保存到服务端，可从「我的会话」恢复 |

提示词模板可以复制后编辑，再应用到助手；技能文件用于补充处理指令，当前只读取单个文件，不执行脚本或安装依赖。MCP 服务需要先连接、发现并批准工具版本，再通过技能绑定助手。具体步骤见 [AI 能力操作说明](docs/AI-CAPABILITIES.md) 和 [MCP 接入说明](docs/LESSON-6.md)。

## 开发

前端是 React + TypeScript + Vite，后端使用 Go 标准库 HTTP 服务。MySQL 保存用户、材料原文、切片和会话，Qdrant 保存向量，模型和 MCP 请求由后端发起。

```text
浏览器 → Nginx → Go API → MySQL / Qdrant
                       → 模型网关
                       → MCP 服务
```

Compose 只向宿主机发布网页端口，其余服务在容器网络内通信。上传文件存放在独立数据卷，下载经过 API 鉴权。

```text
frontend/     页面、样式和前端测试
backend/      API、鉴权、检索、助手和 OpenAPI 文档
mcp-server/   教学工具示例服务
apitest/      隔离验收环境、接口和浏览器测试
scripts/      验证与文档维护脚本
docs/         功能说明和验证记录
openspec/     需求、设计、规格和任务
```

本地检查需要 Go 1.22+、Node.js 和 npm；前端 Docker 构建使用 Node.js 22。在仓库根目录执行：

```sh
(cd backend && go test ./...)
(cd mcp-server && go test ./...)
(cd frontend && npm ci && npm test && npm run build)
(cd backend && go run ./cmd/openapi -check)
```

前端开发用 `cd frontend && npm run dev`，默认代理到本机 `8081`。需要另行启动本地 API；默认 Compose 不发布这个端口。

完整的接口验收可运行 `sh apitest/run_lesson6.sh`。脚本需要 Python 3 和 Docker，会在项目 `.venv` 安装依赖，以独立的 `campusclaw-lesson6` Compose 项目启动服务、生成测试凭据并写入测试数据，不读取应用 `.env`。报告写入 `apitest/reports/`，浏览器验收方式见 [MCP 接入说明](docs/LESSON-6.md#验证)。

隔离验收使用真实 MySQL、Qdrant 和 MCP 服务，对话模型由模拟网关提供。因此，接口验收通过不代表真实模型的回答质量或工具选择已验证。

修改 API 后需重新生成 OpenAPI 并检查；修改需求或规格后，运行 `openspec validate --all --strict`。提交时请把功能改动和无关整理分开，并在 PR 中说明实际做过的验证。

## 常见问题

**为什么上传不了 PDF？** 当前只支持 UTF-8 的 `.txt` 和 `.md`，不解析 PDF、Word 或图片。默认上传上限为 5 MiB，可通过 `MAX_UPLOAD_BYTES` 调整。

**重启后数据还在吗？** `docker compose down` 和 `docker compose up -d` 会保留数据库、上传文件及向量数据。`docker compose down -v` 会删除数据卷，只在需要清空本地数据时使用。

**换了嵌入模型怎么办？** 修改模型及维度配置后，停止 API 并重建索引，材料原文会保留：

```sh
docker compose stop api
docker compose run --rm api reindex-all
docker compose up -d api web
```

索引完成前，检索结果可能不完整。只需重建某份材料时，在页面中操作即可。

**能直接给学校部署使用吗？** 当前是课程项目，按单实例 Compose 运行，使用预置账号，没有自助注册、改密、OAuth/SSO 或作业成绩管理。公网使用还需要配置 HTTPS；多实例运行需要调整进程内登录限流等实现。MCP 仅支持 `2025-06-18` 的 HTTP 工具子集，不支持 stdio、资源、提示模板或会话恢复。

## 文档

| 文档 | 内容 |
| --- | --- |
| [AI 能力操作说明](docs/AI-CAPABILITIES.md) | 助手绑定知识库、标准技能接入 MCP |
| [身份认证与知识库实现](docs/AUTH-AND-KNOWLEDGE.md) | 登录、班级隔离、材料入库、检索与问答 |
| [助手、技能与流式会话](docs/LESSON-5.md) | 会话保存、SSE 事件与 OpenAPI 生成 |
| [MCP 接入说明](docs/LESSON-6.md) | 工具发现、版本批准、配额、审计与隔离测试 |
| [AI 能力验证记录](docs/AI-CAPABILITIES-VALIDATION.md) | 已执行的检查、证据与待验证事项 |
| [OpenSpec 变更](openspec/changes/) | 各次功能的需求、设计与任务 |

课程文档记录对应迭代的实现；接口以当前代码和运行中的 [OpenAPI 文档](http://localhost:8080/openapi.json) 为准。
