# 第六课实现与验证记录

日期：2026-10-21（Asia/Shanghai）。变更：`add-mcp-tools-integration`。

## 本次实现

- 同仓 mcp-server：独立 Go module、容器构建文件、健康检查、Bearer、HTTP JSON-RPC 和两个只读学习提示工具。
- backend/internal/mcp：班级服务器管理、握手/分页发现、Schema 校验、声明版本与技能批准、配额、受限出站、工具执行与审计。
- 幂等迁移：六张 MCP 表及消息调用元信息列；历史审计/声明版本与服务器删除解耦。
- 技能与会话：工具引用解析、保存时原子校验、运行时复核、模型自主工具决策与多轮回传、最终真流式、重复 meta 合并、取消和元信息回放。
- 前端：教师服务器/工具/变更/审计管理，技能保存前声明核对，学生调用状态展示。
- OpenAPI/Swagger 注解与生成物、10 个新增逐接口脚本、独立第六课 Compose 验收入口、模拟模型工具决策、文档与示例评测。

## 已通过的检查

实际运行环境：项目内 `.tools/lesson6-runtime/go/bin/go`，版本 Go 1.27.3，项目 go.mod 目标仍是 1.22；Node 24.19.0。未在系统安装 Go/Node/Python 依赖。Go 工具下载校验了官方发布清单的 SHA-256；Go 依赖和构建缓存均位于项目 `.tools/`。

| 检查 | 实际结果及边界 |
| --- | --- |
| 后端 `go test ./...` | 通过，包含既有认证/材料/知识/教学测试和新增 MCP 测试；数据库单元测试使用 sqlmock |
| 后端 `go test -race ./... -count=1 -timeout=60s` | 通过；覆盖两轮与同轮多调用、配额拒绝、断开取消及慢发现不覆盖断开 |
| mcp-server `go test -race ./...` 与独立构建 | 通过；协议、鉴权、提示与参数行为 |
| 真实独立 MCP 进程直连 | 健康、缺/错令牌 401、握手、通知 202、工具发现、两个 tools/call 均通过；使用本次随机测试令牌，未读私人 .env |
| MCP 缺令牌启动 | 非零退出并提示环境变量缺失，符合预期 |
| 前端 Vitest | 整合最新 main 后 12 项通过，含原知识聊天历史、跨字节 SSE、重复 meta 合并/去重、无调用兼容、错误与取消 |
| 前端 `npm run build` | TypeScript 与 Vite 构建通过 |
| OpenAPI 生成与 `--check` | 通过，10 个 MCP 操作均有独立接口脚本，令牌输入为 writeOnly，SSE 描述含多次 meta |
| Compose `config --quiet` | 主 Compose 与第六课测试 Compose 均通过；显式使用 /dev/null 环境文件及测试占位环境，未读取应用 .env；此项不代表容器已启动 |
| Docker 隔离构建与启动 | Docker 29.8.0；当前源码在缓存的 Go 1.22.12 Linux 容器中离线编译，构建 API、MCP 和 Web 镜像；独立 campusclaw-lesson6 项目启动成功 |
| Docker 逐接口与数据库断言 | 完整重跑 36/36 通过，含真实同网 MCP 的发现、绑定、两轮调用、错误/拒绝、审计、删除留证和消息回放；HTML 报告位于 apitest/reports/index.html（Git 忽略） |
| MySQL 重复迁移 | 隔离 MySQL 8.0；保存应用数据指纹后重启 API，再次迁移通过；用户、材料、绑定、会话、MCP 声明/批准/用量/审计与消息元信息均保持一致 |
| Python 与 shell 静态检查 | 通过，Python 缓存写入项目 .tools，run_lesson6.sh 语法有效 |
| runner 缺环境检查 | 故意缺少独立数据库并禁止测试写入，退出 1、生成 FAIL 环境预检报告；证明不会报完整成功，不是完整接口验收通过 |
| Git ignore / 空白检查 | 报告和工具缓存被忽略，diff --check 通过 |
| OpenSpec strict | 通过，含三条基线依赖 INFO，见下文 |

主要新增可运行用例位于：

- `mcp-server/main_test.go`
- `backend/internal/mcp/client_test.go`、`orchestration_test.go`
- `backend/internal/knowledge/tool_gateway_test.go`
- `backend/internal/tutor/tutor_test.go`
- `frontend/src/tutor-api.test.ts`

后端单元测试验证真实 HTTP 协议往返及宿主调用循环，但其中的数据库由 sqlmock 提供，不能据此声称 MySQL 迁移和完整接口落库验收已通过。

## 待完成的验收

Docker 基础验收已补跑。网络访问 Docker Hub 鉴权端点超时，使用 `APITEST_CACHED_IMAGES=1 sh apitest/run_lesson6.sh` 的缓存基础镜像路径：API/MCP 运行镜像为 golang:1.22-alpine，Web 为 nginx:stable-alpine；生产 Dockerfile 中的基础镜像拉取与构建尚未通过，不能将缓存路径等同于生产镜像验收。测试专用 mcp-fixture 提供故障控制入口，更深入的故障验收尚未运行。以下项目仍待完成：

1. 生产 Dockerfile 的标准基础镜像构建，以及 MCP 离线启动、声明变更再批准、累计配额、审计存储故障和分页边界等专项 Docker 验收。
2. 原第三至第五课逐接口回归已纳入 36/36；浏览器回归和上述专项故障测试仍未完成。首轮 35/36 曾出现引用一致性断言失败；单独用例、连续 30 次引用落库及整套重跑通过，尚未查明首轮差异原因。
3. 浏览器端到端、Swagger 实际试调、首片时序、375px 与深浅主题、令牌表单与版本批准交互。已准备 `apitest/browser_mcp_ui.py` 前端 fixture 脚本，但当前 Chromium 在沙箱内启动失败：`MachPortRendezvousServer ... Permission denied`；没有截图或视觉通过结论。该 fixture 脚本本身也不代替真实后端验收。
4. 通过正常授权的真实模型网关验证自主选择工具、两步多轮、提示被采用和不给最终得数。没有读取现有网关密钥；模拟模型结果仅代表协议链路。
5. 真实同伴交叉验证服务器/工具隔离、令牌不回传、断开失效和审计范围。没有用自动多账号冒充同伴签认。

`mcp-server/evaluation.xml` 为只读提示语义评测题，尚未对真实模型执行。

OpenSpec 的 tasks.md 中，只有实现与对应验证均已完成的复合任务打勾；其余任务的代码已经写入，但受上述验收条件影响继续保留未勾选。待完成项不应被理解为已验证。

## 规约与发布状态

严格校验给出三条 INFO：tutor-skills、tutor-conversations、api-documentation-testing 的主规约尚不存在，它们依赖第五课在途 delta。未来需先同步前置基线，才能独立同步/归档本变更。本次没有同步/归档、部署或重置数据；提交、推送与 PR 合并按用户明确指令执行，以 Git 历史为准。
