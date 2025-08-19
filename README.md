# ProxyFilter

代理订阅聚合管理系统。把多个远程订阅源合并、去重、过滤、重命名，生成 Clash/Mihomo/Surge/Loon/sing-box/QuantumultX 等客户端配置。

## 功能

- **订阅源管理**：支持远程 URL、本地 URI、本地 YAML，可测试/刷新/缓存。
- **聚合订阅**：JS 脚本过滤/重命名/排序，支持多种客户端格式输出。
- **分流规则**：内置规则库（12 分类 58 条规则）支持勾选，支持自定义规则集。
- **管理员登录**：JWT 认证，首次启动自动引导设置密码。
- **Docker 部署**：docker-compose 开箱即用，自动初始化默认数据。

## 快速开始

### go run（开发）

```bash
git clone https://github.com/ganpanlin/go-sub.git
cd go-sub
go run ./cmd/proxy-filter
```

打开 `http://localhost:8080`，首次需设置管理员密码。

### Docker Compose（推荐）

```bash
git clone https://github.com/ganpanlin/go-sub.git
cd go-sub
docker compose up -d --build
```

首次启动自动初始化默认数据（20 个订阅源、1 个 profile、默认分流规则）。

### Docker 单独运行

```bash
docker build -t proxy-filter:local .
docker run -d -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/frontend:/app/frontend \
  proxy-filter:local
```

## 目录结构

```
go-sub/
├── cmd/proxy-filter/       # 入口
├── internal/               # 核心代码
│   ├── auth/               # JWT 认证
│   ├── cache/              # 内存+磁盘缓存
│   ├── converter/          # 输出格式转换
│   ├── handler/            # HTTP 处理器
│   ├── pipeline/           # 核心管线（fetch→dedup→filter→routing）
│   ├── provider/           # HTTP 请求+缓存
│   ├── routing/            # 分流规则+内置规则库
│   ├── rule/               # 聚合订阅+JS 脚本引擎
│   └── ruleset/            # 自定义规则集
├── frontend/index.html     # 单文件前端（Vue 2 + Element UI）
├── default-data/           # 默认数据模板（git 跟踪）
├── data/                   # 运行时数据（git 忽略，自动初始化）
├── Dockerfile
├── docker-compose.yml
└── AGENTS.md
```

## 数据管理

| 目录 | 说明 | git |
|------|------|-----|
| `default-data/` | 默认数据模板 | ✅ 跟踪 |
| `data/` | 运行时数据 | ❌ 忽略 |
| `frontend/` | 前端文件 | ✅ 跟踪 |

首次启动时，如果 `data/` 为空，程序自动从 `default-data/` 复制默认数据。

三种启动方式共享同一份 `data/` 目录：

| 方式 | 数据目录 | 前端目录 |
|------|----------|----------|
| `go run ./cmd/proxy-filter` | `./data/` | `./frontend/` |
| `docker compose up` | `./data/` (挂载) | `./frontend/` (挂载) |
| `docker run -v ...` | `./data/` (挂载) | `./frontend/` (挂载) |

## 分流规则

### 规则库

管理台内置规则库，支持分类勾选：

- 🤖 AI 服务（OpenAI、Claude、Gemini、Copilot、DeepSeek）
- 🎬 流媒体（YouTube、Netflix、Disney+、HBO、Spotify、TikTok...）
- 🐦 社交平台（Twitter、Facebook、Instagram、Telegram、Discord...）
- 🎮 游戏平台（Steam、PlayStation、Xbox、Nintendo、暴雪、Epic...）
- 💻 开发工具（GitHub、GitLab、Docker、JetBrains...）
- 💱 交易所（Binance、OKX、Bybit、Gate.io、Bitget...）
- 🍎 苹果服务、Ⓜ️ 微软服务、📢 谷歌服务
- 🔒 隐私安全、🌐 基础设施、🔧 常用工具、🇨🇳 国内直连

### 自定义规则集

支持创建自定义规则集，对外提供 YAML 格式的 rule-provider 端点：

```
GET /rules/{id}.yaml
```

可在分流规则中直接引用自建规则集 URL。

### 内联规则

```text
DOMAIN-SUFFIX,example.com
DOMAIN-SUFFIX,mycompany.com
IP-CIDR,10.0.0.0/8,no-resolve
IP-CIDR,172.16.0.0/12,no-resolve
```

