# AGENTS.md — ProxyFilter 项目指南

## 项目概述

ProxyFilter 是一个代理订阅聚合管理系统，用 Go 编写后端，单文件 Vue 2 + Element UI 做前端。

核心功能：聚合多个代理订阅源 → 去重/过滤/重命名 → 按分流规则生成 Clash/Surge/Loon/sing-box 等客户端配置。

## 快速启动

```bash
# 开发模式（推荐，自动使用当前目录的 data/ 和 frontend/）
cd /mnt/d/documents/idea/go-sub
go run ./cmd/proxy-filter

# 或指定 Go 路径
/usr/local/go/bin/go run ./cmd/proxy-filter

# Docker
docker-compose up --build -d
```

启动后访问 `http://localhost:8080`，默认用户名 `admin`，首次需设置密码。

## 项目结构

```
go-sub/
├── cmd/
│   └── proxy-filter/main.go     # 入口，初始化所有模块
├── internal/
│   ├── appconfig/config.go       # 应用配置（端口、路径、超时）
│   ├── auth/                     # JWT 鉴权
│   │   ├── auth.go               # 核心：CreateJWT, ValidateJWT, Middleware
│   │   └── auth_save.go          # 配置持久化
│   ├── cache/cache.go            # 内存+磁盘缓存
│   ├── converter/                # 输出格式转换器
│   │   ├── converter.go          # 注册表：Get(type)
│   │   ├── clash.go              # Clash/Mihomo YAML
│   │   ├── surge.go              # Surge
│   │   ├── quantumult.go         # Quantumult X
│   │   ├── loon.go               # Loon
│   │   ├── singbox.go            # sing-box JSON
│   │   └── base64.go             # Base64 URI 列表
│   ├── datastore/datastore.go    # JSON 文件读写
│   ├── handler/                  # HTTP 处理器（路由对应）
│   │   ├── auth.go               # 登录/登出/注册/改密
│   │   ├── source.go             # GET /api/sources（列表，仅读缓存）
│   │   ├── source_management.go  # CRUD /api/sources
│   │   ├── source_refresh.go     # POST /api/sources/refresh
│   │   ├── source_data.go        # GET /api/sources/data
│   │   ├── source_status.go      # 源状态内存追踪
│   │   ├── test.go               # POST /api/sources/test
│   │   ├── profile.go            # CRUD /api/profiles
│   │   ├── routing.go            # CRUD /api/routing + catalog
│   │   ├── ruleset.go            # CRUD /api/rulesets（自定义规则集）
│   │   ├── sub.go                # GET /sub/{token}（订阅输出）+ /api/simulate
│   │   ├── filter.go             # GET /filter（兼容旧端点）
│   │   ├── geoip.go              # GET /api/geoip
│   │   ├── health.go             # GET /api/health
│   │   └── version.go            # GET /api/version
│   ├── middleware/
│   │   ├── api_response.go       # ApiResponseMiddleware：统一 {code,msg,data} 响应
│   │   └── cors.go               # CORS
│   ├── parser/parser.go          # YAML/URI 解析
│   ├── pipeline/pipeline.go      # 核心管线：fetch → dedup → filter → routing
│   ├── provider/provider.go      # HTTP 请求 + 缓存（FetchAndParseYAML）
│   ├── proxy/                    # 节点区域识别
│   ├── router/router.go          # 路由注册（gorilla/mux）
│   ├── routing/
│   │   ├── routing.go            # 分流规则 CRUD
│   │   ├── engine.go             # BuildConfig：生成 proxy-groups + rules
│   │   └── catalog.go            # 内置规则目录（12分类58条）
│   ├── rule/
│   │   ├── rule.go               # 聚合订阅 Profile 管理
│   │   └── engine.go             # JS 脚本引擎（goja）
│   ├── ruleset/ruleset.go        # 自定义规则集 CRUD
│   ├── scheduler/scheduler.go    # 定时刷新任务
│   ├── settings/settings.go      # 设置迁移
│   ├── source/source.go          # 订阅源数据模型
│   └── version/version.go        # 版本信息
├── frontend/
│   └── index.html                # 单文件前端（Vue 2 + Element UI）
├── data/                         # 运行时数据目录
│   ├── sources.json              # 订阅源配置
│   ├── profiles.json             # 聚合订阅配置
│   ├── routing.json              # 分流规则配置
│   ├── rulesets.json             # 自定义规则集
│   ├── auth.json                 # 认证配置（含 JWT 密钥）
│   └── cache/                    # 磁盘缓存
├── docker-compose.yml
├── Dockerfile
└── config.example.json
```

