# PostgreSQL 改造交付说明

## 目标

本次改造的目标是：**RAGFlow 的业务元数据库默认且主路径只使用 PostgreSQL，不再依赖 MySQL 作为默认运行库**。

改造范围覆盖：

- Python 主链路
- Go DAO / admin / migration
- Docker 默认部署
- 核心运维与开发文档

## 改造内容

### 1. Python 主链路

将 Python 侧默认数据库类型切到 PostgreSQL，并清理默认回退逻辑与数据库状态探针。

关键文件：

- [common/settings.py](/Users/leiyang/Desktop/code/ragflowforDM/common/settings.py:69)
- [common/config_utils.py](/Users/leiyang/Desktop/code/ragflowforDM/common/config_utils.py:139)
- [api/db/services/connector_service.py](/Users/leiyang/Desktop/code/ragflowforDM/api/db/services/connector_service.py:180)
- [api/utils/health_utils.py](/Users/leiyang/Desktop/code/ragflowforDM/api/utils/health_utils.py:219)

### 2. Go 主链路

将 Go 侧数据库入口改为支持 PostgreSQL，并补齐 PG DSN、dialector、migration 分支和方言 SQL。

关键文件：

- [internal/server/config.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/server/config.go:419)
- [internal/dao/database.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/dao/database.go:71)
- [internal/dao/migration.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/dao/migration.go:27)
- [internal/dao/time_record.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/dao/time_record.go:23)
- [internal/admin/service.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/admin/service.go:1100)

### 3. 默认配置与部署

将默认配置和 Docker 编排切换为 PostgreSQL。

关键文件：

- [conf/service_conf.yaml](/Users/leiyang/Desktop/code/ragflowforDM/conf/service_conf.yaml:7)
- [docker/service_conf.yaml.template](/Users/leiyang/Desktop/code/ragflowforDM/docker/service_conf.yaml.template:9)
- [docker/.env](/Users/leiyang/Desktop/code/ragflowforDM/docker/.env:25)
- [docker/docker-compose-base.yml](/Users/leiyang/Desktop/code/ragflowforDM/docker/docker-compose-base.yml:176)
- [docker/docker-compose.yml](/Users/leiyang/Desktop/code/ragflowforDM/docker/docker-compose.yml:6)
- [docker/docker-compose-macos.yml](/Users/leiyang/Desktop/code/ragflowforDM/docker/docker-compose-macos.yml:8)
- [docker/docker-compose-CN-oc9.yml](/Users/leiyang/Desktop/code/ragflowforDM/docker/docker-compose-CN-oc9.yml:8)

### 4. 测试与运维文档

补充 PostgreSQL 相关焦点测试，并更新关键文档、CLI 示例和备份迁移脚本。

测试文件：

- [test/unit_test/api/utils/test_health_utils_db_status.py](/Users/leiyang/Desktop/code/ragflowforDM/test/unit_test/api/utils/test_health_utils_db_status.py:1)
- [internal/server/config_test.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/server/config_test.go:1)
- [internal/dao/database_test.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/dao/database_test.go:1)
- [internal/dao/time_record_test.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/dao/time_record_test.go:1)

文档与脚本：

- [docs/administrator/admin/admin_service.md](/Users/leiyang/Desktop/code/ragflowforDM/docs/administrator/admin/admin_service.md:12)
- [docs/administrator/admin/ragflow_cli.md](/Users/leiyang/Desktop/code/ragflowforDM/docs/administrator/admin/ragflow_cli.md:176)
- [docs/administrator/backup_and_migration.md](/Users/leiyang/Desktop/code/ragflowforDM/docs/administrator/backup_and_migration.md:33)
- [docker/migration.sh](/Users/leiyang/Desktop/code/ragflowforDM/docker/migration.sh:12)
- [docs/develop/build_docker_image.mdx](/Users/leiyang/Desktop/code/ragflowforDM/docs/develop/build_docker_image.mdx:46)

## 启动 PostgreSQL 版本

### Docker 默认启动

```bash
cd /Users/leiyang/Desktop/code/ragflowforDM/docker
docker compose -f docker-compose.yml up -d
```

### 源码方式

至少保证以下环境参数可用：

```env
DB_TYPE=postgres
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DBNAME=rag_flow
POSTGRES_USER=rag_flow
POSTGRES_PASSWORD=infini_rag_flow
```

同时确认以下配置文件中的 `postgres` 配置块正确：

- [conf/service_conf.yaml](/Users/leiyang/Desktop/code/ragflowforDM/conf/service_conf.yaml:7)
- [docker/service_conf.yaml.template](/Users/leiyang/Desktop/code/ragflowforDM/docker/service_conf.yaml.template:9)

## 验收标准

### 1. 基础启动

- 不启动 MySQL 容器
- PostgreSQL 容器健康
- API / admin 服务正常启动

### 2. 建表与迁移

- PostgreSQL 中出现业务表
- 首次启动没有 MySQL 方言 DDL 报错
- Go admin / DAO 路径不再因 MySQL driver 失败

### 3. 核心业务

- 登录 / 注册
- 创建租户、知识库、文档
- 创建会话、任务、API token
- 管理端 `list services` / `show service` 可见 `postgres`

### 4. 数据落点确认

- PostgreSQL 中有 `user`、`tenant`、`knowledgebase`、`document`、`chat_session`、`task` 等表写入
- 默认部署下没有任何元数据写入 MySQL

## 已完成验证

- `go test -mod=mod ./internal/server ./internal/dao` 通过
- Python 焦点测试通过：`13 passed`

执行过的 Python 焦点测试命令：

```bash
uv run --with pytest --python /Users/leiyang/.local/share/uv/python/cpython-3.12.9-macos-aarch64-none/bin/python3.12 python -m pytest test/unit_test/api/db/test_oceanbase_peewee.py test/unit_test/api/utils/test_health_utils_db_status.py -q
```

## 已知说明

- 直接运行 `uv run --group test ...` 在当前 macOS arm64 环境下会被 `tensorflow-cpu` 依赖卡住，这是测试依赖组的平台兼容问题，不是本次 PostgreSQL 改造引入的问题。
- 仓库里仍保留部分 `mysql` 相关代码和文案，主要用于 OceanBase / SeekDB 协议兼容、外部数据源接入能力、Agent SQL 工具和历史说明文档。
- 这些残留不影响“默认业务元数据库使用 PostgreSQL”的交付目标。
