# 第五课：智能体、技能与流式会话

对应 OpenSpec：`add-tutor-agents-skills-streaming-chat`。本次在现有 React / Go / MySQL / Qdrant 项目上增量实现，旧材料、知识检索及单次问答继续保留。

## 使用流程

1. 教师上传本班 `.txt/.md` 材料，等待知识索引就绪。
2. 在「助手配置」创建技能：可以编辑中文字段文件，也可以导入文件。示例见 `docs/examples/math-guidance.txt`；空行前的名称、适用条件、启用为必填，空行后写说明；工具仅允许 `knowledge_search`。
3. 创建智能体，填写角色提示词，选择技能与材料，保存并启用。提示词留空使用默认中文助手。学生只能选本班已启用智能体。
4. 学生在「解题会话」选择助手，新建会话后提问或追问。引用先显示，文字随后逐段出现；点击引用进入已有材料详情。刷新后可从「我的会话」恢复。
5. 教师修改提示词、绑定或技能开关，下一轮立即生效。会话固定使用原智能体，换助手需新建；停用助手只阻止新选用，已有会话可以继续。

学生调用教师写接口返回 403。智能体、技能、会话的越界/不可访问/不存在对象均返回同形 404。教师只能读取本人会话，不能读取学生的私有会话。

## 接口与流协议

- 智能体：`GET/POST /api/agents`、`GET/PUT /api/agents/{id}`。写入 `name,prompt,enabled,skill_ids,material_ids`，配置和两类绑定在事务中完整替换。
- 技能：`GET/POST /api/skills`、`GET/PUT/DELETE /api/skills/{id}`。创建/改写只接收 `{"source":"原文"}`；启停修改原文中的「启用：是/否」。
- 会话：`GET/POST /api/conversations`、`GET /api/conversations/{id}`。创建接收 `{"agent_id":1}`。
- 消息：`POST /api/conversations/{id}/messages`，接收 `{"content":"问题"}`，长度 2–500 个 Unicode 字符。额外传入的角色、班级、历史或智能体编号不改变服务端授权和配置。

所有业务接口使用 `Authorization: Bearer <token>`；不使用 Cookie。文档为 `/openapi.json`，Swagger 为 `/swagger/`，两者公开可访问。

事件顺序是 `meta → delta × N → done`，失败则 `error`。meta 的 `skill` 是命中技能或 null，`citations` 是本轮资料；delta 的 `text` 仅为新增片段；done 的 `message_id` 指向已提交的完整助手消息。error 表示失败，不能当成成功完成。

上游以 `stream:true` 调用兼容模型网关，解析真实 SSE。服务端每帧 flush；Nginx 关闭代理缓冲，并转发 `X-Accel-Buffering:no`。浏览器使用带 Bearer 的 POST fetch 读流，支持停止生成，不自动重试写请求。同一会话并发提交返回 409。

会话取最近六条有效消息，原始正文每条最多 500 字；当前提问只组装一次。检索在绑定范围内进行，SQL 排序前和 Qdrant 查询均限制材料，回源再次核对班级、材料、版本与内容哈希。最多四条资料，每条 800 字；学科筛选后引用重新连续编号。材料正文放在 user 消息中，system 仅含角色、单个技能说明及边界规则。

第五课会话无资料仍调用模型，但引用为空，并在流中移除虚构数字编号；第四课 `/api/ask` 无资料仍返回原固定文案并跳过模型。两种行为分别保留。

用户问题先入库，助手回答和当次引用/技能名随后入库。生成中断或停止时，有正文则尽力保存为 `incomplete`；首片前故障不保存虚构助手消息；数据库写失败不发成功 done。引用保存当次证据快照，原材料删除或重建索引不会改写旧消息的证据；打开已删除原材料返回 404。会话只固定智能体 ID，不保存提示词的完整版本历史。

## 文档生成

处理器上方的 `@openapi` JSON 注解是操作描述来源，共享 schema 在 `backend/internal/openapi/schemas.json`。生成器解析 Go AST，对照实际 mux 注册，检查遗漏路由、重复 operationId、处理器不符和无效 schema 引用。

```sh
cd backend
go run ./cmd/openapi
go run ./cmd/openapi -check
# 或 go generate ./internal/openapi
```

Swagger UI 固定为 `swagger-ui-dist@5.33.2`，JS/CSS/许可证随 backend 提供，初始化脚本同源加载文档，不依赖 CDN，不持久保存授权。更新静态资源运行 `sh scripts/update_swagger.sh`；依赖仅装项目 `.tools/docs`。Swagger 可试调登录、Authorize、检索和创建会话；逐段显示用业务会话页面和接口流测试核验。

## 本地隔离验收

一键脚本不会读取现有 `.env`。它生成只用于本地测试的随机口令和签名值，保存至被 Git 忽略的 `.tools/lesson5.env`，依赖安装在项目 `.venv`；测试卷和项目名为 `campusclaw-lesson5`，数据库名 `campusclaw_test_lesson5`。

```sh
sh apitest/run_local.sh
# 本机已有 Go/Nginx 镜像且镜像仓库不可达时：
APITEST_CACHED_IMAGES=1 sh apitest/run_local.sh
# 连同无头浏览器检查：
APITEST_CACHED_IMAGES=1 APITEST_BROWSER=1 sh apitest/run_local.sh
```

本地验收页面是 `http://127.0.0.1:18085`，Swagger 是 `http://127.0.0.1:18085/swagger/`。预置测试账号是 `teacher_a`、`student_a1`、`student_b1`；口令由本机生成，保存在上述测试文件中，不在报告或聊天中输出。测试库仅绑定 `127.0.0.1:13365`，模拟网关统计仅绑定 `127.0.0.1:18766`；这两处映射只存在于独立验收 Compose。

`apitest/run_all.py` 自动发现按 URL/方法命名的脚本，校对接口文档确保没有遗漏，核对响应内容和实际数据库记录。接口脚本也可独立运行，前提是已有测试环境变量。HTML 报告在 `apitest/reports/index.html`，包含响应字段、事件序列及断言结果，省略 Token、口令和连接值；报告、截图、缓存均不入 Git。缺配置、缺数据库核对或失败用例都会非零退出。

重启迁移验证比较用户、原文、智能体、技能、绑定与会话消息的行数和内容指纹。浏览器检查包括教师编辑/导入/启停/删除、学生追问/停止/刷新/引用、恶意 HTML、375px 深色布局及 Swagger 实际请求。浏览器引擎保存在项目 `.tools/playwright`。

结束验收时可停止这次创建的环境，不影响原应用：

```sh
docker compose --env-file .tools/lesson5.env -f apitest/compose.yaml -p campusclaw-lesson5 down
```

该命令保留测试卷。脚本不会自动清除数据卷；原有应用的 Compose、持久卷与 `.env` 不受影响。

## 验证状态与剩余门禁

本次实际结果见 `docs/LESSON-5-VALIDATION.md`。用户已明确要求先完成本地实现与验证；真实网关语义验收和实际同伴交叉验证保持未完成，不能用多个本地账号或模拟回答冒充。OpenSpec 的这两项任务仍未勾选，当前不归档变更。
