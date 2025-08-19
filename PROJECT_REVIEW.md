# ProxyFilter (go-sub) — 项目功能完成度报告

> **生成日期**: 2026-06-04  
> **项目类型**: Go 语言 Web 服务  
> **项目用途**: 动态多源 Clash 代理配置聚合、过滤与分发  

---

## 一、项目概述

ProxyFilter 是一个基于 Go 开发的在线工具，用于从多个订阅源动态拉取 Clash 代理配置文件，支持智能过滤、去重、重命名，最终输出干净的聚合配置。附带基于 Vue.js + Element UI 的 Web 管理界面。

### 技术栈

| 层面 | 技术 |
|------|------|
| 后端语言 | Go 1.22 |
| HTTP 路由 | `gorilla/mux` v1.8.1 |
| YAML 解析 | `gopkg.in/yaml.v3` |
| 前端框架 | Vue 2.6 + Element UI |
| 前端依赖 | CDN 引入（无构建工具） |

---

## 二、项目结构

```
go-sub/
├── cmd/proxy-filter/main.go       # 入口，启动 HTTP 服务
├── internal/
│   ├── appconfig/config.go        # 全局配置单例（端口、路径）
│   ├── cache/cache.go            # 内存缓存（带 TTL）
│   ├── config/config.go          # 环境变量读取（未使用）
│   ├── handler/
│   │   ├── filter.go             # /filter 核心过滤接口
│   │   ├── source.go             # GET /api/sources
│   │   ├── source_management.go  # POST/DELETE /api/sources
│   │   └── test.go               # POST /api/sources/test
│   ├── middleware/cors.go        # CORS 中间件
│   ├── parser/
│   │   ├── parser.go             # YAML/Base64/URI 解析分发
│   │   └── uri.go                # 各协议 URI 解析 (VMess/SS/Trojan/VLESS/Hysteria2)
│   ├── provider/provider.go      # URL 抓取 + 缓存 + 解析
│   ├── proxy/
│   │   ├── proxy.go              # 去重/校验/重命名/代理组更新
│   │   └── region.go             # 区域名称映射与过滤器扩展
│   ├── router/router.go          # HTTP 路由注册
│   ├── scheduler/scheduler.go    # 定时刷新调度器
│   ├── settings/settings.go      # 启动时加载默认源
│   └── sourcemanager/sourcemanager.go  # 订阅源状态管理
├── pkg/utils/utils.go            # SafeToString 工具
├── frontend/index.html           # Web 管理界面（单文件）
├── config.json                   # 默认源列表 + 刷新间隔
├── build.bat / build.sh          # 交叉编译脚本（Linux amd64）
├── run.sh                        # Linux 启动脚本
└── README.md                     # 项目说明
```

---

## 三、模块功能完成度

### 3.1 核心功能 ✅

| 功能 | 状态 | 说明 |
|------|:----:|------|
| 多源 URL 聚合 | ✅ | 支持逗号分隔的多 URL 同时拉取合并 |
| YAML 解析 | ✅ | 标准 Clash YAML 配置解析 |
| Base64 解码 | ✅ | 自动检测并解码 Base64 内容 |
| URI 列表解析 | ✅ | 识别 `vmess://` `ss://` `trojan://` 等前缀 |
| 代理节点去重 | ✅ | 按 `type:server:port` 唯一键去重 |
| 代理节点校验 | ✅ | 校验必需字段 + 端口合法性 |
| 代理组更新 | ✅ | 过滤后自动更新 proxy-groups |
| 节点重命名 | ✅ | 按地区 + 编号重命名（如 `香港_1`） |
| 结果缓存 | ✅ | 过滤结果缓存 10 分钟 |
| 按名称过滤 | ✅ | 支持正则，含区域智能匹配 |
| 按类型过滤 | ✅ | 支持正则（如 `vmess\|ss`） |
| 按 server 过滤 | ✅ | 支持正则 + `domain`/`ip` 精确过滤 |

### 3.2 协议解析 ✅ / ⚠️

| 协议 | 状态 | 说明 |
|------|:----:|------|
| VMess | ✅ | v2 格式完整解析 |
| Shadowsocks (SS) | ✅ | 标准 SIP002 URI |
| Trojan | ✅ | 含 sni 参数支持 |
| VLESS | ⚠️ | `parseVlessURI` 返回 nil，**未实现** |
| Hysteria2 | ⚠️ | `parseHysteria2URI` 返回 nil，**未实现** |
| ShadowsocksR (SSR) | ❌ | 未实现解析 |

### 3.3 区域过滤

| 区域 | 别名支持 |
|------|----------|
| 香港 | hk, hongkong, hong kong |
| 台湾 | tw, taiwan |
| 日本 | jp, japan |
| 韩国 | kr, korea |
| 新加坡 | sg, singapore |
| 美国 | us, usa, united states, america |
| 英国 | uk, united kingdom, britain |

### 3.4 Web 管理界面 ✅

