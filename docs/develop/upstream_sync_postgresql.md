# Upstream 新功能同步到 PostgreSQL 全适配分支操作手册

## 目标

本手册用于指导如何将 `upstream/ragflow` 的新功能、修复和结构调整，稳定同步到本项目的 **PostgreSQL 全适配分支**，并尽量降低重复适配成本。

适用场景：

- 官方 `upstream/main` 有新功能需要合入
- 官方修复了 bug，需要同步到 PostgreSQL 分支
- 官方新增了 DAO、migration、配置项或 Docker 结构

本手册默认：

- 官方仓库 remote 为 `upstream`
- 自己维护的仓库 remote 为 `origin`
- PostgreSQL 主维护分支为 `feat-pg`

## 一、分支策略

建议长期保留以下分支：

- `main`
  用于跟踪自己仓库的主线，按团队约定使用
- `feat-pg`
  PostgreSQL 全适配主分支
- `sync/upstream-YYYYMMDD`
  每次同步官方更新时新建的临时集成分支

推荐原则：

- 不要直接在 `feat-pg` 上同步 `upstream/main`
- 每次同步都新建一个临时分支
- 同步完成并验证通过后，再合回 `feat-pg`

## 二、同步前准备

先确认 remotes 正确：

```bash
git remote -v
```

理想状态应包含：

```bash
origin    <your-fork-url>
upstream  https://github.com/infiniflow/ragflow.git
```

拉取最新远端信息：

```bash
git fetch origin
git fetch upstream
```

建议开启 `rerere`，让 Git 记住冲突解决方式：

```bash
git config --global rerere.enabled true
```

## 三、标准同步流程

### 1. 从 PostgreSQL 主分支切出同步分支

```bash
git checkout feat-pg
git pull origin feat-pg
git checkout -b sync/upstream-20260428
```

分支名建议带日期，便于追踪。

### 2. 合入官方最新代码

推荐使用 `merge`：

```bash
git merge upstream/main
```

说明：

- 对长期维护分支，`merge` 比 `rebase` 更稳
- 冲突点会更清晰
- 更适合团队复盘“这次同步改了什么”

如果团队明确要求线性历史，再考虑 `rebase`。

### 3. 解决冲突

冲突时优先关注以下文件：

- `common/settings.py`
- `common/config_utils.py`
- `api/db/services/connector_service.py`
- `api/utils/health_utils.py`
- `internal/server/config.go`
- `internal/dao/database.go`
- `internal/dao/migration.go`
- `internal/dao/time_record.go`
- `internal/admin/service.go`
- `conf/service_conf.yaml`
- `docker/service_conf.yaml.template`
- `docker/docker-compose-base.yml`

解决冲突原则：

- 保留 upstream 的新功能逻辑
- 恢复 PostgreSQL 默认化改动
- 避免把官方新逻辑错误回退成旧版本
- 不要只为“消冲突”而删掉 PG 分支逻辑

## 四、同步后必须做的检查

每次合入 upstream 后，不要立刻提交，先做数据库适配检查。

### 1. 检查官方是否新增数据库相关代码

重点扫描：

```bash
git diff --name-only feat-pg..upstream/main
```

再定向看数据库相关目录：

```bash
git diff upstream/main -- internal/dao internal/server api/db api/utils common docker conf
```

需要重点关注：

- 新增 migration
- 新增原生 SQL
- 新增配置项
- 新增 Docker 依赖
- 新增 health check / admin status

### 2. 检查新增 SQL 是否带有 MySQL 方言

重点排查关键词：

```bash
rg -n "SHOW PROCESSLIST|AUTO_INCREMENT|IFNULL|JSON_EXTRACT|JSON_CONTAINS|ADD UNIQUE INDEX|ORDER BY .* LIMIT|GET_LOCK|RELEASE_LOCK|information_schema" api internal common
```

如果 upstream 新增了以下内容，通常要补 PostgreSQL 分支：

- `AUTO_INCREMENT`
- `SHOW PROCESSLIST`
- `IFNULL`
- `GET_LOCK / RELEASE_LOCK`
- `DELETE ... ORDER BY ... LIMIT`
- MySQL 专属 `information_schema` 查询
- MySQL JSON 函数

### 3. 检查默认配置是否被改回 MySQL

重点查看：