`payload` 存在时优先生效，`urls` 会被忽略。

## 认证

- 使用 JWT（HS256），token 有效期 24 小时。
- 前端通过 `Authorization: Bearer` 头传递。
- 密钥自动持久化到 `data/auth.json`，重启不失效。

以下接口不需要登录：

- `/api/health`、`/api/version`
- `/api/auth/status`、`/api/auth/login`、`/api/auth/setup`
- `/sub/{token}`（订阅链接）

## 缓存机制

| 缓存类型 | 默认 TTL | 说明 |
|----------|----------|------|
| 源缓存 | 60 分钟 | 订阅源原始内容 |
| 输出缓存 | 10 分钟 | 生成后的订阅结果 |

- 源列表接口优先读缓存，缓存未命中时后台异步请求。
- 订阅接口缓存过期时先返回旧数据，后台异步刷新。
- 单个测试绕过源缓存，全量刷新清理所有输出缓存。

## 启动参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-port` | `8080` | 服务监听端口 |
| `-config` | `config.json` | 配置文件路径 |
| `-data-dir` | `data` | 数据目录（相对于工作目录） |
| `-frontend-dir` | `frontend` | 前端目录（相对于工作目录） |
| `-http-timeout` | `10` | 远程请求超时（秒） |
| `-cache-ttl` | `60` | 源缓存时间（分钟） |
| `-filter-cache-ttl` | `10` | 输出缓存时间（分钟） |

## API 接口

### 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/auth/status` | 查询认证状态 |
| `POST` | `/api/auth/setup` | 初始化管理员 |
| `POST` | `/api/auth/login` | 登录 |
| `POST` | `/api/auth/logout` | 退出 |
| `POST` | `/api/auth/change-password` | 修改密码 |

### 订阅源

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/sources` | 获取订阅源列表 |
| `POST` | `/api/sources` | 添加订阅源 |
| `PUT` | `/api/sources` | 更新订阅源 |
| `DELETE` | `/api/sources` | 删除订阅源 |
| `POST` | `/api/sources/test` | 测试单个订阅源 |
| `POST` | `/api/sources/refresh` | 全量刷新缓存 |
| `GET` | `/api/sources/data` | 查看源节点数据 |

### 聚合订阅

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/profiles` | 获取列表 |
| `POST` | `/api/profiles` | 创建 |
| `PUT` | `/api/profiles?id=` | 更新 |
| `DELETE` | `/api/profiles?id=` | 删除 |

### 分流规则

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/routing` | 获取列表 |
| `POST` | `/api/routing` | 创建 |
| `PUT` | `/api/routing?id=` | 更新 |
| `DELETE` | `/api/routing?id=` | 删除 |
| `GET` | `/api/routing/catalog` | 获取规则库目录 |

### 自定义规则集

| 方法 | 路径 | 说明 |
|------|------|------|
| `GET` | `/api/rulesets` | 获取列表 |
| `POST` | `/api/rulesets` | 创建 |
| `PUT` | `/api/rulesets?id=` | 更新 |
| `DELETE` | `/api/rulesets?id=` | 删除 |
| `GET` | `/rules/{id}.yaml` | 获取 YAML（无需鉴权） |

### 预览

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/simulate` | 模拟生成 |
| `GET` | `/api/preview` | 生成预览 |

## 支持的输出格式

通过 `?type=` 参数指定：

| 参数值 | 格式 |
|--------|------|
| `clash`（默认） | Clash/Mihomo YAML |
| `surge` | Surge |
| `loon` | Loon |
| `quantumult` / `quanx` | Quantumult X |
| `singbox` / `sing-box` | sing-box JSON |
| `base64` / `uri` | Base64 编码 URI 列表 |

## 支持的节点协议

VMess、Shadowsocks、ShadowsocksR、Trojan、VLESS、Hysteria2

## 安全建议

- 首次部署后立即创建管理员密码。
- 不要把 `data/`、`auth.json` 提交到公开仓库。
- 暴露到公网时建议放在反向代理后面并启用 HTTPS。
- 订阅源 URL 可能包含 token，分享日志前注意脱敏。

## 许可证

MIT
