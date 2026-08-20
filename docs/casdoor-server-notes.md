# Casdoor 服务器落地说明

## 已确认事实

- 服务器：`101.96.208.132`
- 架构：`x86_64`
- 系统：Ubuntu 22.04.4 LTS
- PostgreSQL：`sub2api-postgres`
- PostgreSQL 监听：宿主机 `5432`
- Nginx：已占用 `80/443`
- Casdoor 本阶段端口：`127.0.0.1:8000`

## 需要执行的操作

- 在 PostgreSQL 容器中创建 `casdoor` 用户和数据库
- 构建并导入 `casdoor-local:postgres-prod`
- 在 `/opt/casdoor` 写入 compose 和配置
- 启动并验证本机 HTTP 和 OIDC discovery

## 预计配置

```yaml
services:
  casdoor:
    image: casdoor-local:postgres-prod
    restart: always
    entrypoint: /bin/sh -c './server --createDatabase=false'
    ports:
      - "127.0.0.1:8000:8000"
    extra_hosts:
      - "host.docker.internal:host-gateway"
    environment:
      RUNNING_IN_DOCKER: "true"
      driverName: postgres
      dataSourceName: "user=casdoor password=<CASDOOR_DB_PASSWORD> host=host.docker.internal port=5432 sslmode=disable dbname=casdoor"
      dbName: casdoor
      origin: "http://127.0.0.1:8000"
      originFrontend: "http://127.0.0.1:8000"
```
