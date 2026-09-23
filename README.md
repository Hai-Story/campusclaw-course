# CampusClaw · 迭代 1

CampusClaw 是面向中小学的教研智能体。本仓库交付它的第一层可信底座：账号登录、教师/学生权限、班级数据隔离，以及 `.txt` / `.md` 教研材料的上传与知识库正文入库。

## 范围

- 教师：查看、搜索、下载和上传本班材料。
- 学生：查看、搜索和下载本班材料；上传接口由服务端返回 403。
- 班级：只取服务端会话中的 `class_id`。列表强制按班过滤；详情和文件先取行再核对归属。
- 跨班：与不存在资源返回完全相同的 404，避免泄露资源是否存在。
- 知识库：上传后保存正文，供下一迭代检索使用；本版本不做问答或向量化。

## 不做的内容

本版本不包含 RAG/向量检索/问答、作业和成绩、超级管理员、JWT/OAuth/SSO、注册改密、PDF/Word/图片解析、在线编辑、Kubernetes、CI、公网域名、HTTPS 或多副本高可用。

这是单实例 Compose 版本。登录失败限流保存在 API 进程内；在引入共享限流器和相应规约之前不要横向扩容 API。

## 启动

需要 Docker Desktop（含 `docker compose`）。仓库不会提供可用密钥或口令。

```sh
cp .env.example .env
# 编辑 .env：替换 DB_PASSWORD、SESSION_SECRET 与三个 SEED_*_PASSWORD
docker compose up --build -d
docker compose ps
```

打开 [http://localhost:8080](http://localhost:8080)。存活检查地址为 [http://localhost:8080/health](http://localhost:8080/health)。若 8080 已占用，修改 `.env` 的 `WEB_PORT`。

首次启动 MySQL 可能需要几十秒；API 除了等待 Compose 健康状态，还会在进程内重试数据库连接。数据库 3306、API 8081 与上传目录都不会映射到宿主机。

## 预置账号

口令是你在 `.env` 中填写的值；应用启动时只把 bcrypt 哈希写入数据库。

| 账号 | 角色 | 班级 | 权限 |
| --- | --- | --- | --- |
| `teacher_a` | 教师 | A 班 | 上传、查看、搜索、下载 |
| `student_a1` | 学生 | A 班 | 查看、搜索、下载 |
| `student_b1` | 学生 | B 班 | 查看、搜索、下载 |

两班各有一份标题可区分的种子材料。初始化幂等，重启不会复制种子，也不会覆盖教师上传的内容。

## 验收

先执行静态构建与规约校验：

```sh
cd backend && go test ./...
cd ../frontend && npm ci && npm run build
cd .. && openspec validate add-auth-rbac-class-knowledge --strict
```

Compose 启动后运行关键行为验收：

```sh
sh scripts/verify.sh
```

脚本验证公开 health、未登录 401、教师上传 201、学生上传 403、跨班 404，并比较跨班与不存在资源的响应体完全一致。脚本会上传一份 A 班验收材料；数据库与上传文件仍保留在卷中。

## 持久化与重置

```sh
docker compose down
docker compose up -d
```

上面的操作会保留数据库与上传文件。只有明确要清空所有本地数据时才执行：

```sh
docker compose down -v
```

## 安全边界

- Cookie 是带签名的不透明随机会话标识，使用 `HttpOnly` 与 `SameSite=Lax`；角色和班级不写入 `localStorage`。
- 登录成功换发会话，登出删除服务端会话行。用户名不存在、密码错误和锁定期使用同一失败响应。
- 文件名只用于展示；磁盘存储名由服务端随机生成。仅允许非空 UTF-8 的 `.txt` / `.md`，大小上限来自环境变量。
- 材料与知识库行处于同一事务；失败时回滚数据库并删除已写文件。
- Nginx 不提供 `/uploads` 静态目录，文件下载必须通过鉴权 API。

## 规约流程

本迭代的 OpenSpec change 是 `add-auth-rbac-class-knowledge`。实现以 `proposal.md`、`design.md`、delta spec 和带 verify 的 `tasks.md` 为依据。校验与行为验收都通过后，再把 delta 归档为长期规约。