| 功能 | 状态 |
|------|:----:|
| 订阅源列表展示 | ✅ |
| 状态标签（Pending/Success/Error） | ✅ |
| 延迟（Latency）显示 | ✅ |
| 缓存标记（⚡️） | ✅ |
| 添加订阅源 | ✅ |
| 删除订阅源 | ✅ |
| 手动测试单个源 | ✅ |
| 刷新按钮 | ✅ |
| 自动轮询刷新 | ❌ 前端无定时轮询 |

### 3.5 REST API

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/filter` | 核心过滤接口，参数: `url`, `name`, `type`, `server` |
| GET | `/api/sources` | 获取所有订阅源状态 |
| POST | `/api/sources` | 添加订阅源 |
| DELETE | `/api/sources` | 删除订阅源 |
| POST | `/api/sources/test` | 测试单个订阅源 |
| `/*` | — | 静态文件服务（前端） |

### 3.6 基础设施

| 功能 | 状态 | 说明 |
|------|:----:|------|
| 内存缓存 (TTL) | ✅ | 源内容缓存 1h，过滤结果缓存 10min |
| 定时刷新 | ✅ | 可配置间隔，默认 60 分钟 |
| 启动加载默认源 | ✅ | 从 config.json 读取 |
| 并发安全 | ✅ | sync.Mutex / sync.RWMutex |
| 交叉编译脚本 | ✅ | build.bat / build.sh → Linux amd64 |
| CORS | ✅ | 允许所有来源（但只允许 GET/OPTIONS ⚠️） |

---

## 四、发现的问题

### 4.1 🔴 关键问题

| # | 问题 | 位置 | 影响 |
|---|------|------|------|
| 1 | **CORS 中间件只允许 GET/OPTIONS**，但 API 有 POST/DELETE | `middleware/cors.go` | 跨域 POST/DELETE 被浏览器拦截 |
| 2 | **VLESS 解析未实现** | `parser/uri.go:parseVlessURI` | VLESS 节点直接丢弃 |
| 3 | **Hysteria2 解析未实现** | `parser/uri.go:parseHysteria2URI` | Hysteria2 节点直接丢弃 |

### 4.2 🟡 一般问题

| # | 问题 | 位置 | 说明 |
|---|------|------|------|
| 4 | 前端无自动轮询 | `frontend/index.html` | 需手动点刷新才能看到状态更新 |
| 5 | 无健康检查端点 | — | 缺少 `/health` 或 `/ping` |
| 6 | 无日志级别控制 | — | 所有 log 输出到 stdout |
| 7 | `internal/config/config.go` 未使用 | `config.go` | 定义了 `DefaultURL` 但从未引用 |
| 8 | SSR 协议无解析支持 | `uri.go` | `parseSsURI` 只处理标准 SS，不含 SSR |
| 9 | HTTP 超时硬编码 10s | `provider.go` | 不可配置 |

### 4.3 🟢 建议改进

| # | 建议 |
|---|------|
| 1 | 添加单元测试和集成测试 |
| 2 | 支持订阅源分组/标签 |
| 3 | 支持过滤结果导出为 Clash 订阅链接 |
| 4 | 添加 WebSocket 推送源状态实时更新 |
| 5 | 支持更多区域（德国、法国、印度等） |
| 6 | 添加 Prometheus metrics 端点 |
| 7 | 提供 Dockerfile |

---

## 五、配置示例

### config.json
```json
{
  "refresh_interval_minutes": 60,
  "default_urls": [
    "https://example.com/clash.yaml",
    "https://example2.com/sub"
  ]
}
```

### 启动命令
```bash
# 默认端口 8080
go run cmd/proxy-filter/main.go

# 自定义参数
go run cmd/proxy-filter/main.go -port 9090 -config my-config.json -frontend-dir ./dist
```

### 过滤 API 示例
```bash
# 获取香港节点
curl "http://localhost:8080/filter?name=香港"

# 多源合并香港 vmess 节点
curl "http://localhost:8080/filter?url=https://a.com/sub,https://b.com/sub&name=香港&type=vmess"

# 获取域名 server 的节点
curl "http://localhost:8080/filter?server=domain"
```

---

## 六、总体评估

| 维度 | 评分 | 说明 |
|------|:----:|------|
| 核心功能完成度 | **85%** | 主流程完整，VLESS/Hysteria2 解析缺失 |
| 代码质量 | **良好** | 结构清晰，模块划分合理，并发安全 |
| 可用性 | **良好** | 零外部依赖（前端 CDN 除外），开箱即用 |
| 可维护性 | **良好** | Go 标准项目布局，接口清晰 |
| 文档 | **一般** | README 基础，缺 API 文档和部署文档 |
| 测试 | **❌ 无** | 没有单元测试 |

### 结论

项目已实现核心的 **多源 Clash 代理聚合与过滤** 功能，Web 管理界面可用。主要待处理事项为：
1. 修复 CORS 方法限制（影响跨域 POST/DELETE）
2. 补全 VLESS 和 Hysteria2 协议解析
3. 添加基础测试覆盖