## Why

当前 CampusClaw 已具备 Bearer 登录、教师/学生权限、本班材料入库、混合检索和单次问答，但教师不能配置解题助手，学生也不能保存会话或逐段接收回答。本变更对照第五课原型和任务清单，在现有学期仓增量实现可配置、可追问、可溯源的解题助手。

## What Changes

- 教师创建、修改、启停本班智能体，设置角色提示词，绑定本班技能和材料；学生仅选用已启用的本班智能体。
- 教师用课程规定的中文字段技能文件维护技能，支持编写、文件导入、修改、启停、删除和取回原文；提供至少一份可用的「解题引导」示例。
- 持久化私有会话及消息，会话创建时锁定智能体；每轮从其当前绑定且启用的技能中至多选择一个，从其绑定材料中检索。
- 新增 SSE 会话接口，先发技能与引用，再逐段发正文，保存最终消息及当次证据。新会话链路即使没有资料仍调用生成模型，且不给出无依据的引用编号。
- 前端新增会话与教师配置入口，支持连续追问、流式展示、历史恢复及引用材料查看，沿用已有主题和响应式布局。
- 由处理器注解生成 OpenAPI 3 描述，提供同源 Swagger UI 和 Bearer 试调；增加按 URL 组织的 `apitest/`、数据库一致性断言及不入 Git 的 HTML 报告。
- 保留第四课 `GET /api/knowledge/search`、`POST /api/ask`、材料和认证接口的现有语义；本轮不重写或归档此前两项未完成的变更。

## Capabilities

### New Capabilities

- `tutor-agents`: 班级智能体管理、角色提示词、技能与材料绑定、教师写权限及学生选用规则。
- `tutor-skills`: 课程技能文件解析、班级技能管理、确定性路由和解题引导约束。
- `tutor-conversations`: 私有会话、固定智能体、限定检索与上下文、流式回答、证据持久化和前端交互。
- `api-documentation-testing`: 注解生成 OpenAPI、Swagger 试调、按接口组织的自动验收和数据库核对。

### Modified Capabilities

无。`openspec list --specs` 当前没有主规约；已读取两项在途变更的完整 delta specs。本变更新增会话链路，不改变其 `auth-upload` 和 `knowledge-retrieval` 的对外要求，尤其不改变旧单次问答的无依据返回规则。

## Impact

- Go API：扩展 `backend/internal/db/schema.go`，新增智能体、技能、会话编排包；在 `backend/cmd/server/main.go` 注册路由；扩展知识服务的材料范围检索与模型网关真实流式读取。
- MySQL：新增技能、智能体、两张绑定表、会话和消息表；与现有登录 `sessions` 表明确区分。Qdrant 保持已有集合和切片 ID，增加查询时的材料过滤，不另建向量系统。
- React：扩展 `frontend/src/api.ts`，新增会话与教师配置组件，接入 `App.tsx`；Nginx 和 Vite 增加文档路由并处理流式代理。
- 工具与文档：新增 OpenAPI 生成命令、固定版本的本地 Swagger 静态资源、`apitest/` 及第五课验收说明；测试依赖只安装在项目内。
- 无生产部署、主分支合并、远程资源变更、真实凭据读取或公开演示账号口令复制。

参考（2026-10-14 已读取）：[第五课范围](https://devops.hello1023.com/课件/第5课-课件-提示词AI技能与流式会话/pages/scope.html)、[第五课任务清单](https://devops.hello1023.com/课程任务清单-第5课.html)、[原型 OpenAPI](https://devops.hello1023.com/week05/openapi.json)。相关 `session.html`、`skill-model.html`、`route.html`、`stream.html`、`answer.html`、`apply.html` 也已用于细化验收要求。
