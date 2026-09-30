# Design: authentication, class isolation and knowledge ingestion

## Context

这是单实例、同源部署的浏览器应用。浏览器和静态前端不可信；Go API 是认证、授权、租户隔离和上传校验的唯一信任边界。

## Decisions

### D1：统一入口与同源 API

浏览器只访问 Nginx。Nginx 托管前端并反代 `/api` 和 `/health`；数据库、API 端口和上传目录均不映射给宿主机。这样缩小可探测面，也避免跨域 Cookie 规则成为额外变量。

### D2：服务端会话而非客户端声明

Cookie 只保存带 HMAC 签名的不透明随机会话标识，设置 `HttpOnly` 与 `SameSite=Lax`。会话行记录用户上下文，服务端每次通过用户表确认角色和班级。登录换发新 ID，登出删除会话行。JWT/OAuth 会增加撤销和客户端声明可信度问题，不适合本次同源单体边界。

### D3：班级隔离与资源存在性

集合查询以会话 `class_id` 过滤。对象查询先按 ID 取行，再核对班级；跨班与不存在都返回相同 404，同时仅在服务端日志记录真实原因。选择 404 是为了不向枚举者确认资源存在。

### D4：同步、可回滚的上传入库

仅教师可上传 `.txt`/`.md`。服务端生成存储名，验证大小、UTF-8 与非空后写盘，再在同一数据库事务写入材料和知识库正文；任一步失败即回滚并删除文件。同步流程比异步队列更容易在本迭代证明无孤儿状态。

### D5：单实例 Compose

登录失败限流在进程内，数据库会话可立即撤销。当前发布形态固定为单 API 实例；多副本需要共享限流器与额外运维设计，留待后续变更。

### D6：教师删除材料与预置材料墓碑

`DELETE /api/materials/{id}` 仅允许教师执行，班级来自会话；跨班与未知 ID 同形 404，学生返回 403。删除前按材料 ID 加锁并拒绝正在处理索引的材料，避免后台 worker 在删除后重新写入向量。MySQL 事务删除材料行，外键级联删除知识正文、切片和索引任务；Qdrant 按班级与知识条目 ID 删除派生向量。文件先在私有上传卷中重命名隔离，事务失败则恢复，事务提交后移除。前端提供明确确认和删除反馈。

预置材料删除时，在同一事务写入按预置存储名唯一的 `deleted_seed_materials` 墓碑。播种过程先检查墓碑，故重启不会恢复被教师删除的预置材料。墓碑只记录预置标识，不保留已删除的正文。

## Data model

- `classes(id, name)`
- `users(id, username, password_hash, role, class_id)`
- `sessions(token_hash, user_id, role, class_id, expires_at)`
- `materials(id, class_id, uploader_id, title, original_name, stored_name, media_type, size_bytes, source, created_at)`
- `knowledge_entries(id, material_id, class_id, content, source, created_at)`
- `deleted_seed_materials(stored_name, deleted_at)`

`materials.class_id` 与 `knowledge_entries.class_id` 均为非空外键并有索引。

## API

| Method | Path | Policy |
| --- | --- | --- |
| POST | `/api/login` | 公开；统一失败响应与用户名+IP限流 |
| POST | `/api/logout` | 会话；删除会话并清 Cookie |
| GET | `/api/me` | 会话；返回用户、角色、班级 |
| GET | `/api/materials` | 会话；仅查询本班，可用 `q` 搜索 |
| GET | `/api/materials/{id}` | 会话+班级；跨班/不存在同形 404 |
| GET | `/api/materials/{id}/file` | 会话+班级；鉴权下载 |
| POST | `/api/materials` | 会话+teacher；成功 201，student 403 |
| DELETE | `/api/materials/{id}` | 会话+teacher+班级；成功 204，学生 403，跨班/未知 404，索引处理中 409 |
| GET | `/health` | 公开；仅进程存活语义 |

## Configuration and startup

所有数据库凭据、会话密钥、上传限制、TTL、限流阈值和种子口令只来自环境。必填项缺失或格式非法时进程退出。API 在启动时重试等待 MySQL 可连接，再迁移并幂等播种。
