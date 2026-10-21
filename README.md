# CampusClaw

面向中小学课堂的 AI 学习助手与班级知识库，也是「互联网软件开发」课程的实践项目。

教师上传课程材料、配置助手，学生围绕本班资料提问。项目内置数学、语文和英语三个学习助手，支持带来源的回答、可连续追问的流式会话，以及通过 MCP 接入的教学工具。

[快速开始](#快速开始) · [使用指南](#使用指南) · [开发与测试](#开发与测试) · [详细文档](#详细文档)

## 能做什么

- **管理班级资料**：上传、预览和下载 UTF-8 的 `.txt` / `.md` 文件，支持自动、自定义和按 Markdown 标题切分。教师可删除材料或重建索引。
- **检索并引用原文**：支持关键词、向量和混合检索。回答中的引用能定位到材料及切片，方便回到原文核对。
- **使用学科助手**：数学解题、语文阅读、英语学习三个应用开箱可选。会话支持流式输出、连续追问、停止生成和刷新后恢复。
- **调整教学方式**：教师可以复制提示词模板、导入单文件 `SKILL.md`，为助手设置角色、技能和材料范围。
- **接入教学工具**：自带 MCP 示例服务，提供公式步骤提示与单位换算方向；工具需经过白名单、发现和版本批准，调用过程可查看、可审计。

材料和助手按班级隔离，会话仅本人可见。教师可以管理本班资源，但不能查看学生的私有会话。网页适配桌面和手机，并提供深浅两种主题。

## 快速开始

需要 Docker Desktop（或 Docker Engine 与 Compose 插件），以及可用的模型网关。网关需兼容 OpenAI 的 `/embeddings` 和 `/chat/completions` 接口；解题会话需要 SSE 流式输出，MCP 调用还需要模型支持工具调用。

### 1. 获取代码

```sh
git clone https://github.com/Hai-Story/campusclaw-course.git
cd campusclaw-course
cp .env.example .env
```

已有本地配置时保留原 `.env`，按 [.env.example](.env.example) 补齐缺少的字段即可。

### 2. 填写配置

编辑 `.env`，至少替换以下示例值：

| 配置 | 填写内容 |
| --- | --- |
| `DB_PASSWORD` | 本地数据库密码 |
| `SESSION_SECRET` | 至少 32 个随机字符，用于签名登录 Token |
| `SEED_TEACHER_A_PASSWORD` | A 班教师的初始密码 |
| `SEED_STUDENT_A1_PASSWORD`、`SEED_STUDENT_B1_PASSWORD` | 两个学生账号的初始密码 |
| `GATEWAY_BASE_URL`、`GATEWAY_API_KEY` | 模型网关地址（通常以 `/v1` 结尾）与密钥 |
| `CHAT_MODEL` | 网关支持的对话模型名 |
| `EMBEDDING_MODEL`、`EMBEDDING_DIMENSION` | 嵌入模型名及其实际输出维度，二者必须匹配 |
| `MCP_AUTH_TOKEN` | 独立的 MCP 服务令牌，不能与应用登录 Token 混用 |

其余配置可先保留示例值。默认网页端口为 `8080`，如有冲突可修改 `WEB_PORT`。Compose 中 API 使用 `:8081`，与 Nginx 上游和健康检查保持一致。

仓库不附带可用密钥。嵌入请求会将材料切片发送到配置的网关，对话请求会发送问题、相关历史及命中的资料；请使用获准处理这些内容的服务，并勿提交 `.env`。

### 3. 启动并登录

```sh
docker compose up --build -d
docker compose ps
```

首次启动需要拉取镜像、构建应用并等待 MySQL 就绪。完成后打开：

- 网页：[localhost:8080](http://localhost:8080)
- API 文档：[Swagger UI](http://localhost:8080/swagger/)
- 存活检查：[health](http://localhost:8080/health)

如修改了 `WEB_PORT`，请相应替换地址中的端口。启动失败时可用 `docker compose logs --tail=100 api` 查看原因。

| 账号 | 身份 | 初始密码对应配置 |
| --- | --- | --- |
| `teacher_a` | A 班教师 | `SEED_TEACHER_A_PASSWORD` |
| `student_a1` | A 班学生 | `SEED_STUDENT_A1_PASSWORD` |
| `student_b1` | B 班学生 | `SEED_STUDENT_B1_PASSWORD` |

首次启动会创建账号、两班示例材料及默认助手。重复启动不会覆盖教师修改，也不会恢复已删除的默认技能。已有账号密码不会随 `.env` 中的初始密码修改而重置。

## 使用指南

### 先试一次对话

用 `student_a1` 登录，进入「解题会话」，点击「数学解题助手」，输入一道题和你已经写出的步骤。默认助手即使没有绑定材料也能对话，此时不会显示资料引用。

要让回答用上课程资料：

1. 用 `teacher_a` 登录，在「材料库」上传 `.txt` 或 `.md`，等待索引就绪。
2. 在「助手配置」选择助手，勾选本班材料并保存。
3. 切回学生账号，在「解题会话」提问；点击回答中的引用可查看原文。

默认技能按材料标题中的「数学」「语文」「英语」筛选资料。上传时可在标题中写明学科，也可以由教师调整技能的资料范围。修改助手配置后，已有会话的下一轮就会使用新配置。

### 知识问答和解题会话有什么区别

| 入口 | 适合的用途 | 没有匹配资料时 | 历史记录 |
| --- | --- | --- | --- |
| 知识检索 | 查找本班材料中的相关片段 | 显示空结果 | 不创建会话 |
| 知识问答 | 根据本班资料给出简短回答 | 返回「资料中未找到相关内容」，不调用对话模型 | 保留在当前页面运行期间，刷新后清空 |
| 解题会话 | 选择助手，连续提问和追问 | 仍可由模型回答，不附资料引用 | 保存到服务端，可从「我的会话」恢复 |

教师可以上传和管理材料；学生可以查看、检索、下载并使用助手。跨班资源与不存在的资源均返回 404。

### 配置模板、技能和 MCP

**提示词模板**用于快速填写助手的角色提示词。内置模板只读，可以复制为本班模板后编辑；应用模板后还需保存助手。模板正文按副本保存，后续修改模板不会改写已有助手。

**技能文件**用于补充特定问题的处理指令。教师可导入课程格式的 `.txt` / `.md`，也可导入带 `name`、`description` 和正文的标准 `SKILL.md`。预览后设置触发关键词、资料范围和工具引用，再绑定到助手。当前只读取单个指令文件，不执行脚本或安装依赖，也不支持 ZIP 技能包。

**MCP 工具**需要单独连接。使用同仓示例服务时，在「助手配置 → 本班工具服务」登记引用名 `campus-tools`、地址 `http://mcp-demo:8090`，并输入 `.env` 中设置的 `MCP_AUTH_TOKEN`。连接并发现后，核对工具声明，在技能中填写 `formula_hint@campus-tools` 或 `unit_convert@campus-tools`，保存批准，再绑定助手。

其他 MCP 服务需要部署者先配置 `MCP_ALLOWED_BASE_URLS` 和 `MCP_ALLOWED_TOOLS`。模型是否调用工具取决于命中的技能和当前问题；工具声明变更后需要重新批准。完整步骤见 [AI 能力操作说明](docs/AI-CAPABILITIES.md) 和 [MCP 接入说明](docs/LESSON-6.md)。

## 项目结构

前端使用 React、TypeScript 和 Vite；后端使用 Go 标准库 HTTP 服务。MySQL 保存用户、材料原文、切片和会话，Qdrant 保存向量及标识，模型请求由 Go API 发起。

```text
浏览器 → Nginx → Go API → MySQL / Qdrant
                       → 模型网关
                       → MCP 服务
```

Compose 默认只发布网页端口，数据库、API、Qdrant 和 MCP 服务留在容器网络内。上传文件存放在独立数据卷中，下载必须经过 API 鉴权。

```text
frontend/     React 页面、样式与前端测试
backend/      Go API、鉴权、知识检索、助手与 OpenAPI 文档
mcp-server/   独立的教学工具示例服务
apitest/      隔离环境、逐接口数据库验收与浏览器脚本
scripts/      鉴权、知识库、聊天验证及文档维护脚本
docs/         功能说明、实现细节和各轮验证记录
openspec/     需求、设计、规格与任务记录
```

## 开发与测试

本地检查需要 Go 1.22 或更新版本，以及 Node.js/npm；前端 Docker 构建使用 Node.js 22。以下命令从仓库根目录执行：

```sh
(cd backend && go test ./...)
(cd mcp-server && go test ./...)
(cd frontend && npm ci && npm test && npm run build)
(cd backend && go run ./cmd/openapi -check)
```

前端开发服务器通过 `cd frontend && npm run dev` 启动，默认将 `/api` 等请求代理到本机 `8081`。需要自行启动本地 API；默认 Compose 不向宿主机发布这个端口。

需要验证 API、数据库和完整对话链路时，可启动独立验收环境：

```sh
sh apitest/run_lesson6.sh
```

脚本使用独立的 `campusclaw-lesson6` Compose 项目和测试数据卷，不读取应用 `.env`；会生成测试凭据、在项目 `.venv` 安装依赖并写入测试数据。报告位于 `apitest/reports/`。浏览器验收与缓存镜像方式见 [MCP 接入说明](docs/LESSON-6.md#验证)。

隔离验收使用真实 MySQL、Qdrant、MCP 服务和模拟模型。它能检查接口、权限与调用流程，真实模型的回答质量和工具选择仍需单独验证。已有测试结果见 [AI 能力验证记录](docs/AI-CAPABILITIES-VALIDATION.md)。

修改 API 后请重新生成 OpenAPI 并运行检查；修改需求或规格时，使用 OpenSpec CLI 执行 `openspec validate --all --strict`。

## 数据维护

停止、重新启动服务会保留数据库、上传文件和向量数据：

```sh
docker compose down
docker compose up -d
```

更换嵌入模型或维度后，需要重建索引。以下命令保留材料原文，重建向量集合并重新排队索引任务：

```sh
docker compose stop api
docker compose run --rm api reindex-all
docker compose up -d api web
```

索引完成前，检索结果可能不完整。单份材料可在页面中重新索引；`docker compose down -v` 会删除全部 Compose 数据卷，仅在明确需要清空本地数据时使用。

## 当前限制

- 材料仅支持 UTF-8 文本和 Markdown，默认上传上限 5 MiB，可通过 `MAX_UPLOAD_BYTES` 调整；暂不解析 PDF、Word 或图片。
- 提供预置账号，没有自助注册、改密、OAuth/SSO、作业或成绩管理。
- MCP 兼容 `2025-06-18` 的 HTTP 工具子集，不支持 stdio、资源、提示模板、会话恢复或通用写入工具。
- 当前按单实例 Compose 运行，登录限流保存在 API 进程内；多副本部署需要改造。默认配置也不包含公网 HTTPS 和高可用部署。

## 详细文档

| 文档 | 内容 |
| --- | --- |
| [AI 能力操作说明](docs/AI-CAPABILITIES.md) | 预置助手绑定知识库、标准技能接入 MCP |
| [身份认证与知识库实现](docs/AUTH-AND-KNOWLEDGE.md) | Bearer 登录、班级隔离、材料入库、检索与问答 |
| [助手、技能与流式会话](docs/LESSON-5.md) | 会话保存、SSE 事件、OpenAPI 生成和早期验收流程 |
| [MCP 接入说明](docs/LESSON-6.md) | 工具发现、版本批准、调用配额、审计和隔离测试 |
| [AI 能力验证记录](docs/AI-CAPABILITIES-VALIDATION.md) | 已执行的检查、证据位置与尚未验证的事项 |
| [OpenSpec 变更](openspec/changes/) | 各次功能的需求、设计、规格与任务 |

课程文档保留了对应迭代时的接口和行为说明；后续新增能力以当前代码及运行中的 [OpenAPI 文档](http://localhost:8080/openapi.json) 为准。
