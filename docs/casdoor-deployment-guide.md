# Casdoor 部署与接入手册

## 已验证环境

- 目标服务器：`101.96.208.132`
- 系统：Ubuntu 22.04.4 LTS
- 架构：`x86_64`
- PostgreSQL：复用现有容器 `sub2api-postgres`
- Casdoor 访问地址：`http://127.0.0.1:8000`
- 本阶段范围：只验证 Casdoor + PostgreSQL，不配置 Nginx 和 HTTPS

## 服务器现状

- `80/443` 已被现有 Nginx 占用
- `5432` 已被 `sub2api-postgres` 占用
- `8000` 可用于本阶段本机访问
- `login.gepinkeji.com` 仍未切到这台服务器，本阶段不做正式域名验收

## 数据库准备

在 `sub2api-postgres` 中创建了专用角色和数据库：

```sql
CREATE USER casdoor WITH PASSWORD '<CASDOOR_DB_PASSWORD>';
CREATE DATABASE casdoor OWNER casdoor;
GRANT ALL PRIVILEGES ON DATABASE casdoor TO casdoor;
```

实际联通方式为：

```text
host=host.docker.internal
port=5432
dbname=casdoor
user=casdoor
```

## 部署目录

```text
/opt/casdoor
/opt/casdoor/conf/app.conf
/opt/casdoor/docker-data/files
/opt/casdoor/docker-data/logs
/opt/casdoor/docker-data/tmp
```

## 实际启动方式

服务器上的 `docker-compose 1.29.2` 与当前 Docker 环境存在兼容问题，实际启动改为等价的 `docker run`。

镜像使用本地构建的 `linux/amd64` 产物：

```text
casdoor-local:postgres-prod
```

启动参数：

```bash
docker run -d --name casdoor --restart always \
  --add-host host.docker.internal:host-gateway \
  -p 127.0.0.1:8000:8000 \
  -e RUNNING_IN_DOCKER=true \
  -e driverName=postgres \
  -e "dataSourceName=user=casdoor password=<CASDOOR_DB_PASSWORD> host=host.docker.internal port=5432 sslmode=disable dbname=casdoor" \
  -e dbName=casdoor \
  -e "origin=http://127.0.0.1:8000" \
  -e "originFrontend=http://127.0.0.1:8000" \
  -v /opt/casdoor/conf:/conf \
  -v /opt/casdoor/docker-data/files:/files \
  -v /opt/casdoor/docker-data/logs:/logs \
  -v /opt/casdoor/docker-data/tmp:/tmp \
  --entrypoint /bin/sh \
  casdoor-local:postgres-prod \
  -c "./server --createDatabase=false"
```

## 文件权限

容器以 UID `1000` 运行，挂载目录需要可读：

```bash
chown -R 1000:1000 /opt/casdoor/conf /opt/casdoor/docker-data
chmod 755 /opt/casdoor/conf
chmod 644 /opt/casdoor/conf/app.conf
```

## 验证结果

已验证通过：

```bash
curl -I http://127.0.0.1:8000/login
curl http://127.0.0.1:8000/.well-known/openid-configuration
```

关键结果：

- 登录页返回 `HTTP/1.1 200 OK`
- `/.well-known/openid-configuration` 返回可用的 OIDC discovery JSON
- `issuer` 为 `http://127.0.0.1:8000`

## A/B 产品接入附录

### A 产品

- Client ID: `chat-gepinkeji-com`
- Redirect URI: `https://chat.gepinkeji.com/auth/casdoor/callback`

### B 产品

- Client ID: `token-gepinkeji-com`
- Redirect URI: `https://token.gepinkeji.com/auth/casdoor/callback`

### 目标 OIDC 端点

以下是后续切到 `login.gepinkeji.com` 且启用 Nginx/HTTPS 后使用的生产端点：

- Issuer: `https://login.gepinkeji.com`
- Authorization: `https://login.gepinkeji.com/login/oauth/authorize`
- Token: `https://login.gepinkeji.com/api/login/oauth/access_token`
- UserInfo: `https://login.gepinkeji.com/api/userinfo`
- JWKS: `https://login.gepinkeji.com/.well-known/jwks`

## 敏感信息约束

- 不写真实密码
- 不写 root 登录凭据
- 不写服务器私钥或证书路径
