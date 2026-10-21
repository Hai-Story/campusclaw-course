# 第五课本地验证记录

验证日期：2026-10-14（Asia/Shanghai）。变更：`add-tutor-agents-skills-streaming-chat`。

## 实际结果

| 检查 | 结果与证据 |
| --- | --- |
| Go 测试 | `go test ./...` 通过；包含技能边界、路由、消息预算、引用跨块清理、上下游 SSE、材料范围过滤、私有会话、锁释放和落库失败不发 done |
| 前端测试 | `npm --prefix frontend test`：2 个文件、7 项测试通过 |
| 前端构建 | `npm --prefix frontend run build`：TypeScript 与 Vite 构建通过 |
| OpenAPI | `go run ./cmd/openapi -check` 通过，25 个已注册业务/health 操作有注解；陈旧生成物的负向检查确实非零退出 |
| 接口验收 | `apitest/run_all.py`：26/26 脚本通过，核对实际响应、内容和 MySQL；本机 HTML 报告位于 `apitest/reports/index.html` |
| 浏览器 | `apitest/browser_check.py` 通过；桌面及 375px 深色布局、教师编辑/导入/启停/删除、学生增量/追问/停止/刷新、引用打开及来源删除 404、恶意 HTML 不执行、旧入口可用 |
| Swagger | 浏览器实际执行登录、Bearer Authorize、智能体/技能/会话读取、知识检索与创建会话，收到预期 200/201；没有页面 JavaScript 异常 |
| 重复迁移 | 隔离 API 重启再次迁移后，用户、材料原文、智能体、技能、绑定与消息的行数及内容指纹完全一致 |
| 并发删除 | 独立数据库事务删除拟绑定技能，同时 API 修改智能体；返回 404，原提示词和原绑定完整保留 |
| 流式失败 | 无首片故障不造助手消息；中途故障与主动取消保存 incomplete；同会话并发 409 不新增问题；流 delta 拼接与已保存正文一致 |
| 索引重建 | 重建材料索引前后的历史正文、引用快照、技能名和状态完全一致 |
| 报告与配置门禁 | 缺环境配置返回非零且生成失败报告；报告不记录实际 Token/口令/连接值，HTML 转义文本；报告、截图、Python 缓存与工具目录经 Git ignore 核验 |
| OpenSpec | `openspec validate add-tutor-agents-skills-streaming-chat --strict` 通过 |
| 差异检查 | `git diff --check` 通过；用户原 README 认证说明链接及未跟踪的 `docs/AUTH-AND-KNOWLEDGE.md` 保留 |

Go 在 `golang:1.22-alpine` 容器中运行，只挂载当前项目，缓存写在项目 `.tools`。MySQL 8.0、Qdrant v1.12.4、模拟模型网关与 Nginx 在独立 `campusclaw-lesson5` Compose 项目中运行。

标准多阶段镜像构建曾遇到 Docker Hub 认证请求超时。已确认 7897 代理可连接，但没有修改 Docker Desktop/daemon 系统配置；本次使用本机缓存的 Go/Nginx 基础镜像、刚编译的 Go 二进制和刚构建的前端完成本地容器验收。此结果不等于标准多阶段镜像从远程重新构建已通过。复现可用 `APITEST_CACHED_IMAGES=1 sh apitest/run_local.sh`。

## 模拟与真实验收边界

模拟网关验证协议、输入编排、技能切换、证据范围、失败处理和数据库一致性。它按收到的策略构造确定性文本，不能证明真实模型的语义质量或遵从程度。

用户已明确选择“暂时没有，先完成本地实现与验证”。以下两项保持 OpenSpec 未完成状态：

- **10.3 真实模型行为**：通过正常授权的测试网关核验解题引导不报最终答案、关闭技能后直接回答、无资料仍正常回答及引用准确。
- **10.4 同伴交叉验证**：由实际同伴验证本人会话隔离、智能体隔离、班级边界与技能开关，并记录脱敏请求/响应。

本次没有生产部署、主分支合并、强制推送、原应用数据卷变更或 OpenSpec 归档，也未读取已有私人 `.env`。实现任务完成 38/40；全部本地实现和本地验证任务已完成。