## 架构要点

### 数据流

```
用户请求 /sub/{token}
  → 查缓存（命中则直接返回）
  → 查 stale 缓存（命中则返回旧数据，后台异步刷新）
  → pipeline.Run(profile)
      → 并发获取所有订阅源（最多8并发）
      → 去重 + 验证
      → JS 脚本引擎过滤/重命名
      → routing.BuildConfig 生成分流规则
  → converter 转为目标格式
  → 写缓存 + 返回
```

### 认证系统

- 使用 JWT（HS256），token 有效期 24 小时
- 前端存储在 `localStorage`，通过 `Authorization: Bearer` 头传递
- 密钥自动持久化到 `data/auth.json`（`jwt_secret` 字段）
- 路由白名单：`/api/health`, `/api/auth/login`, `/api/auth/status` 等

### API 响应格式

所有 `/api/*` 端点统一包装为：
```json
{"code": 0, "msg": "ok", "data": ...}
```
错误时 `code` 为 HTTP 状态码，`msg` 为错误信息。

### 前端

- 单文件 `index.html`，通过 CDN 加载 Vue 2、Element UI、axios
- axios 拦截器自动解包 `{code, msg, data}` 响应
- JWT token 管理：`setToken()` / `clearToken()`
- 30 秒轮询刷新，页面不可见时自动暂停
- `fetchSources` 带 500ms 防抖 + CancelToken

## 编码规范

### Go

- 包名小写无下划线：`routing`, `provider`, `pipeline`
- Handler 函数命名：`XxxHandler`（如 `GetSourcesHandler`）
- 数据文件用 `datastore.Save()` / `datastore.ReadJSON()` 读写
- 缓存用 `cache.Get()` / `cache.Set()` / `cache.SetWithDisk()`
- 并发用 `sync.WaitGroup` + 带 buffer 的 channel 做限流
- 日志用标准库 `log.Printf`，格式：`[模块名] 消息`

### 前端

- 数据绑定用 `this.xxx`，不用 `let/const`
- API 调用用 `axios.get/post/put/delete`
- 响应处理：`r.data`（已由拦截器解包）
- 错误处理：`apiErrorMessage(err, fallback)`
- 模态框用 `el-dialog`，表单用 `el-form`

## 常见任务

### 添加新的 API 端点

1. 在 `internal/handler/` 新建或编辑 handler 文件
2. 在 `internal/router/router.go` 注册路由
3. 如需鉴权，走默认的 auth.Middleware（白名单外自动检查 JWT）

### 添加新的输出格式

1. 在 `internal/converter/` 新建文件实现 `Converter` 接口
2. 在 `converter.go` 的 `init()` 中注册

### 修改分流规则逻辑

- `internal/routing/engine.go` → `BuildConfig()` 生成配置
- `internal/routing/catalog.go` → 内置规则目录
- `internal/routing/routing.go` → CRUD 持久化

### 修改前端

- 编辑 `frontend/index.html`（单文件，无需构建）
- 刷新浏览器即可看到效果

## 注意事项

- `data/` 目录存放运行时数据，不要删除
- `auth.json` 中的 `jwt_secret` 是自动生成的，删除后所有 token 失效
- `sources.json` 中的源没有 `id` 字段时，运行时会自动生成（基于 URL 的 SHA256）
- 前端是纯静态文件，修改后无需重新编译 Go
- Go 编译缓存可能导致旧代码运行，用 `go clean -cache` 清理
