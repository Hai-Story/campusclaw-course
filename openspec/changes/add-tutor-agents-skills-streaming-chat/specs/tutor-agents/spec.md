## Purpose

让教师为本班配置多个解题智能体，为每个智能体设置角色提示词以及可使用的技能和材料，并让学生在服务端权限与班级隔离下选择已启用的助手开展会话。

## ADDED Requirements

### Requirement: Class-scoped agent management

系统 MUST 提供已登录用户可调用的 `GET /api/agents` 和 `GET /api/agents/{id}`，以及仅教师可调用的 `POST /api/agents`、`PUT /api/agents/{id}`。班级和角色 MUST 取自当前服务端用户。教师列表 MUST 包括本班已停用智能体；学生列表 MUST 只包括本班已启用智能体。学生按编号访问已停用、他班或不存在的智能体 MUST 得到相同的 404 状态和响应体。

#### Scenario: Teacher manages an agent

- **WHEN** 教师提交本班智能体名称、提示词、启用状态和两类绑定列表
- **THEN** 新建返回 201，修改返回 200，随后读取与提交的有效配置一致

#### Scenario: Student visibility and write permissions

- **WHEN** 学生读取智能体列表、读取已停用智能体或调用任一智能体写接口
- **THEN** 列表只含本班已启用智能体，已停用对象返回同形 404，写操作返回 403 且没有数据库变更

#### Scenario: Bearer authentication is required

- **WHEN** 请求没有有效 Bearer Token，或仅提供旧登录 Cookie
- **THEN** 智能体接口返回 401，且不返回配置或绑定信息

### Requirement: Agent fields and prompt fallback

智能体名称 MUST 在去除首尾空白后为 1–64 个 Unicode 字符，并在班内唯一；提示词 MUST 不超过 2000 个 Unicode 字符。重复名称、超长字段或非法启用值 MUST 返回 400。空提示词 MUST 合法，并在回答时回退到默认中文解题助手角色。

#### Scenario: Invalid or duplicate agent name

- **WHEN** 教师提交空名称、65 字名称、2001 字提示词或本班已有名称
- **THEN** 接口返回 400，已有智能体及其绑定保持原值

#### Scenario: Empty role prompt

- **WHEN** 教师保存空提示词并在该智能体的会话中提问
- **THEN** 系统使用默认角色继续回答，不因提示词为空拒绝请求

### Requirement: Atomic same-class bindings

智能体 MUST 支持零到多个技能和零到多个材料的绑定；只能绑定本班存在的资源。更新配置与两类绑定 MUST 全部成功或全部失败。跨班与不存在绑定资源 MUST 返回相同的 404；空绑定列表 MUST 合法。删除技能或材料后 MUST 清除相应绑定，不影响其他绑定或已有会话。

#### Scenario: Bindings are replaced together

- **WHEN** 教师修改名称和提示词，同时替换技能、材料列表
- **THEN** 后续读取和下一轮提问使用同一份完整配置，不出现部分保存状态

#### Scenario: Cross-class binding is hidden

- **WHEN** 教师提交包含他班技能或材料的绑定，与提交不存在编号的绑定相比较
- **THEN** 两者均返回同形 404，智能体和原绑定均不发生变更

#### Scenario: Empty bindings and deleted resources

- **WHEN** 智能体无任何材料绑定，或其某个绑定资源已被依法删除
- **THEN** 无材料绑定时检索结果为空且仍可回答；已删除资源不再参与后续路由或检索
