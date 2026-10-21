# 五项 AI 能力验证记录

日期：2026-10-21。分支：`implement`。OpenSpec：`add-built-in-ai-capabilities`，schema 为 `spec-driven`，先完成 proposal/design/specs/tasks 并严格校验，再按任务实现。

## 本次实际通过

| 检查 | 结果与证据 |
| --- | --- |
| Go 后端 | 项目内 Go 1.27.3，`go test -race ./... -count=1 -timeout=60s` 全量通过；go.mod 目标 1.22，容器构建使用 Go 1.22 |
| 独立 MCP 服务 | `go test -race ./... -count=1 -timeout=60s` 通过 |
| 前端 | Node 24.19.0；Vitest 3 文件、12 项通过，TypeScript/Vite 构建通过 |
| OpenAPI | 生成后 `go run ./cmd/openapi -check` 通过，新增预览及模板操作均有逐接口脚本 |
| OpenSpec | `openspec validate --all --strict`：5 passed、0 failed；前置 MCP delta 有三条主规约尚不存在的 INFO，本轮没有归档 |
| HTTP 与数据库 | `apitest/run_all.py`：41/41 通过，真实独立 MySQL、Qdrant、MCP 与模拟模型 |
| 默认内容 | A/B 两班三项技能与三个应用、各自绑定、无材料会话通过；专用新班级重启后教师改名、提示词编辑、停用及技能删除均保留，无重复 |
| 知识库 | 三个学科的空绑定、绑定材料、正确引用、下一轮读取新配置、跨班/未绑定隔离通过，修正浮点比较后连续五轮通过 |
| 标准技能与 MCP | 两轮成功、声明变更拒绝、重新批准、远端超时、断开、技能停用、跨班/学生权限、审计与回放通过 |
| 重复迁移 | `migration_check.py --before` → 重启 API → `migration_check.py` 通过；比较用户、材料、技能规范化配置、模板、初始化记录、绑定、历史及 MCP 表指纹，不含凭据 |
| 新浏览器验收 | `browser_capabilities.py` 通过：模板只读/复制/覆盖确认/快照，课程和标准文件导入，服务登记与令牌框清空、发现/批准、两轮调用和回放，应用卡片与移动深色主题 |
| 旧浏览器回归 | `browser_check.py` 通过：教师编辑、课程文件导入、学生流式/追问/引用/刷新/取消、移动主题、旧导航、Swagger 实际 Bearer 试调 |
| 静态与差异 | 新增 Python 脚本语法检查、浮点比较单元测试、`git diff --check` 与每次暂存区检查通过 |

真实浏览器测试使用项目内 Chromium 的独立临时配置并启用 Chromium sandbox。普通工具沙箱阻止 macOS MachPort 通信后，仅对本地验收命令取得定向执行授权，没有复用私人浏览器资料或修改系统权限。

## 定位和修正的验收问题

MySQL JSON 回读检索分数时出现一个 ULP 的舍入差异，例如 `0.42394372820854187` → `0.4239437282085418`。原文、材料、切片及偏移一致。验收仅对 `keyword_score`、`vector_score` 使用 `1e-12` 容差，字段集和所有证据仍严格比较；新增负向单元测试保证真实分数变化、跨材料或原文变化仍失败。

旧浏览器脚本使用已过时标题“依据资料问答”，已改为页面现有“知识问答”后完成全套回归。未为满足旧定位器改动产品文案。

## 边界与未完成项

- 模型/嵌入来自测试模拟网关。没有读取私人 `.env` 或使用真实网关密钥，真实模型的技能遵循、工具自主选择及回答质量未验证，OpenSpec 6.4 保持未完成。
- Docker 验收采用缓存基础镜像加最新编译产物，不代表生产 Dockerfile 的远端基础镜像拉取与构建已完成。
- 本轮未同步主规约或归档任何变更，未合并主分支、部署生产或重置原应用数据卷。
- 最终推送在第六次文档提交之后执行；其实际结果以交付回复及远端 Git 回执为准，不预先声称成功。

## 本地证据

`apitest/reports/index.html` 为逐接口报告；`migration-snapshot.json` 为不含凭据的数据指纹。`builtin-apps.png`、`standard-skill-mcp.png`、`builtin-mobile-dark.png` 及原浏览器截图为实际页面证据，均被 Git 忽略。

五项能力按独立 Conventional Commits 提交，最后 README 与本记录单独提交，总计六次；原有未跟踪的 `output/`、`tmp/` 保留且不纳入提交。
