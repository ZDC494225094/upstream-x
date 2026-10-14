# Upstream X

用于监控 `Sub2API` / `New API` 上游余额、全部分组倍率、令牌占用分组关系，并记录历史变化趋势与告警事件。

## 功能

- 统一监控两类上游：
  - `Sub2API`：读取 `/api/v1/auth/me`，优先使用 `/api/v1/model-plaza` 模型目录和 `/api/v1/usage/dashboard/trend` 每日实际用量；旧版再回退到 `/api/v1/groups/available`、`/api/v1/keys` 及 `/api/v1/usage/dashboard/api-keys-usage`
  - `New API`：读取 `/api/user/self`、`/api/user/self/groups`、`/api/token/`
- 统一以美元显示余额
  - `Sub2API.balance` 直接按美元展示
  - `New API.quota / 500000 = $1`
- 定时检测全部启用账号
- 标识“哪些分组当前被令牌使用”
- 汇总分组实际用量，账号详情和大屏支持筛选“已创建令牌 / 已产生用量”
- 新版 Sub2API 模型广场不需要为每个模型或分组创建额外令牌；本项目只读取已有令牌，并在趋势接口不可用时才用其兼容统计
- 总览页统计所有账号的每日额度消耗，并兼容新版趋势与旧版快照两种数据来源
- 记录余额趋势、分组倍率历史、图片价格变化日志
- SMTP 邮件通知：
  - 余额跌破阈值
  - 检测失败 / 恢复
  - 分组倍率或 `Sub2API` 图片价格变化
- `Sub2API` 支持 Refresh Token 和邮箱密码两种直接认证方式
- 单管理员后台，账号密码从配置文件读取

## 一键部署

1. 复制环境变量文件：

```bash
cp .env.example .env
```

2. 复制配置文件模板：

```bash
cp config/config.yaml.example config/config.yaml
```

3. 编辑 `config/config.yaml`

- 设置后台管理员账号密码
- 设置 `auth.session_secret`
- 设置 `security.credential_key`
- 默认已经允许被其他站点通过 iframe 嵌入；生产环境建议通过 HTTPS 部署以保证跨站 iframe 登录态稳定
- 其余运行时配置可在后台“系统设置”页修改和测试发送

4. 启动服务：

```bash
docker compose up -d --build
```

5. 打开后台：

- 地址：`http://localhost:8080`
- 使用 `config/config.yaml` 中的管理员账号密码登录

## 持久化目录

- `./config`
  - 配置文件目录
  - 容器内挂载到 `/app/config`
- `./data`
  - SQLite 数据库与运行数据目录
  - 容器内挂载到 `/app/data`

重建容器不会丢失账号、历史快照、图表数据和日志。

## 远程服务器直接启动

远程服务器无需下载源码或本地构建镜像，只需准备 `docker-compose.remote.yml` 和 `.env.example`：

```bash
cp .env.example .env
# 修改 .env 中的管理员密码、会话密钥和凭据加密密钥
docker compose -f docker-compose.remote.yml pull
docker compose -f docker-compose.remote.yml up -d
```

该编排直接拉取 `hamwong233/upstream-x:latest`，SQLite 数据保存在当前目录的 `./data`。升级时执行：

```bash
docker compose -f docker-compose.remote.yml pull
docker compose -f docker-compose.remote.yml up -d
```

## 本地开发

### 前端

```bash
cd frontend
npm install
npm run dev
```

### 后端

```bash
cp config/config.yaml.example config/config.yaml
go mod download
go run ./cmd/server
```

默认读取 `config/config.yaml`，默认数据库为 `data/app.db`。

## 主要页面

- 登录页
- 总览页
- 账号管理页
- 账号详情页
- 系统设置页

## 配置说明

最小配置项：

- `server.port`
- `database.sqlite_path`
- `admin.username`
- `admin.password`
- `auth.session_secret`
- `auth.session_same_site`
- `auth.session_secure`
- `auth.allow_iframe_embedding`
- `auth.iframe_allowed_origins`
- `security.credential_key`

说明：

- `admin.*`、`server.port`、`database.sqlite_path`、`auth.session_secret`、`auth.session_same_site`、`auth.session_secure`、`auth.allow_iframe_embedding`、`auth.iframe_allowed_origins`、`security.credential_key` 仍属于启动级配置，需要保留在 `config.yaml`
- `SMTP` 完全由后台“系统设置”页管理，支持保存和发送测试邮件
- 定时检测间隔、日志保留时间也完全由后台“系统设置”页管理
- 历史日志支持在后台手动清理，系统也会在每轮定时检测结束后自动清理超出保留时间的历史记录

iframe 嵌入说明：

- 项目现在支持被其他页面通过 iframe 嵌入
- 默认会尽量使用适合 iframe 的会话策略：安全请求下使用 `SameSite=None`，非安全请求自动回退为 `Lax`
- 生产环境通过 HTTPS 或反向代理透传 `X-Forwarded-Proto: https` 时，跨站 iframe 登录态可以正常工作
- 纯 HTTP 的跨站 iframe 受浏览器限制，登录态可能无法保留，这是浏览器策略限制，不需要额外改项目配置
- 可以通过 `auth.iframe_allowed_origins` 限制允许嵌入的来源；留空时表示不额外限制来源

## 注意事项

- `New API` 账号在后台中要求同时填写：
  - `Bearer Token`
  - `New-Api-User`
- 如果 `Sub2API` 站点开启了 Turnstile 或 2FA，邮箱密码模式无法自动完成挑战，请使用该站点签发的 Refresh Token
- 保存账号时，前端会先发起一次即时校验，只有校验成功后才会保存
- SMTP 未启用时，系统照常监控和记录日志，只是不发送邮件