- `common/settings.py`
- `conf/service_conf.yaml`
- `docker/service_conf.yaml.template`
- `docker/.env`
- `docker/docker-compose-base.yml`

检查项：

- `DB_TYPE` 默认值是否仍为 `postgres`
- 默认数据库配置块是否仍为 `postgres`
- compose 默认依赖是否仍为 `postgres`

## 五、PostgreSQL 重点适配点

以后 upstream 增量变化时，最容易出问题的是以下几类。

### 1. Migration

每当官方新增 migration，都要确认：

- 是否只写了 MySQL DDL
- 是否需要补 PostgreSQL 分支
- 是否使用了不兼容语法

当前 PostgreSQL 重点 migration 入口：

- [internal/dao/migration.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/dao/migration.go:1)
- [api/db/db_models.py](/Users/leiyang/Desktop/code/ragflowforDM/api/db/db_models.py:1)

### 2. Go DAO 原生 SQL

每当官方新增 DAO SQL，都要确认：

- 是否使用了 MySQL 语法
- 是否需要补 PG 版本写法

当前重点文件：

- [internal/dao/database.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/dao/database.go:1)
- [internal/dao/time_record.go](/Users/leiyang/Desktop/code/ragflowforDM/internal/dao/time_record.go:1)

### 3. Python 数据库状态与配置

每当官方改动配置或状态检查，都要确认：

- 默认值是否仍走 PostgreSQL
- 状态探针是否仍可在 PostgreSQL 下工作

当前重点文件：

- [common/settings.py](/Users/leiyang/Desktop/code/ragflowforDM/common/settings.py:1)
- [common/config_utils.py](/Users/leiyang/Desktop/code/ragflowforDM/common/config_utils.py:1)
- [api/utils/health_utils.py](/Users/leiyang/Desktop/code/ragflowforDM/api/utils/health_utils.py:1)

## 六、最小验证流程

每次同步完成后，至少执行一轮最小验证。

### 1. Go 焦点测试

```bash
env GOCACHE=/Users/leiyang/Desktop/code/ragflowforDM/.gocache \
GOMODCACHE=/Users/leiyang/Desktop/code/ragflowforDM/.gomodcache \
go test -mod=mod ./internal/server ./internal/dao
```

### 2. Python 焦点测试

```bash
uv run --with pytest --python /Users/leiyang/.local/share/uv/python/cpython-3.12.9-macos-aarch64-none/bin/python3.12 \
python -m pytest \
test/unit_test/api/db/test_oceanbase_peewee.py \
test/unit_test/api/utils/test_health_utils_db_status.py -q
```

### 3. 冒烟验证

至少验证以下流程：

- 启动 PostgreSQL 容器
- 启动 API 服务
- 启动 admin 服务
- 登录 / 注册
- 创建租户
- 创建知识库
- 上传文档
- 创建会话
- 创建任务
- 确认 PostgreSQL 中有元数据写入

## 七、推荐提交方式

同步完成并验证通过后，建议按以下方式提交：

```bash
git status
git add <relevant-files>
git commit -m "Sync upstream/main into PostgreSQL branch"
git push origin sync/upstream-20260428
```

然后发起 PR，目标分支为：

- `feat-pg`

PR 标题建议：

```text
Sync upstream/main into PostgreSQL branch
```

## 八、遇到问题时的排查顺序

如果同步后 PostgreSQL 跑不起来，建议按这个顺序排查：

1. 配置是否被改回 MySQL 默认值
2. Docker 默认依赖是否仍是 PostgreSQL
3. Go migration 是否引入了 MySQL-only DDL
4. Go DAO 是否新增了 MySQL-only SQL
5. Python health/config 路径是否重新依赖 MySQL
6. admin 服务是否又使用了 MySQL 状态探针

## 九、长期维护建议

为了让后续同步成本越来越低，建议继续做这几件事：

- 把 PostgreSQL / MySQL 方言判断尽量集中到 helper 层
- 给 migration 和 SQL 方言补更多单元测试
- 保持 Docker / config / docs 的 PostgreSQL 默认路径一致
- 每次 upstream 同步都记录本次冲突点和修复方式

最终目标不是“每次人工重新适配”，而是把 PostgreSQL 支持收敛成稳定的维护层，使 upstream 新功能自然落到既有适配框架中。
