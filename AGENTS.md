# AGENTS.md — ProxyFilter 项目指南

## 项目概述

ProxyFilter 是一个代理订阅聚合管理系统，Go 后端 + Vue 2 组件化前端。

核心功能：聚合多个代理订阅源 → 去重/过滤/重命名 → 按分流规则生成 Clash/Surge/Loon/sing-box 等客户端配置。

## 快速启动

```bash
# 开发模式
cd /mnt/d/documents/idea/go-sub
go run ./cmd/proxy-filter

# Docker
docker compose up --build -d
```

启动后访问 `http://localhost:8080`，首次需设置密码。

## 项目结构

```
go-sub/
├── cmd/
│   └── proxy-filter/main.go          # 入口，优雅关闭
├── internal/
│   ├── appconfig/config.go            # 应用配置（端口、路径、超时）
│   ├── auth/                          # JWT 鉴权
│   ├── cache/cache.go                 # 内存+磁盘缓存
│   ├── converter/                     # 输出格式转换器
│   │   ├── converter.go              # 注册表 + 工具函数
│   │   ├── clash.go                  # Clash/Mihomo YAML
│   │   ├── surge.go                  # Surge
│   │   ├── quantumult.go             # Quantumult X
│   │   ├── loon.go                   # Loon
│   │   ├── singbox.go               # sing-box JSON
│   │   └── base64.go                 # Base64 URI 列表
│   ├── datastore/datastore.go         # JSON 文件读写
│   ├── geoip/geoip.go                # IP 区域检测
│   ├── handler/                       # HTTP 处理器
│   │   ├── auth.go                   # 登录/登出/注册/改密
│   │   ├── source.go                 # 订阅源列表（异步缓存加载）
│   │   ├── source_management.go      # 订阅源 CRUD
│   │   ├── source_refresh.go         # 全量刷新
│   │   ├── source_data.go            # 查看源节点数据
│   │   ├── source_status.go          # 源状态内存追踪
│   │   ├── test.go                   # 测试单个源
│   │   ├── healthcheck.go            # 节点 TCP 健康检测
│   │   ├── profile.go                # 聚合订阅 CRUD
│   │   ├── routing.go                # 分流规则 CRUD + catalog
│   │   ├── ruleset.go                # 自定义规则集 CRUD
│   │   ├── sub.go                    # 订阅输出 /sub/{token} + 模拟预览
│   │   ├── stats.go                  # 访问统计查询
│   │   ├── config_io.go              # 配置导入/导出
│   │   ├── filter.go                 # 兼容旧 /filter 端点
│   │   ├── geoip.go                  # GeoIP 查询
│   │   ├── health.go                 # 服务健康检查
│   │   ├── version.go                # 版本信息
│   │   └── yaml_helpers.go           # YAML 序列化工具
│   ├── healthcheck/                   # TCP 连通性检测
│   ├── middleware/
│   │   ├── api_response.go           # 统一 {code,msg,data} 响应
│   │   ├── cors.go                   # CORS
│   │   ├── logging.go                # 请求日志（slog）
│   │   └── ratelimit.go              # IP 限流
│   ├── parser/                        # YAML/URI 解析
│   ├── pipeline/pipeline.go           # 核心管线：fetch → dedup → filter → routing
│   ├── provider/provider.go           # HTTP 请求 + 缓存 + 重试 + TLS 降级
│   ├── proxy/                         # 节点区域识别 + 重命名
│   ├── router/router.go               # 路由注册（gorilla/mux）
│   ├── routing/                       # 分流规则
│   │   ├── routing.go                # CRUD
│   │   ├── engine.go                 # BuildConfig 生成 proxy-groups + rules
│   │   └── catalog.go                # 内置规则目录（12分类58条）
│   ├── rule/                          # 聚合订阅 Profile
│   │   ├── rule.go                   # Profile 管理
│   │   └── engine.go                 # JS 脚本引擎（goja）
│   ├── ruleset/ruleset.go             # 自定义规则集 CRUD
│   ├── scheduler/scheduler.go         # 定时刷新
│   ├── settings/settings.go           # 设置迁移
│   ├── source/source.go               # 订阅源数据模型
│   ├── stats/stats.go                 # 访问统计（内存 ring buffer）
│   └── version/version.go             # 版本信息
├── frontend/
│   ├── index.html                     # 入口 HTML + CSS
│   ├── api.js                         # axios 配置、JWT、工具函数
│   ├── app.js                         # Vue 根实例（共享状态 + 鉴权）
│   └── components/
│       ├── source-list.js             # 订阅源 tab
│       ├── profile-list.js            # 聚合订阅 tab + 健康检测 + 统计
│       ├── routing.js                 # 分流规则 tab
│       └── config-io.js               # 配置导入/导出
├── data/                              # 运行时数据（git 忽略）
├── default-data/                      # 默认数据模板
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
  → pipeline.RunCtx(ctx, profile)
      → 并发获取所有订阅源（最多8并发，支持 context 取消）
      → TLS 证书失败自动跳过验证重试
      → 网络错误自动重试（5xx 重试，4xx 不重试）
      → 去重 + 验证
      → JS 脚本引擎过滤/重命名
      → routing.BuildConfig 生成分流规则
  → converter 转为目标格式
  → 记录访问统计
  → 写缓存 + 返回
```

### 认证

- JWT（HS256），24 小时有效期
- 白名单：`/api/health`、`/api/auth/*`、`/sub/{token}`

### API 响应

所有 `/api/*` 统一包装：`{"code": 0, "msg": "ok", "data": ...}`

### 前端

- Vue 2 组件化，通过 CDN 加载（零构建工具）
- 组件通过 `$root` 共享状态，`$refs` 互相调用
- axios 拦截器自动解包响应

## 编码规范

### Go

- 包名小写无下划线：`routing`, `provider`, `pipeline`
- Handler 命名：`XxxHandler`
- 日志用 `log/slog`（结构化），不用 `log.Printf`
- 缓存用 `cache.Get()` / `cache.Set()` / `cache.SetWithDisk()`
- 并发用 `sync.WaitGroup` + channel 限流

### 前端

- 组件注册：`Vue.component('name', { template, data, methods })`
- 工具函数在 `api.js`（`setToken`, `clearToken`, `apiErrorMessage` 等）
- Element UI 组件库

## 常见任务

### 添加新的 API 端点

1. 在 `internal/handler/` 新建或编辑 handler 文件
2. 在 `internal/router/router.go` 注册路由
3. 如需鉴权，走默认的 auth.Middleware（白名单外自动检查 JWT）

### 添加新的输出格式

1. 在 `internal/converter/` 新建文件实现 `Converter` 接口
2. 在 `converter.go` 的 `init()` 中注册

### 修改前端

- 组件文件在 `frontend/components/`
- 入口和全局配置在 `frontend/app.js` 和 `frontend/api.js`
- 刷新浏览器即可看到效果，无需构建
